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

// XRep is an implementation of the XRep protocol.
type XRep struct {
	handle ProtocolHandle
}

// Init implements the Protocol Init method.
func (p *XRep) Init(handle ProtocolHandle) {
	p.handle = handle
}

// Process implements the Protocol Process method.
func (p *XRep) Process() {

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
func (*XRep) Name() string {
	return "XRep"
}

// Number implements the Protocol Number method.
func (*XRep) Number() uint16 {
	return ProtoRep
}

// IsRaw implements the Protocol IsRaw method.
func (*XRep) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*XRep) ValidPeer(peer uint16) bool {
	if peer == ProtoReq {
		return true
	}
	return false
}

// SetOption implements the Protocol SetOption method.  We have no options.
func (*XRep) SetOption(name string, val interface{}) error {
	return EBadOption
}

// GetOption implements the Protocol GetOption method.  We have no options.
func (*XRep) GetOption(name string) (interface{}, error) {
	return nil, EBadOption
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*XRep) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*XRep) SendHook(*Message) bool {
	return true
}
