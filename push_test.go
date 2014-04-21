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
	"testing"
)

type PushTest struct {
	testCase
}

type PullTest struct {
	testCase
}

func (pt *PushTest) Init(t *testing.T, addr string) bool {
	pt.proto = PushName
	return pt.testCase.Init(t, addr)
}

func (pt *PushTest) SendHook(m *Message) bool {
	m.Body = append(m.Body, byte(pt.GetSend()))
	return pt.testCase.SendHook(m)
}

func (pt *PullTest) Init(t *testing.T, addr string) bool {
	pt.proto = PullName
	return pt.testCase.Init(t, addr)
}

func (pt *PullTest) RecvHook(m *Message) bool {
	if len(m.Body) != 1 {
		pt.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	if m.Body[0] != byte(pt.GetRecv()) {
		pt.Errorf("Wrong message: %d != %d", m.Body[0], byte(pt.GetRecv()))
		return false
	}
	return pt.testCase.RecvHook(m)
}

func pushCases() []TestCase {
	push := &PushTest{}
	push.id = 0
	push.msgsz = 1
	push.wanttx = 200
	push.wantrx = 0
	push.server = true

	pull := &PullTest{}
	pull.id = 1
	pull.msgsz = 1
	pull.wanttx = 0
	pull.wantrx = 200

	return []TestCase{push, pull}
}

func TestPushPullInp(t *testing.T) {
	RunTestsInp(t, pushCases())
}

func TestPushPullTCP(t *testing.T) {
	RunTestsTCP(t, pushCases())
}

func TestPushPullIPC(t *testing.T) {
	RunTestsIPC(t, pushCases())
}
