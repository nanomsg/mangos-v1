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

type xresp struct {
	sock     ProtocolSocket
	peer     *xrespPeer
	raw      bool
	surveyID uint32
	surveyOk bool
	sync.Mutex
}

type xrespPeer struct {
	q      chan *Message
	closeq chan struct{}
	ep     Endpoint
	x      *xresp
}

func (x *xresp) Init(sock ProtocolSocket) {
	x.sock = sock
	go x.sender()
}

func (x *xresp) sender() {
	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.
	for {
		var msg *Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		x.Lock()
		peer := x.peer
		x.Unlock()

		if peer == nil {
			msg.Free()
			continue
		}

		// Put it on the outbound queue
		select {
		case peer.q <- msg:
		case <-x.sock.CloseChannel():
			msg.Free()
			return
		default:
			// Backpressure, drop it.
			msg.Free()
		}
	}
}

// When sending, we should have the survey ID in the header.
func (peer *xrespPeer) sender() {
	for {
		var msg *Message
		select {
		case msg = <-peer.q:
		case <-peer.x.sock.CloseChannel():
			return
		case <-peer.closeq:
			return
		}

		if peer.ep.SendMsg(msg) != nil {
			msg.Free()
			return
		}
	}
}

func (peer *xrespPeer) receiver() {
	for {
		msg := peer.ep.RecvMsg()
		if msg == nil {
			return
		}

		// Get survery ID -- this will be passed in the header up
		// to the application.  It should include that in the response.
		err := msg.trimUint32()
		if err != nil {
			msg.Free()
			return
		}

		select {
		case peer.x.sock.RecvChannel() <- msg:
		case <-peer.x.sock.CloseChannel():
			return
		}
	}
}

func (x *xresp) RecvHook(msg *Message) bool {
	var err error
	if x.raw {
		// Raw mode receivers get the message unadulterated.
		return true
	}
	x.Lock()
	defer x.Unlock()

	if x.surveyID, err = msg.getUint32(); err != nil {
		return false
	}
	x.surveyOk = true
	return true
}

func (x *xresp) SendHook(msg *Message) bool {
	if x.raw {
		// Raw mode senders expected to have prepared header already.
		return true
	}
	x.Lock()
	defer x.Unlock()
	if !x.surveyOk {
		return false
	}
	msg.putUint32(x.surveyID)
	x.surveyOk = false
	return true
}

func (x *xresp) AddEndpoint(ep Endpoint) {
	x.Lock()
	if x.peer != nil {
		x.Unlock()
		ep.Close()
		return
	}
	peer := &xrespPeer{ep: ep, x: x, q: make(chan *Message, 1)}
	x.peer = peer
	peer.closeq = make(chan struct{})
	go peer.receiver()
	go peer.sender()
	x.Unlock()
}

func (x *xresp) RemEndpoint(ep Endpoint) {
	x.Lock()
	if x.peer.ep == ep {
		peer := x.peer
		x.peer = nil
		close(peer.closeq)
	}
	x.Unlock()
}

func (*xresp) Number() uint16 {
	return ProtoRespondent
}

func (*xresp) ValidPeer(peer uint16) bool {
	if peer == ProtoSurveyor {
		return true
	}
	return false
}

func (x *xresp) SetOption(name string, v interface{}) error {
	switch name {
	case OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return ErrBadOption
	}
}

func (x *xresp) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionRaw:
		return x.raw, nil
	default:
		return nil, ErrBadOption
	}
}

type respFactory int

func (respFactory) NewProtocol() Protocol {
	return &xresp{}
}

var RespondentFactory respFactory
