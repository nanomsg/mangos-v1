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

package tcp

import (
	"bytes"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
)

var tran = NewTransport()
var protoRep = rep.NewProtocol()
var protoReq = req.NewProtocol()

func TestTCPListenAndAccept(t *testing.T) {
	addr := "tcp://127.0.0.1:3333"
	t.Logf("Establishing accepter")
	l, err := tran.NewListener(addr, protoRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l.Close()
	if err = l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	go func() {
		d, err := tran.NewDialer(addr, protoReq)
		if err != nil {
			t.Errorf("NewDialier failed: %v", err)
			return
		}
		t.Logf("Connecting")
		client, err := d.Dial()
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
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
	defer server.Close()

	t.Logf("Connected server: %d (client %d)",
		server.LocalProtocol(), server.RemoteProtocol())
	t.Logf("Server open: %t", server.IsOpen())
	if !server.IsOpen() {
		t.Error("Server is closed")
		return
	}
}

func TestTCPDuplicateListen(t *testing.T) {
	addr := "tcp://127.0.0.1:3333"
	var err error
	l1, err := tran.NewListener(addr, protoRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l1.Close()
	if err = l1.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	l2, err := tran.NewListener(addr, protoReq)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l2.Close()
	if err = l2.Listen(); err == nil {
		t.Errorf("Duplicate listen should not be permitted!")
		return
	}
	t.Logf("Got expected error: %v", err)
}

func TestTCPConnRefused(t *testing.T) {
	addr := "tcp://127.0.0.1:19" // Port 19 is chargen, rarely in use
	var err error
	d, err := tran.NewDialer(addr, protoReq)
	if err != nil || d == nil {
		t.Errorf("New Dialer failed: %v", err)
		return
	}
	c, err := d.Dial()
	if err == nil || c != nil {
		t.Errorf("Connection not refused (%s)!", addr)
		return
	}
	t.Logf("Got expected error: %v", err)
}

func TestTCPSendRecv(t *testing.T) {
	addr := "tcp://127.0.0.1:3333"
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	ch := make(chan *mangos.Message)

	t.Logf("Establishing listener")
	l, err := tran.NewListener(addr, protoRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l.Close()
	if err = l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	go func() {
		defer close(ch)

		// Client side
		t.Logf("Connecting")
		d, err := tran.NewDialer(addr, protoReq)

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

func TestTCPOptions(t *testing.T) {
	addr := "tcp://127.0.0.1:19" // Port 19 is chargen, rarely in use
	var err error
	d, err := tran.NewDialer(addr, protoReq)
	if err != nil || d == nil {
		t.Errorf("New Dialer failed: %v", err)
		return
	}

	t.Logf("Options are %v", interface{}(d).(*dialer).opts)

	// Valid Boolean Options
	for _, n := range []string{mangos.OptionNoDelay, mangos.OptionKeepAlive} {
		t.Logf("Checking option %s", n)

		if err := d.SetOption(n, true); err != nil {
			t.Errorf("Set option %s failed: %v", n, err)
			return
		}

		if val, err := d.GetOption(n); err != nil {
			t.Errorf("Get option %s failed: %v", n, err)
			return
		} else {
			switch v := val.(type) {
			case bool:
				if !v {
					t.Errorf("Option %s value not true", n)
					return
				}
			default:
				t.Errorf("Option %s wrong type!", n)
				return
			}
		}

		if err := d.SetOption(n, 1234); err != mangos.ErrBadValue {
			t.Errorf("Expected ErrBadValue, but did not get it")
			return
		}
	}

	// Negative test: try a bad option
	if err := d.SetOption("NO-SUCH-OPTION", 0); err != mangos.ErrBadOption {
		t.Errorf("Expected ErrBadOption, but did not get it")
		return
	}
}
