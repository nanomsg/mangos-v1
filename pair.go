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

// pair is an implementation of the Pair protocol.
type pair struct {
	handle ProtocolHandle
	xpair  *xpair
}

// Init implements the Protocol Init method.
func (p *pair) Init(handle ProtocolHandle) {
	p.handle = handle
	p.xpair = new(xpair)
	p.xpair.Init(handle)
}

// Process implements the Protocol Process method.
func (p *pair) Process() {
	p.xpair.Process()
}

// Name implements the Protocol Name method.  It returns "Sub".
func (*pair) Name() string {
	return PairName
}

// Number implements the Protocol Number method.
func (*pair) Number() uint16 {
	return ProtoPair
}

// IsRaw implements the Protocol IsRaw method.
func (*pair) IsRaw() bool {
	return false
}

// ValidPeer implements the Protocol ValidPeer method.
func (p *pair) ValidPeer(peer uint16) bool {
	return p.xpair.ValidPeer(peer)
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*pair) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*pair) SendHook(*Message) bool {
	return true
}

type pairFactory int

func (pairFactory) NewProtocol() Protocol {
	return new(pair)
}

// PairFactory implements the Protocol Factory for the PAIR (Pair) protocol.
var PairFactory pairFactory
