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

// xreq is an implementation of the XREQ protocol.
type xreq struct {
	sock ProtocolSocket
	sync.Mutex
	eps    map[uint32]Endpoint
	resend chan *Message
}

func (x *xreq) Init(socket ProtocolSocket) {
	x.sock = socket
	x.eps = make(map[uint32]Endpoint)
	x.resend = make(chan *Message)
}

func (x *xreq) receiver(ep Endpoint) {
	for {
		msg := ep.RecvMsg()
		if msg == nil {
			return
		}

		if msg.trimUint32() != nil {
			msg.Free()
			continue
		}

		select {
		case x.sock.RecvChannel() <- msg:
		case <-x.sock.CloseChannel():
			msg.Free()
			return
		}
	}
}

func (x *xreq) sender(ep Endpoint) {
	var msg *Message

	for {

		select {
		case msg = <-x.resend:
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		err := ep.SendMsg(msg)
		if err != nil {
			select {
			case x.resend <- msg:
			case <-x.sock.CloseChannel():
				msg.Free()
			}
			return
		}
	}
}

func (*xreq) Name() string {
	return XReqName
}

func (*xreq) Number() uint16 {
	return ProtoReq
}

func (*xreq) IsRaw() bool {
	return true
}

func (*xreq) ValidPeer(peer uint16) bool {
	if peer == ProtoRep {
		return true
	}
	return false
}

func (x *xreq) AddEndpoint(ep Endpoint) {
	x.Lock()
	x.eps[ep.GetID()] = ep
	x.Unlock()
	go x.receiver(ep)
	go x.sender(ep)
}

func (*xreq) RemEndpoint(Endpoint) {}

type xreqFactory int

func (xreqFactory) NewProtocol() Protocol {
	return &xreq{}
}

// XReqFactory implements the Protocol Factory for the XREQ protocol.
// The XREQ Protocol is the raw form of the REQ (Request) protocol.
var XReqFactory xreqFactory
