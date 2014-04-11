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
	"time"
)

type pipelineTester struct {
	id     int
	sock   Socket
	rdoneq chan bool
	sdoneq chan bool
}

func pipelineTestSender(t *testing.T, bt *pipelineTester, cnt int) {
	defer close(bt.sdoneq)
	for i := 0; i < cnt; i++ {
		t.Logf("Push %d: Sending %d", bt.id, i)
		msg := NewMessage(1)
		msg.Body = append(msg.Body, byte(i))
		if err := bt.sock.SendMsg(msg); err != nil {
			t.Errorf("Push fail: %v", bt.id, i, err)
			return
		}
	}
}

func pipelineTestReceiver(t *testing.T, bt *pipelineTester, cnt int) {
	defer close(bt.rdoneq)

	for i := 0; i < cnt; i++ {
		msg, err := bt.sock.RecvMsg()
		if err != nil {
			t.Errorf("Pull fail: %v", err)
			return
		}
		if len(msg.Body) != 1 || msg.Body[0] != byte(i) {
			t.Errorf("Message mismatch (expected %d): %v", i, msg)
			return
		}
		t.Logf("Pull %d: Received", msg.Body[0])

		msg.Free()
	}
}

func pipelineTestNewServer(t *testing.T, addr string, id int) *pipelineTester {
	var err error
	bt := &pipelineTester{id: id, rdoneq: make(chan bool), sdoneq: make(chan bool)}

	if bt.sock, err = NewSocket(PushName); err != nil {
		t.Errorf("Failed getting server %d socket: %v", id, err)
		return nil
	}

	if err = bt.sock.Listen(addr); err != nil {
		t.Errorf("Failed server %d listening: %v", id, err)
		bt.sock.Close()
		return nil
	}
	return bt
}

func pipelineTestNewClient(t *testing.T, addr string, id int) *pipelineTester {
	var err error
	bt := &pipelineTester{id: id, rdoneq: make(chan bool), sdoneq: make(chan bool)}

	if bt.sock, err = NewSocket(PullName); err != nil {
		t.Errorf("Failed getting client %d socket: %v", id, err)
		return nil
	}
	if err = bt.sock.Dial(addr); err != nil {
		t.Errorf("Failed client %d dialing: %v", id, err)
		bt.sock.Close()
		return nil
	}
	return bt
}

func TestPipeLine(t *testing.T) {
	addr := "tcp://127.0.0.1:3539"

	pkts := 5
	var sbts *pipelineTester
	var rbts *pipelineTester

	sbts = pipelineTestNewServer(t, addr, 0)
	defer sbts.sock.Close()
	rbts = pipelineTestNewClient(t, addr, 1)
	defer rbts.sock.Close()
	if sbts == nil || rbts == nil {
		t.Errorf("Failed creating")
		return

	}

	// wait a little bit for connections to establish
	time.Sleep(time.Microsecond * 500)

	t.Logf("Starting send/recv")
	go pipelineTestReceiver(t, rbts, pkts)
	go pipelineTestSender(t, sbts, pkts)

	tmout := time.After(5 * time.Second)
	select {
	case <-sbts.sdoneq:
	case <-tmout:
		t.Errorf("Timeout waiting for sender")
		return
	}

	select {
	case <-rbts.rdoneq:
	case <-tmout:
		t.Errorf("Timeout waiting for receiver")
		return
	}
	t.Logf("All pass")
}
