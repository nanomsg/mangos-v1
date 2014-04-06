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
	"sync"
	"time"
)

// req is an implementation of the Req protocol.
type req struct {
	xreq
	sync.Mutex
	sock   ProtocolSocket
	nextid uint32
	retry  time.Duration
	waker  *time.Timer

	// fields describing the outstanding request
	reqmsg  *Message
	reqid   uint32
	reqep   Endpoint
	reqtime time.Time // when the next retry should be performed
}

// Init implements the Protocol Init method.
func (p *req) Init(sock ProtocolSocket) {
	p.sock = sock
	p.nextid = rand.New(rand.NewSource(time.Now().UnixNano())).Uint32()
	p.retry = time.Minute * 1 // retry after a minute
	//p.retry = time.Millisecond * 100 // retry after a minute

	p.xreq.Init(sock)
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

func (p *req) resend() {
	p.Lock()
	defer p.Unlock()

	if p.reqmsg == nil {
		return
	}

	if time.Now().Before(p.reqtime) {
		return
	}

	ep := p.sock.NextSendEndpoint()
	if ep != nil {
		// If we're resending a message, we shouldn't need
		// to worry about flow control, because there should
		// only be a single outstanding message at a time!
		p.reqmsg.AddRef()
		if ep.SendMsg(p.reqmsg) != nil {
			p.reqmsg.DecRef()
		}
		p.reqtime = time.Now().Add(p.retry)
		go p.reschedule()
	}
}

// reschedule arranges for the existing request to be rescheduled for delivery
// after the configured resend time has passed.
func (p *req) reschedule() {
	p.Lock()
	if p.waker != nil {
		p.waker.Stop()
	}
	// If we don't get a reply, wake us up to resend.
	//p.reqtime = time.Now().Add(p.retry)
	p.waker = time.AfterFunc(p.retry, p.resend)
	p.Unlock()
}

// needresend returns true whenever either the timer has expired,
// or the pipe we sent it on has been closed.
func (p *req) xneedresend() bool {
	if p.reqmsg == nil {
		return false
	}
	if !time.Now().Before(p.reqtime) {
		return true
	}
	if p.reqep == nil {
		return true
	}
	return false
}

func (p *req) ProcessSend() {

	p.resend()
	p.xreq.ProcessSend()
}

func (*req) Name() string {
	return ReqName
}

func (*req) Number() uint16 {
	return ProtoReq
}

func (*req) IsRaw() bool {
	return false
}

func (*req) ValidPeer(peer uint16) bool {
	if peer == ProtoRep {
		return true
	}
	return false
}

func (p *req) SendHook(msg *Message) bool {

	p.Lock()
	defer p.Unlock()

	// We only support a single outstanding request at a time.
	// If any other message was pending, cancel it.
	p.cancel()

	// We need to generate a new request id, and append it to the header.
	p.reqid = p.nextID()
	msg.putUint32(p.reqid)
	msg.AddRef()
	p.reqmsg = msg

	// Schedule a retry, in case we don't get a reply.
	p.reqtime = time.Now().Add(p.retry)
	go p.reschedule()

	return true
}

func (p *req) RecvHook(msg *Message) bool {
	p.Lock()
	defer p.Unlock()

	if p.reqmsg == nil {
		return false
	}
	if id, err := msg.getUint32(); err != nil || id != p.reqid {
		return false
	}
	p.cancel()
	p.reqmsg.Free()
	p.reqmsg = nil
	return true
}

type reqFactory int

func (reqFactory) NewProtocol() Protocol {
	return new(req)
}

// ReqFactory implements the Protocol Factory for the REQ (request) protocol.
var ReqFactory reqFactory
