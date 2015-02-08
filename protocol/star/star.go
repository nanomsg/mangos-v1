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

// Package star implements a new, experimental protocol called "STAR".
// This is like the BUS protocol, except that each member of the network
// automatically forwards any message it receives to any other peers.
// In a star network, this means that all members should receive all messages,
// assuming that there is a central server.  Its important to ensure that
// the topology is free from cycles, as there is no protection against
// that, and cycles can lead to infinite message storms.  (TODO: Add a TTL,
// and basic message ID / anti-replay protection.)
package star

import (
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

type starEp struct {
	ep mangos.Endpoint
	q  chan *mangos.Message
	x  *star
}

type star struct {
	sock    mangos.ProtocolSocket
	eps     map[uint32]*starEp
	raw     bool
	senders mangos.Waiter

	sync.Mutex
}

func (x *star) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*starEp)
	x.senders.Init()
	go x.sender()
}

func (x *star) Shutdown(linger time.Duration) {
	x.senders.WaitRelTimeout(linger)
}

// Bottom sender.
func (pe *starEp) sender() {
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
	pe.x.senders.Done()
}

func (x *star) broadcast(m *mangos.Message, sender *starEp) {

	x.Lock()
	for _, pe := range x.eps {
		if sender == pe {
			continue
		}
		if m != nil {
			m = m.Dup()
		}
		select {
		case pe.q <- m:
		default:
			// No room on outbound queue, drop it.
			if m != nil {
				m.Free()
			}
		}
	}
	x.Unlock()

	// Grab a local copy and send it up if we aren't originator
	if m != nil {
		if sender != nil {
			select {
			case x.sock.RecvChannel() <- m:
			case <-x.sock.CloseChannel():
				m.Free()
				return
			default:
				// No room, so we just drop it.
				m.Free()
			}
		} else {
			// Not sending it up, so we need to release it.
			m.Free()
		}
	}
}

func (x *star) sender() {
	sq := x.sock.SendChannel()

	for {
		m := <-sq
		x.broadcast(m, nil)
		if m == nil {
			break
		}
	}
}

func (pe *starEp) receiver() {
	for {
		msg := pe.ep.RecvMsg()
		if msg == nil {
			return
		}

		if !pe.x.raw {
			pe.x.broadcast(msg, pe)
		}
	}
}

func (x *star) AddEndpoint(ep mangos.Endpoint) {
	depth := 16
	if i, err := x.sock.GetOption(mangos.OptionWriteQLen); err == nil {
		depth = i.(int)
	}
	pe := &starEp{ep: ep, x: x, q: make(chan *mangos.Message, depth)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	x.senders.Add()
	go pe.sender()
	go pe.receiver()
}

func (x *star) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

func (*star) Number() uint16 {
	return mangos.ProtoStar
}

func (*star) PeerNumber() uint16 {
	return mangos.ProtoStar
}

func (*star) Name() string {
	return "star"
}

func (*star) PeerName() string {
	return "star"
}

func (x *star) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *star) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewProtocol returns a new STAR protocol object.
func NewProtocol() mangos.Protocol {
	return &star{}
}

// NewSocket allocates a new Socket using the STAR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&star{}), nil
}
