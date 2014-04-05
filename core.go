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
	"strings"
	"sync"
	"time"
)

// socket is the meaty part of the core information.
type socket struct {
	proto Protocol

	sync.Mutex

	uwq      chan *Message // upper write queue
	urq      chan *Message // upper read queue
	closeq   chan bool     // closed when user requests close
	sndwakeq chan bool
	rcvwakeq chan bool

	sndlock sync.Mutex
	rcvlock sync.Mutex

	closing bool // true if Socket was closed at API level

	rdeadline  time.Time
	wdeadline  time.Time
	reconntime time.Duration // reconnect time after error or disconnect

	pipes   []*pipe
	cansend List // list of pipes that can send
	canrecv List // list of pipes that can receive

	accepters []PipeAccepter

	transports map[string]Transport

	// These are conditional "type aliases" for our self
	setoption ProtocolSetOptionHandler
	getoption ProtocolGetOptionHandler
	sendhook  ProtocolSendHook
	recvhook  ProtocolRecvHook
}

func (sock *socket) notifySendReady(p *pipe) {
	if p != nil {
		sock.sndlock.Lock()
		sock.cansend.InsertTail(&p.cansend)
		sock.sndlock.Unlock()
	}
	select {
	case sock.sndwakeq <- true:
	default:
	}
}

func (sock *socket) wakeSend() {
	select {
	case sock.sndwakeq <- true:
	default:
	}
}

func (sock *socket) waitSendReady() chan bool {
	return sock.sndwakeq
}

func (sock *socket) notifyRecvReady(p *pipe) {
	sock.rcvlock.Lock()
	sock.canrecv.InsertTail(&p.canrecv)
	sock.rcvlock.Unlock()
	select {
	case sock.rcvwakeq <- true:
	default:
	}
}

func (sock *socket) waitRecvReady() chan bool {
	return sock.rcvwakeq
}

func (sock *socket) addPipe(tranpipe Pipe) *pipe {
	p := newPipe(tranpipe)
	sock.Lock()
	p.index = len(sock.pipes)
	sock.pipes = append(sock.pipes, p)
	sock.Unlock()
	sock.proto.AddEndpoint(p)
	p.start(sock)
	sock.notifySendReady(p)
	return p
}

func (sock *socket) remPipe(p *pipe) {

	sock.proto.RemEndpoint(p)

	sock.rcvlock.Lock()
	sock.canrecv.Remove(&p.canrecv)
	sock.rcvlock.Unlock()

	sock.sndlock.Lock()
	sock.cansend.Remove(&p.cansend)
	sock.sndlock.Unlock()

	sock.Lock()
	if p.index >= 0 {
		sock.pipes[p.index] = sock.pipes[len(sock.pipes)-1]
		sock.pipes = sock.pipes[:len(sock.pipes)-1]
		p.index = -1
	}
	sock.Unlock()
}

func newSocket(proto Protocol) *socket {
	sock := new(socket)
	// Load all Transports so that SetOption & GetOption work right away.
	sock.loadTransports()
	sock.uwq = make(chan *Message, 10)
	sock.urq = make(chan *Message, 10)
	sock.closeq = make(chan bool)
	sock.sndwakeq = make(chan bool)
	sock.rcvwakeq = make(chan bool)
	sock.canrecv.Init()
	sock.cansend.Init()
	sock.reconntime = time.Second * 1 // make it a tunable?
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

	go sock.txProcessor()
	go sock.rxProcessor()
	return sock
}

// This routine implements the main processing loop.  Since most of the
// handling is specific to each protocol, we just call the protocol's
// Process function, but we take care to keep doing so until it claims to
// have performed no new work.  Then we wait until some event arrives
// indicating a state change.
func (sock *socket) txProcessor() {
	for {
		sock.proto.ProcessSend()

		select {
		case <-sock.waitSendReady():
			continue
		case <-sock.closeq:
			return
		}
	}
}

func (sock *socket) rxProcessor() {
	for {
		sock.proto.ProcessRecv()

		select {
		case <-sock.waitRecvReady():
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

func (sock *socket) NextSendEndpoint() Endpoint {
	sock.sndlock.Lock()
	for n := sock.cansend.HeadNode(); n != nil; n = sock.cansend.HeadNode() {
		// quick test avoids pointless pushes later
		p := n.Value.(*pipe)
		if len(p.wq) < cap(p.wq) {
			sock.sndlock.Unlock()
			return p
		}
		sock.cansend.Remove(n)
	}
	sock.sndlock.Unlock()
	return nil
}

func (sock *socket) NextRecvEndpoint() Endpoint {
	sock.rcvlock.Lock()
	for n := sock.canrecv.HeadNode(); n != nil; n = sock.canrecv.HeadNode() {
		// quick test avoids pointless pushes later
		p := n.Value.(*pipe)
		if len(p.rq) > 0 {
			sock.rcvlock.Unlock()
			return p
		}
		sock.canrecv.Remove(n)
	}
	sock.rcvlock.Unlock()
	return nil
}

// PullDown implements the ProtocolHandle PullDown method.
func (sock *socket) PullDown() *Message {
	select {
	case msg := <-sock.uwq:
		return msg
	default:
		return nil
	}
}

// PushUp implements the ProtocolHandle PushUp method.
func (sock *socket) PushUp(msg *Message) bool {
	select {
	case sock.urq <- msg:
		return true
	default:
		// Droppped message!
		return false
	}
}

//
// Implementation of Socket bits on socket.  This is the upper API
// presented to applications.
//

func (sock *socket) Close() {
	// XXX: flushq's?  linger?
	sock.Lock()
	if sock.closing {
		sock.Unlock()
		return
	}
	sock.closing = true
	close(sock.closeq)

	for _, a := range sock.accepters {
		a.Close()
	}

	pipes := append([]*pipe{}, sock.pipes...)
	sock.Unlock()

	for _, p := range pipes {
		p.Close()
	}

}

func (sock *socket) SendMsg(msg *Message) error {
	if sock.sendhook != nil {
		if ok := sock.sendhook.SendHook(msg); !ok {
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
			sock.wakeSend()
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
				if ok := sock.recvhook.RecvHook(msg); ok {
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
		select {
		case <-sock.closeq:
			return
		default:
		}

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
	sock.accepters = append(sock.accepters, a)
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
