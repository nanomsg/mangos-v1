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
	"math/rand"
	"time"
)

// req is an implementation of the Req protocol.
type req struct {
	handle ProtocolHandle
	nextid uint32
	retry  time.Duration
	waker  *time.Timer
	xreq   Protocol

	// fields describing the outstanding request
	reqmsg  *Message
	reqid   uint32
	reqkey  PipeKey
	reqtime time.Time // when the next retry should be performed

	// Valid reply received.  This occurs only when the application
	// has backpressure above us.  We'll hold it for delivery
	// indefinitely, as long as the app doesn't send a new request.
	repmsg *Message
}

// Init implements the Protocol Init method.
func (p *req) Init(handle ProtocolHandle) {
	p.handle = handle
	p.nextid = rand.New(rand.NewSource(time.Now().UnixNano())).Uint32()
	// Look this up on the handle?
	p.retry = time.Minute * 1 // retry after a minute
	p.xreq = XReqFactory.NewProtocol()
	p.xreq.Init(handle)
}

// nextID returns the next request ID.
func (p *req) nextID() uint32 {
	// The high order bit is "special", and must always be set.  (This is
	// how the peer will detect the end of the backtrace.)
	v := p.nextid | 0x80000000
	p.nextid++
	return v
}

// cancel cancels any outstanding request, and resend timers.
func (p *req) cancel() {
	if p.waker != nil {
		p.waker.Stop()
		p.waker = nil
	}
}

// reschedule arranges for the existing request to be rescheduled for delivery
// after the configured resend time has passed.
func (p *req) reschedule() {
	if p.waker != nil {
		p.waker.Stop()
	}
	// If we don't get a reply, wake us up to resend.
	p.reqtime = time.Now().Add(p.retry)
	p.waker = time.AfterFunc(p.retry, func() {
		p.handle.WakeUp()
	})
}

// needresend returns true whenever either the timer has expired,
// or the pipe we sent it on has been closed.
func (p *req) needresend() bool {
	if p.reqmsg == nil {
		return false
	}
	if !time.Now().Before(p.reqtime) {
		return true
	}
	if !p.handle.IsOpen(p.reqkey) {
		return true
	}
	return false
}

// Process implements the Protocol Process method.
func (p *req) Process() {

	h := p.handle

	// Check to see if we have to retransmit our request.
	if p.needresend() {
		p.cancel() // stop timer for now
		if key, err := h.Send(p.reqmsg); err != nil {
			// No suitable pipes available for delivery.
			// Arrange to retry the next time we are called.
			// This usually happens in response to a connection
			// completing or backpressure easing.
			p.reqtime = time.Now()
		} else {
			// Successful delivery.  Note the pipe we sent it out
			// on, and schedule a longer time for resending.
			p.reqkey = key
			p.reschedule()
		}
	}

	// The rest of this looks like ordinary XReq handling.
	p.xreq.Process()
}

// Name implements the Protocol Name method.
func (*req) Name() string {
	return ReqName
}

// Number implements the Protocol Number method.
func (*req) Number() uint16 {
	return ProtoReq
}

// IsRaw implements the Protocol Raw method.
func (*req) IsRaw() bool {
	return false
}

// ValidPeer implements the Protocol ValidPeer method.
func (*req) ValidPeer(peer uint16) bool {
	if peer == ProtoRep {
		return true
	}
	return false
}

// SendHook implements the Protocol SendHook method.
func (p *req) SendHook(msg *Message) bool {

	// We only support a single outstanding request at a time.
	// If any other message was pending, cancel it.
	p.cancel()

	// We need to generate a new request id, and append it to the header.
	id := p.nextID()
	msg.putUint32(id)
	p.reqid = id
	p.reqmsg = msg

	// Schedule a retry, in case we don't get a reply.
	p.reschedule()

	return true
}

// RecvHook implements the Protocol RecvHook method.
func (p *req) RecvHook(msg *Message) bool {
	if p.reqmsg == nil {
		return false
	}
	if id, err := msg.getUint32(); err != nil || id != p.reqid {
		return false
	}
	p.cancel()
	p.reqmsg = nil
	p.reqid = 0
	return true
}

type reqFactory int

func (reqFactory) NewProtocol() Protocol {
	return new(req)
}

// ReqFactory implements the Protocol Factory for the REQ (request) protocol.
var ReqFactory reqFactory
