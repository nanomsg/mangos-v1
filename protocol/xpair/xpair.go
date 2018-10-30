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

// Package xpair implements the PAIR protocol. This is a simple 1:1
// messaging pattern.  Only one peer can be connected at a time.
package xpair

import (
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoPair
	Peer     = protocol.ProtoPair
	SelfName = "pair"
	PeerName = "pair"
)

const defaultQlen = 128

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closeq chan struct{}
	closed bool
}

type socket struct {
	closed     bool
	closeq     chan struct{}
	peer       *pipe
	recvQLen   int
	sendQLen   int
	recvExpire time.Duration
	sendExpire time.Duration
	bestEffort bool
	recvq      chan *protocol.Message
	sendq      chan *protocol.Message
	sync.Mutex
}

var (
	nilQ    <-chan time.Time
	closedQ chan time.Time
)

func init() {
	closedQ = make(chan time.Time)
	close(closedQ)
}

const defaultQLen = 128

func (s *socket) SendMsg(m *protocol.Message) error {
	tq := nilQ
	s.Lock()
	if s.bestEffort {
		tq = closedQ
	} else if s.sendExpire > 0 {
		tq = time.After(s.sendExpire)
	}
	s.Unlock()

	select {
	case <-s.closeq:
		return protocol.ErrClosed
	case <-tq:
		if tq == closedQ {
			m.Free()
			return nil
		}
		return protocol.ErrSendTimeout

	case s.sendq <- m:
		return nil
	}
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	// For now this uses a simple unified queue for the entire
	// socket.  Later we can look at moving this to priority queues
	// based on socket pipes.
	tq := nilQ
	s.Lock()
	if s.recvExpire > 0 {
		tq = time.After(s.recvExpire)
	}
	s.Unlock()
	select {
	case <-s.closeq:
		return nil, protocol.ErrClosed
	case <-tq:
		return nil, protocol.ErrRecvTimeout
	case m := <-s.recvq:
		return m, nil
	}
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case protocol.OptionBestEffort:
		if v, ok := value.(bool); ok {
			s.Lock()
			s.bestEffort = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.recvExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.sendExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newchan := make(chan *protocol.Message, v)

			s.Lock()
			s.recvQLen = v
			s.recvq = newchan
			s.Unlock()

			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newchan := make(chan *protocol.Message, v)

			s.Lock()
			s.sendQLen = v
			s.sendq = newchan
			s.Unlock()

			return nil
		}
		return protocol.ErrBadValue

	}

	return protocol.ErrBadOption
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return true, nil
	case protocol.OptionBestEffort:
		s.Lock()
		v := s.bestEffort
		s.Unlock()
		return v, nil
	case protocol.OptionRecvDeadline:
		s.Lock()
		v := s.recvExpire
		s.Unlock()
		return v, nil
	case protocol.OptionSendDeadline:
		s.Lock()
		v := s.sendExpire
		s.Unlock()
		return v, nil
	case protocol.OptionReadQLen:
		s.Lock()
		v := s.recvQLen
		s.Unlock()
		return v, nil
	case protocol.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
		s.Unlock()
		return v, nil

	}

	return nil, protocol.ErrBadOption
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	if s.peer != nil {
		return protocol.ErrProtoState
	}
	p := &pipe{
		p:      pp,
		s:      s,
		closeq: make(chan struct{}),
	}
	s.peer = p
	go p.receiver()
	go p.sender()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	s.Lock()
	p := s.peer
	if p == nil || pp != p.p {
		s.Unlock()
		return
	}
	s.Unlock()
	p.Close()
}

func (s *socket) OpenContext() (protocol.Context, error) {
	return nil, protocol.ErrProtoOp
}

func (*socket) Info() protocol.Info {
	return protocol.Info{
		Self:     Self,
		Peer:     Peer,
		SelfName: SelfName,
		PeerName: PeerName,
	}
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true

	p := s.peer

	s.Unlock()
	close(s.closeq)

	// This allows synchronous close without the lock.
	if p != nil {
		p.Close()
	}

	return nil
}

func (p *pipe) receiver() {
	s := p.s
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		select {
		case s.recvq <- m:
		case <-s.closeq:
			m.Free()
			break outer
		case <-p.closeq:
			m.Free()
			break outer
		}
	}
	p.Close()
}

func (p *pipe) sender() {
	s := p.s
outer:
	for {
		select {
		case m := <-s.sendq:
			if err := p.p.SendMsg(m); err != nil {
				m.Free()
				break outer
			}

		case <-s.closeq:
			break outer
		case <-p.closeq:
			break outer
		}
	}
	p.Close()
}

func (p *pipe) Close() error {
	s := p.s
	s.Lock()
	if p.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	p.closed = true
	if s.peer == p {
		s.peer = nil
	}
	s.Unlock()
	close(p.closeq)
	p.p.Close()
	return nil
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		closeq:   make(chan struct{}),
		recvq:    make(chan *protocol.Message, defaultQLen),
		sendq:    make(chan *protocol.Message, defaultQLen),
		recvQLen: defaultQLen,
		sendQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a raw Socket using the PULL protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
