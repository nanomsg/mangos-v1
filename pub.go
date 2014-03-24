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
type Pub struct {
	handle ProtocolHandle
	xpub   *XPub
}

// Init implements the Protocol Init method.
func (p *Pub) Init(handle ProtocolHandle) {
	p.handle = handle
	p.xpub = &XPub{}
}

// Process implements the Protocol Process method.
func (p *Pub) Process() {
	p.xpub.Process()
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*Pub) Name() string {
	return "Pub"
}

// Number implements the Protocol Number method.
func (*Pub) Number() uint16 {
	return ProtoPub
}

// IsRaw implements the Protocol IsRaw method.
func (*Pub) IsRaw() bool {
	return false
}

// ValidPeer implements the Protocol ValidPeer method.
func (p *Pub) ValidPeer(peer uint16) bool {
	return p.xpub.ValidPeer(peer)
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*Pub) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*Pub) SendHook(*Message) bool {
	return true
}
