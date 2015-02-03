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

package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
)

// Our protocol is simple.  Request packet is empty.  The reply
// is the replier's ID and the time it delayed responding for (us).

func serverWorker(sock mangos.Socket, id int) {
	var err error

	delay := rand.Intn(int(time.Second))

	for {
		var m *mangos.Message

		if m, err = sock.RecvMsg(); err != nil {
			return
		}

		m.Body = make([]byte, 4)

		time.Sleep(time.Duration(delay))

		binary.BigEndian.PutUint32(m.Body[0:], uint32(id))

		if err = sock.SendMsg(m); err != nil {
			return
		}
	}
}

func server(url string, nworkers int) {
	var sock mangos.Socket
	var err error
	var wg sync.WaitGroup

	rand.Seed(time.Now().UnixNano())

	if sock, err = rep.NewSocket(); err != nil {
		die("can't get new rep socket: %s", err)
	}
	if err = sock.SetOption(mangos.OptionRaw, true); err != nil {
		die("can't set raw mode: %s", err)
	}
	sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on rep socket: %s", err.Error())
	}
	wg.Add(nworkers)
	fmt.Printf("Starting %d workers\n", nworkers)
	for id := 0; id < nworkers; id++ {
		go func(id int) {
			defer wg.Done()
			serverWorker(sock, id)
		}(id)
	}
	wg.Wait()
}
