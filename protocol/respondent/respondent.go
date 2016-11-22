// Copyright 2016 The Mangos Authors
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

// Package respondent implements the RESPONDENT protocol.  This protocol
// receives SURVEYOR requests, and responds with an answer.
package respondent

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/go-mangos/mangos"
)

type resp struct {
	sock      mangos.ProtocolSocket
	peers     map[uint32]*respPeer
	raw       bool
	ttl       int
	backbuf   []byte
	backtrace []byte
	w         mangos.Waiter
	sync.Mutex
}

type respPeer struct {
	q  chan *mangos.Message
	ep mangos.Endpoint
	x  *resp
}

func (x *resp) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.ttl = 8
	x.peers = make(map[uint32]*respPeer)
	x.w.Init()
	x.backbuf = make([]byte, 0, 64)
	x.sock.SetSendError(mangos.ErrProtoState)
	x.w.Add()
	go x.sender()
}

func (x *resp) Shutdown(expire time.Time) {
	peers := make(map[uint32]*respPeer)
	x.w.WaitAbsTimeout(expire)
	x.Lock()
	for id, peer := range x.peers {
		delete(x.peers, id)
		peers[id] = peer
	}
	x.Unlock()

	for id, peer := range peers {
		delete(peers, id)
		mangos.DrainChannel(peer.q, expire)
		close(peer.q)
	}
}

func (x *resp) sender() {
	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.

	defer x.w.Done()
	cq := x.sock.CloseChannel()
	sq := x.sock.SendChannel()
	for {
		var m *mangos.Message
		select {
		case m = <-sq:
			if m == nil {
				sq = x.sock.SendChannel()
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

		x.Lock()
		peer := x.peers[id]
		x.Unlock()

		if peer == nil {
			m.Free()
			continue
		}

		// Put it on the outbound queue
		select {
		case peer.q <- m:
		default:
			// Backpressure, drop it.
			m.Free()
		}
	}
}

// When sending, we should have the survey ID in the header.
func (peer *respPeer) sender() {
	for {
		m := <-peer.q
		if m == nil {
			break
		}
		if peer.ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}
}

func (x *resp) receiver(ep mangos.Endpoint) {

	rq := x.sock.RecvChannel()
	cq := x.sock.CloseChannel()

outer:
	for {
		m := ep.RecvMsg()
		if m == nil {
			return
		}

		v := ep.GetID()
		m.Header = append(m.Header,
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
		hops := 0

		for {
			if hops >= x.ttl {
				m.Free() // ErrTooManyHops
				continue outer
			}
			hops++
			if len(m.Body) < 4 {
				m.Free()
				continue outer
			}
			m.Header = append(m.Header, m.Body[:4]...)
			m.Body = m.Body[4:]
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

func (x *resp) RecvHook(m *mangos.Message) bool {
	if x.raw {
		// Raw mode receivers get the message unadulterated.
		return true
	}

	if len(m.Header) < 4 {
		return false
	}

	x.Lock()
	x.backbuf = x.backbuf[0:0] // avoid allocations
	x.backtrace = append(x.backbuf, m.Header...)
	x.Unlock()
	x.sock.SetSendError(nil)
	return true
}

func (x *resp) SendHook(m *mangos.Message) bool {
	if x.raw {
		// Raw mode senders expected to have prepared header already.
		return true
	}
	x.sock.SetSendError(mangos.ErrProtoState)
	x.Lock()
	m.Header = append(m.Header[0:0], x.backtrace...)
	x.backtrace = nil
	x.Unlock()
	if len(m.Header) == 0 {
		return false
	}
	return true
}

func (x *resp) AddEndpoint(ep mangos.Endpoint) {
	peer := &respPeer{ep: ep, x: x, q: make(chan *mangos.Message, 1)}

	x.Lock()
	x.peers[ep.GetID()] = peer
	x.Unlock()

	go x.receiver(ep)
	go peer.sender()
}

func (x *resp) RemoveEndpoint(ep mangos.Endpoint) {
	id := ep.GetID()

	x.Lock()
	peer := x.peers[id]
	delete(x.peers, id)
	x.Unlock()

	if peer != nil {
		close(peer.q)
	}
}

func (*resp) Number() uint16 {
	return mangos.ProtoRespondent
}

func (*resp) PeerNumber() uint16 {
	return mangos.ProtoSurveyor
}

func (*resp) Name() string {
	return "respondent"
}

func (*resp) PeerName() string {
	return "surveyor"
}

func (x *resp) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case mangos.OptionRaw:
		if x.raw, ok = v.(bool); !ok {
			return mangos.ErrBadValue
		}
		if x.raw {
			x.sock.SetSendError(nil)
		} else {
			x.sock.SetSendError(mangos.ErrProtoState)
		}
		return nil
	case mangos.OptionTTL:
		if ttl, ok := v.(int); !ok {
			return mangos.ErrBadValue
		} else if ttl < 1 || ttl > 255 {
			return mangos.ErrBadValue
		} else {
			x.ttl = ttl
		}
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *resp) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	case mangos.OptionTTL:
		return x.ttl, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&resp{}), nil
}
