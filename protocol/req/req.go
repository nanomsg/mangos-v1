// Copyright 2015 The Mangos Authors
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

// Package req implements the REQ protocol, which is the request side of
// the request/response pattern.  (REP is the reponse.)
package req

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

// req is an implementation of the req protocol.
type req struct {
	sync.Mutex
	sock    mangos.ProtocolSocket
	eps     map[uint32]mangos.Endpoint
	resend  chan *mangos.Message
	raw     bool
	retry   time.Duration
	nextid  uint32
	waker   *time.Timer
	senders mangos.Waiter

	// fields describing the outstanding request
	reqmsg *mangos.Message
	reqid  uint32
}

func (r *req) Init(socket mangos.ProtocolSocket) {
	r.sock = socket
	r.eps = make(map[uint32]mangos.Endpoint)
	r.resend = make(chan *mangos.Message)
	r.senders.Init()

	r.nextid = uint32(time.Now().UnixNano()) // quasi-random
	r.retry = time.Minute * 1                // retry after a minute
	r.waker = time.NewTimer(r.retry)
	r.waker.Stop()
}

func (r *req) Shutdown(linger time.Duration) {
	r.senders.WaitRelTimeout(linger)
}

// nextID returns the next request ID.
func (r *req) nextID() uint32 {
	// The high order bit is "special", and must always be set.  (This is
	// how the peer will detect the end of the backtrace.)
	v := r.nextid | 0x80000000
	r.nextid++
	return v
}

// resend sends the request message again, after a timer has expired.
func (r *req) resender() {

	for {
		select {
		case <-r.waker.C:
		}

		r.Lock()
		m := r.reqmsg
		if m == nil {
			r.Unlock()
			continue
		}
		m = m.Dup()
		r.Unlock()

		r.resend <- m
		r.Lock()
		if r.retry > 0 {
			r.waker.Reset(r.retry)
		} else {
			r.waker.Stop()
		}
		r.Unlock()
	}
}

func (r *req) receiver(ep mangos.Endpoint) {
	rq := r.sock.RecvChannel()
	cq := r.sock.CloseChannel()

	for {
		m := ep.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		select {
		case rq <- m:
		case <-cq:
			m.Free()
			break
		}
	}
}

func (r *req) sender(ep mangos.Endpoint) {

	sq := r.sock.SendChannel()

	for {
		var m *mangos.Message

		select {
		case m = <-r.resend:
		case m = <-sq:
		}
		if m == nil {
			break
		}

		if ep.SendMsg(m) != nil {
			r.resend <- m
			break
		}
	}

	r.senders.Done()
}

func (*req) Number() uint16 {
	return mangos.ProtoReq
}

func (*req) PeerNumber() uint16 {
	return mangos.ProtoRep
}

func (*req) Name() string {
	return "req"
}

func (*req) PeerName() string {
	return "rep"
}

func (r *req) AddEndpoint(ep mangos.Endpoint) {
	r.Lock()
	r.eps[ep.GetID()] = ep
	r.Unlock()
	r.senders.Add()
	go r.receiver(ep)
	go r.sender(ep)
}

func (*req) RemoveEndpoint(mangos.Endpoint) {}

func (r *req) SendHook(m *mangos.Message) bool {

	if r.raw {
		// Raw mode has no automatic retry, and must include the
		// request id in the header coming down.
		return true
	}
	r.Lock()
	defer r.Unlock()

	// We need to generate a new request id, and append it to the header.
	r.reqid = r.nextID()
	v := r.reqid
	m.Header = append(m.Header,
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

	r.reqmsg = m.Dup()

	// Schedule a retry, in case we don't get a reply.
	if r.retry > 0 {
		r.waker.Reset(r.retry)
	} else {
		r.waker.Stop()
	}

	return true
}

func (r *req) RecvHook(m *mangos.Message) bool {
	if r.raw {
		// Raw mode just passes up messages unmolested.
		return true
	}
	r.Lock()
	defer r.Unlock()
	if len(m.Header) < 4 {
		return false
	}
	if r.reqmsg == nil {
		return false
	}
	if binary.BigEndian.Uint32(m.Header) != r.reqid {
		return false
	}
	r.waker.Stop()
	r.reqmsg.Free()
	r.reqmsg = nil
	return true
}

func (r *req) SetOption(option string, value interface{}) error {
	switch option {
	case mangos.OptionRaw:
		r.raw = true
		return nil
	case mangos.OptionRetryTime:
		r.Lock()
		r.retry = value.(time.Duration)
		r.Unlock()
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (r *req) GetOption(option string) (interface{}, error) {
	switch option {
	case mangos.OptionRaw:
		return r.raw, nil
	case mangos.OptionRetryTime:
		r.Lock()
		v := r.retry
		r.Unlock()
		return v, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewReq returns a new REQ protocol object.
func NewProtocol() mangos.Protocol {
	return &req{}
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&req{}), nil
}
