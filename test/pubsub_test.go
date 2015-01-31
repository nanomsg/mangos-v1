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
	"bytes"
	"testing"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/pub"
	"github.com/gdamore/mangos/protocol/sub"
)

var publish = []string{
	"/some/like/it/hot",
	"/some/where",
	"/over/the",
	"/rainbow",
	"\\\\C\\SPOT\\RUN",
	"The Quick Brown Fox",
	"END"}

type subTest struct {
	T
}

type pubTest struct {
	pubidx int
	T
}

func (st *subTest) Init(t *testing.T, addr string) bool {
	var err error
	if st.Sock, err = sub.NewSocket(); err != nil {
		st.Errorf("NewSocket(): %v", err)
		return false
	}
	rv := st.T.Init(t, addr)
	// We have to subscribe to the START message!
	if !rv {
		return false
	}
	err = st.Sock.SetOption(mangos.OptionSubscribe, []byte("START"))
	if err != nil {
		st.Errorf("Failed subscription to START: %v", err)
		return false
	}

	err = st.Sock.SetOption(mangos.OptionSubscribe, []byte("END"))
	if err != nil {
		st.Errorf("Failed to subscribe to END: %v", err)
		return false
	}

	err = st.Sock.SetOption(mangos.OptionSubscribe, []byte("/rain"))
	if err != nil {
		st.Errorf("Failed subscribing to the rain: %v", err)
		return false
	}

	return true
}

func (pt *pubTest) Init(t *testing.T, addr string) bool {
	pt.pubidx = 0
	var err error
	if pt.Sock, err = pub.NewSocket(); err != nil {
		pt.Errorf("NewSocket(): %v", err)
		return false
	}
	return pt.T.Init(t, addr)
}

func (st *subTest) RecvHook(m *mangos.Message) bool {
	switch {
	case bytes.HasPrefix(m.Body, []byte("END")):
		st.BumpRecv()
		return true
	case bytes.HasPrefix(m.Body, []byte("/rain")):
		st.BumpRecv()
		return true
	default:
		st.Errorf("Unexpected message! %v", m)
		return false
	}
}

func (pt *pubTest) SendHook(m *mangos.Message) bool {
	if pt.pubidx >= len(publish) {
		pt.Errorf("Nothing left to send! (%d/%d)", pt.pubidx, len(publish))
		return false
	}
	m.Body = append(m.Body, []byte(publish[pt.pubidx])...)
	pt.Debugf("Sending %d, %s", pt.pubidx, string(m.Body))
	pt.pubidx++
	return pt.T.SendHook(m)
}

func pubCases() []TestCase {

	nsub := 5
	cases := make([]TestCase, nsub+1)

	pub := &pubTest{}
	pub.WantTx = int32(len(publish))
	pub.WantRx = 0
	pub.Server = true
	cases[0] = pub

	for i := 0; i < nsub; i++ {
		sub := &subTest{}
		sub.WantRx = 2
		sub.ID = i + 1
		cases[i+1] = sub
	}

	return cases

}

func TestPubSubTCP(t *testing.T) {
	RunTestsTCP(t, pubCases())
}

func TestPubSubIPC(t *testing.T) {
	RunTestsIPC(t, pubCases())
}

func TestPubSubInp(t *testing.T) {
	RunTestsInp(t, pubCases())
}

func TestPubSubTLS(t *testing.T) {
	RunTestsTLS(t, pubCases())
}

func TestPubSubWS(t *testing.T) {
	RunTestsWS(t, pubCases())
}

func TestPubSubWSS(t *testing.T) {
	RunTestsWSS(t, pubCases())
}
