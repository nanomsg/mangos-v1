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

// Package rep implements the REP protocol, which is the response side of
// the request/response pattern.  (REQ is the request.)
package rep

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

type repEp struct {
	q    chan *mangos.Message
	ep   mangos.Endpoint
	sock mangos.ProtocolSocket
	w    mangos.Waiter
	r    *rep
}

type rep struct {
	sock         mangos.ProtocolSocket
	eps          map[uint32]*repEp
	backtracebuf []byte
	backtrace    []byte
	backtraceL   sync.Mutex
	raw          bool
	w            mangos.Waiter

	sync.Mutex
}

func (r *rep) Init(sock mangos.ProtocolSocket) {
	r.sock = sock
	r.eps = make(map[uint32]*repEp)
	r.backtracebuf = make([]byte, 64)
	r.w.Init()
	r.w.Add()
	go r.sender()
}

func (r *rep) Shutdown(linger time.Duration) {
	when := time.Now().Add(linger)
	tq := time.After(linger)
	r.w.WaitAbsTimeout(when)
	r.Lock()
	for _, pe := range r.eps {
		select {
		case pe.q <- nil:
		case <-tq:
			tq = time.After(0)
		}
		pe.w.WaitAbsTimeout(when)
	}
	r.Unlock()
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
	pe.w.Done()
}

func (r *rep) receiver(ep mangos.Endpoint) {
	for {

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		v := ep.GetID()
		m.Header = append(m.Header,
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
		// Move backtrace from body to header.
		for {
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
		case r.sock.RecvChannel() <- m:
		case <-r.sock.CloseChannel():
			m.Free()
			return
		}
	}
}

func (r *rep) sender() {
	sq := r.sock.SendChannel()

	for {
		m := <-sq
		if m == nil {
			break
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
		r.Unlock()
		if pe == nil {
			m.Free()
			continue
		}

		select {
		case pe.q <- m:
		default:
			// If our queue is full, we have no choice but to
			// throw it on the floor.  This shoudn't happen,
			// since each partner should be running synchronously.
			// Devices are a different situation, and this could
			// lead to lossy behavior there.  Initiators will
			// resend if this happens.  Devices need to have deep
			// enough queues and be fast enough to avoid this.
			m.Free()
		}
	}

	r.w.Done()
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
	pe.w.Add()
	go r.receiver(ep)
	go pe.sender()
}

func (r *rep) RemoveEndpoint(ep mangos.Endpoint) {
	r.Lock()
	delete(r.eps, ep.GetID())
	r.Unlock()
}

// We save the backtrace from this message.  This means that if the app calls
// Recv before calling Send, the saved backtrace will be lost.  This is how
// the application discards / cancels a request to which it declines to reply.
// This is only done in cooked mode.
func (r *rep) RecvHook(m *mangos.Message) bool {
	if r.raw {
		return true
	}
	r.backtraceL.Lock()
	r.backtrace = append(r.backtracebuf[0:0], m.Header...)
	r.backtraceL.Unlock()
	m.Header = nil
	return true
}

func (r *rep) SendHook(m *mangos.Message) bool {
	// Store our saved backtrace.  Note that if none was previously stored,
	// there is no one to reply to, and we drop the message.  We only
	// do this in cooked mode.
	if r.raw {
		return true
	}
	r.backtraceL.Lock()
	m.Header = append(m.Header[0:0], r.backtrace...)
	r.backtrace = nil
	r.backtraceL.Unlock()
	if m.Header == nil {
		return false
	}
	return true
}

func (r *rep) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		r.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (r *rep) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return r.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewProtocol returns a new REP protocol object.
func NewProtocol() mangos.Protocol {
	return &rep{}
}

// NewSocket allocates a new Socket using the REP protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&rep{}), nil
}
