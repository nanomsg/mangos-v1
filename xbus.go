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

type busEp struct {
	ep Endpoint
	q  chan *Message
	x  *xbus
}

type xbus struct {
	sock ProtocolSocket
	sync.Mutex
	eps map[uint32]*busEp
}

// Init implements the Protocol Init method.
func (x *xbus) Init(sock ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*busEp)
	go x.sender()
}

// Bottom sender.
func (pe *busEp) sender() {
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

func (x *xbus) broadcast(msg *Message, sender *busEp) {

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

func (x *xbus) sender() {
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

func (pe *busEp) receiver() {
	for {
		msg := pe.ep.RecvMsg()
		if msg == nil {
			return
		}

		pe.x.broadcast(msg, pe)
	}
}

func (x *xbus) AddEndpoint(ep Endpoint) {
	pe := &busEp{ep: ep, x: x, q: make(chan *Message, 5)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go pe.sender()
	go pe.receiver()
}

func (x *xbus) RemEndpoint(ep Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

func (*xbus) Name() string {
	return XBusName
}

func (*xbus) Number() uint16 {
	return ProtoBus
}

func (*xbus) IsRaw() bool {
	return true
}

func (*xbus) ValidPeer(peer uint16) bool {
	if peer == ProtoBus {
		return true
	}
	return false
}

type xbusFactory int

func (xbusFactory) NewProtocol() Protocol {
	return &xbus{}
}

// XBusFactory implements the Protocol Factory for the XBUS protocol.
// The XBUS Protocol is the raw form of the BUS protocol.
var XBusFactory xbusFactory
