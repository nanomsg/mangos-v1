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

// Package surveyor implements the SURVEYOR protocol. This sends messages
// out to RESPONDENT partners, and receives their responses.
package surveyor

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

const defaultSurveyTime = time.Second

type surveyor struct {
	sock     mangos.ProtocolSocket
	peers    map[uint32]*surveyorP
	raw      bool
	nextID   uint32
	surveyID uint32
	duration time.Duration
	timeout  time.Time

	sync.Mutex
}

type surveyorP struct {
	q  chan *mangos.Message
	ep mangos.Endpoint
	x  *surveyor
}

func (x *surveyor) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.peers = make(map[uint32]*surveyorP)
	go x.sender()
}

// Shutdown does nothing. Since we're not going to be able to receive
// any responses; there is no point in draining.
func (x *surveyor) Shutdown(time.Duration) {}

func (x *surveyor) sender() {
	sq := x.sock.SendChannel()
	for {
		m := <-sq
		if m == nil {
			break
		}

		x.Lock()
		for _, pe := range x.peers {
			m := m.Dup()
			select {
			case pe.q <- m:
			default:
				m.Free()
			}
		}
		x.Unlock()
	}
}

// When sending, we should have the survey ID in the header.
func (peer *surveyorP) sender() {
	for {
		if m := <-peer.q; m == nil {
			break
		} else {
			if peer.ep.SendMsg(m) != nil {
				m.Free()
				return
			}
		}
	}
}

func (peer *surveyorP) receiver() {

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
			return
		}
	}
}

func (x *surveyor) AddEndpoint(ep mangos.Endpoint) {
	peer := &surveyorP{ep: ep, x: x, q: make(chan *mangos.Message, 1)}
	x.Lock()
	x.peers[ep.GetID()] = peer
	go peer.receiver()
	go peer.sender()
	x.Unlock()
}

func (x *surveyor) RemoveEndpoint(ep mangos.Endpoint) {
	x.Lock()
	defer x.Unlock()
	peer := x.peers[ep.GetID()]
	if peer == nil {
		return
	}
	delete(x.peers, ep.GetID())
}

func (*surveyor) Number() uint16 {
	return mangos.ProtoSurveyor
}

func (*surveyor) PeerNumber() uint16 {
	return mangos.ProtoRespondent
}

func (*surveyor) Name() string {
	return "surveyor"
}

func (*surveyor) PeerName() string {
	return "respondent"
}

func (x *surveyor) SendHook(m *mangos.Message) bool {

	if x.raw {
		return true
	}

	x.Lock()
	x.surveyID = x.nextID
	x.nextID++
	v := x.surveyID
	m.Header = append(m.Header,
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

	x.Unlock()

	// We cheat and grab the recv deadline.
	x.sock.SetOption(mangos.OptionRecvDeadline, x.duration)
	return true
}

func (x *surveyor) RecvHook(m *mangos.Message) bool {
	if x.raw {
		return true
	}

	x.Lock()
	defer x.Unlock()

	if len(m.Header) < 4 {
		return false
	}
	if binary.BigEndian.Uint32(m.Header) != x.surveyID {
		return false
	}
	m.Header = m.Header[4:]
	if x.timeout.IsZero() {
		return true
	}
	if time.Now().After(x.timeout) {
		return false
	}
	return true
}

func (x *surveyor) SetOption(name string, val interface{}) error {
	switch name {
	case mangos.OptionRaw:
		x.raw = val.(bool)
		return nil
	case mangos.OptionSurveyTime:
		x.Lock()
		x.duration = val.(time.Duration)
		x.Unlock()
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *surveyor) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	case mangos.OptionSurveyTime:
		x.Lock()
		d := x.duration
		x.Unlock()
		return d, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewProtocol returns a new SURVEYOR protocol object.
func NewSurveyor() mangos.Protocol {
	return &surveyor{}
}

// NewSocket allocates a new Socket using the SURVEYOR protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&surveyor{duration: defaultSurveyTime}), nil
}
