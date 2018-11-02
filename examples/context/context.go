// Copyright 2018 The Mangos Authors
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

// context implements a request/reply server that utilizes a pool of worker goroutines to service
// multiple requests simultaneously. Each goroutine has it's own context which keeps track of
// which client the request came from so that it can reply to the correct client.
//
// The server is a listening rep socket, and client is a dialing req socket.
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $
//   $ ./context server $url & server=$! && sleep 1
//   $ ./context client $url "John"
//   $ ./context client $url "Bill"
//   $ ./context client $url "Mary"
//   $ ./context client $url "Susan"
//   $ ./context client $url "Mark"
//
//   $ kill $server
//
package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/rep"
	"nanomsg.org/go/mangos/v2/protocol/req"

	// register transports
	_ "nanomsg.org/go/mangos/v2/transport/all"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

/**
This function will act as the server. It will create a single REP socket and a pool of worker go routines that will
simultaneously service requests from many clients. This means that two clients can both send a request to the
server and the server can deal with both of them concurrently. The server does not need to
respond in the same order the requests come in. Each worker responds on it's own context.
*/
func server(url string) {
	var sock mangos.Socket
	var err error
	const numWorkers = 3

	if sock, err = rep.NewSocket(); err != nil {
		die("can't get new rep socket: %s", err)
	}
	if err = sock.Listen(url); err != nil {
		die("can't listen on rep socket: %s", err.Error())
	}

	// Create a worker go routine that will create it's own context on the REP socket.
	// This context will receive a request that occurs on the socket from one of the clients.
	// This worker can then reply on the context, which will ensure
	// that the client of this particular request will get the response.
	var worker = func(id int) {
		const maxSleepSec = 1
		var ctx mangos.Context
		var err error
		var sleepTimeSec int
		var response string
		var msg []byte

		ctx, err = sock.OpenContext()
		for { // Loop endlessly so workers do not die.
			if err != nil {
				die("can't create context on rep socket: %s", err)
			}
			msg, err = ctx.Recv()
			fmt.Printf("Worker %d: Received request '%s'\n", id, string(msg))

			// Sleep for a random amount of time to simulate "work"
			sleepTimeSec = int(maxSleepSec * rand.Float64() * float64(time.Second))
			fmt.Printf("Worker %d: Sleeping for %v\n", id, time.Duration(sleepTimeSec))
			time.Sleep(time.Duration(sleepTimeSec))

			response = "Hello, " + string(msg)
			fmt.Printf("Worker %d: Sending response '%s'\n", id, response)
			err = ctx.Send([]byte(response))
			if err != nil {
				die("can't send reply: %s", err.Error())
			}
		}
	}

	// Create several workers that are ready to handle incoming requests.
	for ii := 0; ii < numWorkers; ii++ {
		go worker(ii)
	}

	for {
		// Sleep so the process does not exit
		time.Sleep(100 * time.Second)
	}

}

// Create a client that will connect to the REP server and send a message string. Await the response and then quit.
func client(url, msg string) {
	var sock mangos.Socket
	var err error
	var response []byte

	// Generate a random ID for the client so we can track them.
	var id = rand.Intn(9999)

	if sock, err = req.NewSocket(); err != nil {
		die("can't get new req socket: %s", err.Error())
	}
	if err = sock.Dial(url); err != nil {
		die("can't dial on req socket: %s", err.Error())
	}
	fmt.Printf("Client %d: Sending message '%s'\n", id, msg)
	if err = sock.Send([]byte(msg)); err != nil {
		die("can't send message on push socket: %s", err.Error())
	}
	if response, err = sock.Recv(); err != nil {
		die("can't receive date: %s", err.Error())
	}
	fmt.Printf("Client %d: Received response '%s'\n", id, string(response))
	sock.Close()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) > 2 && os.Args[1] == "server" {
		server(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "client" {
		client(os.Args[2], os.Args[3])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: context server <URL>\n")
	fmt.Fprintf(os.Stderr, "       context client <URL> <MSG>\n")
	os.Exit(1)
}
