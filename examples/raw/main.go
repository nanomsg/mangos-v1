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

// raw implements an example concurrent request/reply server, using
// the raw server socket.  (The clients are run using multiple cooked
// sockets.)
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ nservers=20
//   $ nclients=10
//   $ ./raw server $url $nservers & pid=$! && sleep 1
//   $ ./raw client $url $nclients
//   $ kill $pid
//
package main

import (
	"fmt"
	"os"
	"strconv"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "server" {
		nworkers := 16
		if len(os.Args) > 3 {
			var err error
			nworkers, err = strconv.Atoi(os.Args[3])
			if err != nil || nworkers < 1 {
				die("bad worker count")
			}
		}
		server(os.Args[2], nworkers)
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "client" {
		nworkers := 1
		if len(os.Args) > 3 {
			var err error
			nworkers, err = strconv.Atoi(os.Args[3])
			if err != nil || nworkers < 1 {
				die("bad worker count")
			}
		}
		client(os.Args[2], nworkers)
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: %s server|client <URL> [<workers>]\n", os.Args[0])
	os.Exit(1)
}
