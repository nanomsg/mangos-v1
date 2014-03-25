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
	handle ProtocolHandle
}

// Init implements the Protocol Init method.
func (p *xrep) Init(handle ProtocolHandle) {
	p.handle = handle
}

// Process implements the Protocol Process method.
func (p *xrep) Process() {

	h := p.handle
	if msg := h.PullDown(); msg != nil {
		// Lop off the 32-bit peer/pipe id.  If absent, drop it.
		if key, err := msg.getPipeKey(); err == nil {
			// Send it out.  If this fails (no such pipe, closed,
			// or busy) then just drop it.  Peer will retry.
			h.SendTo(msg, key)
		}
	}

	// Check to see if we have a message ready to send up (receiver)
	if msg, key, err := h.Recv(); err == nil && msg != nil {
		msg.putPipeKey(key)
		if msg.trimBackTrace() == nil {
			// Send it up.  If the application can't receive it
			// (pipe full, etc.) just drop it.   The peer will
			// resend.
			h.PushUp(msg)
		}
	}
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*xrep) Name() string {
	return XRepName
}

// Number implements the Protocol Number method.
func (*xrep) Number() uint16 {
	return ProtoRep
}

// IsRaw implements the Protocol IsRaw method.
func (*xrep) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*xrep) ValidPeer(peer uint16) bool {
	if peer == ProtoReq {
		return true
	}
	return false
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*xrep) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*xrep) SendHook(*Message) bool {
	return true
}

type xrepFactory int

func (xrepFactory) NewProtocol() Protocol {
	return new(xrep)
}

var XRepFactory xrepFactory
