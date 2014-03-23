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

func TestReqRep(t *testing.T) {
	url := "tcp://127.0.0.1:3535"
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	clich := make(chan bool, 1)
	srvch := make(chan bool, 1)

	var srvsock Socket

	var pass, ok bool

	t.Log("Starting server")
	go func() {
		var err error
		var req, rep *Message

		defer close(srvch)
		srvsock, err = NewSocket("Rep")
		if err != nil || srvsock == nil {
			t.Errorf("Failed creating server socket: %v", err)
			return
		}
		// XXX: Closing the server socket too soon causes the
		// underlying connecctions to be closed, which breaks the
		// client.  We really need a shutdown().  For now we just
		// close in the outer handler.
		//defer srvsock.Close()

		if err = srvsock.Listen(url); err != nil {
			t.Errorf("Server listen failed: %v", err)
			return
		}
		t.Logf("Server listening")

		if req, err = srvsock.RecvMsg(); err != nil {
			t.Errorf("Server receive failed: %v", err)
			return
		}
		t.Logf("Server got message")

		if !bytes.Equal(req.Body, ping) {
			t.Errorf("Server recd bad message: %v", req)
			return
		}

		t.Logf("Server sending reply")
		rep = new(Message)
		rep.Body = make([]byte, 0, 20)
		rep.Body = append(rep.Body, ack...)
		rep.Header = make([]byte, 0)
		if err = srvsock.SendMsg(rep); err != nil {
			t.Errorf("Server send failed: %v", err)
			return
		}

		t.Logf("Server OK")
		// its all good
		srvch <- true
	}()

	t.Log("Starting client")
	go func() {
		var clisock Socket
		var err error
		var req, rep *Message

		defer close(clich)
		clisock, err = NewSocket("Req")
		if err != nil || clisock == nil {
			t.Errorf("Failed creating client socket: %v", err)
			return
		}
		defer clisock.Close()

		if err = clisock.Dial(url); err != nil {
			t.Errorf("Client dial failed: %v", err)
			return
		}

		t.Logf("Client dial complete")

		req = new(Message)
		req.Body = make([]byte, 0, 20)
		req.Body = append(req.Body, ping...)
		req.Header = make([]byte, 0)
		if err = clisock.SendMsg(req); err != nil {
			t.Errorf("Client send failed: %v", err)
			return
		}
		t.Logf("Sent client request")

		if rep, err = clisock.RecvMsg(); err != nil {
			t.Errorf("Client receive failed: %v", err)
			return
		}
		t.Logf("Client received reply")

		if !bytes.Equal(rep.Body, ack) {
			t.Errorf("Client recd bad message: %v", req)
			return
		}

		t.Logf("Client OK")

		// its all good
		clich <- true

	}()

	// Check server reported OK
	select {
	case pass, ok = <-srvch:
		if !ok {
			t.Error("Server aborted")
			return
		}
		if !pass {
			t.Error("Server reported failure")
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for server")
		return
	}

	// Check client reported OK
	select {
	case pass, ok = <-clich:
		if !ok {
			t.Error("Client aborted")
			return
		}
		if !pass {
			t.Error("Client reported failure")
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for client")
		return
	}

	srvsock.Close()
}
