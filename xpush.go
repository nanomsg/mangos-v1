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

type xpush struct {
	sock ProtocolSocket
}

func (x *xpush) Init(sock ProtocolSocket) {
	x.sock = sock
}

func (x *xpush) sender(ep Endpoint) {
	var msg *Message

	for {
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		err := ep.SendMsg(msg)
		if err != nil {
			select {
			case <-x.sock.CloseChannel():
				msg.Free()
			}
			return
		}
	}
}

func (*xpush) Name() string {
	return XPushName
}

func (*xpush) Number() uint16 {
	return ProtoPush
}

func (*xpush) IsRaw() bool {
	return true
}

func (*xpush) ValidPeer(peer uint16) bool {
	if peer == ProtoPull {
		return true
	}
	return false
}

func (x *xpush) AddEndpoint(ep Endpoint) {
	go x.sender(ep)
}

func (x *xpush) RemEndpoint(ep Endpoint) {}

type xpushFactory int

func (xpushFactory) NewProtocol() Protocol {
	return &xpush{}
}

// XPushFactory implements the Protocol Factory for the XPUSH protocol.
// The XPUSH Protocol is the raw form of the PUSH (Push) protocol.
var XPushFactory xpushFactory
