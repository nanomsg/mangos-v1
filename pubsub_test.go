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
	"testing"
	"time"
)

func TestPubSub(t *testing.T) {
	url := "tcp://127.0.0.1:3535"

	clich := make(chan bool, 1)
	srvch := make(chan bool, 1)

	var srvsock Socket

	var pass, ok bool

	t.Log("Starting server")
	go func() {
		var err error
		publish := []string{
			"/some/like/it/hot",
			"/some/where",
			"/over/the",
			"/rainbow",
			"\\\\C\\SPOT\\RUN",
			"The Quick Brown Fox",
			"END"}

		defer close(srvch)
		srvsock, err = NewSocket(PubName)
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

		// Lets sleep a short bit, to make sure the client starts up
		time.Sleep(200 * time.Millisecond)

		for i, m := range publish {

			t.Logf("Server publishing #%d: %s", i, m)

			if err = srvsock.Send([]byte(m)); err != nil {
				t.Errorf("Server send failed: %v", err)
				return
			}

			// another tiny sleep before sending the next one
			time.Sleep(10 * time.Millisecond)
		}

		t.Logf("Server OK")
		// its all good
		srvch <- true
	}()

	t.Log("Starting client")
	go func() {
		var clisock Socket
		var err error
		var rep *Message
		var rain = 0
		var end = false

		defer close(clich)
		clisock, err = NewSocket(SubName)
		if err != nil || clisock == nil {
			t.Errorf("Failed creating client socket: %v", err)
			return
		}
		defer clisock.Close()
		err = clisock.SetOption(SubOptionSubscribe, []byte("END"))
		if err != nil {
			t.Errorf("Failed to subscribe to END: %v", err)
			return
		}

		err = clisock.SetOption(SubOptionSubscribe, []byte("/rain"))
		if err != nil {
			t.Errorf("Failed")
			return
		}

		if err = clisock.Dial(url); err != nil {
			t.Errorf("Client dial failed: %v", err)
			return
		}
		t.Logf("Client dial complete")

		for !end {
			if rep, err = clisock.RecvMsg(); err != nil {
				t.Errorf("Client receive failed: %v", err)
				return
			}
			str := string(rep.Body)
			t.Logf("Client received pub %s", str)
			switch {
			case str == "END":
				end = true
			case str == "/rainbow":
				rain++
			default:
				t.Errorf("Got unexpected pub %v", rep)
				return
			}
		}

		if rain != 1 {
			t.Errorf("Got wrong number of rainbows")
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
		}
		if !pass {
			t.Error("Client reported failure")
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for client")
	}

	srvsock.Close()
}
