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
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
)

type reqTest struct {
	cur int32
	tot int32
	T
}

type repTest struct {
	T
}

// For now, we take a very simple pairwise approach to req/rep.  We should
// consider additional tests for raw mode multiple responders.

func (rt *reqTest) Init(t *testing.T, addr string) bool {
	var err error
	if rt.Sock, err = req.NewSocket(); err != nil {
		rt.Errorf("NewSocket(): %v", err)
		return false
	}
	rt.cur = 0
	rt.tot = 0
	return rt.T.Init(t, addr)
}

func (rt *reqTest) SendHook(m *mangos.Message) bool {
	m.Body = append(m.Body, byte(rt.GetSend()))
	rt.tot = rt.GetSend()
	return rt.T.SendHook(m)
}

func (rt *reqTest) RecvHook(m *mangos.Message) bool {
	if len(m.Body) != 1 {
		rt.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	if m.Body[0] != byte(rt.GetRecv()) {
		rt.Errorf("Wrong message: %d != %d", m.Body[0], byte(rt.GetRecv()))
		return false
	}
	// After one reply received, send the next
	newm := rt.NewMessage()
	rt.tot++
	rt.cur++
	newm.Body = append(newm.Body, byte(rt.tot))
	rt.SendMsg(newm)
	rt.T.RecvHook(m)
	return rt.cur == rt.tot
}

func (rt *repTest) Init(t *testing.T, addr string) bool {
	var err error
	if rt.Sock, err = rep.NewSocket(); err != nil {
		rt.Errorf("NewSocket(): %v", err)
		return false
	}
	return rt.T.Init(t, addr)
}

func (rt *repTest) RecvHook(m *mangos.Message) bool {
	if len(m.Body) != 1 {
		rt.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}

	// reply
	newm := rt.NewMessage()
	newm.Body = append(newm.Body, m.Body...)
	rt.SendMsg(newm)
	return rt.T.RecvHook(m)
}

// NO SLOW START FOR REQ/REP PROTOCOLS!!

func (*repTest) WantRecvStart() bool {
	return false
}

func (*reqTest) WantRecvStart() bool {
	return false
}

func reqRepCases() []TestCase {
	var nresp int32

	nresp = 100

	repc := &repTest{}
	repc.Server = true
	repc.ID = 0
	repc.MsgSize = 1
	repc.WantTx = 0
	repc.WantRx = nresp
	repc.debug = true

	reqc := &reqTest{}
	reqc.debug = true
	reqc.ID = 1
	reqc.MsgSize = 1
	reqc.WantTx = 1
	reqc.WantRx = nresp
	reqc.tot = nresp

	return []TestCase{repc, reqc}
}

func TestReqRepTCP(t *testing.T) {
	RunTestsTCP(t, reqRepCases())
}

func TestReqRepIPC(t *testing.T) {
	RunTestsIPC(t, reqRepCases())
}

func TestReqRepInp(t *testing.T) {
	RunTestsInp(t, reqRepCases())
}

func TestReqRepTLS(t *testing.T) {
	RunTestsTLS(t, reqRepCases())
}

func TestReqRepWS(t *testing.T) {
	RunTestsWS(t, reqRepCases())
}

func TestReqRepWSS(t *testing.T) {
	RunTestsWSS(t, reqRepCases())
}
