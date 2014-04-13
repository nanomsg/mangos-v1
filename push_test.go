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
	"testing"
)

type PushTest struct {
	spTestCaseImpl
}

func (t *PushTest) SendHook(m *Message) bool {
	m.Body = append(m.Body, byte(t.GetSend()))
	return t.spTestCaseImpl.SendHook(m)
}

func (t *PushTest) RecvHook(m *Message) bool {
	if len(m.Body) != 1 {
		t.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	if m.Body[0] != byte(t.GetRecv()) {
		t.Errorf("Wrong message: %d != %d", m.Body[0], byte(t.GetRecv()))
		return false
	}
	return t.spTestCaseImpl.RecvHook(m)
}

func TestPushPull(t *testing.T) {
	snd := &PushTest{}
	snd.id = 0
	snd.msgsz = 1
	snd.wanttx = 200
	snd.wantrx = 0

	rcv := &PushTest{}
	rcv.id = 1
	rcv.msgsz = 1
	rcv.wanttx = 0
	rcv.wantrx = 200

	clients := []TestCase{rcv}
	servers := []TestCase{snd}

	RunTestsTransports(t, PushName, servers, PullName, clients)
}
