// Copyright 2016 The Mangos Authors
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

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/transport/ws"
)

// reqHandler just spins on the socket and reads messages.  It replies
// with "REPLY <time>".  Not very interesting...

func reqHandler(sock mangos.Socket) {
	count := 0
	for {
		// don't care about the content of received message
		_, e := sock.Recv()
		if e != nil {
			die("Cannot get request: %v", e)
		}
		reply := fmt.Sprintf("REPLY #%d %s", count, time.Now().String())
		if e := sock.Send([]byte(reply)); e != nil {
			die("Cannot send reply: %v", e)
		}
		count++
	}
}

func addReqHandler(mux *http.ServeMux, port int) {
	sock, _ := rep.NewSocket()

	sock.AddTransport(ws.NewTransport())

	url := fmt.Sprintf("ws://127.0.0.1:%d/req", port)

	if l, e := sock.NewListener(url, nil); e != nil {
		die("bad listener: %v", e)
	} else if h, e := l.GetOption(ws.OptionWebSocketHandler); e != nil {
		die("bad handler: %v", e)
	} else {
		mux.Handle("/req", h.(http.Handler))
		l.Listen()
	}
	go reqHandler(sock)
}
