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
type XPub struct {
	handle ProtocolHandle
}

// Init implements the Protocol Init method.
func (p *XPub) Init(handle ProtocolHandle) {
	p.handle = handle
}

// Process implements the Protocol Process method.
func (p *XPub) Process() {

	h := p.handle
	if msg := h.PullDown(); msg != nil {
		// Just send it to the world.  Clients will filter.
		h.SendAll(msg)
	}

	// Check to see if we have a message ready to send up (receiver)
	// We really don't expect to see any messages from subscribers!
	// Possibly we should log this.  nanomsg takesa rather ... severe
	// stance and asserts here.  We just silently drop.  But we do clear
	// the buffers this way.
	h.Recv()
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*XPub) Name() string {
	return "XPub"
}

// Number implements the Protocol Number method.
func (*XPub) Number() uint16 {
	return ProtoPub
}

// IsRaw implements the Protocol IsRaw method.
func (*XPub) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*XPub) ValidPeer(peer uint16) bool {
	if peer == ProtoSub {
		return true
	}
	return false
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*XPub) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*XPub) SendHook(*Message) bool {
	return true
}
