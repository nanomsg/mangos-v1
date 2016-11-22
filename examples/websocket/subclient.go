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

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/sub"
	"github.com/go-mangos/mangos/transport/ws"
)

// subClient implements the client for SUB.
func subClient(port int) {
	sock, err := sub.NewSocket()
	if err != nil {
		die("cannot make req socket: %v", err)
	}
	sock.AddTransport(ws.NewTransport())
	if err = sock.SetOption(mangos.OptionSubscribe, []byte{}); err != nil {
		die("cannot set subscription: %v", err)
	}
	url := fmt.Sprintf("ws://127.0.0.1:%d/sub", port)
	if err = sock.Dial(url); err != nil {
		die("cannot dial req url: %v", err)
	}
	if m, err := sock.Recv(); err != nil {
		die("Cannot recv sub: %v", err)
	} else {
		fmt.Printf("%s\n", string(m))
	}
}
