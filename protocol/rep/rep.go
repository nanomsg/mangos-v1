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

// Package rep implements the REP protocol, which is the response side of
// the request/response pattern.  (REQ is the request.)
package rep

import (
	"encoding/binary"
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2"
)

type repEp struct {
	q    chan *mangos.Message
	ep   mangos.Endpoint
	sock mangos.ProtocolSocket
	w    mangos.Waiter
	r    *rep
}

type rep struct {
	sock mangos.ProtocolSocket
	eps  map[uint32]*repEp
	ttl  int
	w    mangos.Waiter

	sync.Mutex
}

func (r *rep) Init(sock mangos.ProtocolSocket) {
	r.sock = sock
	r.eps = make(map[uint32]*repEp)
	r.ttl = 8 // default specified in the RFC
	r.w.Init()
	r.sock.SetSendError(mangos.ErrProtoState)
	r.w.Add()
	go r.sender()
}

func (r *rep) Shutdown(expire time.Time) {

	r.w.WaitAbsTimeout(expire)

	r.Lock()
	peers := r.eps
	r.eps = make(map[uint32]*repEp)
	r.Unlock()

	for id, peer := range peers {
		delete(peers, id)
		mangos.DrainChannel(peer.q, expire)
		close(peer.q)
	}
}

func (pe *repEp) sender() {
	for {
		m := <-pe.q
		if m == nil {
			break
		}

		if pe.ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}
}

func (r *rep) receiver(ep mangos.Endpoint) {

	rq := r.sock.RecvChannel()
	cq := r.sock.CloseChannel()

	for {

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		v := ep.GetID()
		m.Header = append(m.Header,
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

		hops := 0
		// Move backtrace from body to header.
		for {
			if hops >= r.ttl {
				m.Free() // ErrTooManyHops
				return
			}
			hops++
			if len(m.Body) < 4 {
				m.Free() // ErrGarbled
				return
			}
			m.Header = append(m.Header, m.Body[:4]...)
			m.Body = m.Body[4:]
			// Check for high order bit set (0x80000000, big endian)
			if m.Header[len(m.Header)-4]&0x80 != 0 {
				break
			}
		}

		select {
		case rq <- m:
		case <-cq:
			m.Free()
			return
		}
	}
}

func (r *rep) sender() {
	defer r.w.Done()
	cq := r.sock.CloseChannel()
	sq := r.sock.SendChannel()

	for {
		var m *mangos.Message

		select {
		case m = <-sq:
			if m == nil {
				sq = r.sock.SendChannel()
				continue
			}
		case <-cq:
			return
		}

		// Lop off the 32-bit peer/pipe ID.  If absent, drop.
		if len(m.Header) < 4 {
			m.Free()
			continue
		}
		id := binary.BigEndian.Uint32(m.Header)
		m.Header = m.Header[4:]
		r.Lock()
		pe := r.eps[id]
		if pe == nil {
			r.Unlock()
			m.Free()
			continue
		}

		select {
		case pe.q <- m:
		default:
			// If our queue is full, we have no choice but to
			// throw it on the floor.  This shouldn't happen,
			// since each partner should be running synchronously.
			// Devices are a different situation, and this could
			// lead to lossy behavior there.  Initiators will
			// resend if this happens.  Devices need to have deep
			// enough queues and be fast enough to avoid this.
			m.Free()
		}
		r.Unlock()
	}
}

func (*rep) Number() uint16 {
	return mangos.ProtoRep
}

func (*rep) PeerNumber() uint16 {
	return mangos.ProtoReq
}

func (*rep) Name() string {
	return "rep"
}

func (*rep) PeerName() string {
	return "req"
}

func (r *rep) AddEndpoint(ep mangos.Endpoint) {
	pe := &repEp{ep: ep, r: r, q: make(chan *mangos.Message, 2)}
	pe.w.Init()
	r.Lock()
	r.eps[ep.GetID()] = pe
	r.Unlock()
	go r.receiver(ep)
	go pe.sender()
}

func (r *rep) RemoveEndpoint(ep mangos.Endpoint) {
	id := ep.GetID()

	r.Lock()
	pe := r.eps[id]
	delete(r.eps, id)

	if pe != nil {
		close(pe.q)
	}
	r.Unlock()
}

func (r *rep) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionTTL:
		if ttl, ok := v.(int); !ok {
			return mangos.ErrBadValue
		} else if ttl < 1 || ttl > 255 {
			return mangos.ErrBadValue
		} else {
			r.ttl = ttl
		}
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (r *rep) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return true, nil
	case mangos.OptionTTL:
		return r.ttl, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewRawSocket allocates a raw Socket using the REP protocol.
func NewRawSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&rep{}), nil
}
