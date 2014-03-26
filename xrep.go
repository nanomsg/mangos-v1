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

// xrep is an implementation of the XREP Protocol.
type xrep struct {
	sock ProtocolSocket
}

func (p *xrep) Init(sock ProtocolSocket) {
	p.sock = sock
}

func (p *xrep) Process() {

	sock := p.sock
	if msg := sock.PullDown(); msg != nil {
		// Lop off the 32-bit peer/pipe id.  If absent, drop it.
		if key, err := msg.getPipeKey(); err == nil {
			// Send it out.  If this fails (no such pipe, closed,
			// or busy) then just drop it.  Peer will retry.
			sock.SendToPipe(msg, key)
		}
	}

	// Check to see if we have a message ready to send up (receiver)
	if msg, key, err := sock.RecvAnyPipe(); err == nil && msg != nil {
		msg.putPipeKey(key)

		if msg.trimBackTrace() == nil {
			// Send it up.  If the application can't receive it
			// (pipe full, etc.) just drop it.   The peer will
			// resend.
			sock.PushUp(msg)
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

type xrepFactory int

func (xrepFactory) NewProtocol() Protocol {
	return new(xrep)
}

// XRepFactory implements the Protocol Factory for the XREP protocol.
// The XREP Protocol is the raw form of the REP (Reply) protocol.
var XRepFactory xrepFactory
