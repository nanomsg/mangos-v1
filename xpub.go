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

// xpub is an implementation of the XPub protocol.
type xpub struct {
	sock ProtocolSocket
	sync.Mutex
	eps map[uint32]Endpoint
}

// Init implements the Protocol Init method.
func (p *xpub) Init(sock ProtocolSocket) {
	p.sock = sock
	p.eps = make(map[uint32]Endpoint)
}

func (x *xpub) ProcessSend() {

	for {
		msg := x.sock.PullDown()
		if msg == nil {
			return
		}
		x.Lock()
		for _, ep := range x.eps {
			msg.AddRef()
			if ep.SendMsg(msg) != nil {
				msg.DecRef()
			}
		}
		x.Unlock()
		msg.Free()
	}
}

func (x *xpub) ProcessRecv() {
}

func (x *xpub) SendHook(m *Message) bool {
	return true
}

func (x *xpub) AddEndpoint(ep Endpoint) {
	x.Lock()
	x.eps[ep.GetID()] = ep
	x.Unlock()
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
	return new(xpub)
}

// XPubFactory implements the Protocol Factory for the XPUB protocol.
// The XPUB Protocol is the raw form of the PUB (Publish) protocol.
var XPubFactory xpubFactory
