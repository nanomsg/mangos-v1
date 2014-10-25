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

// Package respondent implements the RESPONDENT protocol.  This protocol
// receives SURVEYOR requests, and responds with an answer.
package respondent

import (
	"github.com/gdamore/mangos"
	"encoding/binary"
	"sync"
)

type resp struct {
	sock     mangos.ProtocolSocket
	peer     *respPeer
	raw      bool
	surveyID uint32
	surveyOk bool
	sync.Mutex
}

type respPeer struct {
	q      chan *mangos.Message
	closeq chan struct{}
	ep     mangos.Endpoint
	x      *resp
}

func (x *resp) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	go x.sender()
}

func (x *resp) sender() {
	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.
	for {
		var msg *mangos.Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.DrainChannel():
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
		case <-x.sock.DrainChannel():
			msg.Free()
			return
		default:
			// Backpressure, drop it.
			msg.Free()
		}
	}
}

// When sending, we should have the survey ID in the header.
func (peer *respPeer) sender() {
	for {
		var msg *mangos.Message
		select {
		case msg = <-peer.q:
		case <-peer.x.sock.DrainChannel():
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

func (peer *respPeer) receiver() {
	for {
		m := peer.ep.RecvMsg()
		if m == nil {
			return
		}
		if len(m.Body) < 4 {
			m.Free()
			continue
		}

		// Get survery ID -- this will be passed in the header up
		// to the application.  It should include that in the response.
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		select {
		case peer.x.sock.RecvChannel() <- m:
		case <-peer.x.sock.CloseChannel():
			return
		}
	}
}

func (x *resp) RecvHook(m *mangos.Message) bool {
	if x.raw {
		// Raw mode receivers get the message unadulterated.
		return true
	}
	x.Lock()
	defer x.Unlock()

	if len(m.Header) < 4 {
		return false
	}
	x.surveyID = binary.BigEndian.Uint32(m.Header)
	x.surveyOk = true
	return true
}

func (x *resp) SendHook(m *mangos.Message) bool {
	if x.raw {
		// Raw mode senders expected to have prepared header already.
		return true
	}
	x.Lock()
	defer x.Unlock()
	if !x.surveyOk {
		return false
	}
	v := x.surveyID
	m.Header = append(m.Header,
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	x.surveyOk = false
	return true
}

func (x *resp) AddEndpoint(ep mangos.Endpoint) {
	x.Lock()
	if x.peer != nil {
		x.Unlock()
		ep.Close()
		return
	}
	peer := &respPeer{ep: ep, x: x, q: make(chan *mangos.Message, 1)}
	x.peer = peer
	peer.closeq = make(chan struct{})
	go peer.receiver()
	go peer.sender()
	x.Unlock()
}

func (x *resp) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	if x.peer.ep == ep {
		peer := x.peer
		x.peer = nil
		close(peer.closeq)
	}
	x.Unlock()
}

func (*resp) Number() uint16 {
	return mangos.ProtoRespondent
}

func (*resp) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoSurveyor {
		return true
	}
	return false
}

func (x *resp) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *resp) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&resp{}), nil
}
