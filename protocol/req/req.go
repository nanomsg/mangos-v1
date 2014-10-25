// Copyright 2014 The Mangos Authors
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
	"github.com/gdamore/mangos"
	"encoding/binary"
	"sync"
	"time"
)

// req is an implementation of the req protocol.
type req struct {
	sock mangos.ProtocolSocket
	sync.Mutex
	eps    map[uint32]mangos.Endpoint
	resend chan *mangos.Message
	raw    bool
	retry  time.Duration
	nextid uint32
	waker  *time.Timer

	// fields describing the outstanding request
	reqmsg *mangos.Message
	reqid  uint32
}

func (r *req) Init(socket mangos.ProtocolSocket) {
	r.sock = socket
	r.eps = make(map[uint32]mangos.Endpoint)
	r.resend = make(chan *mangos.Message)

	r.nextid = uint32(time.Now().UnixNano()) // quasi-random
	r.retry = time.Minute * 1                // retry after a minute
	r.waker = time.NewTimer(r.retry)
	r.waker.Stop()
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
		case <-r.sock.DrainChannel():
			return
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

		select {
		case r.resend <- m:
			r.Lock()
			if r.retry > 0 {
				r.waker.Reset(r.retry)
			} else {
				r.waker.Stop()
			}
			r.Unlock()
		case <-r.sock.DrainChannel():
			continue
		}
	}
}

func (r *req) receiver(ep mangos.Endpoint) {
	for {
		m := ep.RecvMsg()
		if m == nil {
			return
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		select {
		case r.sock.RecvChannel() <- m:
		case <-r.sock.CloseChannel():
			m.Free()
			return
		}
	}
}

func (r *req) sender(ep mangos.Endpoint) {
	var m *mangos.Message

	for {

		select {
		case m = <-r.resend:
		case m = <-r.sock.SendChannel():
		case <-r.sock.DrainChannel():
			return
		}

		err := ep.SendMsg(m)
		if err != nil {
			select {
			case r.resend <- m:
			case <-r.sock.DrainChannel():
				m.Free()
			}
			return
		}
	}
}

func (*req) Number() uint16 {
	return mangos.ProtoReq
}

func (*req) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoRep {
		return true
	}
	return false
}

func (r *req) AddEndpoint(ep mangos.Endpoint) {
	r.Lock()
	r.eps[ep.GetID()] = ep
	r.Unlock()
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

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&req{}), nil
}
