// Copyright 2014 The Mangos Authors
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
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/transport/inproc"
	"bytes"
	"testing"
	"time"
)

var inp = inproc.NewTransport()

func TestInpListenAndAccept(t *testing.T) {

	addr := "inp_test1"
	t.Logf("Establishing accepter")
	accepter, err := inp.NewAccepter(addr, mangos.ProtoRep)
	if err != nil {
		t.Errorf("NewAccepter failed: %v", err)
		return
	}
	defer accepter.Close()

	go func() {
		d, err := inp.NewDialer(addr, mangos.ProtoReq)
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

func TestInpDuplicateListen(t *testing.T) {

	addr := "inp_test2"
	var err error
	listener, err := inp.NewAccepter(addr, mangos.ProtoRep)
	if err != nil {
		t.Errorf("NewAccepter failed: %v", err)
		return
	}
	defer listener.Close()

	_, err = inp.NewAccepter(addr, mangos.ProtoReq)
	if err == nil {
		t.Errorf("Duplicate listen should not be permitted!")
		return
	}
	t.Logf("Got expected error: %v", err)
}

func TestInpConnRefused(t *testing.T) {
	addr := "/tmp/inp_test3"
	var err error
	d, err := inp.NewDialer(addr, mangos.ProtoReq)
	if err != nil || d == nil {
		t.Errorf("New Dialer failed: %v", err)
	}
	c, err := d.Dial()
	if err == nil || c != nil {
		t.Errorf("Connection not refused (%s)!", addr)
		return
	}
	t.Logf("Got expected error: %v", err)
}

func TestInpSendRecv(t *testing.T) {

	addr := "/tmp/inp_test4"
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	ch := make(chan *mangos.Message)

	t.Logf("Establishing listener")
	listener, err := inp.NewAccepter(addr, mangos.ProtoRep)
	if err != nil {
		t.Errorf("NewAccepter failed: %v", err)
		return
	}
	defer listener.Close()

	go func() {
		defer close(ch)

		// Client side
		t.Logf("Connecting")
		d, err := inp.NewDialer(addr, mangos.ProtoReq)

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
			t.Errorf("Client forward mismatch: %v, %v", nrep, ack)
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Client timeout?")
		return
	}
}
