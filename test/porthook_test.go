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
	"sync"
	"testing"
	"time"

	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/rep"
	"nanomsg.org/go-mangos/protocol/req"
	"nanomsg.org/go-mangos/transport/tcp"

	. "github.com/smartystreets/goconvey/convey"
)

type hookinfo struct {
	action mangos.PortAction
	server bool
	isopen bool
	addr   string
}

func (i hookinfo) String() string {
	var s string
	switch i.action {
	case mangos.PortActionAdd:
		s = "Add "
	case mangos.PortActionRemove:
		s = "Rem "
	default:
		s = "WTH "
	}
	if i.server {
		s += "SRV "
	} else {
		s += "CLI "
	}
	s += i.addr + " "
	if i.isopen {
		s += "Open "
	} else {
		s += "Closed "
	}
	return s
}

type hooktest struct {
	t      *testing.T
	calls  []hookinfo
	expect []hookinfo
	allow  bool
	sync.Mutex
}

func (h *hooktest) Hook(action mangos.PortAction, p mangos.Port) bool {
	h.t.Logf("Hook called - %v!", action)
	i := hookinfo{action: action, addr: p.Address(), isopen: p.IsOpen(), server: p.IsServer()}
	h.Lock()
	h.calls = append(h.calls, i)
	h.Unlock()
	return h.allow
}

func TestPortHook(t *testing.T) {
	Convey("Testing Add Hook", t, func() {

		srvtest := &hooktest{allow: true, t: t}
		clitest := &hooktest{allow: true, t: t}

		addr := AddrTestTCP()

		srvtest.expect = []hookinfo{{
			action: mangos.PortActionAdd,
			addr:   addr,
			server: true,
			isopen: true,
		}, {
			action: mangos.PortActionRemove,
			addr:   addr,
			server: true,
			isopen: false,
		}}

		clitest.expect = []hookinfo{{
			action: mangos.PortActionAdd,
			addr:   addr,
			server: false,
			isopen: true,
		}, {
			action: mangos.PortActionRemove,
			addr:   addr,
			server: false,
			isopen: false,
		}}

		Convey("Given a REQ & REP sockets", func() {
			sockreq, err := req.NewSocket()
			So(err, ShouldBeNil)
			So(sockreq, ShouldNotBeNil)

			defer sockreq.Close()
			sockreq.AddTransport(tcp.NewTransport())

			sockrep, err := rep.NewSocket()
			So(err, ShouldBeNil)
			So(sockrep, ShouldNotBeNil)

			defer sockrep.Close()
			sockrep.AddTransport(tcp.NewTransport())

			d, err := sockreq.NewDialer(addr, nil)
			So(err, ShouldBeNil)
			So(d, ShouldNotBeNil)

			l, err := sockrep.NewListener(addr, nil)
			So(err, ShouldBeNil)
			So(l, ShouldNotBeNil)

			Convey("We can set port hooks", func() {
				hook := sockreq.SetPortHook(clitest.Hook)
				So(hook, ShouldBeNil)

				hook = sockrep.SetPortHook(srvtest.Hook)
				So(hook, ShouldBeNil)

				Convey("And establish a connection", func() {
					err = l.Listen()
					So(err, ShouldBeNil)

					err = d.Dial()
					So(err, ShouldBeNil)

					// time for conn to establish
					time.Sleep(time.Millisecond * 100)

					// Shutdown the sockets
					d.Close()
					l.Close()

					sockrep.Close()
					sockreq.Close()

					Convey("The hooks were called", func() {

						time.Sleep(100 * time.Millisecond)

						clitest.Lock()
						defer clitest.Unlock()

						srvtest.Lock()
						defer srvtest.Unlock()

						So(len(srvtest.calls), ShouldEqual, len(srvtest.expect))
						for i := range srvtest.calls {
							So(srvtest.calls[i].String(), ShouldEqual, srvtest.expect[i].String())
						}
						So(len(clitest.calls), ShouldEqual, len(clitest.expect))
						for i := range clitest.calls {
							So(clitest.calls[i].String(), ShouldEqual, clitest.expect[i].String())
						}
					})
				})
			})
		})
	})
}
