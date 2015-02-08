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

// Package respondent implements the RESPONDENT protocol.  This protocol
// receives SURVEYOR requests, and responds with an answer.
package respondent

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

type resp struct {
	sock     mangos.ProtocolSocket
	peer     *respPeer
	raw      bool
	surveyID uint32
	surveyOk bool
	senders  mangos.Waiter
	sync.Mutex
}

type respPeer struct {
	q  chan *mangos.Message
	ep mangos.Endpoint
	x  *resp
}

func (x *resp) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.senders.Init()
	go x.sender()
}

// Shutdown just returns.  There is no point in draining a survey --
// we're going to close before we can receive any responses anyway.
func (x *resp) Shutdown(linger time.Duration) {
	x.senders.WaitRelTimeout(linger)
}

func (x *resp) sender() {
	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, we'll just drop the message on the floor.

	sq := x.sock.SendChannel()
	for {
		m := <-sq
		if m == nil {
			// We want to send the nil down to the peer to alert it
			// to drain.
			x.Lock()
			if peer := x.peer; peer != nil {
				select {
				case peer.q <- nil:
				default:
				}
			}
			x.Unlock()
			break
		}

		x.Lock()
		peer := x.peer
		x.Unlock()

		if peer == nil {
			m.Free()
			continue
		}

		// Put it on the outbound queue
		select {
		case peer.q <- m:
		default:
			// Backpressure, drop it.
			m.Free()
		}
	}
}

// When sending, we should have the survey ID in the header.
func (peer *respPeer) sender() {
	for {
		m := <-peer.q
		if m == nil {
			break
		}
		if peer.ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}
	peer.x.senders.Done()
}

func (peer *respPeer) receiver() {

	rq := peer.x.sock.RecvChannel()
	cq := peer.x.sock.CloseChannel()

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
		case rq <- m:
		case <-cq:
			m.Free()
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
	x.Unlock()

	x.senders.Add()
	go peer.receiver()
	go peer.sender()
}

func (x *resp) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	if x.peer.ep == ep {
		x.peer = nil
	}
	x.Unlock()
}

func (*resp) Number() uint16 {
	return mangos.ProtoRespondent
}

func (*resp) PeerNumber() uint16 {
	return mangos.ProtoSurveyor
}

func (*resp) Name() string {
	return "respondent"
}

func (*resp) PeerName() string {
	return "surveyor"
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

// NewProtocol returns a new RESPONDENT protocol object.
func NewProtocol() mangos.Protocol {
	return &resp{}
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&resp{}), nil
}
