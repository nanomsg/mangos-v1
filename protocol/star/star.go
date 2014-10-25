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
	"github.com/gdamore/mangos"
	"sync"
)

type starEp struct {
	ep mangos.Endpoint
	q  chan *mangos.Message
	x  *star
}

type star struct {
	sock mangos.ProtocolSocket
	sync.Mutex
	eps map[uint32]*starEp
	raw bool
}

func (x *star) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*starEp)
	go x.sender()
}

// Bottom sender.
func (pe *starEp) sender() {
	for {
		var msg *mangos.Message

		select {
		case msg = <-pe.q:
		case <-pe.x.sock.CloseChannel():
			return
		}

		if err := pe.ep.SendMsg(msg); err != nil {
			msg.Free()
			return
		}
	}
}

func (x *star) broadcast(msg *mangos.Message, sender *starEp) {

	x.Lock()
	for _, pe := range x.eps {
		if sender == pe {
			continue
		}
		msg := msg.Dup()
		select {
		case pe.q <- msg:
		default:
			// No room on outbound queue, drop it.
			msg.Free()
		}
	}
	x.Unlock()

	// Grab a local copy and send it up if we aren't originator
	if sender != nil {
		select {
		case x.sock.RecvChannel() <- msg:
		case <-x.sock.CloseChannel():
			return
		default:
			// No room, so we just drop it.
		}
	} else {
		// Not sending it up, so we need to release it.
		msg.Free()
	}
}

func (x *star) sender() {
	for {
		var msg *mangos.Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}
		x.broadcast(msg, nil)
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
	pe := &starEp{ep: ep, x: x, q: make(chan *mangos.Message, 5)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
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

func (*star) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoStar {
		return true
	}
	return false
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

// NewSocket allocates a new Socket using the STAR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&star{}), nil
}
