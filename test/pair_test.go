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
	"github.com/gdamore/mangos/protocol/pair"
)

type pairTest struct {
	T
}

func (pt *pairTest) Init(t *testing.T, addr string) bool {
	var err error
	if pt.Sock, err = pair.NewSocket(); err != nil {
		pt.Errorf("NewSocket failed: %v", err)
		return false
	}
	return pt.T.Init(t, addr)
}

func (pt *pairTest) SendHook(m *mangos.Message) bool {
	m.Body = append(m.Body, byte(pt.GetSend()))
	return pt.T.SendHook(m)
}

func (pt *pairTest) RecvHook(m *mangos.Message) bool {
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

func pairCases() []TestCase {
	snd := &pairTest{}
	snd.ID = 0
	snd.MsgSize = 1
	snd.WantTx = 200
	snd.WantRx = 300
	snd.Server = true

	rcv := &pairTest{}
	rcv.ID = 1
	rcv.MsgSize = 1
	rcv.WantTx = 300
	rcv.WantRx = 200

	return []TestCase{snd, rcv}
}

func TestPairInp(t *testing.T) {
	RunTestsInp(t, pairCases())
}

func TestPairTCP(t *testing.T) {
	RunTestsTCP(t, pairCases())
}

func TestPairIPC(t *testing.T) {
	RunTestsIPC(t, pairCases())
}

func TestPairTLS(t *testing.T) {
	RunTestsTLS(t, pairCases())
}

func TestPairWS(t *testing.T) {
	RunTestsWS(t, pairCases())
}

func TestPairWSS(t *testing.T) {
	RunTestsWSS(t, pairCases())
}
