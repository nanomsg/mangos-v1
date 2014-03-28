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
	"strings"
	"sync"
	"time"
)

// This file contains some of the "core" handling logic for the SP
// implementation.  None of the interfaces here are to be exposed externally.

// corePipe wraps the Pipe data structure with the stuff we need to keep
type corePipe struct {
	pipe    Pipe
	rq      chan *Message // messages sent to user
	wq      chan *Message // messages sent to wire
	closeq  chan struct{} // only closed, never passes data
	key     PipeKey
	cansend *list.Element // linkage to socket cansend
	canrecv *list.Element // linkage to socket canrecv

	s       *socket
	closing bool // true if we were closed
}

// socket is the meaty part of the core information.
type socket struct {
	proto   Protocol
	nextkey PipeKey

	sync.Mutex

	uwq    chan *Message // upper write queue
	urq    chan *Message // upper read queue
	closeq chan bool     // closed when user requests close
	wakeq  chan bool     // basically a semaphore/condvar

	closed bool // true if Socket was closed at API level
	work   bool // Used to track work progress in Process

	rdeadline  time.Time
	wdeadline  time.Time
	reconntime time.Duration // reconnect time after error or disconnect

	pipes   map[PipeKey]*corePipe
	cansend *list.List // list of corePipes that can send
	canrecv *list.List // list of corePipes that can recv

	accepters *list.List

	transports map[string]Transport

	// These are conditional "type aliases" for our self
	setoption ProtocolSetOptionHandler
	getoption ProtocolGetOptionHandler
	sendhook  ProtocolSendHook
	recvhook  ProtocolRecvHook
}

func (sock *socket) addPipe(pipe Pipe) *corePipe {
	cp := &corePipe{s: sock, pipe: pipe}
	// queue depths are kind of arbitrary.  deep enough to avoid
	// stalls, but hopefully shallow enough to avoid latency.
	cp.rq = make(chan *Message, 5)
	cp.wq = make(chan *Message, 5)
	cp.closeq = make(chan struct{})
	sock.Lock()
	for {
		// PipeKey zero is special, it represents an unopen/unassigned
		// PipeKey.  Therefore we must avoid it.  (XXX: Should we
		// insist on a minimum value, e.g. 1000 or somesuch?)
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
	sock.Unlock()
	go cp.sender()
	go cp.receiver()
	cp.notifySend()
	return cp
}

func (p *corePipe) shutdown() {
	s := p.s
	// We have to ensure that we only close any channels one time, so
	// we use a lock to check this.  Its the only time we use this lock.
	s.Lock()
	defer s.Unlock()
	if p.closing {
		return
	}
	p.closing = true
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
			p.shutdown()
			return
		}

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
					p.shutdown()
					return
				}
			}
			if ok == false {
				p.shutdown()
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
	p.s.Lock()
	defer p.s.Unlock()

	if p.cansend == nil {
		p.cansend = p.s.cansend.PushBack(p)
		p.s.signal()
	}
}

// notifyRecv is called by a pipe to let its Socket know that it received data.
func (p *corePipe) notifyRecv() {
	p.s.Lock()
	defer p.s.Unlock()

	if p.canrecv == nil {
		p.canrecv = p.s.canrecv.PushBack(p)
		p.s.signal()
	}
}

func newSocket(proto Protocol) *socket {
	sock := new(socket)
	// Load all Transports so that SetOption & GetOption work right away.
	sock.loadTransports()
	sock.uwq = make(chan *Message, 10)
	sock.urq = make(chan *Message, 10)
	sock.closeq = make(chan bool)
	sock.wakeq = make(chan bool)
	sock.canrecv = list.New()
	sock.cansend = list.New()
	sock.accepters = list.New()
	sock.pipes = make(map[PipeKey]*corePipe)
	sock.reconntime = time.Second * 1 // make it a tunable?
	rnum := rand.New(rand.NewSource(time.Now().UnixNano()))
	// We only consider the lower 31 bits.
	sock.nextkey = PipeKey(rnum.Uint32() & 0x7fffffff)
	sock.proto = proto

	// Add some conditionals now -- saves checks later
	if i, ok := interface{}(proto).(ProtocolGetOptionHandler); ok {
		sock.getoption = i.(ProtocolGetOptionHandler)
	}
	if i, ok := interface{}(proto).(ProtocolSetOptionHandler); ok {
		sock.setoption = i.(ProtocolSetOptionHandler)
	}
	if i, ok := interface{}(proto).(ProtocolRecvHook); ok {
		sock.recvhook = i.(ProtocolRecvHook)
	}
	if i, ok := interface{}(proto).(ProtocolSendHook); ok {
		sock.sendhook = i.(ProtocolSendHook)
	}

	proto.Init(sock)

	go sock.processor()
	return sock
}

func (sock *socket) signal() {
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
func (sock *socket) processor() {
	for {
		sock.Lock()
		if sock.closed {
			sock.Unlock()
			return
		}
		sock.work = false
		sock.proto.Process()
		if sock.work {
			sock.Unlock()
			continue
		}
		sock.Unlock()

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

// PullDown implements the ProtocolHandle PullDown method.
func (sock *socket) PullDown() *Message {
	select {
	case msg := <-sock.uwq:
		sock.work = true
		return msg
	default:
		return nil
	}
}

// PushUp implements the ProtocolHandle PushUp method.
func (sock *socket) PushUp(msg *Message) bool {
	select {
	case sock.urq <- msg:
		sock.work = true
		return true
	default:
		// Droppped message!
		return false
	}
}

// SendAnyPipe implements the ProtocolHandle SendAnyPipe method.
func (sock *socket) SendAnyPipe(msg *Message) (PipeKey, error) {
	// Sends to an open Pipe, that is ready...
	for {
		var p *corePipe
		var e *list.Element
		var l = sock.cansend
		// Look for a pipe to send to.  Note that this should only
		// be called in the context of the Process routine, so
		// we can reasonably assume that we are holding the lock.

		if e = l.Front(); e == nil {
			return 0, ErrPipeFull
		}

		p = l.Remove(e).(*corePipe)
		p.cansend = nil

		select {
		case p.wq <- msg:
			// move the element to the end of the list
			// for FIFO handling
			p.cansend = l.PushBack(p)
			sock.work = true
			return p.key, nil
		default:
		}
	}
}

// SendToPipe implements the ProtocolHandle SendToPipe method.
func (sock *socket) SendToPipe(msg *Message, key PipeKey) error {
	p := sock.pipes[key]
	if p == nil || p.closing {
		return ErrClosed
	}
	if p.cansend == nil {
		return ErrPipeFull
	}
	l := sock.cansend
	l.Remove(p.cansend)
	p.cansend = nil

	select {
	case p.wq <- msg:
		// move the element to the end of the list
		// for FIFO handling
		p.cansend = l.PushBack(p)
		sock.work = true
		return nil
	default:
		return ErrPipeFull
	}
}

// SendAllPipes implements the ProtocolHandle SendAllPipes method.
func (sock *socket) SendAllPipes(msg *Message) {
	l := sock.cansend

	var n, e *list.Element
	for e = l.Front(); e != nil; e = n {
		// We have to save the next node for the next iteration,
		// because we might remove the node from the list.
		n = e.Next()
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

// RecvAnyPipe implements the ProtocolHandle RecvAnyPipe method.
func (sock *socket) RecvAnyPipe() (*Message, PipeKey, error) {

	for {
		var p *corePipe
		var e *list.Element
		var l = sock.canrecv
		// Look for a pipe to recv from.  Note that this should only
		// be called in the context of the Process routine, so
		// we can reasonably assume that we are holding the lock.

		if e = l.Front(); e == nil {
			return nil, 0, ErrPipeEmpty
		}

		p = l.Remove(e).(*corePipe)
		p.canrecv = nil

		select {
		case msg := <-p.rq:
			// move the element to the end of the list
			// for FIFO handling -- it might have more data
			p.canrecv = l.PushBack(p)
			sock.work = true
			return msg, p.key, nil
		default:
			// no data in pipe, remove it from the list
		}
	}
}

// WakeUp implements the ProtocolHandle WakeUp method.
func (sock *socket) WakeUp() {
	sock.signal()
}

// IsOpen implements the ProtocolHandle IsOpen method.
func (sock *socket) IsOpen(key PipeKey) bool {
	if p, ok := sock.pipes[key]; ok == true && !p.closing {
		return true
	}
	return false
}

// OpenPipes implements the ProtocolHandle OpenPipes method.
func (sock *socket) OpenPipes() []PipeKey {
	pipes := make([]PipeKey, 0, len(sock.pipes))
	for _, v := range sock.pipes {
		pipes = append(pipes, v.key)
	}
	return pipes
}

// ClosePipe implements the ProtocolHandle ClosePipe method.
func (sock *socket) ClosePipe(key PipeKey) error {
	if key == 0 {
		return nil
	}
	if p, ok := sock.pipes[key]; ok == true && !p.closing {
		p.closing = true
		p.pipe.Close()
		close(p.closeq)
		return nil
	}
	return ErrClosed
}

//
// Implementation of Socket bits on socket.  This is the upper API
// presented to applications.
//

func (sock *socket) Close() {
	// XXX: flushq's?  linger?
	// Arguably we could/should close the write pipe as well.
	// It would be an error for any caller to issue any further
	// operations on the socket after Close -- results in panic.
	sock.Lock()
	defer sock.Unlock()

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
		if !p.closing {
			p.closing = true
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

func (sock *socket) SendMsg(msg *Message) error {
	if sock.sendhook != nil {
		sock.Lock()
		ok := sock.sendhook.SendHook(msg)
		sock.Unlock()

		if !ok {
			// just drop it silently
			return nil
		}
	}
	timeout := mkTimer(sock.wdeadline)
	for {
		select {
		case <-timeout:
			return ErrSendTimeout
		case <-sock.closeq:
			return ErrClosed
		case sock.uwq <- msg:
			sock.signal()
			return nil
		}
	}
}

func (sock *socket) Send(b []byte) error {
	msg := new(Message)
	msg.Body = b
	msg.Header = nil
	return sock.SendMsg(msg)
}

func (sock *socket) RecvMsg() (*Message, error) {
	timeout := mkTimer(sock.rdeadline)

	for {
		select {
		case <-timeout:
			return nil, ErrRecvTimeout
		case msg := <-sock.urq:
			if sock.recvhook != nil {
				sock.Lock()
				ok := sock.recvhook.RecvHook(msg)
				sock.Unlock()
				if ok {
					return msg, nil
				} // else loop
			} else {
				return msg, nil
			}
		case <-sock.closeq:
			return nil, ErrClosed
		}
	}
}

func (sock *socket) Recv() ([]byte, error) {
	msg, err := sock.RecvMsg()
	if err != nil {
		return nil, err
	}
	return msg.Body, nil
}

func (sock *socket) getTransport(addr string) Transport {
	var i int

	sock.Lock()
	defer sock.Unlock()

	if i = strings.Index(addr, "://"); i < 0 {
		return nil
	}
	scheme := addr[:i]
	t, ok := sock.transports[scheme]
	if t != nil && ok {
		return t
	}
	return nil
}

// loadTransports is required to handle the case where an option is
// accessed for a specific Transport, before that Transport has been bound
// to the Socket.  Basically, if someone calls SetOption, we're going to
// instantiate *all* transports we know about.  It turns out that this is
// a pretty cheap operation since Transports generally have very very little
// state associated with them.
func (sock *socket) loadTransports() {
	sock.transports = make(map[string]Transport)

	for scheme, factory := range transports {
		if _, ok := sock.transports[scheme]; ok == true {
			continue
		}

		sock.transports[scheme] = factory.NewTransport()
	}
}

// Dial implements the Socket Dial method.
func (sock *socket) Dial(addr string) error {
	// This function should fire off a dialer goroutine.  The dialer
	// will monitor the connection state, and when it becomes closed
	// will redial.
	t := sock.getTransport(addr)
	if t == nil {
		return ErrBadTran
	}
	// skip the tcp:// or ipc:// or whatever
	addr = addr[len(t.Scheme())+len("://"):]
	d, err := t.NewDialer(addr, sock.proto.Number())
	if err != nil {
		return err
	}
	go sock.dialer(d)
	return nil
}

// dialer is used to dial or redial from a goroutine.
func (sock *socket) dialer(d PipeDialer) {
	for {
		p, err := d.Dial()
		if err == nil {
			cp := sock.addPipe(p)

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
func (sock *socket) serve(a PipeAccepter) {
	for {
		sock.Lock()
		// check to see if the application has closed the socket
		if sock.closed {
			sock.Unlock()
			return
		}
		sock.Unlock()

		// note that if the underlying Accepter is closed, then
		// we expect to return back with an error.
		if pipe, err := a.Accept(); err == nil {
			sock.addPipe(pipe)
		}
	}
}

// Listen implements the Socket Listen method.
func (sock *socket) Listen(addr string) error {
	// This function sets up a goroutine to accept inbound connections.
	// The accepted connection will be added to a list of accepted
	// connections.  The Listener just needs to listen continuously,
	// as we assume that we want to continue to receive inbound
	// connections without limit.
	t := sock.getTransport(addr)
	if t == nil {
		return ErrBadTran
	}
	// skip the tcp:// or ipc:// or whatever
	addr = addr[len(t.Scheme())+len("://"):]
	a, err := t.NewAccepter(addr, sock.proto.Number())
	if err != nil {
		return err
	}
	sock.accepters.PushBack(a)
	go sock.serve(a)
	return nil
}

// SetOption implements the Socket SetOption method.
func (sock *socket) SetOption(name string, value interface{}) error {
	if sock.setoption != nil {
		err := sock.setoption.SetOption(name, value)
		if err == nil {
			return nil
		}
		if err != ErrBadOption {
			return err
		}
	}
	for _, t := range sock.transports {
		err := t.SetOption(name, value)
		if err == nil {
			return nil
		}
		if err != ErrBadOption {
			return err
		}
	}
	return ErrBadOption
}

// GetOption implements the Socket GetOption method.
func (sock *socket) GetOption(name string) (interface{}, error) {
	if sock.getoption != nil {
		val, err := sock.getoption.GetOption(name)
		if err == nil {
			return val, nil
		}
		if err != ErrBadOption {
			return nil, err
		}
	}
	for _, t := range sock.transports {
		val, err := t.GetOption(name)
		if err == nil {
			return val, nil
		}
		if err != ErrBadOption {
			return nil, err
		}
	}
	return nil, ErrBadOption
}
