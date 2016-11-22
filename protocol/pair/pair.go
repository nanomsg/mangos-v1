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

// Package pair implements the PAIR protocol.  This protocol is a 1:1
// peering protocol.
package pair

import (
	"sync"
	"time"

	"github.com/go-mangos/mangos"
)

type pair struct {
	sock mangos.ProtocolSocket
	peer *pairEp
	raw  bool
	w    mangos.Waiter
	sync.Mutex
}

type pairEp struct {
	ep mangos.Endpoint
	cq chan struct{}
}

func (x *pair) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.w.Init()
}

func (x *pair) Shutdown(expire time.Time) {
	x.w.WaitAbsTimeout(expire)
}

func (x *pair) sender(ep *pairEp) {

	defer x.w.Done()
	sq := x.sock.SendChannel()
	cq := x.sock.CloseChannel()

	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.
	for {
		select {
		case m := <-sq:
			if m == nil {
				sq = x.sock.SendChannel()
				continue
			}
			if ep.ep.SendMsg(m) != nil {
				m.Free()
				return
			}
		case <-ep.cq:
			return
		case <-cq:
			return
		}
	}
}

func (x *pair) receiver(ep *pairEp) {

	rq := x.sock.RecvChannel()
	cq := x.sock.CloseChannel()

	for {
		m := ep.ep.RecvMsg()
		if m == nil {
			return
		}

		select {
		case rq <- m:
		case <-cq:
			return
		}
	}
}

func (x *pair) AddEndpoint(ep mangos.Endpoint) {
	peer := &pairEp{cq: make(chan struct{}), ep: ep}
	x.Lock()
	if x.peer != nil {
		// We already have a connection, reject this one.
		x.Unlock()
		ep.Close()
		return
	}
	x.peer = peer
	x.Unlock()

	x.w.Add()
	go x.receiver(peer)
	go x.sender(peer)
}

func (x *pair) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	if peer := x.peer; peer != nil && peer.ep == ep {
		x.peer = nil
		close(peer.cq)
	}
	x.Unlock()
}

func (*pair) Number() uint16 {
	return mangos.ProtoPair
}

func (*pair) Name() string {
	return "pair"
}

func (*pair) PeerNumber() uint16 {
	return mangos.ProtoPair
}

func (*pair) PeerName() string {
	return "pair"
}

func (x *pair) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case mangos.OptionRaw:
		if x.raw, ok = v.(bool); !ok {
			return mangos.ErrBadValue
		}
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *pair) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PAIR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&pair{}), nil
}
