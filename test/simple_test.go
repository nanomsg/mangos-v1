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
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pair"
	"nanomsg.org/go-mangos/transport/tcp"

	. "github.com/smartystreets/goconvey/convey"
)

// This test case just tests that the simple Send/Recv (suboptimal) interfaces
// work as advertised.  This covers verification that was reported in GitHub
// issue 139: Race condition in simple Send()/Recv() code

func TestSimpleCorrect(t *testing.T) {

	Convey("We can use the simple Send/Recv API", t, func() {
		tx, e := pair.NewSocket()
		So(e, ShouldBeNil)
		So(tx, ShouldNotBeNil)

		rx, e := pair.NewSocket()
		So(e, ShouldBeNil)
		So(rx, ShouldNotBeNil)

		tx.AddTransport(tcp.NewTransport())
		rx.AddTransport(tcp.NewTransport())

		Convey("When a simple TCP pair is created", func() {

			a := AddrTestTCP()
			e = rx.Listen(a)
			So(e, ShouldBeNil)

			e = tx.Dial(a)
			So(e, ShouldBeNil)

			iter := 100000
			Convey(fmt.Sprintf("We can send/recv %d msgs async", iter), func() {
				wg := &sync.WaitGroup{}
				wg.Add(2)
				goodtx := true
				goodrx := true
				go simpleSend(tx, wg, iter, &goodtx)
				go simpleRecv(rx, wg, iter, &goodrx)
				wg.Wait()
				So(goodtx, ShouldBeTrue)
				So(goodrx, ShouldBeTrue)
				e = tx.Close()
				So(e, ShouldBeNil)
				e = rx.Close()
				So(e, ShouldBeNil)
			})
		})
	})
}

func simpleSend(tx mangos.Socket, wg *sync.WaitGroup, iter int, pass *bool) {
	defer wg.Done()
	var buf [256]byte
	good := true
	for i := 0; i < iter; i++ {
		l := rand.Intn(255) + 1
		buf[0] = uint8(l)
		if e := tx.Send(buf[:l]); e != nil {
			good = false
			break
		}
	}
	*pass = good
}

func simpleRecv(rx mangos.Socket, wg *sync.WaitGroup, iter int, pass *bool) {
	defer wg.Done()
	good := true
	for i := 0; i < iter; i++ {
		buf, e := rx.Recv()
		if buf == nil || e != nil || len(buf) < 1 || len(buf) != int(buf[0]) {
			good = false
			break
		}
	}
	*pass = good
}
