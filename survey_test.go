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
	"runtime"
	"testing"
	"time"
)

type SurveyTest struct {
	resp []bool
	spTestCaseImpl
}

type RespTest struct {
	spTestCaseImpl
}

func (t *SurveyTest) SendHook(m *Message) bool {
	m.Body = append(m.Body, byte(t.GetSend()))
	return t.spTestCaseImpl.SendHook(m)
}

func (t *SurveyTest) RecvHook(m *Message) bool {
	if len(m.Body) != 1 {
		t.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	if int(m.Body[0]) > len(t.resp) {
		t.Errorf("Response from unknown id %d", m.Body[0])
		return false
	}

	if t.resp[m.Body[0]] {
		t.Logf("Duplicate response from id %d", m.Body[0])
	} else {
		t.Logf("Response from id %d", m.Body[0])
		t.resp[m.Body[0]] = true
		t.BumpRecv()
	}
	return true
}

func (t *RespTest) RecvHook(m *Message) bool {
	if len(m.Body) != 1 {
		t.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	t.Logf("Got survey ID %d", m.Body[0])

	// reply
	newm := t.NewMessage()
	newm.Body = append(newm.Body, byte(t.GetID()))
	t.SendMsg(newm)
	t.spTestCaseImpl.RecvHook(m)
	return true
}

func testSurvey(t *testing.T, addr string) {
	nresp := 3

	clients := make([]TestCase, nresp)
	for i := 0; i < nresp; i++ {
		resp := &RespTest{}
		resp.id = i + 1
		resp.msgsz = 1
		resp.wanttx = 0 // reply is done in response to receipt
		resp.wantrx = 1
		clients[i] = resp
	}

	rcv := &SurveyTest{}
	rcv.resp = make([]bool, nresp+1)
	rcv.id = 0
	rcv.msgsz = 1
	rcv.wanttx = 1
	rcv.wantrx = int32(nresp)
	rcv.txdelay = 20 * time.Millisecond

	servers := []TestCase{rcv}

	RunTests(t, addr, SurveryorName, servers, RespondentName, clients)
}

func TestSurveyTCP(t *testing.T) {
	testSurvey(t, "tcp://127.0.0.1:3699")
}

// Disabled for now.  For whatever reason, IPC seems incredibly prone to
// frequent slow start problems.  (It works sometimes, but often misses
// packets during startup.)
func xTestSurveyIPC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows")
	}
	testSurvey(t, "ipc:///tmp/mytest")
}

func TestSurveyInp(t *testing.T) {
	testSurvey(t, "inproc://myname")
}
