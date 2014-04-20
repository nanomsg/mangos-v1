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
	"encoding/binary"
	"testing"
	"time"
)

type surveyTest struct {
	nresp  int32
	nstart int32
	resp   map[uint32]bool
	start  map[uint32]bool
	testCase
}

type responderTest struct {
	testCase
}

func (st *surveyTest) Init(t *testing.T, addr string) bool {
	st.proto = SurveryorName
	st.resp = make(map[uint32]bool)
	st.start = make(map[uint32]bool)
	st.nstart = 0
	return st.testCase.Init(t, addr)
}

func (st *surveyTest) SendHook(m *Message) bool {
	m.Body = m.Body[0:4]
	binary.BigEndian.PutUint32(m.Body, uint32(st.GetSend()))
	return st.testCase.SendHook(m)
}

func (st *surveyTest) RecvHook(m *Message) bool {
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

func (st *surveyTest) RecvStart() bool {
	m, err := st.RecvMsg()
	if err != nil {
		st.Errorf("RecvMsg failed: %v", err)
		return false
	}
	defer m.Free()
	v, ok := ParseStart(m)
	if !ok {
		st.Errorf("Bad START message received: %v", m)
		return false
	}
	if yes, ok := st.start[v]; ok && yes {
		st.Debugf("Got dup START from %d", v)
		return false
	}
	st.Debugf("Got START from %d", v)
	st.start[v] = true
	st.nstart++
	return st.nstart == st.nresp
}

func (rt *responderTest) Init(t *testing.T, addr string) bool {
	rt.proto = RespondentName
	return rt.testCase.Init(t, addr)
}

func (rt *responderTest) RecvHook(m *Message) bool {
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
	rt.testCase.RecvHook(m)
	return true
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
	surv.server = true
	surv.id = 0
	surv.msgsz = 8
	surv.wanttx = 1
	surv.wantrx = int32(nresp)
	surv.txdelay = 20 * time.Millisecond
	cases[0] = surv

	for i := 0; i < int(nresp); i++ {
		resp := &responderTest{}
		resp.id = i + 1
		resp.msgsz = 8
		resp.wanttx = 0 // reply is done in response to receipt
		resp.wantrx = 1
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
