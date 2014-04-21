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

type pairTest struct {
	testCase
}

func (pt *pairTest) Init(t *testing.T, addr string) bool {
	pt.proto = PairName
	return pt.testCase.Init(t, addr)
}

func (pt *pairTest) SendHook(m *Message) bool {
	m.Body = append(m.Body, byte(pt.GetSend()))
	return pt.testCase.SendHook(m)
}

func (pt *pairTest) RecvHook(m *Message) bool {
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

func pairCases() []TestCase {
	snd := &pairTest{}
	snd.id = 0
	snd.msgsz = 1
	snd.wanttx = 200
	snd.wantrx = 300
	snd.server = true

	rcv := &pairTest{}
	rcv.id = 1
	rcv.msgsz = 1
	rcv.wanttx = 300
	rcv.wantrx = 200

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
