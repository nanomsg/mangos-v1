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
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testCase struct {
	t       *testing.T
	debug   bool
	id      int
	addr    string
	proto   string
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
	readyq  chan struct{}
	server  bool
	sync.Mutex
}

type TestCase interface {
	Init(t *testing.T, addr string) bool
	NewMessage() *Message
	SendMsg(*Message) error
	RecvMsg() (*Message, error)
	SendHook(*Message) bool // Hook to build send message
	RecvHook(*Message) bool // Hook to check recv message
	IsServer() bool
	Logf(string, ...interface{})
	Errorf(string, ...interface{})
	Debugf(string, ...interface{})
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
	Ready() bool
	SendStart() bool
	RecvStart() bool
	WaitReady() bool
	WantSendStart() bool
	WantRecvStart() bool
	SetReady()
}

func (c *testCase) Init(t *testing.T, addr string) bool {
	// Initial defaults
	c.Lock()
	defer c.Unlock()
	var err error
	c.t = t
	if len(c.proto) == 0 {
		c.Errorf("Missing protocol")
		return false
	}
	c.addr = addr
	c.numrx = 0 // Reset
	c.numtx = 0 // Reset
	c.sdoneq = make(chan struct{})
	c.rdoneq = make(chan struct{})
	c.readyq = make(chan struct{})
	c.timeout = time.Second * 3
	c.txdelay = time.Duration(time.Now().UnixNano()) % time.Millisecond
	if c.sock, err = NewSocket(c.proto); err != nil {
		c.Errorf("Failed creating socket: %v", err)
		return false
	}
	if !SetTLSTest(t, c.sock) {
		return false
	}
	return true
}

func (c *testCase) NewMessage() *Message {
	return NewMessage(c.msgsz)
}

func (c *testCase) SendHook(m *Message) bool {
	c.BumpSend()
	return true
}

func (c *testCase) RecvHook(m *Message) bool {
	c.BumpRecv()
	return true
}

func (c *testCase) SendMsg(m *Message) error {
	// We sleep a tiny bit, to avoid cramming too many messages on
	// busses, etc. all at once.  (The test requires no dropped messages.)
	time.Sleep(c.SendDelay())
	c.sock.SetOption(OptionSendDeadline, time.Now().Add(time.Second*5))
	return c.sock.SendMsg(m)
}

func (c *testCase) RecvMsg() (*Message, error) {
	c.sock.SetOption(OptionRecvDeadline, time.Now().Add(time.Second*5))
	return c.sock.RecvMsg()
}

func (c *testCase) Debugf(f string, v ...interface{}) {
	if !c.debug {
		return
	}
	now := time.Now().Format(time.StampMilli)
	c.t.Logf("%s: Id %d: %s", now, c.id, fmt.Sprintf(f, v...))
}

func (c *testCase) Logf(f string, v ...interface{}) {
	now := time.Now().Format(time.StampMilli)
	c.t.Logf("%s: Id %d: %s", now, c.id, fmt.Sprintf(f, v...))
}

func (c *testCase) Errorf(f string, v ...interface{}) {
	now := time.Now().Format(time.StampMilli)
	c.t.Errorf("%s: Id %d: %s", now, c.id, fmt.Sprintf(f, v...))
}

func (c *testCase) WantSend() int32 {
	return c.wanttx
}

func (c *testCase) BumpSend() {
	atomic.AddInt32(&c.numtx, 1)
}

func (c *testCase) GetSend() int32 {
	return atomic.AddInt32(&c.numtx, 0)
}

func (c *testCase) WantRecv() int32 {
	return c.wantrx
}

func (c *testCase) BumpRecv() {
	atomic.AddInt32(&c.numrx, 1)
}

func (c *testCase) GetRecv() int32 {
	return atomic.AddInt32(&c.numrx, 0)
}

func (c *testCase) GetID() int {
	return c.id
}

func (c *testCase) SendDone() {
	c.Lock()
	close(c.sdoneq)
	c.Unlock()
}

func (c *testCase) RecvDone() {
	c.Lock()
	close(c.rdoneq)
	c.Unlock()
}

func (c *testCase) WaitSend() bool {
	select {
	case <-c.sdoneq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *testCase) WaitRecv() bool {
	select {
	case <-c.rdoneq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *testCase) Dial() bool {
	err := c.sock.Dial(c.addr)
	if err != nil {
		c.Errorf("Dial (%s) failed: %v", c.addr, err)
		return false
	}
	// Allow time for transports to establish connection
	time.Sleep(time.Millisecond * 500)
	return true
}

func (c *testCase) Listen() bool {
	err := c.sock.Listen(c.addr)
	if err != nil {
		c.Errorf("Listen (%s) failed: %v", c.addr, err)
		return false
	}
	// Allow time for transports to establish connection
	time.Sleep(time.Millisecond * 500)
	return true
}

func (c *testCase) Close() {
	if c.sock != nil {
		c.sock.Close()
		c.sock = nil
	}
}

func (c *testCase) SendDelay() time.Duration {
	return c.txdelay
}

func (c *testCase) IsServer() bool {
	return c.server
}

func (c *testCase) Ready() bool {
	select {
	case <-c.readyq:
		return true
	default:
		return false
	}
}

func (c *testCase) SetReady() {
	close(c.readyq)
}

func (c *testCase) WaitReady() bool {
	select {
	case <-c.readyq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *testCase) SendStart() bool {

	c.Debugf("Sending START")
	m := MakeStart(uint32(c.GetID()))
	if err := c.SendMsg(m); err != nil {
		c.Errorf("SendStart failed: %v", err)
		return false
	}
	return true
}

func (c *testCase) RecvStart() bool {
	m, err := c.RecvMsg()
	if err != nil {
		c.Errorf("RecvMsg failed: %v", err)
		return false
	}
	defer m.Free()

	if v, ok := ParseStart(m); ok {
		c.Debugf("Got START from %d", v)
		return true
	}
	c.Debugf("Got unexpected message: %v", m.Body)
	return false
}

func (c *testCase) WantSendStart() bool {
	return c.wanttx > 0
}

func (c *testCase) WantRecvStart() bool {
	return c.wantrx > 0
}

func MakeStart(v uint32) *Message {
	m := NewMessage(10)
	m.Body = append(m.Body, []byte("START")...)
	m.Body = append(m.Body, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	return m
}

func ParseStart(m *Message) (uint32, bool) {
	if bytes.HasPrefix(m.Body, []byte("START")) && len(m.Body) >= 9 {
		v := binary.BigEndian.Uint32(m.Body[5:])
		return v, true
	}
	return 0, false
}

func sendTester(c TestCase) bool {

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
		c.Debugf("Good send (%d/%d)", c.GetSend(), c.WantSend())
	}
	c.Logf("Sent all %d messages", c.GetSend())
	return true
}

func recvTester(c TestCase) bool {
	defer c.RecvDone()
	for c.GetRecv() < c.WantRecv() {
		msg, err := c.RecvMsg()
		if err != nil {
			c.Errorf("RecvMsg failed: %v", err)
			return false
		}
		if bytes.HasPrefix(msg.Body, []byte("START")) {
			c.Debugf("Extra start message")
			// left over slow start message, toss it
			msg.Free()
			continue
		}
		if !c.RecvHook(msg) {
			c.Errorf("RecvHook failed")
			return false
		}
		c.Debugf("Good recv (%d/%d)", c.GetRecv(), c.WantRecv())
		msg.Free()
	}
	c.Logf("Got all %d messages", c.GetRecv())
	return true
}

func waitTest(c TestCase) {
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

func startDialTest(c TestCase) {
	go func() {
		if !c.Dial() {
			c.SendDone()
			c.RecvDone()
			return
		}
	}()
}

func startListenTest(c TestCase) {
	go func() {

		if !c.Listen() {
			c.SendDone()
			c.RecvDone()
			return
		}
	}()
}

func startSendRecv(c TestCase) {
	go recvTester(c)
	go sendTester(c)
}

func slowStartSender(c TestCase, exitq chan bool) {
	if !c.WantSendStart() {
		return
	}
	for {
		select {
		case <-exitq:
			return
		case <-time.After(time.Millisecond * 100):
			if !c.SendStart() {
				return
			}
		}
	}
}

func slowStartReceiver(c TestCase, wakeq chan bool, exitq chan bool) {
	defer func() {
		wakeq <- true
	}()
	if !c.WantRecvStart() {
		c.SetReady()
		return
	}
	for {
		if c.RecvStart() {
			c.SetReady()
			return
		}
		select {
		case <-exitq:
			return
		default:
		}
	}
}

func slowStart(t *testing.T, cases []TestCase) bool {
	exitq := make(chan bool)
	wakeq := make(chan bool)
	needrdy := len(cases)
	numexit := 0
	numrdy := 0
	exitqclosed := false

	for i := range cases {
		go slowStartSender(cases[i], exitq)
		go slowStartReceiver(cases[i], wakeq, exitq)
	}

	timeout := time.After(time.Second * 5)
	for numexit < needrdy {
		select {
		case <-timeout:
			if !exitqclosed {
				close(exitq)
				exitqclosed = true
			}
			break
		case <-wakeq:
			numexit++
		}
	}

	if !exitqclosed {
		close(exitq)
		exitqclosed = true
	}

	for i := range cases {
		if cases[i].Ready() {
			numrdy++
		} else {
			cases[i].Errorf("Timed out waiting to become ready")
		}
	}
	return numrdy == needrdy
}

func RunTests(t *testing.T, addr string, cases []TestCase) {

	// We need to inject a slight bit of sleep to allow any sessions to
	// drain before we close connections.
	defer time.Sleep(50 * time.Millisecond)

	t.Logf("Address %s, %d Cases", addr, len(cases))
	for i := range cases {
		if !cases[i].Init(t, addr) {
			return
		}
		defer cases[i].Close()
	}

	for i := range cases {
		if cases[i].IsServer() {
			startListenTest(cases[i])
		} else {
			startDialTest(cases[i])
		}
	}

	if !slowStart(t, cases) {
		return
	}

	for i := range cases {
		startSendRecv(cases[i])
	}
	for i := range cases {
		waitTest(cases[i])
	}
}

func RunTestsTCP(t *testing.T, cases []TestCase) {
	RunTests(t, "tcp://127.0.0.1:39093", cases)
}

func RunTestsIPC(t *testing.T, cases []TestCase) {
	if runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows yet")
		return
	}
	RunTests(t, "ipc:///tmp/MYTEST_IPC", cases)
}

func RunTestsInp(t *testing.T, cases []TestCase) {
	RunTests(t, "inproc://MYTEST_INPROC", cases)
}

func RunTestsTLS(t *testing.T, cases []TestCase) {
	RunTests(t, "tls+tcp://127.0.0.1:43934", cases)
}
