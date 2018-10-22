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

// Package xstar implements the experimental star protocol. This sends messages
// out to all peers, and receives their responses.  It also implicitly resends
// any message it receives to all of its peers, but it will not rebroadcast
// a message to the peer it was received from.
package xstar

import (
	"encoding/binary"
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
	closed     bool
	closeq     chan struct{}
	pipes      map[uint32]*pipe
	recvQLen   int
	sendQLen   int
	recvExpire time.Duration
	recvq      chan *protocol.Message
	ttl        int
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

	// Raw mode messages are required to come wth the header.
	if len(m.Header) != 4 {
		m.Free()
		return nil
	}

	// This is coming back to us - its a forwarded message
	// from an earlier pipe.  Note that we could also have
	// used the m.Pipe but this is how mangos v1 and nanomsg
	// did it historically.
	id := binary.BigEndian.Uint32(m.Header)

	// This could benefit from optimization to avoid useless duplicates.
	for _, p := range s.pipes {

		// Don't deliver the message back up to the same pipe it
		// arrived from.
		if p.p.GetID() == id {
			continue
		}
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

	case protocol.OptionTTL:
		if v, ok := value.(int); ok && v > 0 && v < 256 {
			s.Lock()
			s.ttl = v
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

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {
			s.Lock()
			s.sendQLen = v
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
	case protocol.OptionTTL:
		s.Lock()
		v := s.ttl
		s.Unlock()
		return v, nil
	case protocol.OptionRecvDeadline:
		s.Lock()
		v := s.recvExpire
		s.Unlock()
		return v, nil
	case protocol.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
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

	close(s.closeq)

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
	s := p.s
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = m.Body[:4]
		m.Body = m.Body[4:]
		if m.Header[0] != 0 || m.Header[1] != 0 || m.Header[2] != 0 {
			// non-zero reserved fields are illegal
			m.Free()
			continue
		}
		if int(m.Header[3]) >= s.ttl { // TTL expired?
			// XXX: bump a stat
			m.Free()
			continue
		}
		m.Header[3]++

		userm := m.Dup()
		s.Lock()
		for _, p2 := range s.pipes {
			if p2 == p {
				continue
			}
			if p2.closed {
				continue
			}

			m2 := m.Dup()
			select {
			case p2.sendq <- m2:
			case <-s.closeq:
				m2.Free()
			default:
				m2.Free()
			}
		}
		s.Unlock()
		m.Free()

		select {
		case s.recvq <- userm:
		case <-p.closeq:
			userm.Free()
			break outer
		case <-s.closeq:
			userm.Free()
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
	delete(p.s.pipes, p.p.GetID())
	p.s.Unlock()

	close(p.closeq)
	p.p.Close()
	return nil
}

// Info returns protocol information.
func Info() protocol.Info {
	return protocol.Info{
		Self:     protocol.ProtoStar,
		Peer:     protocol.ProtoStar,
		SelfName: "star",
		PeerName: "star",
	}
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		closeq:   make(chan struct{}),
		recvq:    make(chan *protocol.Message, defaultQLen),
		sendQLen: defaultQLen,
		recvQLen: defaultQLen,
		ttl:      8,
	}
	return s
}

// NewSocket allocates a raw Socket using the BUS protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
