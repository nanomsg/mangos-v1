// Copyright 2014 The Mangos Authors
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

// Package pub implements the PUB protocol.  This protocol publishes messages
// to subscribers (SUB peers).  The subscribers will filter incoming messages
// from the publisher based on their subscription.
package pub

import (
	"github.com/gdamore/mangos"
	"sync"
)

type pubEp struct {
	ep   mangos.Endpoint
	q    chan *mangos.Message
	sock mangos.ProtocolSocket
}

type pub struct {
	sock mangos.ProtocolSocket
	sync.Mutex
	eps map[uint32]*pubEp
	raw bool
}

func (p *pub) Init(sock mangos.ProtocolSocket) {
	p.sock = sock
	p.eps = make(map[uint32]*pubEp)
	go p.sender()
}

// Bottom sender.
func (pe *pubEp) sender() {
	for {
		var m *mangos.Message

		select {
		case m = <-pe.q:
		case <-pe.sock.DrainChannel():
			return
		}

		if err := pe.ep.SendMsg(m); err != nil {
			m.Free()
			return
		}
	}
}

// Top sender.
func (p *pub) sender() {
	for {
		var m *mangos.Message
		select {
		case m = <-p.sock.SendChannel():
		case <-p.sock.DrainChannel():
			return
		}

		p.Lock()
		for _, pe := range p.eps {
			m := m.Dup()
			select {
			case pe.q <- m:
			default:
				m.Free()
			}
		}
		p.Unlock()
	}
}

func (p *pub) AddEndpoint(ep mangos.Endpoint) {
	pe := &pubEp{ep: ep, sock: p.sock, q: make(chan *mangos.Message, 5)}
	p.Lock()
	p.eps[ep.GetID()] = pe
	p.Unlock()
	go pe.sender()
}

func (p *pub) RemoveEndpoint(ep mangos.Endpoint) {
	p.Lock()
	delete(p.eps, ep.GetID())
	p.Unlock()
}

func (*pub) Number() uint16 {
	return mangos.ProtoPub
}

func (*pub) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoSub {
		return true
	}
	return false
}

func (p *pub) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		p.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (p *pub) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return p.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PUB protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&pub{}), nil
}
