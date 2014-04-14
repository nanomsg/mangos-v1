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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type spTestCaseImpl struct {
	t       *testing.T
	id      int
	addr    string
	msgsz   int // size of messages
	txdelay time.Duration
	wanttx  int32
	wantrx  int32
	numtx   int32
	numrx   int32
	timeout time.Duration
	sock    Socket
	rdoneq  chan struct{}
	sdoneq  chan struct{}
	sync.Mutex
}

type TestCase interface {
	Init(t *testing.T, addr string, proto string) bool
	NewMessage() *Message
	SendMsg(*Message) error
	RecvMsg() (*Message, error)
	SendHook(*Message) bool // Hook to build send message
	RecvHook(*Message) bool // Hook to check recv message
	Logf(string, ...interface{})
	Errorf(string, ...interface{})
	WantSend() int32
	BumpSend()
	GetSend() int32
	WantRecv() int32
	BumpRecv()
	GetRecv() int32
	GetID() int
	Dial() bool
	Listen() bool
	WaitRecv() bool
	RecvDone()
	WaitSend() bool
	SendDone()
	Close()
	SendDelay() time.Duration
}

func (c *spTestCaseImpl) Init(t *testing.T, addr string, proto string) bool {
	// Initial defaults
	var err error
	c.t = t
	c.addr = addr
	c.numrx = 0 // Reset
	c.numtx = 0 // Reset
	c.Lock()
	c.sdoneq = make(chan struct{})
	c.rdoneq = make(chan struct{})
	c.Unlock()
	c.timeout = time.Second * 3
	if c.sock, err = NewSocket(proto); err != nil {
		c.t.Errorf("Id %d: Failed creating socket: %v", c.id, err)
		return false
	}
	return true
}

func (c *spTestCaseImpl) NewMessage() *Message {
	return NewMessage(c.msgsz)
}

func (c *spTestCaseImpl) SendHook(m *Message) bool {
	c.BumpSend()
	return true
}

func (c *spTestCaseImpl) RecvHook(m *Message) bool {
	c.BumpRecv()
	return true
}

func (c *spTestCaseImpl) SendMsg(m *Message) error {
	return c.sock.SendMsg(m)
}

func (c *spTestCaseImpl) RecvMsg() (*Message, error) {
	return c.sock.RecvMsg()
}

func (c *spTestCaseImpl) Logf(f string, v ...interface{}) {
	now := time.Now().Format(time.StampMilli)
	c.t.Logf("%s: Id %d: %s", now, c.id, fmt.Sprintf(f, v...))
}

func (c *spTestCaseImpl) Errorf(f string, v ...interface{}) {
	now := time.Now().Format(time.StampMilli)
	c.t.Errorf("%s: Id %d: %s", now, c.id, fmt.Sprintf(f, v...))
}

func (c *spTestCaseImpl) WantSend() int32 {
	return c.wanttx
}

func (c *spTestCaseImpl) BumpSend() {
	atomic.AddInt32(&c.numtx, 1)
}

func (c *spTestCaseImpl) GetSend() int32 {
	return atomic.AddInt32(&c.numtx, 0)
}

func (c *spTestCaseImpl) WantRecv() int32 {
	return c.wantrx
}

func (c *spTestCaseImpl) BumpRecv() {
	atomic.AddInt32(&c.numrx, 1)
}

func (c *spTestCaseImpl) GetRecv() int32 {
	return atomic.AddInt32(&c.numrx, 0)
}

func (c *spTestCaseImpl) GetID() int {
	return c.id
}

func (c *spTestCaseImpl) SendDone() {
	c.Lock()
	close(c.sdoneq)
	c.Unlock()
}

func (c *spTestCaseImpl) RecvDone() {
	c.Lock()
	close(c.rdoneq)
	c.Unlock()
}

func (c *spTestCaseImpl) WaitSend() bool {
	select {
	case <-c.sdoneq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *spTestCaseImpl) WaitRecv() bool {
	select {
	case <-c.rdoneq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *spTestCaseImpl) Dial() bool {
	err := c.sock.Dial(c.addr)
	if err != nil {
		c.Errorf("Dial (%s) failed: %v", c.addr, err)
		return false
	}
	// Allow time for transports to establish connection
	time.Sleep(time.Millisecond * 500)
	return true
}

func (c *spTestCaseImpl) Listen() bool {
	err := c.sock.Listen(c.addr)
	if err != nil {
		c.Errorf("Listen (%s) failed: %v", c.addr, err)
		return false
	}
	// Allow time for transports to establish connection
	time.Sleep(time.Millisecond * 500)
	return true
}

func (c *spTestCaseImpl) Close() {
	if c.sock != nil {
		c.sock.Close()
		c.sock = nil
	}
}

func (c *spTestCaseImpl) SendDelay() time.Duration {
	return c.txdelay
}

func SendTester(c TestCase) bool {

	time.Sleep(c.SendDelay())
	defer c.SendDone()
	for c.GetSend() < c.WantSend() {
		msg := c.NewMessage()
		if !c.SendHook(msg) {
			c.Errorf("SendHook failed")
			return false
		}
		if err := c.SendMsg(msg); err != nil {
			c.Errorf("SendMsg failed: %v", err)
			return false
		}
		c.Logf("Good send (%d/%d)", c.GetSend(), c.WantSend())
	}
	c.Logf("Sent all messages")
	return true
}

func RecvTester(c TestCase) bool {
	defer c.RecvDone()
	for c.GetRecv() < c.WantRecv() {
		msg, err := c.RecvMsg()
		if err != nil {
			c.Errorf("RecvMsg failed: %v", err)
			return false
		}
		if !c.RecvHook(msg) {
			c.Errorf("RecvHook failed")
			return false
		}
		c.Logf("Good recv (%d/%d)", c.GetRecv(), c.WantRecv())
		msg.Free()
	}
	c.Logf("Got all messages")
	return true
}

func WaitTest(c TestCase) {
	if !c.WaitSend() {
		c.Errorf("Timeout waiting for send")
		return
	}
	if !c.WaitRecv() {
		c.Errorf("Timeout waiting for recv")
		return
	}
	c.Logf("Testing complete")
}

func StartDialTest(c TestCase) {
	go func() {
		if !c.Dial() {
			c.SendDone()
			c.RecvDone()
			return
		}
		go RecvTester(c)
		// Give receiver a chance to start before sending traffic
		time.Sleep(100 * time.Millisecond)
		go SendTester(c)
	}()
}

func StartListenTest(c TestCase) {
	go func() {

		if !c.Listen() {
			c.SendDone()
			c.RecvDone()
			return
		}
		go RecvTester(c)
		// Give receiver a chance to start before sending traffic
		time.Sleep(100 * time.Millisecond)
		go SendTester(c)
	}()

}

func RunTests(t *testing.T, addr string, srvProto string, servers []TestCase,
	cliProto string, clients []TestCase) {

	// We need to inject a slight bit of sleep to allow any sessions to
	// drain before we close connections.
	defer time.Sleep(50 * time.Millisecond)

	t.Logf("Address %s, %d %s Servers, %d %s Clients", addr, len(servers),
		srvProto, len(clients), cliProto)
	for i := range servers {
		if !servers[i].Init(t, addr, srvProto) {
			return
		}
		defer servers[i].Close()
	}
	for i := range clients {
		if !clients[i].Init(t, addr, cliProto) {
			return
		}
		defer clients[i].Close()
	}
	for i := range servers {
		StartListenTest(servers[i])
	}
	for i := range clients {
		StartDialTest(clients[i])
	}
	for i := range servers {
		WaitTest(servers[i])
	}
	for i := range clients {
		WaitTest(clients[i])
	}
}

// This runs all tests on all transports.
func RunTestsTransports(t *testing.T, srvProto string, servers []TestCase,
	cliProto string, clients []TestCase) {
	//RunTests(t, "inproc://MYNAME", srvProto, servers, cliProto, clients)
	//RunTests(t, "ipc:///tmp/testing", srvProto, servers, cliProto, clients)
	RunTests(t, "tcp://127.0.0.1:3939", srvProto, servers, cliProto, clients)
}
