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

// Package bus implements the BUS protocol.  In this protocol, participants
// send a message to each of their peers.
package bus

import (
	"github.com/gdamore/mangos"
	"encoding/binary"
	"sync"
)

type busEp struct {
	ep mangos.Endpoint
	q  chan *mangos.Message
	x  *bus
}

type bus struct {
	sock mangos.ProtocolSocket
	sync.Mutex
	eps map[uint32]*busEp
	raw bool
}

// Init implements the Protocol Init method.
func (x *bus) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*busEp)
	go x.sender()
}

// Bottom sender.
func (pe *busEp) sender() {
	for {
		var m *mangos.Message

		select {
		case m = <-pe.q:
		case <-pe.x.sock.DrainChannel():
			return
		}

		if err := pe.ep.SendMsg(m); err != nil {
			m.Free()
			return
		}
	}
}

func (x *bus) broadcast(m *mangos.Message, sender uint32) {

	x.Lock()
	for id, pe := range x.eps {
		if sender == id {
			continue
		}
		msg := m.Dup()
		select {
		case pe.q <- msg:
		default:
			// No room on outbound queue, drop it.
			m.Free()
		}
	}
	x.Unlock()
}

func (x *bus) sender() {
	for {
		var m *mangos.Message
		var id uint32
		select {
		case m = <-x.sock.SendChannel():
		case <-x.sock.DrainChannel():
			return
		}

		// If a header was present, it means this message is being
		// rebroadcast.  It should be a pipe ID.
		if len(m.Header) >= 4 {
			id = binary.BigEndian.Uint32(m.Header)
			m.Header = m.Header[4:]
		}
		x.broadcast(m, id)
		m.Free()
	}
}

func (pe *busEp) receiver() {
	for {
		m := pe.ep.RecvMsg()
		if m == nil {
			return
		}
		v := pe.ep.GetID()
		m.Header = append(m.Header,
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

		select {
		case pe.x.sock.RecvChannel() <- m:
		case <-pe.x.sock.CloseChannel():
			return
		default:
			// No room, so we just drop it.
		}
	}
}

func (x *bus) AddEndpoint(ep mangos.Endpoint) {
	pe := &busEp{ep: ep, x: x, q: make(chan *mangos.Message, 5)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go pe.sender()
	go pe.receiver()
}

func (x *bus) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

func (*bus) Number() uint16 {
	return mangos.ProtoBus
}

func (*bus) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoBus {
		return true
	}
	return false
}

func (x *bus) RecvHook(m *mangos.Message) bool {
	if !x.raw && len(m.Header) >= 4 {
		m.Header = m.Header[4:]
	}
	return true
}

func (x *bus) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *bus) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the BUS protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&bus{}), nil
}
