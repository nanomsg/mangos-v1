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

package core

import (
	"math/rand"
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

// The pipes global state is just an ID allocator; it manages the
// list of which IDs are in use.  Nothing looks things up this way,
// so this doesn't keep references to other state.
var pipes struct {
	sync.Mutex
	IDs    map[uint32]struct{}
	nextID uint32
}

// pipe wraps the Pipe data structure with the stuff we need to keep
// for the core.  It implements the Pipe interface.
type pipe struct {
	sync.Mutex
	id     uint32
	p      transport.Pipe
	l      *listener
	d      *dialer
	s      *socket
	closed bool // true if we were closed
}

func init() {
	pipes.IDs = make(map[uint32]struct{})
	pipes.nextID = uint32(rand.NewSource(time.Now().UnixNano()).Int63())
}

func newPipe(tp transport.Pipe, s *socket, d *dialer, l *listener) *pipe {
	p := &pipe{
		p: tp,
		d: d,
		l: l,
		s: s,
	}
	pipes.Lock()
	for {
		p.id = pipes.nextID & 0x7fffffff
		pipes.nextID++
		if p.id == 0 {
			continue
		}
		if _, ok := pipes.IDs[p.id]; !ok {
			pipes.IDs[p.id] = struct{}{}
			break
		}
	}
	pipes.Unlock()
	return p
}

func (p *pipe) ID() uint32 {
	return p.id
}

func (p *pipe) Close() error {
	s := p.s

	p.Lock()
	if p.closed {
		p.Unlock()
		return nil
	}
	p.closed = true
	p.Unlock()

	if s != nil {
		s.remPipe(p)
	}
	p.p.Close()

	// If the pipe was from a inform it so that it can redial.
	if d := p.d; d != nil {
		go d.pipeClosed()
	}

	// This is last, as we keep the ID reserved until everything is
	// done with it.
	pipes.Lock()
	delete(pipes.IDs, p.id)
	pipes.Unlock()

	return nil
}

func (p *pipe) SendMsg(msg *mangos.Message) error {

	if err := p.p.Send(msg); err != nil {
		p.Close()
		return err
	}
	return nil
}

func (p *pipe) RecvMsg() *mangos.Message {

	msg, err := p.p.Recv()
	if err != nil {
		p.Close()
		return nil
	}
	msg.Pipe = p
	return msg
}

func (p *pipe) Address() string {
	switch {
	case p.l != nil:
		return p.l.Address()
	case p.d != nil:
		return p.d.Address()
	}
	return ""
}

func (p *pipe) GetOption(name string) (interface{}, error) {
	val, err := p.p.GetOption(name)
	if err == mangos.ErrBadOption {
		if p.d != nil {
			val, err = p.d.GetOption(name)
		} else if p.l != nil {
			val, err = p.l.GetOption(name)
		}
	}
	return val, err
}

func (p *pipe) Dialer() mangos.Dialer {
	if p.d == nil {
		return nil
	}
	return p.d
}

func (p *pipe) Listener() mangos.Listener {
	if p.l == nil {
		return nil
	}
	return p.l
}
