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
	reqmsg *Message
	reqid  uint32
}

// Init implements the Protocol Init method.
func (p *req) Init(sock ProtocolSocket) {
	p.sock = sock
	p.nextid = rand.New(rand.NewSource(time.Now().UnixNano())).Uint32()
	p.retry = time.Minute * 1 // retry after a minute
	p.waker = time.NewTimer(p.retry)
	p.waker.Stop()

	p.xreq.Init(sock)
	go p.resender()
}

// nextID returns the next request ID.
func (p *req) nextID() uint32 {
	// The high order bit is "special", and must always be set.  (This is
	// how the peer will detect the end of the backtrace.)
	v := p.nextid | 0x80000000
	p.nextid++
	return v
}

// resend sends the request message again, after a timer has expired.
// It makes use of the underlying resend channel on the xreq.
func (p *req) resender() {

	for {
		select {
		case <-p.sock.CloseChannel():
			return
		case <-p.waker.C:
			debugf("RESEND!")
		}

		p.Lock()
		msg := p.reqmsg
		if msg == nil {
			p.Unlock()
			continue
		}
		msg = msg.Dup()
		p.Unlock()

		select {
		case p.xreq.resend <- msg:
			p.Lock()
			p.waker.Reset(p.retry)
			p.Unlock()
		case <-p.sock.CloseChannel():
			continue
		}
	}
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

	// We need to generate a new request id, and append it to the header.
	p.reqid = p.nextID()
	msg.putUint32(p.reqid)
	p.reqmsg = msg.Dup()

	// Schedule a retry, in case we don't get a reply.
	p.waker.Reset(p.retry)

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
	p.waker.Stop()
	p.reqmsg.Free()
	p.reqmsg = nil
	return true
}

type reqFactory int

func (reqFactory) NewProtocol() Protocol {
	return &req{}
}

// ReqFactory implements the Protocol Factory for the REQ (request) protocol.
var ReqFactory reqFactory
