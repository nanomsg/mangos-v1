// Copyright 2018 The Mangos Authors
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

package test

import (
	"encoding/binary"
	"testing"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/respondent"
	"nanomsg.org/go/mangos/v2/protocol/surveyor"
	"nanomsg.org/go/mangos/v2/protocol/xrespondent"
	"nanomsg.org/go/mangos/v2/protocol/xsurveyor"
)

type surveyTest struct {
	nresp int32
	resp  map[uint32]bool
	T
}

type responderTest struct {
	T
}

func (st *surveyTest) Init(t *testing.T, addr string) bool {
	var err error
	st.resp = make(map[uint32]bool)
	if st.Sock, err = surveyor.NewSocket(); err != nil {
		st.Errorf("NewSocket(): %v", err)
		return false
	}
	return st.T.Init(t, addr)
}

func (st *surveyTest) WantRecvStart() bool {
	return false
}

func (st *surveyTest) SendHook(m *mangos.Message) bool {
	m.Body = m.Body[0:4]
	binary.BigEndian.PutUint32(m.Body, uint32(st.GetSend()))
	return st.T.SendHook(m)
}

func (st *surveyTest) RecvHook(m *mangos.Message) bool {
	if len(m.Body) != 4 {
		st.Errorf("Recv message length %d != 4", len(m.Body))
		return false
	}
	v := binary.BigEndian.Uint32(m.Body)
	if st.resp[v] {
		st.Logf("Duplicate response from id %d", v)
	} else {
		st.Logf("Response from id %d", v)
		st.resp[v] = true
		st.BumpRecv()
	}
	return true
}

func (rt *responderTest) Init(t *testing.T, addr string) bool {
	var err error
	if rt.Sock, err = respondent.NewSocket(); err != nil {
		rt.Errorf("NewSocket(): %v", err)
		return false
	}
	return rt.T.Init(t, addr)
}

func (rt *responderTest) RecvHook(m *mangos.Message) bool {
	if len(m.Body) < 4 {
		rt.Errorf("Recv message length %d < 4", len(m.Body))
		return false
	}
	rt.Logf("Got survey ID %d", binary.BigEndian.Uint32(m.Body))

	// reply
	newm := rt.NewMessage()
	newm.Body = newm.Body[0:4]
	binary.BigEndian.PutUint32(newm.Body, uint32(rt.GetID()))
	rt.SendMsg(newm)
	return rt.T.RecvHook(m)
}

func (rt *responderTest) RecvStart() bool {
	m, err := rt.RecvMsg()
	if err != nil {
		rt.Errorf("RecvMsg failed: %v", err)
		return false
	}
	defer m.Free()
	if _, ok := ParseStart(m); !ok {
		rt.Errorf("Unexpected survey message: %v", m)
		return false
	}

	rm := MakeStart(uint32(rt.GetID()))
	rt.Debugf("Sending START reply")
	rt.SendMsg(rm)
	return true
}

func surveyCases() []TestCase {
	var nresp int32 = 3

	cases := make([]TestCase, nresp+1)
	surv := &surveyTest{nresp: nresp}
	surv.Server = true
	surv.ID = 0
	surv.MsgSize = 8
	surv.WantTx = 1
	surv.WantRx = int32(nresp)
	surv.txdelay = time.Second / 7
	surv.Synch = true
	surv.NReply = int(nresp)
	cases[0] = surv

	for i := 0; i < int(nresp); i++ {
		resp := &responderTest{}
		resp.ID = i + 1
		resp.MsgSize = 8
		resp.WantTx = 0 // reply is done in response to receipt
		resp.WantRx = 1
		cases[i+1] = resp
	}

	return cases
}

func TestSurveyTCP(t *testing.T) {
	RunTestsTCP(t, surveyCases())
}

func TestSurveyIPC(t *testing.T) {
	RunTestsIPC(t, surveyCases())
}

func TestSurveyInp(t *testing.T) {
	RunTestsInp(t, surveyCases())
}

func TestSurveyTLS(t *testing.T) {
	RunTestsTLS(t, surveyCases())
}

func TestSurveyWS(t *testing.T) {
	RunTestsWS(t, surveyCases())
}

func TestSurveyWSS(t *testing.T) {
	RunTestsWSS(t, surveyCases())
}

func TestSurveyTTLZero(t *testing.T) {
	SetTTLZero(t, xrespondent.NewSocket)
}

func TestSurveyTTLNegative(t *testing.T) {
	SetTTLNegative(t, xrespondent.NewSocket)
}

func TestSurveyTTLTooBig(t *testing.T) {
	SetTTLTooBig(t, xrespondent.NewSocket)
}

func TestSurveyTTLNotInt(t *testing.T) {
	SetTTLNotInt(t, xrespondent.NewSocket)
}

func TestSurveyTTLSet(t *testing.T) {
	SetTTL(t, xrespondent.NewSocket)
}

func TestSurveyTTLDrop(t *testing.T) {
	TTLDropTest(t, surveyor.NewSocket, respondent.NewSocket,
		xsurveyor.NewSocket, xrespondent.NewSocket)
}
