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

// Package test contains a framework for testing.
package test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/transport/all"
)

var protoReq = req.NewProtocol()
var protoRep = rep.NewProtocol()
var cliCfg, _ = NewTlsConfig(false)
var srvCfg, _ = NewTlsConfig(true)

// T is a structure that subtests can inherit from.
type T struct {
	t       *testing.T
	debug   bool
	ID      int
	addr    string
	MsgSize int // size of messages
	txdelay time.Duration
	WantTx  int32
	WantRx  int32
	numtx   int32
	numrx   int32
	timeout time.Duration
	Sock    mangos.Socket
	rdone   bool
	rdoneq  chan struct{}
	sdone   bool
	sdoneq  chan struct{}
	readyq  chan struct{}
	dialer  mangos.Dialer
	listen  mangos.Listener
	Server  bool
	sync.Mutex
}

// TestCase represents a single test case.
type TestCase interface {
	Init(t *testing.T, addr string) bool
	NewMessage() *mangos.Message
	SendMsg(*mangos.Message) error
	RecvMsg() (*mangos.Message, error)
	SendHook(*mangos.Message) bool
	RecvHook(*mangos.Message) bool
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

func (c *T) Init(t *testing.T, addr string) bool {
	// Initial defaults
	c.Lock()
	defer c.Unlock()
	c.t = t
	c.addr = addr
	c.numrx = 0 // Reset
	c.numtx = 0 // Reset
	c.sdoneq = make(chan struct{})
	c.rdoneq = make(chan struct{})
	c.readyq = make(chan struct{})
	c.timeout = time.Second * 3
	c.txdelay = time.Duration(time.Now().UnixNano()) % time.Millisecond

	all.AddTransports(c.Sock)
	return true
}

func (c *T) NewMessage() *mangos.Message {
	return mangos.NewMessage(c.MsgSize)
}

func (c *T) SendHook(m *mangos.Message) bool {
	c.BumpSend()
	return true
}

func (c *T) RecvHook(m *mangos.Message) bool {
	c.BumpRecv()
	return true
}

func (c *T) SendMsg(m *mangos.Message) error {
	// We sleep a tiny bit, to avoid cramming too many messages on
	// busses, etc. all at once.  (The test requires no dropped messages.)
	time.Sleep(c.SendDelay())
	c.Sock.SetOption(mangos.OptionSendDeadline, time.Second*5)
	return c.Sock.SendMsg(m)
}

func (c *T) RecvMsg() (*mangos.Message, error) {
	c.Sock.SetOption(mangos.OptionRecvDeadline, time.Second*5)
	return c.Sock.RecvMsg()
}

func (c *T) Debugf(f string, v ...interface{}) {
	if !c.debug {
		return
	}
	now := time.Now().Format(time.StampMilli)
	c.t.Logf("%s: Id %d: %s", now, c.ID, fmt.Sprintf(f, v...))
}

func (c *T) Logf(f string, v ...interface{}) {
	now := time.Now().Format(time.StampMilli)
	c.t.Logf("%s: Id %d: %s", now, c.ID, fmt.Sprintf(f, v...))
}

func (c *T) Errorf(f string, v ...interface{}) {
	now := time.Now().Format(time.StampMilli)
	c.t.Errorf("%s: Id %d: %s", now, c.ID, fmt.Sprintf(f, v...))
}

func (c *T) WantSend() int32 {
	return c.WantTx
}

func (c *T) BumpSend() {
	atomic.AddInt32(&c.numtx, 1)
}

func (c *T) GetSend() int32 {
	return atomic.AddInt32(&c.numtx, 0)
}

func (c *T) WantRecv() int32 {
	return c.WantRx
}

func (c *T) BumpRecv() {
	atomic.AddInt32(&c.numrx, 1)
}

func (c *T) GetRecv() int32 {
	return atomic.AddInt32(&c.numrx, 0)
}

func (c *T) GetID() int {
	return c.ID
}

func (c *T) SendDone() {
	c.Lock()
	if !c.sdone {
		c.sdone = true
		close(c.sdoneq)
	}
	c.Unlock()
}

func (c *T) RecvDone() {
	c.Lock()
	if !c.rdone {
		c.rdone = true
		close(c.rdoneq)
	}
	c.Unlock()
}

func (c *T) WaitSend() bool {
	select {
	case <-c.sdoneq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *T) WaitRecv() bool {
	select {
	case <-c.rdoneq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *T) Dial() bool {
	options := make(map[string]interface{})
	switch {
	case strings.HasPrefix(c.addr, "tls+tcp://"):
		fallthrough
	case strings.HasPrefix(c.addr, "wss://"):
		options[mangos.OptionTlsConfig] = cliCfg
	}

	err := c.Sock.DialOptions(c.addr, options)
	if err != nil {
		c.Errorf("Dial (%s) failed: %v", c.addr, err)
		return false
	}
	// Allow time for transports to establish connection
	time.Sleep(time.Millisecond * 500)
	return true
}

func (c *T) Listen() bool {
	options := make(map[string]interface{})
	switch {
	case strings.HasPrefix(c.addr, "tls+tcp://"):
		fallthrough
	case strings.HasPrefix(c.addr, "wss://"):
		options[mangos.OptionTlsConfig] = srvCfg
	}
	err := c.Sock.ListenOptions(c.addr, options)
	if err != nil {
		c.Errorf("Listen (%s) failed: %v", c.addr, err)
		return false
	}
	// Allow time for transports to establish connection
	time.Sleep(time.Millisecond * 500)
	return true
}

func (c *T) Close() {
	c.Sock.Close()
}

func (c *T) SendDelay() time.Duration {
	return c.txdelay
}

func (c *T) IsServer() bool {
	return c.Server
}

func (c *T) Ready() bool {
	select {
	case <-c.readyq:
		return true
	default:
		return false
	}
}

func (c *T) SetReady() {
	close(c.readyq)
}

func (c *T) WaitReady() bool {
	select {
	case <-c.readyq:
		return true
	case <-time.After(c.timeout):
		return false
	}
}

func (c *T) SendStart() bool {

	c.Debugf("Sending START")
	m := MakeStart(uint32(c.GetID()))
	if err := c.SendMsg(m); err != nil {
		c.Errorf("SendStart failed: %v", err)
		return false
	}
	return true
}

func (c *T) RecvStart() bool {
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

func (c *T) WantSendStart() bool {
	return c.WantTx > 0
}

func (c *T) WantRecvStart() bool {
	return c.WantRx > 0
}

// MakeStart makes a start message, storing a 32-bit ID in the body.
func MakeStart(v uint32) *mangos.Message {
	m := mangos.NewMessage(10)
	m.Body = append(m.Body, []byte("START")...)
	m.Body = append(m.Body, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	return m
}

// ParseStart parses a start message, returning the ID stored therein.
func ParseStart(m *mangos.Message) (uint32, bool) {
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

// RunTests runs tests.
func RunTests(t *testing.T, addr string, cases []TestCase) {

	if strings.HasPrefix(addr, "ipc://") && runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows yet")
		return
	}

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

// We have to expose these, so that device tests can use them.

// AddrTestTCP is a suitable TCP address for testing.
var AddrTestTCP = "tcp://127.0.0.1:39093"

// AddrTestIPC is a suitable IPC address for testing.
var AddrTestIPC = "ipc:///tmp/MYTEST_IPC"

// AddrTestInp is a suitable Inproc address for testing.
var AddrTestInp = "inproc://MYTEST_INPROC"

// AddrTestTLS is a suitable TLS address for testing.
var AddrTestTLS = "tls+tcp://127.0.0.1:43934"

// AddrTestWS is a suitable websocket address for testing.
var AddrTestWS = "ws://127.0.0.1:53935"

// AddrTestWSS is a suitable secure websocket address for testing.
var AddrTestWSS = "wss://127.0.0.1:53936"

// RunTestsTCP runs the TCP tests.
func RunTestsTCP(t *testing.T, cases []TestCase) {
	RunTests(t, AddrTestTCP, cases)
}

// RunTestsIPC runs the IPC tests.
func RunTestsIPC(t *testing.T, cases []TestCase) {
	RunTests(t, AddrTestIPC, cases)
}

// RunTestsInp runs the inproc tests.
func RunTestsInp(t *testing.T, cases []TestCase) {
	RunTests(t, AddrTestInp, cases)
}

// RunTestsTLS runs the TLS tests.
func RunTestsTLS(t *testing.T, cases []TestCase) {
	RunTests(t, AddrTestTLS, cases)
}

// RunTestsWS runs the websock tests.
func RunTestsWS(t *testing.T, cases []TestCase) {
	RunTests(t, AddrTestWS, cases)
}

// RunTestsWSS runs the websock tests.
func RunTestsWSS(t *testing.T, cases []TestCase) {
	RunTests(t, AddrTestWSS, cases)
}
