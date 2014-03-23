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

// Package chanstream provides an API that is similar to that used for TCP
// and Unix Domain sockets (see net.TCP), for use in intra-process
// communication on top of Go channels.  This makes it easy to swap it for
// another net.Conn interface.
//
// By using channels, we avoid exposing any
// interface to other processors, or involving the kernel to perform data
// copying.
package sp

import (
	"bytes"
	"testing"
	"time"
)

var tcp = &TCPTransport{}

func TestTCPListenAndAccept(t *testing.T) {
	addr := "tcp://127.0.0.1:3333"
	t.Logf("Establishing accepter")
	accepter, err := tcp.NewAccepter(addr, ProtoRep)
	if err != nil {
		t.Errorf("NewAccepter failed: %v", err)
		return
	}
	defer accepter.Close()

	go func() {
		d, err := tcp.NewDialer(addr, ProtoReq)
		if err != nil {
			t.Errorf("NewDialier failed: %v", err)
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
		}
	}()

	server, err := accepter.Accept()
	if err != nil {
		t.Errorf("Accept failed: %v", err)
	}
	defer server.Close()

	t.Logf("Connected server: %d (client %d)",
		server.LocalProtocol(), server.RemoteProtocol())
	t.Logf("Server open: %t", server.IsOpen())
	if !server.IsOpen() {
		t.Error("Server is closed")
	}
}

func TestTCPDuplicateListen(t *testing.T) {
	url := "tcp://127.0.0.1:3333"
	var err error
	listener, err := tcp.NewAccepter(url, ProtoRep)
	if err != nil {
		t.Errorf("NewAccepter failed: %v", err)
		return
	}
	defer listener.Close()

	_, err = tcp.NewAccepter(url, ProtoReq)
	if err == nil {
		t.Errorf("Duplicate listen should not be permitted!")
		return
	}
	t.Logf("Got expected error: %v", err)
}

func TestTCPConnRefused(t *testing.T) {
	url := "tcp://127.0.0.1:19" // Port 19 is chargen, rarely in use
	var err error
	d, err := tcp.NewDialer(url, ProtoReq)
	if err != nil || d == nil {
		t.Errorf("New Dialer failed: %v", err)
	}
	c, err := d.Dial()
	if err == nil || c != nil {
		t.Errorf("Connection not refused (%s)!", url)
		return
	}
	t.Logf("Got expected error: %v", err)
}

func TestTCPSendRecv(t *testing.T) {
	url := "tcp://127.0.0.1:3333"
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	ch := make(chan *Message)

	t.Logf("Establishing listener")
	listener, err := tcp.NewAccepter(url, ProtoRep)
	if err != nil {
		t.Errorf("NewAccepter failed: %v", err)
		return
	}
	defer listener.Close()

	go func() {
		defer close(ch)

		// Client side
		t.Logf("Connecting")
		d, err := tcp.NewDialer(url, ProtoReq)

		client, err := d.Dial()
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
		}
		t.Logf("Connected client: %t", client.IsOpen())
		defer client.Close()

		req := new(Message)
		req.Body = make([]byte, 0, 20)
		req.Header = make([]byte, 0)
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
		}
	}()

	server, err := listener.Accept()
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
	rep := new(Message)
	rep.Body = make([]byte, 0, 20)
	rep.Header = make([]byte, 0)
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
		if !bytes.Equal(nrep.Body, rep.Body) ||
			!bytes.Equal(rep.Header, nrep.Header) {
			t.Error("Client forward mismatch: %v, %v", nrep, rep)
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Client timeout?")
		return
	}
}
