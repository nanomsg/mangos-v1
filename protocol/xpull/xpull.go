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

// Package xpull implements the PULL protocol. This read only protocol
// simply receives messages from pipes.
package xpull

import (
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoPull
	Peer     = protocol.ProtoPush
	SelfName = "pull"
	PeerName = "push"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closed bool
	closeq chan struct{}
}

type socket struct {
	closed     bool
	closeq     chan struct{}
	pipes      map[uint32]*pipe
	recvQLen   int
	recvExpire time.Duration
	recvq      chan *protocol.Message
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (s *socket) SendMsg(m *protocol.Message) error {
	return protocol.ErrProtoOp
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

	case protocol.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.recvExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newchan := make(chan *protocol.Message, v)
			s.Lock()
			s.recvQLen = v
			oldchan := s.recvq
			s.recvq = newchan
			s.Unlock()

			for {
				var m *protocol.Message
				select {
				case m = <-oldchan:
				default:
				}
				if m == nil {
					break
				}
				select {
				case newchan <- m:
				default:
					m.Free()
				}
			}
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
	case protocol.OptionRecvDeadline:
		s.Lock()
		v := s.recvExpire
		s.Unlock()
		return v, nil
	case protocol.OptionReadQLen:
		s.Lock()
		v := s.recvQLen
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
	p := &pipe{
		p:      pp,
		s:      s,
		closeq: make(chan struct{}),
	}
	s.pipes[pp.ID()] = p

	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	s.Lock()
	p, ok := s.pipes[pp.ID()]
	s.Unlock()
	if ok && p.p == pp {
		p.Close()
	}
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

	pipes := make([]*pipe, 0, len(s.pipes))
	for _, p := range s.pipes {
		pipes = append(pipes, p)
	}

	s.Unlock()
	close(s.closeq)

	// This allows synchronous close without the lock.
	for _, p := range pipes {
		p.Close()
	}

	return nil
}

func (p *pipe) receiver() {
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		select {
		case p.s.recvq <- m:
		case <-p.closeq:
			m.Free()
			break outer
		case <-p.s.closeq:
			m.Free()
			break outer
		}
	}
	p.Close()
}

func (p *pipe) Close() error {
	p.s.Lock()
	if p.closed {
		p.s.Unlock()
		return protocol.ErrClosed
	}
	p.closed = true
	delete(p.s.pipes, p.p.ID())
	p.s.Unlock()

	close(p.closeq)
	p.p.Close()
	return nil
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		closeq:   make(chan struct{}),
		recvq:    make(chan *protocol.Message, defaultQLen),
		recvQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a raw Socket using the PULL protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
