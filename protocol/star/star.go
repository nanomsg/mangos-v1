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

// Package star implements a new, experimental protocol called "STAR".
// This is like the BUS protocol, except that each member of the network
// automatically forwards any message it receives to any other peers.
// In a star network, this means that all members should receive all messages,
// assuming that there is a central server.  Its important to ensure that
// the topology is free from cycles, as there is no protection against
// that, and cycles can lead to infinite message storms.  (TODO: Add a TTL,
// and basic message ID / anti-replay protection.)
package star

import (
	"sync"
	"time"

	"github.com/go-mangos/mangos"
)

type starEp struct {
	ep mangos.Endpoint
	q  chan *mangos.Message
	x  *star
}

type star struct {
	sock mangos.ProtocolSocket
	eps  map[uint32]*starEp
	raw  bool
	w    mangos.Waiter
	init sync.Once
	ttl  int

	sync.Mutex
}

func (x *star) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*starEp)
	x.ttl = 8
	x.w.Init()
}

func (x *star) Shutdown(expire time.Time) {

	x.w.WaitAbsTimeout(expire)

	x.Lock()
	peers := x.eps
	x.eps = make(map[uint32]*starEp)
	x.Unlock()

	for id, peer := range peers {
		delete(peers, id)
		mangos.DrainChannel(peer.q, expire)
		close(peer.q)
	}
}

// Bottom sender.
func (pe *starEp) sender() {
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

func (x *star) broadcast(m *mangos.Message, sender *starEp) {

	x.Lock()
	if sender == nil || !x.raw {
		for _, pe := range x.eps {
			if sender == pe {
				continue
			}
			m = m.Dup()
			select {
			case pe.q <- m:
			default:
				// No room on outbound queue, drop it.
				if m != nil {
					m.Free()
				}
			}
		}
	}
	x.Unlock()

	// Grab a local copy and send it up if we aren't originator
	if sender != nil {
		select {
		case x.sock.RecvChannel() <- m:
		case <-x.sock.CloseChannel():
			m.Free()
			return
		default:
			// No room, so we just drop it.
			m.Free()
		}
	} else {
		// Not sending it up, so we need to release it.
		m.Free()
	}
}

func (x *star) sender() {
	defer x.w.Done()
	sq := x.sock.SendChannel()
	cq := x.sock.CloseChannel()

	for {
		select {
		case <-cq:
			return
		case m := <-sq:
			x.broadcast(m, nil)
		}
	}
}

func (pe *starEp) receiver() {
	for {
		m := pe.ep.RecvMsg()
		if m == nil {
			return
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		if m.Body[0] != 0 || m.Body[1] != 0 || m.Body[2] != 0 {
			// non-zero reserved fields are illegal
			m.Free()
			continue
		}
		if int(m.Body[3]) >= pe.x.ttl { // TTL expired?
			// XXX: bump a stat
			m.Free()
			continue
		}
		m.Header = append(m.Header, 0, 0, 0, m.Body[3]+1)
		m.Body = m.Body[4:]

		// if we're in raw mode, this does only a sendup, otherwise
		// it does both a retransmit + sendup
		pe.x.broadcast(m, pe)
	}
}

func (x *star) AddEndpoint(ep mangos.Endpoint) {
	x.init.Do(func() {
		x.w.Add()
		go x.sender()
	})
	depth := 16
	if i, err := x.sock.GetOption(mangos.OptionWriteQLen); err == nil {
		depth = i.(int)
	}
	pe := &starEp{ep: ep, x: x, q: make(chan *mangos.Message, depth)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go pe.sender()
	go pe.receiver()
}

func (x *star) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	if peer := x.eps[ep.GetID()]; peer != nil {
		delete(x.eps, ep.GetID())
		close(peer.q)
	}
	x.Unlock()
}

func (*star) Number() uint16 {
	return mangos.ProtoStar
}

func (*star) PeerNumber() uint16 {
	return mangos.ProtoStar
}

func (*star) Name() string {
	return "star"
}

func (*star) PeerName() string {
	return "star"
}

func (x *star) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case mangos.OptionRaw:
		if x.raw, ok = v.(bool); !ok {
			return mangos.ErrBadValue
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

func (x *star) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	case mangos.OptionTTL:
		return x.ttl, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

func (x *star) SendHook(m *mangos.Message) bool {

	if x.raw {
		// TTL header must be present.
		return true
	}
	// new message has a zero hop count
	m.Header = append(m.Header, 0, 0, 0, 0)
	return true
}

// NewSocket allocates a new Socket using the STAR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&star{}), nil
}
