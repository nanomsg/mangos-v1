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

type busEp struct {
	ep Endpoint
	q  chan *Message
	x  *xbus
}

type xbus struct {
	sock ProtocolSocket
	sync.Mutex
	eps map[uint32]*busEp
	raw bool
}

// Init implements the Protocol Init method.
func (x *xbus) Init(sock ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*busEp)
	go x.sender()
}

// Bottom sender.
func (pe *busEp) sender() {
	for {
		var msg *Message

		select {
		case msg = <-pe.q:
		case <-pe.x.sock.CloseChannel():
			return
		}

		if err := pe.ep.SendMsg(msg); err != nil {
			msg.Free()
			return
		}
	}
}

func (x *xbus) broadcast(msg *Message, sender uint32) {

	x.Lock()
	for id, pe := range x.eps {
		if sender == id {
			continue
		}
		msg := msg.Dup()
		select {
		case pe.q <- msg:
		default:
			// No room on outbound queue, drop it.
			msg.Free()
		}
	}
	x.Unlock()
}

func (x *xbus) sender() {
	for {
		var msg *Message
		var id uint32
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		// If a header was present, it means this message is being
		// rebroadcast.  It should be a pipe ID.
		if len(msg.Header) >= 4 {
			id, _ = msg.getUint32()
			msg.Header = msg.Header[4:]
		}
		x.broadcast(msg, id)
		msg.Free()
	}
}

func (pe *busEp) receiver() {
	for {
		msg := pe.ep.RecvMsg()
		if msg == nil {
			return
		}
		msg.putUint32(pe.ep.GetID())

		select {
		case pe.x.sock.RecvChannel() <- msg:
		case <-pe.x.sock.CloseChannel():
			return
		default:
			// No room, so we just drop it.
		}
	}
}

func (x *xbus) AddEndpoint(ep Endpoint) {
	pe := &busEp{ep: ep, x: x, q: make(chan *Message, 5)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go pe.sender()
	go pe.receiver()
}

func (x *xbus) RemoveEndpoint(ep Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

func (*xbus) Number() uint16 {
	return ProtoBus
}

func (*xbus) ValidPeer(peer uint16) bool {
	if peer == ProtoBus {
		return true
	}
	return false
}

func (x *xbus) RecvHook(msg *Message) bool {
	if !x.raw && len(msg.Header) >= 4 {
		msg.Header = msg.Header[4:]
	}
	return true
}

func (x *xbus) SetOption(name string, v interface{}) error {
	switch name {
	case OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return ErrBadOption
	}
}

func (x *xbus) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionRaw:
		return x.raw, nil
	default:
		return nil, ErrBadOption
	}
}

type busFactory int

func (busFactory) NewProtocol() Protocol {
	return &xbus{}
}

var BusFactory busFactory
