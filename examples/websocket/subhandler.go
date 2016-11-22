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
	"github.com/go-mangos/mangos/protocol/pub"
	"github.com/go-mangos/mangos/transport/ws"
)

// subHandler just spins on the socket and publishes messages.  It sends
// "PUB #<count> <time>".  Not very interesting...

func subHandler(sock mangos.Socket) {
	count := 0
	for {
		msg := fmt.Sprintf("PUB #%d %s", count, time.Now().String())
		if e := sock.Send([]byte(msg)); e != nil {
			die("Cannot send pub: %v", e)
		}
		time.Sleep(5 * time.Second)
		count++
	}
}

func addSubHandler(mux *http.ServeMux, port int) {
	sock, _ := pub.NewSocket()

	sock.AddTransport(ws.NewTransport())

	url := fmt.Sprintf("ws://127.0.0.1:%d/sub", port)

	if l, e := sock.NewListener(url, nil); e != nil {
		die("bad listener: %v", e)
	} else if h, e := l.GetOption(ws.OptionWebSocketHandler); e != nil {
		die("cannot get handler:", e)
	} else {
		mux.Handle("/sub", h.(http.Handler))
		l.Listen()
	}

	go subHandler(sock)
}
