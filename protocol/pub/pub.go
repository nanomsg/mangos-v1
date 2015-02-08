// Copyright 2015 The Mangos Authors
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
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

type pubEp struct {
	ep mangos.Endpoint
	q  chan *mangos.Message
	p  *pub
	w  mangos.Waiter
}

type pub struct {
	sock mangos.ProtocolSocket
	eps  map[uint32]*pubEp
	raw  bool
	w    mangos.Waiter

	sync.Mutex
}

func (p *pub) Init(sock mangos.ProtocolSocket) {
	p.sock = sock
	p.eps = make(map[uint32]*pubEp)
	p.w.Init()
	p.w.Add()
	go p.sender()
}

func (p *pub) Shutdown(linger time.Duration) {

	when := time.Now().Add(linger)
	tq := time.After(linger)

	p.w.WaitAbsTimeout(when)

	p.Lock()
	for _, pe := range p.eps {
		select {
		case pe.q <- nil:
		case <-tq:
			tq = time.After(0)
		}
		pe.w.WaitAbsTimeout(when)
	}
	p.Unlock()
}

// Bottom sender.
func (pe *pubEp) sender() {

	for {
		m := <-pe.q
		if m == nil {
			break
		}

		if pe.ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}

	pe.w.Done()
}

// Top sender.
func (p *pub) sender() {
	sq := p.sock.SendChannel()

	for {
		m := <-sq
		if m == nil {
			break
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
	p.w.Done()
}

func (pe *pubEp) receiver() {
	// In order for us to detect a dropped connection, we need to poll
	// on the socket.  We don't care about the results and discard them,
	// but this allows the disconnect to be noticed.  Note that we will
	// be blocked in this call forever, until the connection is dropped.
	pe.ep.RecvMsg()
}

func (p *pub) AddEndpoint(ep mangos.Endpoint) {
	depth := 16
	if i, err := p.sock.GetOption(mangos.OptionWriteQLen); err == nil {
		depth = i.(int)
	}
	pe := &pubEp{ep: ep, p: p, q: make(chan *mangos.Message, depth)}
	pe.w.Init()
	p.Lock()
	p.eps[ep.GetID()] = pe
	p.Unlock()

	pe.w.Add()
	go pe.sender()
	go pe.receiver()
}

func (p *pub) RemoveEndpoint(ep mangos.Endpoint) {
	p.Lock()
	delete(p.eps, ep.GetID())
	p.Unlock()
}

func (*pub) Number() uint16 {
	return mangos.ProtoPub
}

func (*pub) PeerNumber() uint16 {
	return mangos.ProtoSub
}

func (*pub) Name() string {
	return "pub"
}

func (*pub) PeerName() string {
	return "sub"
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

// NewProtocol returns a new PUB protocol object.
func NewProtocol() mangos.Protocol {
	return &pub{}
}

// NewSocket allocates a new Socket using the PUB protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&pub{}), nil
}
