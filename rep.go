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

import ()

// Rep is an implementation of the REP Protocol.
type rep struct {
	handle ProtocolHandle

	backtrace []byte
	key       PipeKey // pipe we got the request on
	xrep      Protocol
}

// Init implements the Protocol Init method.
func (p *rep) Init(handle ProtocolHandle) {
	p.handle = handle
	p.xrep = XRepFactory.NewProtocol()
	p.xrep.Init(handle)
}

// Process implements the Protocol Process method.
func (p *rep) Process() {
	p.xrep.Process()
}

// Name implements the Protocol Name method.
func (*rep) Name() string {
	return RepName
}

// Number implements the Protocol Number method.
func (*rep) Number() uint16 {
	return ProtoReq
}

// IsRaw implements the Protocol Raw method.
func (*rep) IsRaw() bool {
	return false
}

// ValidPeer implements the Protocol ValidPeer method.
func (*rep) ValidPeer(peer uint16) bool {
	if peer == ProtoReq {
		return true
	}
	return false
}

// RecvHook implements the Protocol RecvHook Method.
// We save the backtrace from this message.  This means that if the app calls
// Recv before calling Send, the saved backtrace will be lost.  This is how
// the application discards / cancels a request to which it declines to reply.
func (p *rep) RecvHook(m *Message) bool {
	p.backtrace = m.Header
	m.Header = nil
	return true
}

// SendHook implements the Protocol SendHook Method.
func (p *rep) SendHook(m *Message) bool {
	// Store our saved backtrace.  Note that if none was previously stored,
	// there is no one to reply to, and we drop the message.
	m.Header = p.backtrace
	p.backtrace = nil
	if m.Header == nil {
		return false
	}
	return true
}

type repFactory int

func (repFactory) NewProtocol() Protocol {
	return new(rep)
}

// RepFactory implements the Protocol Factory for the REP (reply) protocol.
var RepFactory repFactory
