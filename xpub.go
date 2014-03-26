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

// xpub is an implementation of the XPub protocol.
type xpub struct {
	sock ProtocolSocket
}

// Init implements the Protocol Init method.
func (p *xpub) Init(sock ProtocolSocket) {
	p.sock = sock
}

// Process implements the Protocol Process method.
func (p *xpub) Process() {

	sock := p.sock
	if msg := sock.PullDown(); msg != nil {
		// Just send it to the world.  Clients will filter.
		sock.SendAllPipes(msg)
	}

	// Check to see if we have a message ready to send up (receiver)
	// We really don't expect to see any messages from subscribers!
	// Possibly we should log this.  nanomsg takes a rather severe
	// stance and asserts here.  We just silently drop.  But we do clear
	// the buffers this way.
	sock.RecvAnyPipe()
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
