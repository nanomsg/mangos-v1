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

type pubEp struct {
	ep   Endpoint
	q    chan *Message
	sock ProtocolSocket
}

// xpub is an implementation of the XPub protocol.
type xpub struct {
	sock ProtocolSocket
	sync.Mutex
	eps map[uint32]*pubEp
}

// Init implements the Protocol Init method.
func (x *xpub) Init(sock ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*pubEp)
	go x.sender()
}

// Bottom sender.
func (pe *pubEp) sender() {
	for {
		var msg *Message

		select {
		case msg = <-pe.q:
		case <-pe.sock.CloseChannel():
			return
		}

		if err := pe.ep.SendMsg(msg); err != nil {
			msg.Free()
			return
		}
	}
}

// Top sender.
func (x *xpub) sender() {
	for {
		var msg *Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		x.Lock()
		for _, pe := range x.eps {
			msg := msg.Dup()
			select {
			case pe.q <- msg:
			default:
				msg.Free()
			}
		}
		x.Unlock()
	}
}

func (x *xpub) SendHook(m *Message) bool {
	return true
}

func (x *xpub) AddEndpoint(ep Endpoint) {
	pe := &pubEp{ep: ep, sock: x.sock, q: make(chan *Message)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go pe.sender()
}

func (x *xpub) RemEndpoint(ep Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*xpub) Name() string {
	return XPubName
}

// Number implements the Protocol Number method.
func (*xpub) Number() uint16 {
	return ProtoPub
}

// IsRaw implements the Protocol IsRaw method.
func (*xpub) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*xpub) ValidPeer(peer uint16) bool {
	if peer == ProtoSub {
		return true
	}
	return false
}

type xpubFactory int

func (xpubFactory) NewProtocol() Protocol {
	return &xpub{}
}

// XPubFactory implements the Protocol Factory for the XPUB protocol.
// The XPUB Protocol is the raw form of the PUB (Publish) protocol.
var XPubFactory xpubFactory
