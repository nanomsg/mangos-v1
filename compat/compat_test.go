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

package nanomsg

import (
	"testing"
	"time"
)

type creqTest struct {
	cur   uint32
	tot   uint32
	ok    bool
	debug bool
	addr  string
	sock  *Socket
	done  chan struct{}
	t     *testing.T
}

type crepTest struct {
	cur   uint32
	tot   uint32
	ok    bool
	debug bool
	addr  string
	sock  *Socket
	done  chan struct{}
	t     *testing.T
}

// For now, we take a very simple pairwise approach to req/rep.  We should
// consider additional tests for raw mode multiple responders.

func (rt *creqTest) Init(t *testing.T, addr string, num uint32) bool {
	var e error
	if rt.sock, e = NewSocket(AF_SP, REQ); e != nil {
		t.Errorf("NewSocket(): %v", e)
		return false
	}
	if e = rt.sock.SetSendTimeout(time.Second); e != nil {
		t.Errorf("Failed SetSendTimeout: %s", e)
		return false
	}
	if e = rt.sock.SetRecvTimeout(time.Second); e != nil {
		t.Errorf("Failed SetRecvTimeout: %s", e)
		return false
	}
	rt.t = t
	rt.cur = 0
	rt.tot = num
	rt.addr = addr
	rt.done = make(chan struct{})
	return true
}

func (rt *creqTest) Finish() {
	rt.ok = rt.cur == rt.tot
	close(rt.done)
}

func (rt *creqTest) DoTest() bool {
	defer rt.Finish()
	if _, err := rt.sock.Connect(rt.addr); err != nil {
		rt.t.Fatalf("Failed to connect: %s", err)
	}
	for rt.cur < rt.tot {
		var e error
		var n int
		m := make([]byte, 1)
		m[0] = byte(rt.cur)

		if rt.debug {
			rt.t.Logf("Send request %d", rt.cur)
		}
		if n, e = rt.sock.Send(m, 0); n != 1 || e != nil {
			rt.t.Errorf("%d/%d: Failed to send, %d sent, err %s", rt.cur, rt.tot, n, e)
			return false
		}
		if rt.debug {
			rt.t.Logf("Sent request %d", rt.cur)
		}

		m, e = rt.sock.Recv(0)
		if e != nil {
			rt.t.Errorf("%d/%d: Failed to recv reply: %s", rt.cur, rt.tot, e)
			return false
		}
		if len(m) != 1 {
			rt.t.Errorf("%d/%d: Got wrong length: %d != 1", rt.cur, rt.tot, len(m))
			return false
		}
		if m[0] != byte(rt.cur) {
			rt.t.Errorf("%d/%d: Got wrong reply: %d != %d", rt.cur, rt.tot, m[0], byte(rt.cur))
			return false
		}
		if rt.debug {
			rt.t.Logf("Got good reply %d", rt.cur)
		}
		rt.cur++
	}
	rt.t.Logf("Req got all %d replies", rt.cur)
	return true
}

func (rt *crepTest) Init(t *testing.T, addr string, num uint32) bool {
	var e error
	if rt.sock, e = NewSocket(AF_SP, REP); e != nil {
		t.Errorf("NewSocket(): %v", e)
		return false
	}
	if e = rt.sock.SetSendTimeout(time.Second); e != nil {
		t.Errorf("Failed SetSendTimeout: %s", e)
		return false
	}
	if e = rt.sock.SetRecvTimeout(time.Second); e != nil {
		t.Errorf("Failed SetRecvTimeout: %s", e)
		return false
	}
	rt.t = t
	rt.cur = 0
	rt.tot = num
	rt.addr = addr
	rt.done = make(chan struct{})
	return true
}

func (rt *crepTest) Finish() {
	rt.sock.Close()
	rt.ok = rt.cur == rt.tot
	if !rt.ok {
		close(rt.done)
	}
}

func (rt *crepTest) DoTest() bool {
	defer rt.Finish()
	if _, err := rt.sock.Bind(rt.addr); err != nil {
		rt.t.Errorf("Failed Bind: %s", err)
		return false
	}

	for rt.cur < rt.tot {
		var m []byte
		var e error
		var n int

		if rt.debug {
			rt.t.Logf("Wait for request %d", rt.cur)
		}
		if m, e = rt.sock.Recv(0); e != nil {
			rt.t.Errorf("%d/%d: Failed to recv request: %s", rt.cur, rt.tot, e)
			return false
		}
		if len(m) != 1 {
			rt.t.Errorf("%d/%d: Got wrong length: %d != 1", rt.cur, rt.tot, len(m))
			return false
		}
		if m[0] != byte(rt.cur) {
			rt.t.Errorf("%d/%d: Got wrong request: %d != %d", rt.cur, rt.tot, m[0], byte(rt.cur))
			return false
		}
		if rt.debug {
			rt.t.Logf("Got good request %d", rt.cur)
		}

		if n, e = rt.sock.Send(m, 0); e != nil || n != 1 {
			rt.t.Errorf("%d/%d: Failed to send reply: %v", rt.cur, rt.tot, e)
			return false
		}
		if rt.debug {
			rt.t.Logf("Sent reply %d", rt.cur)
		}
		rt.cur++
	}
	rt.t.Logf("Rep sent %d replies", rt.cur)
	return true
}

func ReqRepCompat(t *testing.T, addr string, num uint32) {
	req := &creqTest{}
	rep := &crepTest{}
	req.Init(t, addr, num)
	rep.Init(t, addr, num)

	t.Logf("Doing %d exchanges", num)

	go func() {
		rep.ok = rep.DoTest()
	}()

	go func() {
		req.ok = req.DoTest()
	}()

	t.Logf("Waiting for tests to complete")

	select {
	case <-req.done:
		t.Logf("Req complete")
		break
	case <-rep.done:
		t.Logf("Rep complete")
		break
	}

	req.sock.Close()
	rep.sock.Close()
}

func TestCompatTCP(t *testing.T) {
	addr := "tcp://127.0.0.1:34444"
	ReqRepCompat(t, addr, 50000)
}

func TestCompatInp(t *testing.T) {
	addr := "inproc:///SOMENAME"
	ReqRepCompat(t, addr, 500000)
}

// Don't do this right now.  There was a request at one point for send to
// timeout when no transport was present.  But that semantic is diametrically
// opposed with guaranteed delivery.  The only time we block or timeout is
// if there is backpressure.
func TestCompatSendTimeout(t *testing.T) {

	addr := "tcp://127.0.0.1:19"
	push, err := NewSocket(AF_SP, PUSH)
	if err != nil {
		t.Errorf("NewSocket: %v", err)
		return
	}
	if err = push.SetSendTimeout(time.Duration(3000) * time.Millisecond); err != nil {
		t.Errorf("SetSendTimeout: %v", err)
		return
	}
	if _, err = push.Connect(addr); err != nil {
		t.Errorf("Connect: %v", err)
		return
	}
	t.Logf("Trying to send (should timeout)")
	_, err = push.Send([]byte("TEST"), 0)
	if err == nil {
		t.Errorf("Passed, but should have timed out")
		return
	}
	t.Logf("Expected timeout, got %v", err)
}
