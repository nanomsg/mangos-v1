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

// Package xpub implements the PUB protocol. This broadcasts messages
// out to SUB partners, where they may be filtered.
package xpub

import (
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2/protocol"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closed bool
	closeq chan struct{}
	sendq  chan *protocol.Message
}

type socket struct {
	closed   bool
	pipes    map[uint32]*pipe
	sendQLen int
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (s *socket) SendMsg(m *protocol.Message) error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	// This could benefit from optimization to avoid useless duplicates.
	for _, p := range s.pipes {
		pm := m.Dup()
		select {
		case p.sendq <- pm:
		case <-p.closeq:
			pm.Free()
		default:
			// backpressure, but we do not exert
			pm.Free()
		}
	}
	s.Unlock()
	m.Free()
	return nil
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return nil, protocol.ErrProtoOp
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {
			s.Lock()
			s.sendQLen = v
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
	p := &pipe{
		p:      pp,
		s:      s,
		closeq: make(chan struct{}),
		sendq:  make(chan *protocol.Message, s.sendQLen),
	}
	s.pipes[pp.GetID()] = p

	go p.sender()
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	s.Lock()
	p, ok := s.pipes[pp.GetID()]
	s.Unlock()
	if ok && p.p == pp {
		p.Close()
	}
}

func (s *socket) OpenContext() (protocol.Context, error) {
	return nil, protocol.ErrProtoOp
}

func (*socket) Info() protocol.Info {
	return Info()
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

	// close and remove each and every pipe
	for _, p := range pipes {
		p.Close()
	}
	return nil

}

func (p *pipe) sender() {
outer:
	for {
		var m *protocol.Message
		select {
		case <-p.closeq:
			break outer
		case m = <-p.sendq:
		}

		if err := p.p.SendMsg(m); err != nil {
			m.Free()
			break
		}
	}
	p.Close()
}

func (p *pipe) receiver() {
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		m.Free()
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
	delete(p.s.pipes, p.p.GetID())
	p.s.Unlock()

	close(p.closeq)
	p.p.Close()
	return nil
}

// Info returns protocol information.
func Info() protocol.Info {
	return protocol.Info{
		Self:     protocol.ProtoPub,
		Peer:     protocol.ProtoSub,
		SelfName: "pub",
		PeerName: "sub",
	}
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		sendQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
