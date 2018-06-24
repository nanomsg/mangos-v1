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

	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/rep"
	"nanomsg.org/go-mangos/protocol/req"
	"nanomsg.org/go-mangos/transport/inproc"

	. "github.com/smartystreets/goconvey/convey"
)

func TestReqRetry(t *testing.T) {
	Convey("Testing Req Retry", t, func() {
		//addr := "inproc://port"
		addr := AddrTestInp()

		// Let's first try request issued with no connection, and
		// completing immediately after connect is established.
		// Req will have multiple connections to separate servers,
		Convey("Given a REQ & REP sockets", func() {
			sockreq, err := req.NewSocket()
			So(err, ShouldBeNil)
			So(sockreq, ShouldNotBeNil)

			defer sockreq.Close()
			sockreq.AddTransport(inproc.NewTransport())

			sockrep, err := rep.NewSocket()
			So(err, ShouldBeNil)
			So(sockrep, ShouldNotBeNil)
			defer sockrep.Close()
			sockrep.AddTransport(inproc.NewTransport())

			d, err := sockreq.NewDialer(addr, nil)
			So(err, ShouldBeNil)
			So(d, ShouldNotBeNil)

			err = sockreq.SetOption(mangos.OptionReconnectTime, time.Millisecond*100)
			So(err, ShouldBeNil)

			l, err := sockrep.NewListener(addr, nil)
			So(err, ShouldBeNil)
			So(l, ShouldNotBeNil)

			err = d.Dial()
			So(err, ShouldBeNil)

			Convey("A request is issued on late server connect", func() {
				m := mangos.NewMessage(0)
				m.Body = append(m.Body, []byte("hello")...)
				err = sockreq.SendMsg(m)
				So(err, ShouldBeNil)

				err = l.Listen()
				So(err, ShouldBeNil)

				m, err = sockrep.RecvMsg()
				So(m, ShouldNotBeNil)
				So(err, ShouldBeNil)

				m.Body = append(m.Body, []byte(" there")...)
				err = sockrep.SendMsg(m)
				So(err, ShouldBeNil)

				m, err = sockreq.RecvMsg()
				So(m, ShouldNotBeNil)
				So(err, ShouldBeNil)

				m.Free()
			})

			// Following is skipped for now because of the backout
			// of e5e6478f44cda1eb8427b590755270e2704a990d
			SkipConvey("A request is reissued on server re-connect", func() {

				rep2, err := rep.NewSocket()
				So(err, ShouldBeNil)
				So(rep2, ShouldNotBeNil)
				defer rep2.Close()
				rep2.AddTransport(inproc.NewTransport())

				l2, err := rep2.NewListener(addr, nil)
				So(err, ShouldBeNil)
				So(l2, ShouldNotBeNil)

				err = l.Listen()
				time.Sleep(time.Millisecond * 50)

				m := mangos.NewMessage(0)
				m.Body = append(m.Body, []byte("hello")...)
				err = sockreq.SendMsg(m)
				So(err, ShouldBeNil)

				So(err, ShouldBeNil)

				m, err = sockrep.RecvMsg()
				So(m, ShouldNotBeNil)
				So(err, ShouldBeNil)

				// Now close the connection -- no reply!
				l.Close()
				sockrep.Close()

				// Open the new one on the other socket
				err = l2.Listen()
				m, err = rep2.RecvMsg()
				So(m, ShouldNotBeNil)
				So(err, ShouldBeNil)

				m.Body = append(m.Body, []byte(" again")...)
				err = rep2.SendMsg(m)
				So(err, ShouldBeNil)

				m, err = sockreq.RecvMsg()
				So(m, ShouldNotBeNil)
				So(err, ShouldBeNil)

				m.Free()
			})

		})
	})
}
