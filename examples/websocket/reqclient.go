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
	"time"

	"github.com/go-mangos/mangos/protocol/req"
	"github.com/go-mangos/mangos/transport/ws"
)

// reqClient implements the client for REQ.
func reqClient(port int) {
	sock, e := req.NewSocket()
	if e != nil {
		die("cannot make req socket: %v", e)
	}
	sock.AddTransport(ws.NewTransport())
	url := fmt.Sprintf("ws://127.0.0.1:%d/req", port)
	if e = sock.Dial(url); e != nil {
		die("cannot dial req url: %v", e)
	}
	// Time for TCP connection set up
	time.Sleep(time.Millisecond * 10)
	if e = sock.Send([]byte("Hello")); e != nil {
		die("Cannot send req: %v", e)
	}
	if m, e := sock.Recv(); e != nil {
		die("Cannot recv reply: %v", e)
	} else {
		fmt.Printf("%s\n", string(m))
	}
}
