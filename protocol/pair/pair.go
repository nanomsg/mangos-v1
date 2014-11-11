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

// Package pair implements the PAIR protocol.  This protocol is a 1:1
// peering protocol.
package pair

import (
	"github.com/gdamore/mangos"
	"sync"
)

type pair struct {
	sock mangos.ProtocolSocket
	peer mangos.Endpoint
	raw  bool
	sync.Mutex
}

func (x *pair) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
}

func (x *pair) sender(ep mangos.Endpoint) {
	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.
	for {
		var msg *mangos.Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.DrainChannel():
			return
		}

		if ep.SendMsg(msg) != nil {
			msg.Free()
			return
		}
	}
}

func (x *pair) receiver(ep mangos.Endpoint) {
	for {
		msg := ep.RecvMsg()
		if msg == nil {
			return
		}

		select {
		case x.sock.RecvChannel() <- msg:
		case <-x.sock.CloseChannel():
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
	go x.receiver(ep)
	go x.sender(ep)
	x.Unlock()
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

func (*pair) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoPair {
		return true
	}
	return false
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

// NewSocket allocates a new Socket using the PAIR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&pair{}), nil
}
