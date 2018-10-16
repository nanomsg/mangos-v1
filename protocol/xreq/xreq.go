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

// Package xreq implements the raw REQ protocol, which is the request side of
// the request/response pattern.  (REP is the response.)
package xreq

import (
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/errors"
	"nanomsg.org/go/mangos/v2/impl"
)

// Borrow common error codes for convenience.
const (
	ErrClosed      = errors.ErrClosed
	ErrSendTimeout = errors.ErrSendTimeout
	ErrRecvTimeout = errors.ErrRecvTimeout
	ErrBadValue    = errors.ErrBadValue
	ErrBadOption   = errors.ErrBadOption
	ErrProtoOp     = errors.ErrProtoOp
)

type pipe struct {
	ep     mangos.Endpoint
	s      *socket
	closed bool
	closeq chan struct{}
}

type socket struct {
	closed     bool
	closeq     chan struct{}
	recvq      chan *mangos.Message
	sendq      chan *mangos.Message
	pipes      map[uint32]*pipe
	recvExpire time.Duration
	sendExpire time.Duration
	sendQLen   int
	recvQLen   int
	bestEffort bool
	sync.Mutex
}

var (
	nilQ    <-chan time.Time
	closedQ chan time.Time
)

const defaultQLen = 128

func init() {
	closedQ := make(chan time.Time)
	close(closedQ)
}

// SendMsg implements sending a message.  The message must come with
// its headers already prepared.  This will be at a minimum the request
// ID at the end of the header, plus any leading backtrace information
// coming from a paired REP socket.
func (s *socket) SendMsg(m *mangos.Message) error {
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
		return nil
	case <-s.closeq:
		return ErrClosed
	case <-tq:
		if bestEffort {
			m.Free()
			return nil
		}
		return ErrSendTimeout
	}
}

func (s *socket) RecvMsg() (*mangos.Message, error) {
	tq := nilQ
	s.Lock()
	if s.recvExpire > 0 {
		tq = time.After(s.recvExpire)
	}
	s.Unlock()
	select {
	case <-s.closeq:
		return nil, ErrClosed
	case <-tq:
		return nil, ErrRecvTimeout
	case m := <-s.recvq:
		return m, nil
	}
}

type reqCtxBad struct {
	cond       *sync.Cond
	resendTime time.Duration   // tunable resend time
	sendExpire time.Duration   // how long to wait in send
	recvExpire time.Duration   // how long to wait in recv
	sendTimer  *time.Timer     // send timer
	recvTimer  *time.Timer     // recv timer
	resender   *time.Timer     // resend timeout
	reqMsg     *mangos.Message // message for transmit
	repMsg     *mangos.Message // received reply
	sendMsg    *mangos.Message // messaging waiting for send
	lastPipe   *pipe           // last pipe used for transmit
	recvWait   bool            // true if a thread is blocked in RecvMsg
	wantw      bool            // true if we need to send a message
	closed     bool            // true if we are closed
}

func (p *pipe) receiver() {
	s := p.s
outer:
	for {
		m := p.ep.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}

		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		select {
		case s.recvq <- m:
			continue
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

// This is a puller, and doesn't permit for priorities.  We might want
// to refactor this to use a push based scheme later.
func (p *pipe) sender() {
	s := p.s
outer:
	for {
		var m *mangos.Message
		select {
		case m = <-s.sendq:
		case <-p.closeq:
			break outer
		case <-s.closeq:
			break outer
		}

		if e := p.ep.SendMsg(m); e != nil {
			break
		}
	}
	p.ep.Close()
}

func (p *pipe) Close() error {
	s := p.s
	s.Lock()
	if p.closed {
		s.Unlock()
		return ErrClosed
	}
	p.closed = true
	delete(s.pipes, p.ep.GetID())
	s.Unlock()
	close(p.closeq)
	p.ep.Close()
	return nil
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case mangos.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.recvExpire = v
			s.Unlock()
			return nil
		}
		return ErrBadValue
	case mangos.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.sendExpire = v
			s.Unlock()
			return nil
		}
		return ErrBadValue

	case mangos.OptionBestEffort:
		if v, ok := value.(bool); ok {
			s.Lock()
			s.bestEffort = v
			s.Unlock()
			return nil
		}
		return ErrBadValue

	case mangos.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {

			newchan := make(chan *mangos.Message, v)
			s.Lock()
			s.sendQLen = v
			oldchan := s.sendq
			s.sendq = newchan
			s.Unlock()

			for {
				var m *mangos.Message
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
		return ErrBadValue

	case mangos.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newchan := make(chan *mangos.Message, v)
			s.Lock()
			s.recvQLen = v
			oldchan := s.recvq
			s.recvq = newchan
			s.Unlock()

			for {
				var m *mangos.Message
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
		// We don't support these
		// case OptionLinger:
	}

	return ErrBadOption
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case mangos.OptionRaw:
		return true, nil
	case mangos.OptionRecvDeadline:
		s.Lock()
		v := s.recvExpire
		s.Unlock()
		return v, nil
	case mangos.OptionSendDeadline:
		s.Lock()
		v := s.sendExpire
		s.Unlock()
		return v, nil
	case mangos.OptionBestEffort:
		s.Lock()
		v := s.bestEffort
		s.Unlock()
		return v, nil
	case mangos.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
		s.Unlock()
		return v, nil
	case mangos.OptionReadQLen:
		s.Lock()
		v := s.recvQLen
		s.Unlock()
		return v, nil
	}

	return nil, ErrBadOption
}

func (s *socket) Close() error {
	s.Lock()

	if s.closed {
		s.Unlock()
		return ErrClosed
	}
	s.closed = true
	s.Unlock()

	// close and remove each and every pipe
	for _, p := range s.pipes {
		go p.Close()
	}
	return nil
}

func (s *socket) AddPipe(ep mangos.Endpoint) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrClosed
	}
	p := &pipe{
		ep:     ep,
		s:      s,
		closeq: make(chan struct{}),
	}
	s.pipes[ep.GetID()] = p

	go p.sender()
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(ep mangos.Endpoint) {
	s.Lock()
	p, ok := s.pipes[ep.GetID()]
	s.Unlock()
	if ok && p.ep == ep {
		p.Close()
	}
}

func (s *socket) OpenContext() (mangos.ProtocolContext, error) {
	return nil, ErrProtoOp
}

func (*socket) Info() mangos.ProtocolInfo {
	return Info()
}

// Info returns protocol information.
func Info() mangos.ProtocolInfo {
	return mangos.ProtocolInfo{
		Self:     mangos.ProtoReq,
		Peer:     mangos.ProtoRep,
		SelfName: "req",
		PeerName: "rep",
	}
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() mangos.ProtocolBase {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		closeq:   make(chan struct{}),
		recvq:    make(chan *mangos.Message, defaultQLen),
		sendq:    make(chan *mangos.Message, defaultQLen),
		sendQLen: defaultQLen,
		recvQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (mangos.Socket, error) {
	return impl.MakeSocket(NewProtocol()), nil
}
