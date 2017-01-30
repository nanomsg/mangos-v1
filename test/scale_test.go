// +build !race
// Copyright 2017 The Mangos Authors
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

// This simple test just fires off a crapton of inproc clients, to see
// how many connections we could handle.  We do this using inproc, because
// we would absolutely exhaust TCP ports before we would hit any of the
// natural limits.  The inproc transport has no such limits, so we are
// effectively just testing goroutine scalability, which is what we want.
// The intention is to demonstrate that mangos can address the C10K problem
// without breaking a sweat.

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/protocol/req"
	"github.com/go-mangos/mangos/transport/inproc"
)

func scalabilityClient(errp *error, loops int, wg *sync.WaitGroup) {
	defer wg.Done()
	sock, err := req.NewSocket()
	if err != nil {
		*errp = err
		return
	}
	sock.AddTransport(inproc.NewTransport())
	defer sock.Close()
	if err := sock.Dial("inproc://atscale"); err != nil {
		*errp = err
		return
	}

	for i := 0; i < loops; i++ {
		// Inject a random sleep to avoid pounding the CPU too hard.
		time.Sleep(time.Duration(rand.Int31n(1000)) * time.Microsecond)

		msg := mangos.NewMessage(3)
		msg.Body = append(msg.Body, []byte("ping")...)
		if err := sock.SendMsg(msg); err != nil {
			*errp = err
			return
		}

		if msg, err = sock.RecvMsg(); err != nil {
			*errp = err
			return
		}
		if string(msg.Body) != "pong" {
			*errp = errors.New("response mismatch")
			return
		}
		msg.Free()
	}
}

func scalabilityServer(sock mangos.Socket) {
	defer sock.Close()
	for {
		msg, e := sock.RecvMsg()
		if e != nil {
			return
		}
		msg.Body = append(msg.Body[:0], []byte("pong")...)
		e = sock.SendMsg(msg)
		if e != nil {
			return
		}
	}
}

func TestScalability(t *testing.T) {
	// Beyond this things get crazy.
	// Note that the thread count actually indicates that you will
	// have this many client sockets, and an equal number of server
	// side pipes.  On my Mac, 20K leads to around 30 sec to run the
	// program, whereas 10k can run in under 10 sec.  This proves we
	// can handle 10K connections.
	loops := 1
	threads := 10000

	errs := make([]error, threads)

	ssock, e := rep.NewSocket()
	if e != nil {
		t.Fatalf("Cannot make server socket: %v", e)
	}
	ssock.AddTransport(inproc.NewTransport())
	if e = ssock.Listen("inproc://atscale"); e != nil {
		t.Fatalf("Cannot listen: %v", e)
	}
	time.Sleep(time.Millisecond * 100)
	go scalabilityServer(ssock)
	wg := &sync.WaitGroup{}
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go scalabilityClient(&errs[i], loops, wg)
	}

	wg.Wait()
	for i := 0; i < threads; i++ {
		if errs[i] != nil {
			t.Fatalf("Test failed: %v", errs[i])
		}
	}
}
