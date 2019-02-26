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

// Package sub implements the SUB protocol.  This protocol receives messages
// from publishers (PUB peers).  The messages are filtered based on
// subscription, such that only subscribed messages (see OptionSubscribe) are
// received.
//
// Note that in order to receive any messages, at least one subscription must
// be present.  If no subscription is present (the default state), receive
// operations will block forever.
package sub

import (
	"bytes"
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoSub
	Peer     = protocol.ProtoPub
	SelfName = "sub"
	PeerName = "pub"
)

type socket struct {
	master *context
	ctxs   map[*context]struct{}
	pipes  map[uint32]*pipe
	closed bool
	sync.Mutex
}

type pipe struct {
	s      *socket
	p      protocol.Pipe
	closed bool
}

type context struct {
	recvq      chan *protocol.Message
	recvQLen   int
	recvExpire time.Duration
	closeq     chan struct{}
	closed     bool
	subs       [][]byte
	s          *socket
}

const defaultQLen = 128

func (*context) SendMsg(m *protocol.Message) error {
	return protocol.ErrProtoOp
}

func (c *context) RecvMsg() (*protocol.Message, error) {

	s := c.s
	var timeq <-chan time.Time
	s.Lock()
	if c.recvExpire > 0 {
		timeq = time.After(c.recvExpire)
	}
	s.Unlock()

	select {
	case <-timeq:
		return nil, protocol.ErrRecvTimeout
	case <-c.closeq:
		return nil, protocol.ErrClosed
	case m := <-c.recvq:
		return m, nil
	}
}

func (c *context) Close() error {
	s := c.s
	s.Lock()
	if c.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true
	delete(s.ctxs, c)
	s.Unlock()
	close(c.closeq)
	return nil
}

func (*socket) SendMsg(m *protocol.Message) error {
	return protocol.ErrProtoOp
}

func (p *pipe) receiver() {
	s := p.s
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		s.Lock()
		for c := range s.ctxs {
			if c.matches(m) {
				// Matched, send it up.  Best effort.
				// As we are passing this to the user,
				// we need to ensure that the message
				// may be modified.
				dm := m.Dup()
				select {
				case c.recvq <- dm:
				default:
					dm.Free()
				}
			}
		}
		s.Unlock()
		m.Free()
	}

	p.Close()
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p: pp,
		s: s,
	}
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	s.pipes[p.p.ID()] = p
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	s.Lock()
	defer s.Unlock()
	p := s.pipes[pp.ID()]
	if p != nil && p.p == pp && !p.closed {
		p.closed = true
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
	ctxs := make([]*context, 0, len(s.ctxs))
	for c := range s.ctxs {
		ctxs = append(ctxs, c)
	}
	pipes := make([]*pipe, 0, len(s.pipes))
	for _, p := range s.pipes {
		pipes = append(pipes, p)
	}
	s.Unlock()
	for _, c := range ctxs {
		c.Close()
	}
	for _, p := range pipes {
		p.Close()
	}
	return nil
}

func (p *pipe) Close() error {
	s := p.s
	s.Lock()
	if p.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	p.closed = true
	s.Unlock()

	p.p.Close()
	return nil
}

func (c *context) matches(m *protocol.Message) bool {
	for _, sub := range c.subs {
		if bytes.HasPrefix(m.Body, sub) {
			return true
		}
	}
	return false

}

func (c *context) subscribe(topic []byte) error {
	for _, sub := range c.subs {
		if bytes.Equal(sub, topic) {
			// Already present
			return nil
		}
	}
	c.subs = append(c.subs, topic)
	return nil
}

func (c *context) unsubscribe(topic []byte) error {
	for i, sub := range c.subs {
		if !bytes.Equal(sub, topic) {
			continue
		}
		c.subs = append(c.subs[:i], c.subs[i+1:]...)

		// Because we have changed the subscription,
		// we may have messages in the channel that
		// we don't want any more.  Lets prune those.
		newchan := make(chan *protocol.Message, c.recvQLen)
		oldchan := c.recvq
		c.recvq = newchan
		for m := range oldchan {
			if !c.matches(m) {
				m.Free()
				continue
			}
			select {
			case newchan <- m:
			default:
				m.Free()
			}
		}
		return nil
	}
	// Subscription not present
	return protocol.ErrBadValue
}

func (c *context) SetOption(name string, value interface{}) error {
	s := c.s

	switch name {
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

	case protocol.OptionSubscribe:
	case protocol.OptionUnsubscribe:
	default:
		return protocol.ErrBadOption
	}

	var vb []byte

	switch v := value.(type) {
	case []byte:
		vb = v
	case string:
		vb = []byte(v)
	default:
		return protocol.ErrBadValue
	}

	s.Lock()
	defer s.Unlock()

	switch name {
	case protocol.OptionSubscribe:
		return c.subscribe(vb)

	case protocol.OptionUnsubscribe:
		return c.unsubscribe(vb)

	default:
		return protocol.ErrBadOption
	}
}

func (c *context) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionReadQLen:
		c.s.Lock()
		v := c.recvQLen
		c.s.Unlock()
		return v, nil
	}
	return nil, protocol.ErrBadOption
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return s.master.RecvMsg()
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
		recvq:      make(chan *protocol.Message, s.master.recvQLen),
		recvQLen:   s.master.recvQLen,
		recvExpire: s.master.recvExpire,
		subs:       [][]byte{},
	}
	s.ctxs[c] = struct{}{}
	return c, nil
}

func (s *socket) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionRaw:
		return false, nil
	default:
		return s.master.GetOption(name)
	}
}

func (s *socket) SetOption(name string, val interface{}) error {
	return s.master.SetOption(name, val)
}

func (s *socket) Info() protocol.Info {
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
		pipes: make(map[uint32]*pipe),
		ctxs:  make(map[*context]struct{}),
	}
	s.master = &context{
		s:        s,
		recvq:    make(chan *protocol.Message, defaultQLen),
		closeq:   make(chan struct{}),
		recvQLen: defaultQLen,
	}
	s.ctxs[s.master] = struct{}{}
	return s
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
