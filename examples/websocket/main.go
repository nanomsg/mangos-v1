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

// websocket implements a simple websocket server for mangos, demonstrating
// how to use multiplex multiple sockets on a single HTTP server instance.
//
// The server listens, and offers three paths:
//
//  - sub/    - SUB socket, publishes a message "BING!" once per second
//  - req/    - REQ socket, responds with a reply "REPLY"
//  - static/ - static content, provided as ASCII "STATIC"
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ ./websocket server $url & pid=$! && sleep 1
//   $ ./websocket req $url
//   $ ./websocket sub $url
//   $ ./websocket static $url
//   $ kill $pid
//
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func usage() {
	die("Usage: %s <server|req|sub|static> <port>\n", os.Args[0])
}

func main() {

	if len(os.Args) != 3 {
		usage()
	}
	port, e := strconv.Atoi(os.Args[2])
	if e != nil || port < 0 || port > 65535 {
		die("Invalid port number")
	}
	switch os.Args[1] {
	case "server":
		server(port)
	case "req":
		reqClient(port)
	case "sub":
		subClient(port)
	case "static":
		staticClient(port)
	default:
		usage()
	}
	os.Exit(0)
}

func server(port int) {
	mux := http.NewServeMux()

	addStaticHandler(mux)
	addSubHandler(mux, port)
	addReqHandler(mux, port)

	e := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	die("Http server died: %v", e)
}

func staticClient(port int) {
	url := fmt.Sprintf("http://127.0.0.1:%d/static", port)
	resp, err := http.Get(url)
	if err != nil {
		die("Get failed: %v", err)
	}
	if resp.StatusCode != 200 {
		die("Status code %d", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("%s\n", string(body))
	os.Exit(0)
}
