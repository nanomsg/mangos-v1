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

// Package xpush implements the raw PUSH protocol.
package xpush

import (
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoPush
	Peer     = protocol.ProtoPull
	SelfName = "push"
	PeerName = "pull"
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
	sendq      chan *protocol.Message
	pipes      map[uint32]*pipe
	sendExpire time.Duration
	sendQLen   int
	bestEffort bool
	readyq     []*pipe
	cv         *sync.Cond
	sync.Mutex
}

var (
	nilQ    <-chan time.Time
	closedQ chan time.Time
)

const defaultQLen = 128

func init() {
	closedQ = make(chan time.Time)
	close(closedQ)
}

// SendMsg implements sending a message.  The message must come with
// its headers already prepared.  This will be at a minimum the request
// ID at the end of the header, plus any leading backtrace information
// coming from a paired REP socket.
func (s *socket) SendMsg(m *protocol.Message) error {
	s.Lock()
	bestEffort := s.bestEffort
	tq := nilQ
	if bestEffort {
		tq = closedQ
	} else if s.sendExpire > 0 {
		tq = time.After(s.sendExpire)
	}
	s.Unlock()

	select {
	case s.sendq <- m:
	case <-s.closeq:
		return protocol.ErrClosed
	case <-tq:
		if bestEffort {
			m.Free()
			return nil
		}
		return protocol.ErrSendTimeout
	}

	s.Lock()
	s.cv.Signal()
	s.Unlock()
	return nil
}

func (s *socket) sender() {
	s.Lock()
	defer s.Unlock()
	for {
		if s.closed {
			return
		}
		if len(s.readyq) == 0 || len(s.sendq) == 0 {
			s.cv.Wait()
			continue
		}
		m := <-s.sendq
		p := s.readyq[0]
		s.readyq = s.readyq[1:]
		go p.send(m)
	}
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return nil, protocol.ErrProtoOp
}

func (p *pipe) receiver() {
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		// We really never expected to receive this.
		m.Free()
	}
	p.Close()
}

func (p *pipe) send(m *protocol.Message) {
	s := p.s
	if err := p.p.SendMsg(m); err != nil {
		m.Free()
		if err == protocol.ErrClosed {
			return
		}
	}
	s.Lock()
	if !s.closed && !p.closed {
		s.readyq = append(s.readyq, p)
		s.cv.Broadcast()
	}
	s.Unlock()

}

func (p *pipe) Close() error {
	s := p.s
	s.Lock()
	if p.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	p.closed = true
	for i, rp := range s.readyq {
		if p == rp {
			s.readyq = append(s.readyq[:i], s.readyq[i+1:]...)
			break
		}
	}
	delete(s.pipes, p.p.ID())
	s.Unlock()
	close(p.closeq)
	p.p.Close()
	return nil
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case protocol.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.sendExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionBestEffort:
		if v, ok := value.(bool); ok {
			s.Lock()
			s.bestEffort = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {

			newchan := make(chan *protocol.Message, v)
			s.Lock()
			s.sendQLen = v
			oldchan := s.sendq
			s.sendq = newchan
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
		}
		return protocol.ErrBadValue
	}
	return protocol.ErrBadOption
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return true, nil
	case protocol.OptionSendDeadline:
		s.Lock()
		v := s.sendExpire
		s.Unlock()
		return v, nil
	case protocol.OptionBestEffort:
		s.Lock()
		v := s.bestEffort
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

	// close and remove each and every pipe
	for _, p := range pipes {
		p.Close()
	}
	return nil
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

	s.readyq = append(s.readyq, p)
	s.cv.Broadcast()
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

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		closeq:   make(chan struct{}),
		sendq:    make(chan *protocol.Message, defaultQLen),
		sendQLen: defaultQLen,
	}
	s.cv = sync.NewCond(s)
	go s.sender()
	return s
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
