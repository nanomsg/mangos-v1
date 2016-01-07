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
	"math/rand"
	"sync"
	"testing"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pair"
	"github.com/go-mangos/mangos/transport/tcp"
)

// This test case just tests that the simple Send/Recv (suboptimal) interfaces
// work as advertised.  This covers verification that was reported in GitHub
// issue 139: Race condition in simple Send()/Recv() code

type simpleTest struct {
	T
}

func TestSimpleCorrect(t *testing.T) {
	tx, e := pair.NewSocket()
	if e != nil {
		t.Fatalf("NewSocket: %v", e)
		return
	}
	rx, e := pair.NewSocket()
	if e != nil {
		t.Fatalf("NewSocket: %v", e)
		return
	}
	tx.AddTransport(tcp.NewTransport())
	rx.AddTransport(tcp.NewTransport())

	if e = rx.Listen(AddrTestTCP); e != nil {
		t.Fatalf("Listen: %v", e)
		return
	}
	if e = tx.Dial(AddrTestTCP); e != nil {
		t.Fatalf("Dial: %v", e)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go simpleSend(t, tx, wg)
	go simpleRecv(t, rx, wg)
	wg.Wait()
}

func simpleSend(t *testing.T, tx mangos.Socket, wg *sync.WaitGroup) {
	var buf [256]byte
	i := 0
	for i = 0; i < 10000; i++ {
		l := rand.Intn(255) + 1
		buf[0] = uint8(l)
		if e := tx.Send(buf[:l]); e != nil {
			t.Fatalf("Send: %v", e)
			break
		}
	}
	t.Logf("Sent %d Msgs", i)
	if e := tx.Close(); e != nil {
		t.Fatalf("Tx Close: %v", e)
	}
	wg.Done()
}

func simpleRecv(t *testing.T, rx mangos.Socket, wg *sync.WaitGroup) {
	i := 0
	for i = 0; i < 10000; i++ {
		buf, e := rx.Recv()
		if e != nil {
			t.Fatalf("Recv: %v", e)
			break
		}
		if len(buf) < 1 {
			t.Fatalf("Recv: empty buf")
			break
		}
		if len(buf) != int(buf[0]) {
			t.Fatalf("Recv: length %d != expected %d",
				len(buf), buf[0])
			break
		}
	}
	t.Logf("Recvd %d Msgs", i)
	if e := rx.Close(); e != nil {
		t.Fatalf("Rx Close: %v", e)
	}
	wg.Done()
}
