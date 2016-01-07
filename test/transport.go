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

package test

import (
	"bytes"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/protocol/req"
)

// TranTest provides a common test structure for transports, so that they
// can implement a battery of standard tests.
type TranTest struct {
	addr    string
	tran    mangos.Transport
	cliCfg  *tls.Config
	srvCfg  *tls.Config
	sockRep mangos.Socket
	sockReq mangos.Socket
}

// NewTranTest creates a TranTest.
func NewTranTest(tran mangos.Transport, addr string) *TranTest {
	tt := &TranTest{addr: addr, tran: tran}
	if strings.HasPrefix(tt.addr, "tls+tcp://") || strings.HasPrefix(tt.addr, "wss://") {
		tt.cliCfg, _ = GetTLSConfig(false)
		tt.srvCfg, _ = GetTLSConfig(true)
	}
	tt.sockRep, _ = rep.NewSocket()
	tt.sockReq, _ = req.NewSocket()
	return tt
}

// TestListenAndAccept tests that we can both listen and accept connections
// for the given transport.
func (tt *TranTest) TestListenAndAccept(t *testing.T) {
	t.Logf("Establishing listener for %s", tt.addr)
	l, err := tt.tran.NewListener(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l.Close()
	if tt.srvCfg != nil {
		if err = l.SetOption(mangos.OptionTLSConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		t.Logf("Connecting on %s", tt.addr)
		d, err := tt.tran.NewDialer(tt.addr, tt.sockReq)
		if err != nil {
			t.Errorf("NewDialer failed: %v", err)
			return
		}
		if tt.cliCfg != nil {
			if err = d.SetOption(mangos.OptionTLSConfig, tt.cliCfg); err != nil {
				t.Errorf("Failed setting TLS config: %v", err)
				return
			}
		}
		client, err := d.Dial()
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
		}
		if v, err := client.GetProp(mangos.PropLocalAddr); err == nil {
			addr := v.(net.Addr)
			t.Logf("Dialed on local net %s addr %s", addr.Network(), addr.String())
		} else {
			t.Logf("err is %v", err.Error())
		}
		if v, err := client.GetProp(mangos.PropRemoteAddr); err == nil {
			addr := v.(net.Addr)
			t.Logf("Dialed remote peer %s addr %s", addr.Network(), addr.String())
		}
		t.Logf("Connected client: %d (server %d)",
			client.LocalProtocol(), client.RemoteProtocol())
		t.Logf("Client open: %t", client.IsOpen())
		if !client.IsOpen() {
			t.Error("Client is closed")
			return
		}
	}()

	server, err := l.Accept()
	if err != nil {
		t.Errorf("Accept failed: %v", err)
		return
	}
	if v, err := server.GetProp(mangos.PropLocalAddr); err == nil {
		addr := v.(net.Addr)
		t.Logf("Accepted on local net %s addr %s", addr.Network(), addr.String())
	}
	if v, err := server.GetProp(mangos.PropRemoteAddr); err == nil {
		addr := v.(net.Addr)
		t.Logf("Accepted remote peer %s addr %s", addr.Network(), addr.String())
	}
	defer server.Close()

	t.Logf("Connected server: %d (client %d)",
		server.LocalProtocol(), server.RemoteProtocol())
	t.Logf("Server open: %t", server.IsOpen())
	if !server.IsOpen() {
		t.Error("Server is closed")
	}
	wg.Wait()
}

// TestDuplicateListen checks to make sure that an attempt to listen
// on a second socket, when another listener is already present, properly
// fails with ErrAddrInUse.
func (tt *TranTest) TestDuplicateListen(t *testing.T) {
	var err error
	time.Sleep(100 * time.Millisecond)
	t.Logf("Testing Duplicate Listen on %s", tt.addr)
	l1, err := tt.tran.NewListener(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l1.Close()
	if tt.srvCfg != nil {
		if err = l1.SetOption(mangos.OptionTLSConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l1.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	l2, err := tt.tran.NewListener(tt.addr, tt.sockReq)
	if err != nil {
		t.Errorf("NewListener faield: %v", err)
		return
	}
	defer l2.Close()
	if tt.srvCfg != nil {
		if err = l2.SetOption(mangos.OptionTLSConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l2.Listen(); err == nil {
		t.Errorf("Duplicate listen should not be permitted!")
		return
	}
	t.Logf("Got expected error: %v", err)
}

// TestConnRefused tests that attempts to dial to an address without a listener
// properly fail with EConnRefused.
func (tt *TranTest) TestConnRefused(t *testing.T) {
	d, err := tt.tran.NewDialer(tt.addr, tt.sockReq)
	if err != nil || d == nil {
		t.Errorf("New Dialer failed: %v", err)
	}
	if tt.cliCfg != nil {
		if err = d.SetOption(mangos.OptionTLSConfig, tt.cliCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	c, err := d.Dial()
	if err == nil || c != nil {
		t.Errorf("Connection not refused (%s)!", tt.addr)
		return
	}
	t.Logf("Got expected error: %v", err)
}

// TestSendRecv test that the transport can send and receive.  It uses the
// REQ/REP protocol for messages.
func (tt *TranTest) TestSendRecv(t *testing.T) {
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	ch := make(chan *mangos.Message)

	t.Logf("Establishing REP listener on %s", tt.addr)
	l, err := tt.tran.NewListener(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l.Close()
	if tt.srvCfg != nil {
		if err = l.SetOption(mangos.OptionTLSConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	go func() {
		defer close(ch)

		// Client side
		t.Logf("Connecting REQ on %s", tt.addr)
		d, err := tt.tran.NewDialer(tt.addr, tt.sockReq)
		if tt.cliCfg != nil {
			if err = d.SetOption(mangos.OptionTLSConfig, tt.cliCfg); err != nil {
				t.Errorf("Failed setting TLS config: %v", err)
				return
			}
		}

		client, err := d.Dial()
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
		}
		t.Logf("Connected client: %t", client.IsOpen())
		defer client.Close()

		req := mangos.NewMessage(len(ping))
		req.Body = append(req.Body, ping...)

		// Now try to send data
		t.Logf("Sending %d bytes", len(req.Body))

		err = client.Send(req)
		if err != nil {
			t.Errorf("Client send error: %v", err)
			return
		}
		t.Logf("Client sent")

		rep, err := client.Recv()
		if err != nil {
			t.Errorf("Client receive error: %v", err)
			return
		}

		if !bytes.Equal(rep.Body, ack) {
			t.Errorf("Reply mismatch: %v, %v", rep.Body, ack)
			return
		}
		if len(rep.Header) != 0 {
			t.Errorf("Client reply non-empty header: %v",
				rep.Header)
			return
		}
		select {
		case ch <- rep:
			t.Log("Client reply forwarded")
		case <-time.After(5 * time.Second): // 5 secs should be plenty
			t.Error("Client timeout forwarding reply")
			return
		}
	}()

	server, err := l.Accept()
	if err != nil {
		t.Errorf("Accept failed: %v", err)
		return
	}
	t.Logf("Connected server: %t", server.IsOpen())
	defer server.Close()

	// Now we can try to send and receive
	req, err := server.Recv()
	if err != nil {
		t.Errorf("Server receive error: %v", err)
		return
	}
	t.Logf("Server received %d bytes", len(req.Body))
	if !bytes.Equal(req.Body, ping) {
		t.Errorf("Request mismatch: %v, %v", req.Body, ping)
		return
	}

	if len(req.Header) != 0 {
		t.Errorf("Server request non-empty header: %v", req.Header)
		return
	}

	// Now reply
	rep := mangos.NewMessage(len(ack))
	rep.Body = append(rep.Body, ack...)

	t.Logf("Server sending %d bytes", len(rep.Body))

	err = server.Send(rep)
	if err != nil {
		t.Errorf("Server send error: %v", err)
		return
	}
	t.Log("Server reply sent")

	// Wait for client to ack reply over back channel.
	select {
	case nrep := <-ch:
		if !bytes.Equal(nrep.Body, ack) {
			t.Errorf("Client forward mismatch: %v, %v", ack, rep)
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Client timeout?")
		return
	}
}

// TestScheme tests the Scheme() entry point on the transport.
func (tt *TranTest) TestScheme(t *testing.T) {
	scheme := tt.tran.Scheme()
	t.Log("Checking scheme")
	if !strings.HasPrefix(tt.addr, scheme+"://") {
		t.Errorf("Wrong scheme: addr %s, scheme %s", tt.addr, scheme)
		return
	}
	t.Log("Scheme match")
}

// TestListenerSetOptionInvalid tests passing invalid options to a listener.
func (tt *TranTest) TestListenerSetOptionInvalid(t *testing.T) {
	t.Log("Trying invalid listener SetOption")
	l, err := tt.tran.NewListener(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("Unable to create listener")
		return
	}
	err = l.SetOption("NO-SUCH-OPTION", true)
	switch err {
	case mangos.ErrBadOption:
		t.Log("Got expected err BadOption")
	case nil:
		t.Errorf("Got nil err, but expected BadOption!")
	default:
		t.Errorf("Got unexpected error %v, expected BadOption", err)
	}
}

// TestListenerGetOptionInvalid tests trying to get an invalid option on
// a listener.
func (tt *TranTest) TestListenerGetOptionInvalid(t *testing.T) {
	t.Log("Trying invalid listener GetOption")
	l, err := tt.tran.NewListener(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("Unable to create listener")
		return
	}
	_, err = l.GetOption("NO-SUCH-OPTION")
	switch err {
	case mangos.ErrBadOption:
		t.Log("Got expected err BadOption")
	case nil:
		t.Errorf("Got nil err, but expected BadOption!")
	default:
		t.Errorf("Got unexpected error %v, expected BadOption", err)
	}
}

// TestDialerSetOptionInvalid tests trying to set an invalid option on a Dialer.
func (tt *TranTest) TestDialerSetOptionInvalid(t *testing.T) {
	t.Log("Trying invalid dialer SetOption")
	d, err := tt.tran.NewDialer(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("Unable to create dialer")
		return
	}
	err = d.SetOption("NO-SUCH-OPTION", true)
	switch err {
	case mangos.ErrBadOption:
		t.Log("Got expected err BadOption")
	case nil:
		t.Errorf("Got nil err, but expected BadOption!")
	default:
		t.Errorf("Got unexpected error %v, expected BadOption", err)
	}
}

// TestDialerGetOptionInvalid tests attempting to get an invalid option on
// a Dialer.
func (tt *TranTest) TestDialerGetOptionInvalid(t *testing.T) {
	t.Log("Trying invalid listener GetOption")
	d, err := tt.tran.NewDialer(tt.addr, tt.sockRep)
	if err != nil {
		t.Errorf("Unable to create dialer")
		return
	}
	_, err = d.GetOption("NO-SUCH-OPTION")
	switch err {
	case mangos.ErrBadOption:
		t.Log("Got expected err BadOption")
	case nil:
		t.Errorf("Got nil err, but expected BadOption!")
	default:
		t.Errorf("Got unexpected error %v, expected BadOption", err)
	}
}

// TestDialerBadScheme tests to makes sure that giving a bogus scheme
// to create a dialer fails properly.
func (tt *TranTest) TestDialerBadScheme(t *testing.T) {
	t.Logf("NewDialer with bogus scheme")
	d, err := tt.tran.NewDialer("bogus://address", tt.sockRep)
	if err == nil {
		t.Errorf("Expected error, got nil")
	} else if d != nil {
		t.Errorf("Got non-nil error, and non-nil dialer")
	} else {
		t.Logf("Got expected error %v", err)
	}
}

// TestListenerBadScheme tests to makes sure that giving a bogus scheme
// to create a listener fails properly.
func (tt *TranTest) TestListenerBadScheme(t *testing.T) {
	t.Logf("NewListener with bogus scheme")
	d, err := tt.tran.NewListener("bogus://address", tt.sockRep)
	if err == nil {
		t.Errorf("Expected error, got nil")
	} else if d != nil {
		t.Errorf("Got non-nil error, and non-nil listener")
	} else {
		t.Logf("Got expected error %v", err)
	}
}

// TestAll runs a full battery of standard tests on the transport.
func (tt *TranTest) TestAll(t *testing.T) {
	tt.TestScheme(t)
	tt.TestListenAndAccept(t)
	tt.TestConnRefused(t)
	tt.TestDuplicateListen(t)
	tt.TestSendRecv(t)
	tt.TestDialerSetOptionInvalid(t)
	tt.TestDialerGetOptionInvalid(t)
	tt.TestListenerSetOptionInvalid(t)
	tt.TestListenerGetOptionInvalid(t)
	tt.TestDialerBadScheme(t)
	tt.TestListenerBadScheme(t)
}
