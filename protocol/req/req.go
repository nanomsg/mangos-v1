// Copyright 2018 The Mangos Authors
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
// the request/response pattern.  (REP is the response.)
package req

import (
	"sync"
	"time"

	"nanomsg.org/go-mangos"
)

// req is an implementation of the req protocol.
type req struct {
	sync.Mutex
	sock mangos.ProtocolSocket
	eps  map[uint32]*reqEp
	w    mangos.Waiter
	init sync.Once

	// fields describing the outstanding request
	reqmsg *mangos.Message
	reqid  uint32
}

type reqEp struct {
	ep mangos.Endpoint
	cq chan struct{}
}

func (r *req) Init(socket mangos.ProtocolSocket) {
	r.sock = socket
	r.eps = make(map[uint32]*reqEp)
	r.w.Init()

	r.sock.SetRecvError(mangos.ErrProtoState)
}

func (r *req) Shutdown(expire time.Time) {
	r.w.WaitAbsTimeout(expire)
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

func (r *req) sender(pe *reqEp) {

	// NB: Because this function is only called when an endpoint is
	// added, we can reasonably safely cache the channels -- they won't
	// be changing after this point.

	defer r.w.Done()
	sq := r.sock.SendChannel()
	cq := r.sock.CloseChannel()

	for {
		var m *mangos.Message

		select {
		case m = <-sq:
		case <-cq:
			return
		case <-pe.cq:
			return
		}

		if pe.ep.SendMsg(m) != nil {
			m.Free()
		}
	}
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

	pe := &reqEp{cq: make(chan struct{}), ep: ep}
	r.Lock()
	r.eps[ep.GetID()] = pe

	r.Unlock()
	go r.receiver(ep)
	r.w.Add()
	go r.sender(pe)
}

func (r *req) RemoveEndpoint(ep mangos.Endpoint) {
	id := ep.GetID()
	r.Lock()
	pe := r.eps[id]
	delete(r.eps, id)
	r.Unlock()
	if pe != nil {
		close(pe.cq)
	}
}

func (r *req) SetOption(option string, value interface{}) error {
	switch option {
	default:
		return mangos.ErrBadOption
	}
}

func (r *req) GetOption(option string) (interface{}, error) {
	switch option {
	case mangos.OptionRaw:
		return true, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewRawSocket allocates a raw Socket using the REQ protocol.
func NewRawSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&req{}), nil
}
