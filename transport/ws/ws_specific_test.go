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

package ws

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/protocol/req"
	//"github.com/go-mangos/mangos/test"
)

func TestWebsockPath(t *testing.T) {
	sockReq, _ := req.NewSocket()
	sockRep, _ := rep.NewSocket()
	tran := NewTransport()
	l, e := tran.NewListener("ws://127.0.0.1:3335/mysock", sockReq)
	if e != nil {
		t.Errorf("Failed new Listener: %v", e)
		return
	}
	d, e := tran.NewDialer("ws://127.0.0.1:3335/boguspath", sockRep)
	if e != nil {
		t.Errorf("Failed new Dialer: %v", e)
		return
	}

	if e = l.Listen(); e != nil {
		t.Errorf("Listen failed")
		return
	}
	defer l.Close()

	p, e := d.Dial()
	if p != nil {
		defer p.Close()
	}
	if e == nil {
		t.Errorf("Dial passed, when should not have!")
		return
	}
	t.Logf("Got expected error %v", e)
}

var bogusstr = "THIS IS BOGUS"

func bogusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, bogusstr)
}

func TestWebsockMux(t *testing.T) {
	sockReq, _ := req.NewSocket()
	sockRep, _ := rep.NewSocket()
	tran := NewTransport()
	l, e := tran.NewListener("ws://127.0.0.1:3336/mysock", sockReq)
	if e != nil {
		t.Errorf("Failed new Listener: %v", e)
		return
	}
	muxi, e := l.GetOption(OptionWebSocketMux)
	if e != nil {
		t.Errorf("Failed get mux: %v", e)
	}
	mux := muxi.(*http.ServeMux)
	mux.HandleFunc("/bogus", bogusHandler)
	d, e := tran.NewDialer("ws://127.0.0.1:3336/bogus", sockRep)
	if e != nil {
		t.Errorf("Failed new Dialer: %v", e)
		return
	}

	if e = l.Listen(); e != nil {
		t.Errorf("Listen failed")
		return
	}
	defer l.Close()

	p, e := d.Dial()
	if p != nil {
		defer p.Close()
	}
	if e == nil {
		t.Errorf("Dial passed, when should not have!")
		return
	}
	t.Logf("Got expected error %v", e)

	// Now let's try to use http client.
	resp, err := http.Get("http://127.0.0.1:3336/bogus")

	if err != nil {
		t.Errorf("Get of boguspath failed: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		t.Errorf("Response code wrong: %d", resp.StatusCode)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("ReadAll Failed: %v", err)
		return
	}
	if string(body) != bogusstr {
		t.Errorf("Results mismatch: %s != %s", string(body), bogusstr)
	}
	t.Logf("Got body: %s", string(body))
}

// This test verifies that we can use stock http server instances with
// our own websocket handler.
func TestWebsockHandler(t *testing.T) {
	sockReq, _ := req.NewSocket()
	sockRep, _ := rep.NewSocket()
	tran := NewTransport()
	l, e := tran.NewListener("ws://127.0.0.1:3337/mysock", sockReq)
	if e != nil {
		t.Errorf("Failed new Listener: %v", e)
		return
	}
	hi, e := l.GetOption(OptionWebSocketHandler)
	if e != nil {
		t.Errorf("Failed get WebSocketHandler: %v", e)
	}
	handler := hi.(http.Handler)

	mux := http.NewServeMux()
	mux.HandleFunc("/bogus", bogusHandler)
	mux.Handle("/mysock", handler)

	// Note that we are *counting* on this to die gracefully when our
	// program exits. There appears to be no way to shutdown http
	// instances gracefully.
	go http.ListenAndServe("127.0.0.1:3337", mux)

	d, e := tran.NewDialer("ws://127.0.0.1:3337/bogus", sockRep)
	if e != nil {
		t.Errorf("Failed new Dialer: %v", e)
		return
	}

	defer l.Close()

	p, e := d.Dial()
	if p != nil {
		defer p.Close()
	}
	if e == nil {
		t.Errorf("Dial passed, when should not have!")
		return
	}
	t.Logf("Got expected error %v", e)

	// Now let's try to use http client.
	resp, err := http.Get("http://127.0.0.1:3337/bogus")

	if err != nil {
		t.Errorf("Get of boguspath failed: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		t.Errorf("Response code wrong: %d", resp.StatusCode)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("ReadAll Failed: %v", err)
		return
	}
	if string(body) != bogusstr {
		t.Errorf("Results mismatch: %s != %s", string(body), bogusstr)
	}
	t.Logf("Got body: %s", string(body))
}
