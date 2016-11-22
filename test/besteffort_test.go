// Copyright 2016 The Mangos Authors
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
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pair"
	"github.com/go-mangos/mangos/transport/tcp"

	. "github.com/smartystreets/goconvey/convey"
)

func testBestEffort(addr string, tran mangos.Transport) {
	timeout := time.Millisecond * 10
	msg := []byte{'A', 'B', 'C'}

	Convey("Given a new listening (unconnected) socket", func() {
		rp, err := pair.NewSocket()
		So(err, ShouldBeNil)
		So(rp, ShouldNotBeNil)

		defer rp.Close()
		rp.AddTransport(tran)

		err = rp.SetOption(mangos.OptionWriteQLen, 0)
		So(err, ShouldBeNil)

		err = rp.SetOption(mangos.OptionSendDeadline, timeout)
		So(err, ShouldBeNil)

		err = rp.Listen(addr)
		So(err, ShouldBeNil)

		Convey("Messages are discarded in besteffort mode", func() {
			err = rp.SetOption(mangos.OptionBestEffort, true)
			So(err, ShouldBeNil)

			for i := 0; i < 2; i++ {
				err = rp.Send(msg)
				So(err, ShouldBeNil)
			}
		})

		Convey("Messages timeout when not in besteffort mode", func() {
			err = rp.SetOption(mangos.OptionBestEffort, false)
			So(err, ShouldBeNil)

			for i := 0; i < 2; i++ {
				err = rp.Send(msg)
				So(err, ShouldEqual, mangos.ErrSendTimeout)
			}
		})
	})
}

func TestBestEffortTCP(t *testing.T) {
	Convey("Testing TCP Best Effort", t, func() {
		testBestEffort(AddrTestTCP, tcp.NewTransport())
	})
}
