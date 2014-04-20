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
	"time"
)

// xreq is an implementation of the XREQ protocol.
type xreq struct {
	sock ProtocolSocket
	sync.Mutex
	eps    map[uint32]Endpoint
	resend chan *Message
	raw    bool
	retry  time.Duration
	nextid uint32
	waker  *time.Timer

	// fields describing the outstanding request
	reqmsg *Message
	reqid  uint32
}

func (x *xreq) Init(socket ProtocolSocket) {
	x.sock = socket
	x.eps = make(map[uint32]Endpoint)
	x.resend = make(chan *Message)

	x.nextid = uint32(time.Now().UnixNano()) // quasi-random
	x.retry = time.Minute * 1                // retry after a minute
	x.waker = time.NewTimer(x.retry)
	x.waker.Stop()
}

// nextID returns the next request ID.
func (x *xreq) nextID() uint32 {
	// The high order bit is "special", and must always be set.  (This is
	// how the peer will detect the end of the backtrace.)
	v := x.nextid | 0x80000000
	x.nextid++
	return v
}

// resend sends the request message again, after a timer has expired.
func (x *xreq) resender() {

	for {
		select {
		case <-x.sock.CloseChannel():
			return
		case <-x.waker.C:
		}

		x.Lock()
		msg := x.reqmsg
		if msg == nil {
			x.Unlock()
			continue
		}
		msg = msg.Dup()
		x.Unlock()

		select {
		case x.resend <- msg:
			x.Lock()
			if x.retry > 0 {
				x.waker.Reset(x.retry)
			} else {
				x.waker.Stop()
			}
			x.Unlock()
		case <-x.sock.CloseChannel():
			continue
		}
	}
}

func (x *xreq) receiver(ep Endpoint) {
	for {
		msg := ep.RecvMsg()
		if msg == nil {
			return
		}

		if msg.trimUint32() != nil {
			msg.Free()
			continue
		}

		select {
		case x.sock.RecvChannel() <- msg:
		case <-x.sock.CloseChannel():
			msg.Free()
			return
		}
	}
}

func (x *xreq) sender(ep Endpoint) {
	var msg *Message

	for {

		select {
		case msg = <-x.resend:
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		err := ep.SendMsg(msg)
		if err != nil {
			select {
			case x.resend <- msg:
			case <-x.sock.CloseChannel():
				msg.Free()
			}
			return
		}
	}
}

func (*xreq) Number() uint16 {
	return ProtoReq
}

func (*xreq) ValidPeer(peer uint16) bool {
	if peer == ProtoRep {
		return true
	}
	return false
}

func (x *xreq) AddEndpoint(ep Endpoint) {
	x.Lock()
	x.eps[ep.GetID()] = ep
	x.Unlock()
	go x.receiver(ep)
	go x.sender(ep)
}

func (*xreq) RemEndpoint(Endpoint) {}

func (x *xreq) SendHook(msg *Message) bool {

	if x.raw {
		// Raw mode has no automatic retry, and must include the
		// request id in the header coming down.
		return true
	}
	x.Lock()
	defer x.Unlock()

	// We need to generate a new request id, and append it to the header.
	x.reqid = x.nextID()
	msg.putUint32(x.reqid)
	x.reqmsg = msg.Dup()

	// Schedule a retry, in case we don't get a reply.
	if x.retry > 0 {
		x.waker.Reset(x.retry)
	} else {
		x.waker.Stop()
	}

	return true
}

func (x *xreq) RecvHook(msg *Message) bool {
	if x.raw {
		// Raw mode just passes up messages unmolested.
		return true
	}
	x.Lock()
	defer x.Unlock()

	if x.reqmsg == nil {
		return false
	}
	if id, err := msg.getUint32(); err != nil || id != x.reqid {
		return false
	}
	x.waker.Stop()
	x.reqmsg.Free()
	x.reqmsg = nil
	return true
}

func (x *xreq) SetOption(option string, value interface{}) error {
	switch option {
	case OptionRaw:
		x.raw = true
		return nil
	case OptionRetryTime:
		x.Lock()
		x.retry = value.(time.Duration)
		x.Unlock()
		return nil
	default:
		return ErrBadOption
	}
}

func (x *xreq) GetOption(option string) (interface{}, error) {
	switch option {
	case OptionRaw:
		return x.raw, nil
	case OptionRetryTime:
		x.Lock()
		v := x.retry
		x.Unlock()
		return v, nil
	default:
		return nil, ErrBadOption
	}
}

type reqFactory int

func (reqFactory) NewProtocol() Protocol {
	return &xreq{}
}

// XReqFactory implements the Protocol Factory for the XREQ protocol.
// The XREQ Protocol is the raw form of the REQ (Request) protocol.
var ReqFactory reqFactory
