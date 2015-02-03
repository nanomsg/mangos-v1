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

// bus implements a bus example.
//
// To use:
//
//   $ go build .
//   $ url0=tcp://127.0.0.1:40890
//   $ url1=tcp://127.0.0.1:40891
//   $ url2=tcp://127.0.0.1:40892
//   $ url3=tcp://127.0.0.1:40893
//   $ ./bus node0 $url0 $url1 $url2 & node0=$!
//   $ ./bus node1 $url1 $url2 $url3 & node1=$!
//   $ ./bus node2 $url2 $url3 & node2=$!
//   $ ./bus node3 $url3 $url0 & node3=$!
//   $ sleep 5
//   $ kill $node0 $node1 $node2 $node3
//
package main

import (
	"fmt"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/bus"
	"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
	"os"
	"time"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func node(args []string) {
	var sock mangos.Socket
	var err error
	var msg []byte
	var x int

	if sock, err = bus.NewSocket(); err != nil {
		die("bus.NewSocket: %s", err)
	}
	sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(args[2]); err != nil {
		die("sock.Listen: %s", err.Error())
	}

	// wait for everyone to start listening
	time.Sleep(time.Second)
	for x = 3; x < len(args); x++ {
		if err = sock.Dial(args[x]); err != nil {
			die("socket.Dial: %s", err.Error())
		}
	}

	// wait for everyone to join
	time.Sleep(time.Second)

	fmt.Printf("%s: SENDING '%s' ONTO BUS\n", args[1], args[1])
	if err = sock.Send([]byte(args[1])); err != nil {
		die("sock.Send: %s", err.Error())
	}
	for {
		if msg, err = sock.Recv(); err != nil {
			die("sock.Recv: %s", err.Error())
		}
		fmt.Printf("%s: RECEIVED \"%s\" FROM BUS\n", args[1],
			string(msg))

	}
}

func main() {
	if len(os.Args) > 3 {
		node(os.Args)
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: bus <NODENAME> <URL> <URL>... \n")
	os.Exit(1)
}
