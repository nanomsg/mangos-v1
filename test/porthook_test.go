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
// WITHOUT WARRANTIES O R CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"sync"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/transport/tcp"
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
	t.Logf("Testing Add Hook")

	srvtest := &hooktest{allow: true, t: t}
	clitest := &hooktest{allow: true, t: t}

	addr := AddrTestTCP

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

	sockreq, err := req.NewSocket()
	if err != nil {
		t.Errorf("NewSocket failed: %v", err)
		return
	}
	defer sockreq.Close()
	sockreq.AddTransport(tcp.NewTransport())
	if sockreq.SetPortHook(clitest.Hook) != nil {
		t.Errorf("SetPortHook result not nil!")
		return
	}
	d, err := sockreq.NewDialer(addr, nil)
	if err != nil {
		t.Errorf("NewDialer failed: %v", err)
		return
	}

	sockrep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("NewSocket failed: %v", err)
		return
	}
	defer sockrep.Close()
	sockrep.AddTransport(tcp.NewTransport())
	if sockrep.SetPortHook(srvtest.Hook) != nil {
		t.Errorf("SetPortHook result not nil!")
		return
	}
	l, err := sockrep.NewListener(addr, nil)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}

	if err := l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	if err := d.Dial(); err != nil {
		t.Errorf("Dial failed: %v", err)
		return
	}

	// wait a second for connection to establish
	// could also issue a req/rep...
	t.Logf("Waiting a bit...")
	time.Sleep(100 * time.Millisecond)

	d.Close()
	l.Close()

	// shut down the server
	sockrep.Close()
	sockreq.Close()

	clitest.Lock()
	defer clitest.Unlock()

	srvtest.Lock()
	defer srvtest.Unlock()

	for i, info := range clitest.expect {
		t.Logf("Exp C[%d]: %s", i, info.String())
	}
	for i, info := range clitest.calls {
		t.Logf("Got C[%d]: %s", i, info.String())
	}
	for i, info := range srvtest.expect {
		t.Logf("Exp S[%d]: %s", i, info.String())
	}
	for i, info := range srvtest.calls {
		t.Logf("Got S[%d]: %s", i, info.String())
	}

	if len(srvtest.calls) != len(srvtest.expect) {
		t.Errorf("Server got wrong # calls, %d != %d", len(srvtest.calls), len(srvtest.expect))
		return
	}
	for i := range srvtest.calls {
		if srvtest.calls[i].String() != srvtest.expect[i].String() {
			t.Errorf("Server hook %d wrong: %s != %s", i)
		}
	}

	if len(clitest.calls) != len(clitest.expect) {
		t.Errorf("Client got wrong # calls, %d != %d", len(clitest.calls), len(clitest.expect))
		return
	}
	for i := range clitest.calls {
		if clitest.calls[i].String() != clitest.expect[i].String() {
			t.Errorf("Server hook %d wrong: %s != %s", i)
		}
	}
}
