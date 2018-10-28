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
	"testing"

	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/bus"
	"nanomsg.org/go/mangos/v2/protocol/xbus"
	"nanomsg.org/go/mangos/v2/transport/inproc"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBusDevice(t *testing.T) {

	Convey("Testing Bus devices", t, func() {
		Convey("Device works with xbus", func() {
			s1, err := xbus.NewSocket()
			So(err, ShouldBeNil)
			So(s1, ShouldNotBeNil)
			defer s1.Close()
			s1.AddTransport(inproc.NewTransport())
			So(mangos.Device(s1, s1), ShouldBeNil)
		})
		Convey("Device does not work with cooked", func() {
			s1, err := bus.NewSocket()
			So(err, ShouldBeNil)
			So(s1, ShouldNotBeNil)
			defer s1.Close()
			s1.AddTransport(inproc.NewTransport())
			So(mangos.Device(s1, s1), ShouldBeError)
		})

		Convey("Create bus device topo", func() {

			s1, err := xbus.NewSocket()
			So(err, ShouldBeNil)
			So(s1, ShouldNotBeNil)
			defer s1.Close()
			s1.AddTransport(inproc.NewTransport())
			So(s1.Listen("inproc://busdevicetest"), ShouldBeNil)
			// Create the device
			So(mangos.Device(s1, s1), ShouldBeNil)

			c1, err := bus.NewSocket()
			So(err, ShouldBeNil)
			defer c1.Close()
			c1.AddTransport(inproc.NewTransport())

			c2, err := bus.NewSocket()
			So(err, ShouldBeNil)
			defer c2.Close()
			c2.AddTransport(inproc.NewTransport())

			So(c1.SetOption(mangos.OptionRecvDeadline, time.Millisecond*100), ShouldBeNil)
			So(c2.SetOption(mangos.OptionRecvDeadline, time.Millisecond*100), ShouldBeNil)

			So(c1.Dial("inproc://busdevicetest"), ShouldBeNil)
			So(c2.Dial("inproc://busdevicetest"), ShouldBeNil)

			Convey("Bus forwards", func() {
				m := mangos.NewMessage(0)
				m.Body = append(m.Body, []byte{1, 2, 3, 4}...)

				// Because dial is not synchronous...
				time.Sleep(time.Millisecond * 100)

				So(c1.SendMsg(m), ShouldBeNil)
				m2, e := c2.RecvMsg()
				So(e, ShouldBeNil)
				So(m2, ShouldNotBeNil)
				So(len(m2.Body), ShouldEqual, 4)

				Convey("But not to ourself", func() {
					m3, e := c1.RecvMsg()
					So(e, ShouldBeError)
					So(m3, ShouldBeNil)
				})
			})
		})
	})
}
