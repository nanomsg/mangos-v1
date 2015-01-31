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

// pipeline implements a one way pipe example.  node0 is a listening
// pull socket, and node1 is a dialing push socket.
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ ./pipeline node0 $url & node0=$! && sleep 1
//   $ ./pipeline node1 $url "Hello, World."
//   $ ./pipeline node1 $url "Goodbye."
//   $ kill $node0
//
package main

import (
	"fmt"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/pull"
	"github.com/gdamore/mangos/protocol/push"
	"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
	"os"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func node0(url string) {
	var sock mangos.Socket
	var err error
	var msg []byte
	if sock, err = pull.NewSocket(); err != nil {
		die("can't get new pull socket: %s", err)
	}
	sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on pull socket: %s", err.Error())
	}
	for {
		// Could also use sock.RecvMsg to get header
		msg, err = sock.Recv()
		fmt.Printf("NODE0: RECEIVED \"%s\"\n", msg)
	}
}

func node1(url string, msg string) {
	var sock mangos.Socket
	var err error

	if sock, err = push.NewSocket(); err != nil {
		die("can't get new push socket: %s", err.Error())
	}
	sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(url); err != nil {
		die("can't dial on push socket: %s", err.Error())
	}
	fmt.Printf("NODE1: SENDING \"%s\"\n", msg)
	if err = sock.Send([]byte(msg)); err != nil {
		die("can't send message on push socket: %s", err.Error())
	}
	sock.Close()
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "node0" {
		node0(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 3 && os.Args[1] == "node1" {
		node1(os.Args[2], os.Args[3])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr,
		"Usage: pipeline node0|node1 <URL> <ARG> ...\n")
	os.Exit(1)
}
