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

// Package pair implements the PAIR protocol.  This protocol is a 1:1
// peering protocol.
package pair

import (
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

type pair struct {
	sock    mangos.ProtocolSocket
	peer    mangos.Endpoint
	raw     bool
	senders mangos.Waiter
	sync.Mutex
}

func (x *pair) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.senders.Init()
}

func (x *pair) Shutdown(linger time.Duration) {
	x.senders.WaitRelTimeout(linger)
}

func (x *pair) sender(ep mangos.Endpoint) {

	sq := x.sock.SendChannel()

	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.
	for {
		m := <-sq
		if m == nil {
			break
		}

		if ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}

	x.senders.Done()
}

func (x *pair) receiver(ep mangos.Endpoint) {

	rq := x.sock.RecvChannel()
	cq := x.sock.CloseChannel()

	for {
		m := ep.RecvMsg()
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
	x.Lock()
	if x.peer != nil {
		x.Unlock()
		ep.Close()
		return
	}
	x.peer = ep
	x.Unlock()

	x.senders.Add()
	go x.receiver(ep)
	go x.sender(ep)
}

func (x *pair) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	if x.peer == ep {
		x.peer = nil
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
	switch name {
	case mangos.OptionRaw:
		x.raw = v.(bool)
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

// NewProtocol returns a new PAIR protocol object.
func NewProtocol() mangos.Protocol {
	return &pair{}
}

// NewSocket allocates a new Socket using the PAIR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&pair{}), nil
}
