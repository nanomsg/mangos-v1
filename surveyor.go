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

package sp

import (
	"math/rand"
	"sync"
	"time"
)

type surveyor struct {
	nextid   uint32
	surveyid uint32
	duration time.Duration
	timeout  time.Time
	xsurveyor
	sync.Mutex
}

func (s *surveyor) Init(sock ProtocolSocket) {
	// Start with a random survey ID
	s.nextid = uint32(rand.NewSource(time.Now().UnixNano()).Int63())
	s.duration = time.Second * 1
	s.xsurveyor.Init(sock)
}

func (*surveyor) Name() string {
	return SurveryorName
}

func (*surveyor) IsRaw() bool {
	return false
}

func (s *surveyor) RecvHook(msg *Message) bool {
	var err error
	var surveyid uint32
	s.Lock()
	defer s.Unlock()

	if surveyid, err = msg.getUint32(); err != nil {
		return false
	}
	if surveyid != s.surveyid {
		return false
	}
	if s.timeout.IsZero() {
		return true
	}
	if time.Now().After(s.timeout) {
		return false
	}
	return true
}

func (s *surveyor) SendHook(msg *Message) bool {

	var timeout time.Time
	s.Lock()
	s.surveyid = s.nextid
	s.nextid++
	msg.putUint32(s.surveyid)
	if s.duration > 0 {
		timeout = time.Now().Add(s.duration)
	}
	s.Unlock()

	// We cheat and grab the recv deadline.
	s.sock.SetOption(OptionRecvDeadline, timeout)
	return true
}

func (s *surveyor) SetOption(name string, val interface{}) error {
	switch name {
	case OptionSurveyTime:
		s.Lock()
		s.duration = val.(time.Duration)
		s.Unlock()
		return nil
	default:
		return ErrBadOption
	}
}

func (s *surveyor) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionSurveyTime:
		s.Lock()
		d := s.duration
		s.Unlock()
		return d, nil
	default:
		return nil, ErrBadOption
	}
}

type surveyorFactory int

func (surveyorFactory) NewProtocol() Protocol {
	return &surveyor{}
}

var SurveyorFactory surveyorFactory
