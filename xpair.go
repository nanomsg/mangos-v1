// Copyright 2014 Garrett D'Amore
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

package mangos

import (
	"sync"
)

type xpair struct {
	sock ProtocolSocket
	peer Endpoint
	raw  bool
	sync.Mutex
}

func (x *xpair) Init(sock ProtocolSocket) {
	x.sock = sock
}

func (x *xpair) sender(ep Endpoint) {
	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.
	for {
		var msg *Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		if ep.SendMsg(msg) != nil {
			msg.Free()
			return
		}
	}
}

func (x *xpair) receiver(ep Endpoint) {
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

func (x *xpair) AddEndpoint(ep Endpoint) {
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

func (x *xpair) RemoveEndpoint(ep Endpoint) {
	x.Lock()
	if x.peer == ep {
		x.peer = nil
	}
	x.Unlock()
}

func (*xpair) Number() uint16 {
	return ProtoPair
}

func (*xpair) ValidPeer(peer uint16) bool {
	if peer == ProtoPair {
		return true
	}
	return false
}

func (x *xpair) SetOption(name string, v interface{}) error {
	switch name {
	case OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return ErrBadOption
	}
}

func (x *xpair) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionRaw:
		return x.raw, nil
	default:
		return nil, ErrBadOption
	}
}

type pairFactory int

func (pairFactory) NewProtocol() Protocol {
	return &xpair{}
}

// XPairFactory implements the Protocol Factory for the XPAIR protocol.
// The XPAIR Protocol is the raw form of the PAIR (Pair) protocol.
var PairFactory pairFactory
