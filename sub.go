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

// sub is an implementation of the Sub protocol.
type sub struct {
	xsub *xsub
}

// Init implements the Protocol Init method.
func (p *sub) Init(sock ProtocolSocket) {
	p.xsub = new(xsub)
	p.xsub.Init(sock)
}

// Process implements the Protocol Process method.
func (p *sub) Process() {
	p.xsub.Process()
}

// Name implements the Protocol Name method.  It returns "Sub".
func (*sub) Name() string {
	return SubName
}

// Number implements the Protocol Number method.
func (*sub) Number() uint16 {
	return ProtoSub
}

// IsRaw implements the Protocol IsRaw method.
func (*sub) IsRaw() bool {
	return false
}

// ValidPeer implements the Protocol ValidPeer method.
func (p *sub) ValidPeer(peer uint16) bool {
	return p.xsub.ValidPeer(peer)
}

const (
	// SubOptionSubscribe is the name of the subscribe option.
	SubOptionSubscribe = "SUB.SUBSCRIBE"

	// SubOptionUnsubscribe is the name of the unsubscribe option
	SubOptionUnsubscribe = "SUB.UNSUBSCRIBE"
)

// SetOption implements the ProtocolSetOptionHandler SetOption method.
func (p *sub) SetOption(name string, value interface{}) error {
	switch {
	case name == SubOptionSubscribe:
		return p.xsub.SetOption(XSubOptionSubscribe, value)
	case name == SubOptionUnsubscribe:
		return p.xsub.SetOption(XSubOptionUnsubscribe, value)
	}
	return ErrBadOption
}

type subFactory int

func (subFactory) NewProtocol() Protocol {
	return new(sub)
}

// SubFactory implements the Protocol Factory for the SUB (Subscribe) protocol.
var SubFactory subFactory
