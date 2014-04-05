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
type xrep struct {
	sock ProtocolSocket
	eps  map[uint32]Endpoint
	sync.Mutex
}

func (x *xrep) Init(sock ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]Endpoint)
}

func (x *xrep) Process() {
	x.ProcessRecv()
	x.ProcessSend()
}

func (x *xrep) ProcessSend() {
	var msg *Message
	sock := x.sock

	for {
		if msg = sock.PullDown(); msg == nil {
			break
		}
		// Lop off the 32-bit peer/pipe id.  If absent, drop it.
		if id, err := msg.getUint32(); err == nil {
			x.Lock()
			ep := x.eps[id]
			x.Unlock()
			if ep != nil {
				ep.SendMsg(msg)
			}
		}
	}
}

func (x *xrep) ProcessRecv() {
	for {
		ep := x.sock.NextRecvEndpoint()
		if ep == nil {
			break
		}
		msg := ep.RecvMsg()
		if msg == nil {
			continue
		}
		msg.putUint32(ep.GetID())
		if msg.trimBackTrace() == nil {
			// Send it up.  If the application can't receive it
			// (pipe full, etc.) just drop it.   The peer will
			// resend.
			x.sock.PushUp(msg)
		}
	}
}

func (*xrep) Name() string {
	return XRepName
}

func (*xrep) Number() uint16 {
	return ProtoRep
}

func (*xrep) IsRaw() bool {
	return true
}

func (*xrep) ValidPeer(peer uint16) bool {
	if peer == ProtoReq {
		return true
	}
	return false
}

func (x *xrep) AddEndpoint(ep Endpoint) {
	x.Lock()
	x.eps[ep.GetID()] = ep
	x.Unlock()
}

func (x *xrep) RemEndpoint(ep Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

type xrepFactory int

func (xrepFactory) NewProtocol() Protocol {
	return new(xrep)
}

// XRepFactory implements the Protocol Factory for the XREP protocol.
// The XREP Protocol is the raw form of the REP (Reply) protocol.
var XRepFactory xrepFactory
