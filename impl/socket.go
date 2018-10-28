// Copyright 2018 The Mangos Authors
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

package impl

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

// Message is just a local alias for mangos.Message
type Message = mangos.Message

// defaultMaxRxSize is the default maximum Rx size
const defaultMaxRxSize = 1024 * 1024

const defaultReconnMinTime = time.Millisecond * 100

const defaultReconnMaxTime = time.Duration(0)

// socket is the meaty part of the core information.
type socket struct {
	proto mangos.ProtocolBase

	sync.Mutex

	closed        bool          // true if Socket was closed at API level
	reconnMinTime time.Duration // reconnect time after error or disconnect
	reconnMaxTime time.Duration // max reconnect interval
	maxRxSize     int           // max recv size
	dialAsynch    bool          // asynchronous dialing?

	listeners []*listener
	dialers   []*dialer
	pipes     map[*pipe]struct{}

	translk sync.RWMutex
	trans   map[string]transport.Transport

	porthook mangos.PortHook // called when a port is added or removed
}

type context struct {
	mangos.ProtocolContext
}

func (s *socket) addPipe(tp transport.Pipe, d *dialer, l *listener) {
	p := newPipe(tp, s, d, l)

	// Either listener or dialer is non-nil.
	if l == nil && d == nil {
		panic("both l and d should not be nil")
	}

	s.Lock()
	fn := s.porthook
	s.Unlock()

	if fn != nil && !fn(mangos.PortActionAdd, p) {
		go p.Close()
		return
	}

	s.Lock()
	if s.pipes == nil || s.proto.AddPipe(p) != nil {
		s.Unlock()
		go p.Close()
		return
	}
	s.pipes[p] = struct{}{}
	if p.d != nil {
		// This call resets the redial time in the dialer.  Its
		// kind of ugly that we have the socket doing this, but
		// the scope is narrow, and it works.
		go p.d.pipeConnected()
	}
	s.Unlock()
}

func (s *socket) remPipe(p *pipe) {

	s.proto.RemovePipe(p)

	s.Lock()
	delete(s.pipes, p)
	if fn := s.porthook; fn != nil {
		go fn(mangos.PortActionRemove, p)
	}
	s.Unlock()
}

func newSocket(proto mangos.ProtocolBase) *socket {
	s := &socket{
		proto:         proto,
		reconnMinTime: defaultReconnMinTime,
		reconnMaxTime: defaultReconnMaxTime,
		maxRxSize:     defaultMaxRxSize,
		trans:         make(map[string]transport.Transport),
		pipes:         make(map[*pipe]struct{}),
	}
	return s
}

// MakeSocket is intended for use by Protocol implementations.  The intention
// is that they can wrap this to provide a "proto.NewSocket()" implementation.
func MakeSocket(proto mangos.ProtocolBase) mangos.Socket {
	return newSocket(proto)
}

func (s *socket) Close() error {

	s.Lock()
	if s.closed {
		s.Unlock()
		return mangos.ErrClosed
	}
	listeners := s.listeners
	dialers := s.dialers
	pipes := s.pipes

	s.listeners = nil
	s.dialers = nil
	s.pipes = nil
	s.Unlock()

	for _, l := range listeners {
		l.Close()
	}
	for _, d := range dialers {
		d.Close()
	}

	for p := range pipes {
		p.Close()
	}

	s.proto.Close()
	return nil
}

func (ctx context) Send(b []byte) error {
	msg := mangos.NewMessage(len(b))
	msg.Body = append(msg.Body, b...)
	return ctx.SendMsg(msg)
}
func (ctx context) Recv() ([]byte, error) {
	msg, err := ctx.RecvMsg()
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, len(msg.Body))
	b = append(b, msg.Body...)
	msg.Free()
	return b, nil
}

func (s *socket) OpenContext() (mangos.Context, error) {
	c, err := s.proto.OpenContext()
	if err != nil {
		return nil, err
	}
	return &context{c}, nil
}

func (s *socket) SendMsg(msg *Message) error {
	return s.proto.SendMsg(msg)
}

func (s *socket) Send(b []byte) error {
	msg := mangos.NewMessage(len(b))
	msg.Body = append(msg.Body, b...)
	return s.SendMsg(msg)
}

func (s *socket) RecvMsg() (*Message, error) {
	return s.proto.RecvMsg()
}

func (s *socket) Recv() ([]byte, error) {
	msg, err := s.RecvMsg()
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, len(msg.Body))
	b = append(b, msg.Body...)
	msg.Free()
	return b, nil
}

// String just emits a very high level debug.  This avoids
// triggering race conditions from trying to print %v without
// holding locks on structure members.
func (s *socket) String() string {
	return fmt.Sprintf("SOCKET[%s](%p)", s.proto.Info().SelfName, s)
}

func (s *socket) getTransport(addr string) transport.Transport {
	var i int

	if i = strings.Index(addr, "://"); i < 0 {
		return nil
	}
	scheme := addr[:i]

	s.translk.RLock()
	t, ok := s.trans[scheme]
	s.translk.RUnlock()

	if t != nil && ok {
		return t
	}
	return nil
}

func (s *socket) AddTransport(t transport.Transport) {
	s.translk.Lock()
	s.trans[t.Scheme()] = t
	s.translk.Unlock()
}

func (s *socket) DialOptions(addr string, opts map[string]interface{}) error {

	d, err := s.NewDialer(addr, opts)
	if err != nil {
		return err
	}
	return d.Dial()
}

func (s *socket) Dial(addr string) error {
	return s.DialOptions(addr, nil)
}

func (s *socket) NewDialer(addr string, options map[string]interface{}) (mangos.Dialer, error) {
	t := s.getTransport(addr)
	if t == nil {
		return nil, mangos.ErrBadTran
	}
	td, err := t.NewDialer(addr, s)
	if err != nil {
		return nil, err
	}
	d := &dialer{
		d:             td,
		s:             s,
		reconnMinTime: s.reconnMinTime,
		reconnMaxTime: s.reconnMaxTime,
		addr:          addr,
	}
	for n, v := range options {
		switch n {
		case mangos.OptionReconnectTime:
			fallthrough
		case mangos.OptionMaxReconnectTime:
			fallthrough
		case mangos.OptionDialAsynch:
			if err := d.SetOption(n, v); err != nil {
				return nil, err
			}
		default:
			if err = td.SetOption(n, v); err != nil {
				return nil, err
			}
		}
	}
	if _, ok := options[mangos.OptionMaxRecvSize]; !ok {
		err = td.SetOption(mangos.OptionMaxRecvSize, s.maxRxSize)
		if err != nil && err != mangos.ErrBadOption {
			return nil, err
		}
	}

	s.Lock()
	if s.closed {
		s.Unlock()
		return nil, mangos.ErrClosed
	}
	s.dialers = append(s.dialers, d)
	s.Unlock()
	return d, nil
}

func (s *socket) ListenOptions(addr string, options map[string]interface{}) error {
	l, err := s.NewListener(addr, options)
	if err != nil {
		return err
	}
	if err = l.Listen(); err != nil {
		return err
	}
	return nil
}

func (s *socket) Listen(addr string) error {
	return s.ListenOptions(addr, nil)
}

func (s *socket) NewListener(addr string, options map[string]interface{}) (mangos.Listener, error) {
	// This function sets up a goroutine to accept inbound connections.
	// The accepted connection will be added to a list of accepted
	// connections.  The Listener just needs to listen continuously,
	// as we assume that we want to continue to receive inbound
	// connections without limit.
	t := s.getTransport(addr)
	if t == nil {
		return nil, mangos.ErrBadTran
	}
	tl, err := t.NewListener(addr, s)
	if err != nil {
		return nil, err
	}
	for n, v := range options {
		if err = tl.SetOption(n, v); err != nil {
			tl.Close()
			return nil, err
		}
	}
	if _, ok := options[mangos.OptionMaxRecvSize]; !ok {
		err = tl.SetOption(mangos.OptionMaxRecvSize, s.maxRxSize)
		if err != nil && err != mangos.ErrBadOption {
			return nil, err
		}
	}
	l := &listener{
		l:    tl,
		s:    s,
		addr: addr,
	}
	s.Lock()
	if s.closed {
		s.Unlock()
		tl.Close()
		return nil, mangos.ErrClosed
	}
	s.listeners = append(s.listeners, l)
	s.Unlock()

	return l, nil
}

func (s *socket) SetOption(name string, value interface{}) error {
	if err := s.proto.SetOption(name, value); err != mangos.ErrBadOption {
		return err
	}

	s.Lock()
	defer s.Unlock()

	switch name {
	case mangos.OptionMaxRecvSize:
		if v, ok := value.(int); ok && v >= 0 {
			s.maxRxSize = v
		} else {
			return mangos.ErrBadValue
		}
		break
	case mangos.OptionReconnectTime:
		if v, ok := value.(time.Duration); ok {
			s.reconnMinTime = v
		} else {
			return mangos.ErrBadValue
		}
	case mangos.OptionMaxReconnectTime:
		if v, ok := value.(time.Duration); ok {
			s.reconnMaxTime = v
		} else {
			return mangos.ErrBadValue
		}
	case mangos.OptionDialAsynch:
		if v, ok := value.(bool); ok {
			s.dialAsynch = v
		} else {
			return mangos.ErrBadValue
		}
	default:
		return mangos.ErrBadOption
	}
	for _, d := range s.dialers {
		d.SetOption(name, value)
	}
	for _, l := range s.listeners {
		l.SetOption(name, value)
	}
	return nil
}

func (s *socket) GetOption(name string) (interface{}, error) {
	if val, err := s.proto.GetOption(name); err != mangos.ErrBadOption {
		return val, err
	}

	s.Lock()
	defer s.Unlock()

	switch name {
	case mangos.OptionMaxRecvSize:
		return s.maxRxSize, nil
	case mangos.OptionReconnectTime:
		return s.reconnMinTime, nil
	case mangos.OptionMaxReconnectTime:
		return s.reconnMaxTime, nil
	}
	return nil, mangos.ErrBadOption
}

func (s *socket) Info() mangos.ProtocolInfo {
	return s.proto.Info()
}

func (s *socket) SetPortHook(newhook mangos.PortHook) mangos.PortHook {
	s.Lock()
	oldhook := s.porthook
	s.porthook = newhook
	s.Unlock()
	return oldhook
}
