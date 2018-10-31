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
	"nanomsg.org/go/mangos/v2/protocol/bus"
)

type busTest struct {
	nbus   uint32
	nstart uint32
	start  map[uint32]bool
	resp   map[uint32]uint32
	send   uint32
	T
}

func (bt *busTest) Init(t *testing.T, addr string) bool {
	var err error
	bt.resp = make(map[uint32]uint32)
	bt.start = make(map[uint32]bool)
	bt.send = 0
	bt.nstart = 0
	if bt.Sock, err = bus.NewSocket(); err != nil {
		bt.Errorf("NewSocket(): %v", err)
		return false
	}
	return bt.T.Init(t, addr)
}

func (bt *busTest) WaitRecv() bool {
	// We provide our own, so that we can provide more
	// detailed error reporting about packet mismatches.
	r := bt.T.WaitRecv()
	bt.Lock()
	defer bt.Unlock()
	if !r {
		bt.Errorf("Timeout socket %d", uint32(bt.GetID()))
		for v, c := range bt.resp {
			bt.Errorf("Last packet %d from %d", c, v)
		}
	}
	return r
}

func (bt *busTest) RecvStart() bool {
	m, err := bt.RecvMsg()
	if err != nil {
		bt.Errorf("RecvMsg failed: %v", err)
		return false
	}
	defer m.Free()
	v, ok := ParseStart(m)
	if !ok {
		bt.Errorf("Bad START message received: %v", m)
		return false
	}
	if v == uint32(bt.GetID()) {
		bt.Errorf("Got my own START message")
		return false
	}
	if yes, ok := bt.start[v]; ok && yes {
		bt.Logf("Got dup START from %d", v)
		return false
	}
	bt.Debugf("Got START from %d", v)
	bt.start[v] = true
	bt.nstart++
	if bt.Server {
		return bt.nstart == bt.nbus-1
	}
	return true
}

func (bt *busTest) SendHook(m *mangos.Message) bool {
	bt.Lock()
	defer bt.Unlock()
	v := uint32(bt.GetID())
	w := bt.send
	bt.send++
	m.Body = m.Body[0:8]

	binary.BigEndian.PutUint32(m.Body, v)
	binary.BigEndian.PutUint32(m.Body[4:], w)

	// Inject a sleep to avoid overwhelming the bus and dropping messages.
	//d := time.Duration(rand.Uint32() % 10000)
	//time.Sleep(d * time.Microsecond)

	return bt.T.SendHook(m)
}

func (bt *busTest) RecvHook(m *mangos.Message) bool {
	bt.Lock()
	defer bt.Unlock()
	if len(m.Body) < 8 {
		bt.Errorf("Recv message length %d < 8", len(m.Body))
		return false
	}

	v := binary.BigEndian.Uint32(m.Body)
	w := binary.BigEndian.Uint32(m.Body[4:])
	if v == uint32(bt.GetID()) {
		bt.Errorf("Got my own message %v", m.Body)
		return false
	}
	if w != uint32(bt.resp[v]) {
		bt.Errorf("Got wrong message #%d (!= %d) from %d", w, bt.resp[v], v)
		return false
	}
	bt.resp[v]++
	bt.Debugf("Response %d from id %d", w, v)
	bt.BumpRecv()
	return true
}

func busCases() []TestCase {

	nbus := 5
	npkt := 7

	cases := make([]TestCase, nbus)
	for i := 0; i < nbus; i++ {
		bus := &busTest{}
		bus.ID = i
		bus.nbus = uint32(nbus)
		bus.MsgSize = 8
		bus.WantTx = int32(npkt)
		bus.txdelay = time.Second / 7
		// Only the server receives from all peers.  The clients
		// only get packets sent by the server.
		if i == 0 {
			bus.Server = true
			bus.WantRx = int32(npkt * (nbus - 1))
		} else {
			bus.WantRx = int32(npkt)
		}
		cases[i] = bus
	}
	return cases
}

func TestBusInp(t *testing.T) {
	RunTestsInp(t, busCases())
}

func TestBusTCP(t *testing.T) {
	RunTestsTCP(t, busCases())
}

func TestBusIPC(t *testing.T) {
	RunTestsIPC(t, busCases())
}

func TestBusTLS(t *testing.T) {
	RunTestsTLS(t, busCases())
}
