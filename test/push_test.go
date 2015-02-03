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

package test

import (
	"testing"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/pull"
	"github.com/gdamore/mangos/protocol/push"
)

type PushTest struct {
	T
}

type PullTest struct {
	T
}

func (pt *PushTest) Init(t *testing.T, addr string) bool {
	var err error
	if pt.Sock, err = push.NewSocket(); err != nil {
		pt.Errorf("NewSocket(): %v", err)
		return false
	}
	return pt.T.Init(t, addr)
}

func (pt *PushTest) SendHook(m *mangos.Message) bool {
	m.Body = append(m.Body, byte(pt.GetSend()))
	return pt.T.SendHook(m)
}

func (pt *PullTest) Init(t *testing.T, addr string) bool {
	var err error
	if pt.Sock, err = pull.NewSocket(); err != nil {
		pt.Errorf("NewSocket(): %v", err)
		return false
	}
	return pt.T.Init(t, addr)
}

func (pt *PullTest) RecvHook(m *mangos.Message) bool {
	if len(m.Body) != 1 {
		pt.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	if m.Body[0] != byte(pt.GetRecv()) {
		pt.Errorf("Wrong message: %d != %d", m.Body[0], byte(pt.GetRecv()))
		return false
	}
	return pt.T.RecvHook(m)
}

func pushCases() []TestCase {
	push := &PushTest{}
	push.ID = 0
	push.MsgSize = 1
	push.WantTx = 200
	push.WantRx = 0
	push.Server = true

	pull := &PullTest{}
	pull.ID = 1
	pull.MsgSize = 1
	pull.WantTx = 0
	pull.WantRx = 200

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

func TestPushPullTLS(t *testing.T) {
	RunTestsTLS(t, pushCases())
}

func TestPushPullWS(t *testing.T) {
	RunTestsWS(t, pushCases())
}

func TestPushPullWSS(t *testing.T) {
	RunTestsWSS(t, pushCases())
}
