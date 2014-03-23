// Copyright 2014 Garrett D'Amore
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sp

import (
	"container/list"
	"math/rand"
	"sync"
	"time"
)

// This file contains some of the "core" handling logic for the SP
// implementation.  None of the interfaces here are to be exposed externally.

// coreHandle implements the ProtocolHandle interface.
type coreHandle struct {
	s    *coreSocket
	work bool // indicates some work was done
}

// corePipe wraps the Pipe data structure with the stuff we need to keep
type corePipe struct {
	pipe    Pipe
	rq      chan *Message // messages sent to user
	wq      chan *Message // messages sent to wire
	closeq  chan bool     // only closed, never passes data
	key     PipeKey
	cansend *list.Element // linkage to coreSocket cansend
	canrecv *list.Element // linkage to coreSocket canrecv

	s      *coreSocket
	lk     sync.Mutex // protect access - only close channels once
	closed bool       // true if we were closed
}

// coreSocket is the meaty part of the core information.
type coreSocket struct {
	hndl    coreHandle
	proto   Protocol
	nextkey PipeKey

	lk *sync.Mutex
	cv *sync.Cond

	uwq    chan *Message // upper write queue
	urq    chan *Message // upper read queue
	closeq chan bool     // closed when user requests close
	wakeq  chan bool     // basically a semaphore/condvar

	closed bool // true if Socket was closed at API level

	rdeadline  time.Time
	wdeadline  time.Time
	reconntime time.Duration // reconnect time after error or disconnect

	pipes   map[PipeKey]*corePipe
	cansend *list.List // list of corePipes that can send
	canrecv *list.List // list of corePipes that can recv

	accepters *list.List
}

func (sock *coreSocket) addPipe(pipe Pipe) {
	cp := new(corePipe)
	pipe.SetCoreData(cp)
	// queue depths are kind of arbitrary.  deep enough to avoid
	// stalls, but hopefully shallow enough to avoid latency.
	cp.rq = make(chan *Message, 5)
	cp.wq = make(chan *Message, 5)
	cp.closeq = make(chan bool)
	cp.s = sock
	cp.pipe = pipe
	sock.lock()
	for {
		// We chose to avoid pipe key of zero.  There is nothing
		// fundamental here, but avoiding such keys may help in
		// debugging problems.  (XXX: Should we insist on a minimum
		// value, e.g. avoid all keys less than 1000 or somesuch?)
		if sock.nextkey == 0 {
			sock.nextkey++
		}
		// Keys must only use the lower 31-bits.  The high-order bit
		// is reserved to identify the request ID in a backtrace.
		if sock.nextkey&0x80000000 != 0 {
			sock.nextkey = 1
		}
		// Ensure we have a key that is not already in use (if we
		// wrap we can have a long lived PipeKey that is stil in use)
		if _, ok := sock.pipes[sock.nextkey]; ok == false {
			break
		}
		sock.nextkey++
	}
	cp.key = sock.nextkey
	sock.nextkey++
	sock.pipes[cp.key] = cp
	sock.unlock()
	go cp.sender()
	go cp.receiver()
	cp.notifySend()
}

func (p *corePipe) close() {
	s := p.s
	// We have to ensure that we only close any channels one time, so
	// we use a lock to check this.  Its the only time we use this lock.
	s.lock()
	defer s.unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.pipe.Close()
	delete(s.pipes, p.key)
	if p.canrecv != nil {
		s.canrecv.Remove(p.canrecv)
	}
	if p.cansend != nil {
		s.cansend.Remove(p.cansend)
	}
	close(p.closeq)
}

func (p *corePipe) receiver() {
	var msg *Message
	var err error
	for {
		if msg, err = p.pipe.Recv(); err != nil {
			p.close()
			return
		}

		// Now try to send
		select {
		case p.rq <- msg:
			p.notifyRecv()

		case <-p.closeq:
			return
		}
	}
}

func (p *corePipe) sender() {

	for {
		select {
		case msg, ok := <-p.wq:
			p.notifySend()
			if msg != nil {
				if err := p.pipe.Send(msg); err != nil {
					p.close()
					return
				}
			}
			if ok == false {
				p.close()
				return
			}

		case <-p.closeq:
			return
		}
	}
}

// notifySend is called by a pipe to let its Socket know that it is ready
// to send.
func (p *corePipe) notifySend() {
	p.s.lock()
	defer p.s.unlock()

	if p.cansend == nil {
		p.cansend = p.s.cansend.PushBack(p)
		p.s.signal()
	}
}

// notifyRecv is called by a pipe to let its Socket know that it received data.
func (p *corePipe) notifyRecv() {
	p.s.lock()
	defer p.s.unlock()

	if p.canrecv == nil {
		p.canrecv = p.s.canrecv.PushBack(p)
		p.s.signal()
	}
}

func newCoreSocket() *coreSocket {
	s := new(coreSocket)
	s.lk = new(sync.Mutex)
	s.uwq = make(chan *Message, 1000)
	s.urq = make(chan *Message, 100)
	s.closeq = make(chan bool)
	s.wakeq = make(chan bool)
	s.hndl.s = s
	s.canrecv = list.New()
	s.cansend = list.New()
	s.accepters = list.New()
	s.pipes = make(map[PipeKey]*corePipe)
	s.reconntime = time.Second * 1 // make it a tunable?
	rnum := rand.New(rand.NewSource(time.Now().UnixNano()))
	// We only consider the lower 31 bits.
	s.nextkey = PipeKey(rnum.Uint32() & 0x7fffffff)
	go s.processor()
	return s
}

func (sock *coreSocket) lock() {
	sock.lk.Lock()
}

func (sock *coreSocket) unlock() {
	sock.lk.Unlock()
}

func (sock *coreSocket) signal() {
	// We try to send just a single message on the wakeq.
	// If one is already there, then there is no need to do so again.
	select {
	case sock.wakeq <- true:
	default:
	}
}

// This routine implements the main processing loop.  Since most of the
// handling is specific to each protocol, we just call the protocol's
// Process function, but we take care to keep doing so until it claims to
// have performed no new work.  Then we wait until some event arrives
// indicating a state change.
func (sock *coreSocket) processor() {
	for {
		sock.lock()
		if sock.closed {
			sock.unlock()
			return
		}
		sock.hndl.work = false
		sock.proto.Process()
		if sock.hndl.work {
			sock.unlock()
			continue
		}
		sock.unlock()

		select {
		case <-sock.wakeq:
			continue
		case <-sock.closeq:
			return
		}
	}
}

//
// Implementation of ProtocolHandle bits on coreHandle.  This is the middle
// API presented to Protocol implementations.
//

// PullDown implements the PipeHandle PullDown method.
func (h *coreHandle) PullDown() *Message {
	select {
	case msg := <-h.s.uwq:
		h.work = true
		return msg
	default:
		return nil
	}
}

// PushUp implements the PipeHandle PushUp method.
func (h *coreHandle) PushUp(msg *Message) bool {
	select {
	case h.s.urq <- msg:
		h.work = true
		return true
	default:
		return false
	}
}

// Send implements the PipeHandle Send method.
func (h *coreHandle) Send(msg *Message) (PipeKey, error) {
	// Sends to an open Pipe, that is ready...
	for {
		var p *corePipe
		var e *list.Element
		var l = h.s.cansend
		// Look for a pipe to send to.  Note that this should only
		// be called in the context of the Process routine, so
		// we can reasonably assume that we are holding the lock.

		if e = l.Front(); e == nil {
			return 0, EPipeFull
		}

		p = l.Remove(e).(*corePipe)
		p.cansend = nil

		select {
		case p.wq <- msg:
			// move the element to the end of the list
			// for FIFO handling
			p.cansend = l.PushBack(p)
			h.work = true
			return p.key, nil
		default:
		}
	}
	// we should never get here
}

// SendTo implements the PipeHandle SendTo method.
func (h *coreHandle) SendTo(msg *Message, key PipeKey) error {
	p := h.s.pipes[key]
	if p == nil || p.closed {
		return EClosed
	}
	if p.cansend == nil {
		return EPipeFull
	}
	l := h.s.cansend
	l.Remove(p.cansend)
	p.cansend = nil

	select {
	case p.wq <- msg:
		// move the element to the end of the list
		// for FIFO handling
		p.cansend = l.PushBack(p)
		h.work = true
		return nil
	default:
		return EPipeFull
	}
}

// SendAll implements the PipeHandle SendAll method.
func (h *coreHandle) SendAll(msg *Message) {
	l := h.s.cansend
	for e := l.Front(); e != nil; e = e.Next() {
		p := e.Value.(*corePipe)
		select {
		case p.wq <- msg:
			// queued it for delivery, all's well.
			// No notification about work, because sending this
			// won't free anything up ... we never ever block
			// or save a message that is broadcast.
		default:
			e = e.Prev()
			l.Remove(p.cansend)
			p.cansend = nil
		}
	}
}

// Recv implements the PipeHandle Recv method.
func (h *coreHandle) Recv() (*Message, PipeKey, error) {

	for {
		var p *corePipe
		var e *list.Element
		var l = h.s.canrecv
		// Look for a pipe to recv from.  Note that this should only
		// be called in the context of the Process routine, so
		// we can reasonably assume that we are holding the lock.

		if e = l.Front(); e == nil {
			return nil, 0, EPipeEmpty
		}

		p = l.Remove(e).(*corePipe)
		p.canrecv = nil

		select {
		case msg := <-p.rq:
			// move the element to the end of the list
			// for FIFO handling -- it might have more data
			p.canrecv = l.PushBack(p)
			h.work = true
			return msg, p.key, nil
		default:
			// no data in pipe, remove it from the list
		}
	}
}

// WakeUp implements the PipeHandle WakeUp method.
func (h *coreHandle) WakeUp() {
	h.s.signal()
}

// IsOpen implements the PipeHandle IsOpen method.
func (h *coreHandle) IsOpen(key PipeKey) bool {
	if p, ok := h.s.pipes[key]; ok == true && !p.closed {
		return true
	}
	return false
}

//
// Implementation of Socket bits on coreSocket.  This is the upper API
// presented to applications.
//

func (sock *coreSocket) Close() {
	// XXX: flushq's?  linger?
	// Arguably we could/should close the write pipe as well.
	// It would be an error for any caller to issue any further
	// operations on the socket after Close -- results in panic.
	sock.lock()
	defer sock.unlock()

	if sock.closed {
		return
	}
	sock.closed = true

	for e := sock.accepters.Front(); e != nil; e = sock.accepters.Front() {
		a := e.Value.(PipeAccepter)
		a.Close()
		sock.accepters.Remove(e)
	}

	for k, p := range sock.pipes {
		sock.pipes[k] = nil
		if !p.closed {
			p.closed = true
			p.pipe.Close()
			close(p.closeq)
		}
		if p.cansend != nil {
			sock.cansend.Remove(p.cansend)
			p.cansend = nil
		}
		if p.canrecv != nil {
			sock.canrecv.Remove(p.canrecv)
			p.canrecv = nil
		}
	}

	close(sock.closeq)
	sock.signal()
}

func (sock *coreSocket) SendMsg(msg *Message) error {
	sock.lock()
	ok := sock.proto.SendHook(msg)
	sock.unlock()
	if !ok {
		// just drop it silently
		return nil
	}
	timeout := mkTimer(sock.wdeadline)
	for {
		select {
		case <-timeout:
			return ESendTimeout
		case <-sock.closeq:
			return EClosed
		case sock.uwq <- msg:
			sock.signal()
			return nil
		}
	}
}

func (sock *coreSocket) Send(b []byte) error {
	msg := new(Message)
	msg.Body = b
	msg.Header = make([]byte, 0)
	return sock.SendMsg(msg)
}

func (sock *coreSocket) RecvMsg() (*Message, error) {
	timeout := mkTimer(sock.rdeadline)

	for {
		select {
		case <-timeout:
			return nil, ERecvTimeout
		case msg := <-sock.urq:
			sock.lock()
			ok := sock.proto.RecvHook(msg)
			sock.unlock()
			if ok {
				return msg, nil
			} // else loop
		case <-sock.closeq:
			return nil, EClosed
		}
	}
}

func (sock *coreSocket) Recv() ([]byte, error) {
	msg, err := sock.RecvMsg()
	if err != nil {
		return nil, err
	}
	return msg.Body, nil
}

// Dial implements the Socket Dial method.
func (sock *coreSocket) Dial(addr string) error {
	// This function should fire off a dialer goroutine.  The dialer
	// will monitor the connection state, and when it becomes closed
	// will redial.
	t := GetTransport(addr)
	if t == nil {
		return EBadTran
	}
	d, err := t.NewDialer(addr, sock.proto.Number())
	if err != nil {
		return err
	}
	go sock.dialer(d)
	return nil
}

// dialer is used to dial or redial from a goroutine.
func (sock *coreSocket) dialer(d PipeDialer) {
	for {
		p, err := d.Dial()
		if err == nil {
			sock.addPipe(p)
			cp := p.GetCoreData().(*corePipe)

			select {
			case <-sock.closeq: // parent socket closed
			case <-cp.closeq: // disconnect event
			}
		}

		// we're redialing here
		select {
		case <-sock.closeq: // exit if parent socket closed
			return
		case <-time.After(sock.reconntime):
			continue
		}
	}
}

// serve spins in a loop, calling the accepter's Accept routine.
func (sock *coreSocket) serve(a PipeAccepter) {
	for {
		sock.lock()
		// check to see if the application has closed the socket
		if sock.closed {
			sock.unlock()
			return
		}
		sock.unlock()

		// note that if the underlying Accepter is closed, then
		// we expect to return back with an error.
		if pipe, err := a.Accept(); err == nil {
			sock.addPipe(pipe)
		}
	}
}

// Listen implements the Socket Listen method.
func (sock *coreSocket) Listen(addr string) error {
	// This function sets up a goroutine to accept inbound connections.
	// The accepted connection will be added to a list of accepted
	// connections.  The Listener just needs to listen continuously,
	// as we assume that we want to continue to receive inbound
	// connections without limit.
	t := GetTransport(addr)
	if t == nil {
		return EBadTran
	}
	a, err := t.NewAccepter(addr, sock.proto.Number())
	if err != nil {
		return err
	}
	sock.accepters.PushBack(a)
	go sock.serve(a)
	return nil
}
