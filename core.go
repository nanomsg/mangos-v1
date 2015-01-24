// Copyright 2014 The Mangos Authors
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

package mangos

import (
	"strings"
	"sync"
	"time"
)

// socket is the meaty part of the core information.
type socket struct {
	proto Protocol

	sync.Mutex

	uwq    chan *Message // upper write queue
	urq    chan *Message // upper read queue
	closeq chan struct{} // closed when user requests close
	drainq chan struct{}

	closing bool // true if Socket was closed at API level

	rdeadline  time.Duration
	wdeadline  time.Duration
	reconntime time.Duration // reconnect time after error or disconnect
	linger     time.Duration

	pipes []*pipe

	accepters []PipeAccepter

	transports map[string]Transport

	// These are conditional "type aliases" for our self
	sendhook ProtocolSendHook
	recvhook ProtocolRecvHook
}

func (sock *socket) addPipe(tranpipe Pipe) *pipe {
	p := newPipe(tranpipe, sock)
	sock.Lock()
	p.index = len(sock.pipes)
	sock.pipes = append(sock.pipes, p)
	sock.Unlock()
	sock.proto.AddEndpoint(p)
	return p
}

func (sock *socket) remPipe(p *pipe) {

	sock.proto.RemoveEndpoint(p)

	sock.Lock()
	if p.index >= 0 {
		sock.pipes[p.index] = sock.pipes[len(sock.pipes)-1]
		sock.pipes[p.index].index = p.index
		sock.pipes = sock.pipes[:len(sock.pipes)-1]
		p.index = -1
	}
	sock.Unlock()
}

func newSocket(proto Protocol) *socket {
	sock := new(socket)
	sock.uwq = make(chan *Message, 10)
	sock.urq = make(chan *Message, 10)
	sock.closeq = make(chan struct{})
	sock.drainq = make(chan struct{})
	sock.reconntime = time.Second * 1 // make it a tunable?
	sock.proto = proto
	sock.transports = make(map[string]Transport)
	sock.linger = time.Second

	// Add some conditionals now -- saves checks later
	if i, ok := interface{}(proto).(ProtocolRecvHook); ok {
		sock.recvhook = i.(ProtocolRecvHook)
	}
	if i, ok := interface{}(proto).(ProtocolSendHook); ok {
		sock.sendhook = i.(ProtocolSendHook)
	}

	proto.Init(sock)

	return sock
}

// MakeSocket is intended for use by Protocol implementations.  The intention
// is that they can wrap this to provide a "proto.NewSocket()" implementation.
func MakeSocket(proto Protocol) *socket {
	return newSocket(proto)
}

// Implementation of ProtocolHandle bits on coreHandle.  This is the middle
// API presented to Protocol implementations.

func (sock *socket) SendChannel() <-chan *Message {
	return sock.uwq
}

func (sock *socket) RecvChannel() chan<- *Message {
	return sock.urq
}

func (sock *socket) CloseChannel() chan struct{} {
	return sock.closeq
}

func (sock *socket) DrainChannel() chan struct{} {
	return sock.drainq
}

//
// Implementation of Socket bits on socket.  This is the upper API
// presented to applications.
//

func (sock *socket) Close() error {
	sock.Lock()
	if sock.closing {
		sock.Unlock()
		return ErrClosed
	}
	sock.closing = true
	close(sock.closeq)

	for _, a := range sock.accepters {
		a.Close()
	}

	pipes := append([]*pipe{}, sock.pipes...)
	sock.Unlock()

	if sock.linger > 0 {
		fin := time.After(sock.linger)

		for {
			if len(sock.uwq) == 0 {
				break
			}
			select {
			case <-fin:
				break
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	for _, p := range pipes {
		p.Close()
	}

	return nil
}

func (sock *socket) SendMsg(msg *Message) error {
	if sock.sendhook != nil {
		if ok := sock.sendhook.SendHook(msg); !ok {
			// just drop it silently
			msg.Free()
			return nil
		}
	}
	sock.Lock()
	timeout := mkTimer(sock.wdeadline)
	sock.Unlock()
	select {
	case <-timeout:
		return ErrSendTimeout
	case <-sock.closeq:
		return ErrClosed
	case sock.uwq <- msg:
		return nil
	}
}

func (sock *socket) Send(b []byte) error {
	msg := &Message{Body: b, Header: nil, refcnt: 1}
	return sock.SendMsg(msg)
}

func (sock *socket) RecvMsg() (*Message, error) {
	sock.Lock()
	timeout := mkTimer(sock.rdeadline)
	sock.Unlock()

	for {
		select {
		case <-timeout:
			return nil, ErrRecvTimeout
		case msg := <-sock.urq:
			if sock.recvhook != nil {
				if ok := sock.recvhook.RecvHook(msg); ok {
					return msg, nil
				} // else loop
				msg.Free()
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

func (sock *socket) AddTransport(t Transport) {
	sock.Lock()
	sock.transports[t.Scheme()] = t
	sock.Unlock()
}

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

func (sock *socket) SetOption(name string, value interface{}) error {
	err := sock.proto.SetOption(name, value)
	if err == nil {
		return nil
	}
	if err != ErrBadOption {
		return err
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
	switch name {
	case OptionRecvDeadline:
		sock.Lock()
		sock.rdeadline = value.(time.Duration)
		sock.Unlock()
		return nil
	case OptionSendDeadline:
		sock.Lock()
		sock.wdeadline = value.(time.Duration)
		sock.Unlock()
		return nil
	}
	return ErrBadOption
}

func (sock *socket) GetOption(name string) (interface{}, error) {
	val, err := sock.proto.GetOption(name)
	if err == nil {
		return val, nil
	}
	if err != ErrBadOption {
		return nil, err
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

	switch name {
	case OptionRecvDeadline:
		sock.Lock()
		defer sock.Unlock()
		return sock.rdeadline, nil
	case OptionSendDeadline:
		sock.Lock()
		defer sock.Unlock()
		return sock.wdeadline, nil
	}
	return nil, ErrBadOption
}

func (sock *socket) GetProtocol() Protocol {
	return sock.proto
}
