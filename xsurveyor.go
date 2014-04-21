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
	"time"
)

type xsurveyor struct {
	sock     ProtocolSocket
	peers    map[uint32]*xsurveyorP
	raw      bool
	nextID   uint32
	surveyID uint32
	duration time.Duration
	timeout  time.Time

	sync.Mutex
}

type xsurveyorP struct {
	q      chan *Message
	closeq chan struct{}
	ep     Endpoint
	x      *xsurveyor
}

func (x *xsurveyor) Init(sock ProtocolSocket) {
	x.sock = sock
	x.peers = make(map[uint32]*xsurveyorP)
	go x.sender()
}

func (x *xsurveyor) sender() {
	for {
		var msg *Message
		select {
		case msg = <-x.sock.SendChannel():
		case <-x.sock.CloseChannel():
			return
		}

		x.Lock()
		for _, pe := range x.peers {
			msg := msg.Dup()
			select {
			case pe.q <- msg:
			default:
				msg.Free()
			}
		}
		x.Unlock()
	}
}

// When sending, we should have the survey ID in the header.
func (peer *xsurveyorP) sender() {
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

func (peer *xsurveyorP) receiver() {
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
		case <-peer.closeq:
			return
		}
	}
}

func (x *xsurveyor) AddEndpoint(ep Endpoint) {
	peer := &xsurveyorP{ep: ep, x: x, q: make(chan *Message, 1)}
	x.Lock()
	x.peers[ep.GetID()] = peer
	peer.closeq = make(chan struct{})
	go peer.receiver()
	go peer.sender()
	x.Unlock()
}

func (x *xsurveyor) RemEndpoint(ep Endpoint) {
	x.Lock()
	defer x.Unlock()
	peer := x.peers[ep.GetID()]
	if peer == nil {
		return
	}
	delete(x.peers, ep.GetID())
	close(peer.closeq)
}

func (*xsurveyor) Number() uint16 {
	return ProtoSurveyor
}

func (*xsurveyor) ValidPeer(peer uint16) bool {
	if peer == ProtoRespondent {
		return true
	}
	return false
}

func (x *xsurveyor) SendHook(msg *Message) bool {

	var timeout time.Time
	if x.raw {
		return true
	}

	x.Lock()
	x.surveyID = x.nextID
	x.nextID++
	msg.putUint32(x.surveyID)
	if x.duration > 0 {
		timeout = time.Now().Add(x.duration)
	}
	x.Unlock()

	// We cheat and grab the recv deadline.
	x.sock.SetOption(OptionRecvDeadline, timeout)
	return true
}

func (x *xsurveyor) RecvHook(msg *Message) bool {
	if x.raw {
		return true
	}

	x.Lock()
	defer x.Unlock()

	if id, err := msg.getUint32(); err != nil || id != x.surveyID {
		return false
	}
	if x.timeout.IsZero() {
		return true
	}
	if time.Now().After(x.timeout) {
		return false
	}
	return true
}

func (x *xsurveyor) SetOption(name string, val interface{}) error {
	switch name {
	case OptionRaw:
		x.raw = val.(bool)
		return nil
	case OptionSurveyTime:
		x.Lock()
		x.duration = val.(time.Duration)
		x.Unlock()
		return nil
	default:
		return ErrBadOption
	}
}

func (x *xsurveyor) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionRaw:
		return x.raw, nil
	case OptionSurveyTime:
		x.Lock()
		d := x.duration
		x.Unlock()
		return d, nil
	default:
		return nil, ErrBadOption
	}
}

type surveyorFactory int

func (surveyorFactory) NewProtocol() Protocol {
	return &xsurveyor{}
}

var SurveyorFactory surveyorFactory
