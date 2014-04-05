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

// xrep is an implementation of the XREP Protocol.
type xpair struct {
	sock   ProtocolSocket
	peer   Endpoint
	sndmsg *Message // Pending message for outbound delivery
	rcvmsg *Message // Pending message for inbound delivery
	sndlk  sync.Mutex
	rcvlk  sync.Mutex
	sync.Mutex
}

// Init implements the Protocol Init method.
func (x *xpair) Init(sock ProtocolSocket) {
	x.sock = sock
}

func (x *xpair) Process() {
	x.ProcessRecv()
	x.ProcessSend()
}

func (x *xpair) ProcessSend() {
	x.sndlk.Lock()
	defer x.sndlk.Unlock()
	for {
		msg := x.sndmsg
		x.sndmsg = nil
		if msg == nil {
			msg = x.sock.PullDown()
		}
		if msg == nil {
			return
		}
		ep := x.sock.NextSendEndpoint()
		if ep == nil {
			x.sndmsg = msg
			return
		}
		switch ep.SendMsg(msg) {
		case nil:
		case ErrPipeFull:
			x.sndmsg = msg
			return
		default:
			msg.Recycle()
			continue
		}
	}
}

func (x *xpair) ProcessRecv() {
	x.rcvlk.Lock()
	defer x.rcvlk.Unlock()
	for {
		msg := x.rcvmsg
		x.rcvmsg = nil
		if msg == nil {
			ep := x.sock.NextRecvEndpoint()
			if ep == nil {
				return
			}
			msg = ep.RecvMsg()
		}
		if msg == nil {
			return
		}
		if !x.sock.PushUp(msg) {
			x.rcvmsg = msg
			return
		}
	}
}

func (x *xpair) AddEndpoint(ep Endpoint) {
	x.Lock()
	if x.peer != nil {
		x.Unlock()
		ep.Close()
		return
	}
	x.peer = ep
	x.Unlock()
}

func (x *xpair) RemEndpoint(ep Endpoint) {
	x.Lock()
	if x.peer == ep {
		x.peer = nil
	}
	x.Unlock()
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*xpair) Name() string {
	return XPairName
}

// Number implements the Protocol Number method.
func (*xpair) Number() uint16 {
	return ProtoPair
}

// IsRaw implements the Protocol IsRaw method.
func (*xpair) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*xpair) ValidPeer(peer uint16) bool {
	if peer == ProtoPair {
		return true
	}
	return false
}

type xpairFactory int

func (xpairFactory) NewProtocol() Protocol {
	return new(xpair)
}

// XPairFactory implements the Protocol Factory for the XPAIR protocol.
// The XPAIR Protocol is the raw form of the PAIR (Pair) protocol.
var XPairFactory xpairFactory
