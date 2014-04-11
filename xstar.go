// Copyright 2014 Garrett D'Amore
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

package sp

import (
	"sync"
)

type starEp struct {
	ep Endpoint
	q  chan *Message
	x  *xstar
}

type xstar struct {
	sock ProtocolSocket
	sync.Mutex
	eps    map[uint32]*starEp
	redist bool
}

func (x *xstar) Init(sock ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*starEp)
	go x.sender()
}

// Bottom sender.
func (pe *starEp) sender() {
	for {
		var msg *Message

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

func (x *xstar) broadcast(msg *Message, sender *starEp) {

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

func (x *xstar) sender() {
	for {
		var msg *Message
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

		if pe.x.redist {
			pe.x.broadcast(msg, pe)
		}
	}
}

func (x *xstar) AddEndpoint(ep Endpoint) {
	pe := &starEp{ep: ep, x: x, q: make(chan *Message, 5)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go pe.sender()
	go pe.receiver()
}

func (x *xstar) RemEndpoint(ep Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

func (*xstar) Name() string {
	return XStarName
}

func (*xstar) Number() uint16 {
	return ProtoStar
}

func (*xstar) IsRaw() bool {
	return true
}

func (*xstar) ValidPeer(peer uint16) bool {
	if peer == ProtoStar {
		return true
	}
	return false
}

type xstarFactory int

func (xstarFactory) NewProtocol() Protocol {
	return &xstar{}
}

// XStarFactory implements the Protocol Factory for the XSTAR protocol.
// The XSTAR Protocol is the raw form of the STAR protocol.
var XStarFactory xstarFactory
