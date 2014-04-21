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

// xrep is an implementation of the XREP Protocol.

type xrepEp struct {
	q    chan *Message
	ep   Endpoint
	sock ProtocolSocket
}

type xrep struct {
	sock         ProtocolSocket
	eps          map[uint32]*xrepEp
	backtracebuf []byte
	backtrace    []byte
	backtraceL   sync.Mutex
	raw          bool

	sync.Mutex
}

func (x *xrep) Init(sock ProtocolSocket) {
	x.sock = sock
	x.eps = make(map[uint32]*xrepEp)
	x.backtracebuf = make([]byte, 64)

	go x.sender()
}

func (pe *xrepEp) sender() {
	for {
		var msg *Message
		select {
		case msg = <-pe.q:
		case <-pe.sock.CloseChannel():
			return
		}

		err := pe.ep.SendMsg(msg)
		if err != nil {
			msg.Free()
			return
		}
	}
}

func (x *xrep) receiver(ep Endpoint) {
	for {

		msg := ep.RecvMsg()
		if msg == nil {
			return
		}

		msg.putUint32(ep.GetID())
		if msg.trimBackTrace() != nil {
			msg.Free()
			continue
		}

		select {
		case x.sock.RecvChannel() <- msg:
		case <-x.sock.CloseChannel():
			msg.Free()
			return
		}
	}
}

func (x *xrep) sender() {
	for {
		var msg *Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		// Lop off the 32-bit peer/pipe ID.  If absent, drop.
		id, err := msg.getUint32()
		if err != nil {
			msg.Free()
			continue
		}
		x.Lock()
		pe := x.eps[id]
		x.Unlock()
		if pe == nil {
			msg.Free()
			continue
		}

		select {
		case pe.q <- msg:
		default:
			// If our queue is full, we have no choice but to
			// throw it on the floor.  This shoudn't happen,
			// since each partner should be running synchronously.
			// Devices are a different situation, and this could
			// lead to lossy behavior there.  Initiators will
			// resend if this happens.  Devices need to have deep
			// enough queues and be fast enough to avoid this.
			msg.Free()
		}
	}
}

func (*xrep) Number() uint16 {
	return ProtoRep
}

func (*xrep) ValidPeer(peer uint16) bool {
	if peer == ProtoReq {
		return true
	}
	return false
}

func (x *xrep) AddEndpoint(ep Endpoint) {
	pe := &xrepEp{ep: ep, sock: x.sock, q: make(chan *Message, 2)}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	go x.receiver(ep)
	go pe.sender()
}

func (x *xrep) RemEndpoint(ep Endpoint) {
	x.Lock()
	delete(x.eps, ep.GetID())
	x.Unlock()
}

// We save the backtrace from this message.  This means that if the app calls
// Recv before calling Send, the saved backtrace will be lost.  This is how
// the application discards / cancels a request to which it declines to reply.
// This is only done in cooked mode.
func (x *xrep) RecvHook(m *Message) bool {
	if x.raw {
		return true
	}
	x.backtraceL.Lock()
	x.backtrace = append(x.backtracebuf[0:0], m.Header...)
	x.backtraceL.Unlock()
	m.Header = nil
	return true
}

func (x *xrep) SendHook(m *Message) bool {
	// Store our saved backtrace.  Note that if none was previously stored,
	// there is no one to reply to, and we drop the message.  We only
	// do this in cooked mode.
	if x.raw {
		return true
	}
	x.backtraceL.Lock()
	m.Header = append(m.Header[0:0], x.backtrace...)
	x.backtrace = nil
	x.backtraceL.Unlock()
	if m.Header == nil {
		return false
	}
	return true
}

func (x *xrep) SetOption(name string, v interface{}) error {
	switch name {
	case OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return ErrBadOption
	}
}

func (x *xrep) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionRaw:
		return x.raw, nil
	default:
		return nil, ErrBadOption
	}
}

type repFactory int

func (repFactory) NewProtocol() Protocol {
	return &xrep{}
}

var RepFactory repFactory
