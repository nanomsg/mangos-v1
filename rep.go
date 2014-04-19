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

import (
	"sync"
)

// Rep is an implementation of the REP Protocol.
type rep struct {
	backtracebuf []byte
	backtrace    []byte
	xrep
	sync.Mutex
}

// Name implements the Protocol Name method.
func (*rep) Name() string {
	return RepName
}

// IsRaw implements the Protocol Raw method.
func (*rep) IsRaw() bool {
	return false
}

func (p *rep) Init(sock ProtocolSocket) {
	// alloc up some scratch space; we reuse this to avoid hitting up
	// the allocator frequently
	p.backtracebuf = make([]byte, 64)
	p.xrep.Init(sock)
}

// RecvHook implements the Protocol RecvHook Method.
// We save the backtrace from this message.  This means that if the app calls
// Recv before calling Send, the saved backtrace will be lost.  This is how
// the application discards / cancels a request to which it declines to reply.
func (p *rep) RecvHook(m *Message) bool {
	p.Lock()
	p.backtrace = append(p.backtracebuf[0:0], m.Header...)
	p.Unlock()
	m.Header = nil
	return true
}

// SendHook implements the Protocol SendHook Method.
func (p *rep) SendHook(m *Message) bool {
	// Store our saved backtrace.  Note that if none was previously stored,
	// there is no one to reply to, and we drop the message.
	p.Lock()
	m.Header = append(m.Header[0:0], p.backtrace...)
	p.backtrace = nil
	p.Unlock()
	if m.Header == nil {
		return false
	}
	return true
}

// Arguably, for REP, we should try harder to deliver the message to the
// recipient.  For now we layer on top of XREP which offers only minimal
// delivery guarantees.  Note that a REP/REQ pair should never encounter
// back pressure as the synchronous nature prevents such from occurring.
// The special consideration here is a DEVICE that may be an XREQ partner.
// In that case the partner should have a queue of at least depth one on
// each pipe, so that no single REP socket ever encounters backpressure.

type repFactory int

func (repFactory) NewProtocol() Protocol {
	return &rep{}
}

// RepFactory implements the Protocol Factory for the REP (reply) protocol.
var RepFactory repFactory
