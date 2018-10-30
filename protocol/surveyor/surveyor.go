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

// Package surveyor implements the SURVEYOR protocol. This sends messages
// out to RESPONDENT partners, and receives their responses.
package surveyor

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"nanomsg.org/go/mangos/v2/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoSurveyor
	Peer     = protocol.ProtoRespondent
	SelfName = "surveyor"
	PeerName = "respondent"
)

const defaultSurveyTime = time.Second

type pipe struct {
	s      *socket
	p      protocol.Pipe
	closed bool
	closeq chan struct{}
	sendq  chan *protocol.Message
}

type context struct {
	s          *socket
	closed     bool
	closeq     chan struct{}
	recvq      chan *protocol.Message
	recvQLen   int
	recvExpire time.Duration
	survExpire time.Duration
	survID     uint32
}

type socket struct {
	master   *context              // default context
	ctxs     map[*context]struct{} // all contexts
	surveys  map[uint32]*context   // contexts by survey ID
	pipes    map[uint32]*pipe      // all pipes by pipe ID
	nextID   uint32                // next survey ID
	closed   bool                  // true if closed
	sendQLen int                   // send Q depth
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (c *context) cancel() {
	s := c.s
	if id := c.survID; id != 0 {
		delete(s.surveys, id)
		c.survID = 0
		oldrecvq := c.recvq
		c.recvq = nil

		// drain and close the old queue
		close(oldrecvq)
		for {
			if m := <-oldrecvq; m != nil {
				m.Free()
			} else {
				break
			}
		}
	}
}

func (c *context) SendMsg(m *protocol.Message) error {
	s := c.s

	id := atomic.AddUint32(&s.nextID, 1)
	id |= 0x80000000

	m.Header = make([]byte, 4)
	binary.BigEndian.PutUint32(m.Header, id)

	s.Lock()
	defer s.Unlock()
	if s.closed || c.closed {
		return protocol.ErrClosed
	}
	c.cancel()
	c.survID = id
	c.recvq = make(chan *protocol.Message, c.recvQLen)
	s.surveys[id] = c
	time.AfterFunc(c.survExpire, func() {
		s.Lock()
		if c.survID == id {
			c.cancel()
		}
		s.Unlock()
	})

	// Best-effort broadcast on all pipes
	for _, p := range s.pipes {
		dm := m.Dup()
		select {
		case p.sendq <- dm:
		default:
			dm.Free()
		}
	}
	m.Free()
	return nil
}

func (c *context) RecvMsg() (*protocol.Message, error) {
	s := c.s

	s.Lock()
	recvq := c.recvq
	timeq := nilQ
	if c.recvExpire > 0 {
		timeq = time.After(c.recvExpire)
	}
	s.Unlock()

	if recvq == nil {
		return nil, protocol.ErrProtoState
	}

	select {
	case <-c.closeq:
		return nil, protocol.ErrClosed

	case m := <-recvq:
		if m == nil {
			return nil, protocol.ErrProtoState
		}
		return m, nil

	case <-timeq:
		return nil, protocol.ErrRecvTimeout
	}
}

func (c *context) Close() error {
	s := c.s
	s.Lock()
	if c.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	c.closed = true
	close(c.closeq)
	if id := c.survID; id != 0 {
		c.recvq = nil
		c.survID = 0
		delete(s.surveys, id)
		// Leave the recvq open, so that closeq wins
	}
	delete(s.ctxs, c)
	s.Unlock()
	return nil
}

func (c *context) SetOption(name string, value interface{}) error {
	switch name {
	case protocol.OptionSurveyTime:
		if v, ok := value.(time.Duration); ok {
			c.s.Lock()
			c.survExpire = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			c.s.Lock()
			c.recvExpire = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok {
			newchan := make(chan *protocol.Message, v)
			c.s.Lock()
			c.recvq = newchan
			c.recvQLen = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue
	}

	return protocol.ErrBadOption
}

func (c *context) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionSurveyTime:
		c.s.Lock()
		v := c.survExpire
		c.s.Unlock()
		return v, nil
	case protocol.OptionRecvDeadline:
		c.s.Lock()
		v := c.recvExpire
		c.s.Unlock()
		return v, nil
	case protocol.OptionReadQLen:
		c.s.Lock()
		v := c.recvQLen
		c.s.Unlock()
		return v, nil
	}

	return nil, protocol.ErrBadOption
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
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		id := binary.BigEndian.Uint32(m.Header)

		s.Lock()
		if c, ok := s.surveys[id]; ok {
			select {
			case c.recvq <- m:
			default:
				m.Free()
			}
		} else {
			m.Free()
		}
		s.Unlock()
	}
}

func (s *socket) OpenContext() (protocol.Context, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, protocol.ErrClosed
	}
	c := &context{
		s:          s,
		closeq:     make(chan struct{}),
		survExpire: s.master.survExpire,
		recvExpire: s.master.recvExpire,
	}
	s.ctxs[c] = struct{}{}
	return c, nil
}

func (s *socket) SendMsg(m *protocol.Message) error {
	return s.master.SendMsg(m)
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return s.master.RecvMsg()
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p:      pp,
		s:      s,
		sendq:  make(chan *protocol.Message, s.sendQLen),
		closeq: make(chan struct{}),
	}
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	s.pipes[p.p.ID()] = p
	go p.receiver()
	go p.sender()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	s.Lock()
	defer s.Unlock()
	p := s.pipes[pp.ID()]
	if p != nil && p.p == pp && !p.closed {
		p.closed = true
		close(p.closeq)
		pp.Close()
		delete(s.pipes, pp.ID())
	}
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true
	for c := range s.ctxs {
		delete(s.ctxs, c)
		if !c.closed {
			c.closed = true
			close(c.closeq)
			if c.survID != 0 {
				delete(s.surveys, c.survID)
			}
		}
	}
	pipes := make([]*pipe, 0, len(s.pipes))
	for _, p := range s.pipes {
		pipes = append(pipes, p)
	}
	s.Unlock()

	// This allows synchronous close without the lock.
	for _, p := range pipes {
		p.Close()
	}
	return nil
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return false, nil
	case protocol.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
		s.Unlock()
		return v, nil

	default:
		return s.master.GetOption(option)
	}
}

func (s *socket) SetOption(option string, value interface{}) error {
	switch option {
	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok {
			s.Lock()
			s.sendQLen = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue
	}
	return s.master.SetOption(option, value)
}

func (*socket) Info() protocol.Info {
	return protocol.Info{
		Self:     Self,
		Peer:     Peer,
		SelfName: SelfName,
		PeerName: PeerName,
	}
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		surveys:  make(map[uint32]*context),
		ctxs:     make(map[*context]struct{}),
		sendQLen: defaultQLen,
		nextID:   uint32(time.Now().UnixNano()), // quasi-random
	}
	s.master = &context{
		s:          s,
		recvq:      make(chan *protocol.Message, defaultQLen),
		closeq:     make(chan struct{}),
		recvQLen:   defaultQLen,
		survExpire: defaultSurveyTime,
	}
	return s
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
