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

// Package perf provides utilities to measure mangos peformance against
// libnanomsg' perf tools.

package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
)

func usage() {
	fmt.Printf("Bad Usage!\n")
	os.Exit(1)
}

func doRemoteLatency(args []string) {
	if len(args) < 3 {
		log.Fatalf("Usage: remote_lat <connect-to> <msg-size> <roundtrips>")
	}
	addr := args[0]
	msgSize, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Bad msgsize: %v", err)
	}
	roundTrips, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalf("Bad roundtrips: %v", err)
	}
	LatencyClient(addr, msgSize, roundTrips)
	os.Exit(0)
}

func doLocalLatency(args []string) {
	if len(args) < 3 {
		log.Fatalf("Usage: local_lat <connect-to> <msg-size> <roundtrips>")
	}
	addr := args[0]
	msgSize, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Bad msgsize: %v", err)
	}
	roundTrips, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalf("Bad roundtrips: %v", err)
	}
	LatencyServer(addr, msgSize, roundTrips)
	os.Exit(0)
}

func doRemoteThroughput(args []string) {
	if len(args) < 3 {
		log.Fatalf("Usage: remote_thr <connect-to> <msg-size> <msg-count>")
	}
	addr := args[0]
	msgSize, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Bad msg-size: %v", err)
	}
	msgCount, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalf("Bad msg-count: %v", err)
	}
	ThroughputClient(addr, msgSize, msgCount)
	os.Exit(0)
}

func doLocalThroughput(args []string) {
	if len(args) < 3 {
		log.Fatalf("Usage: local_thr <connect-to> <msg-size> <msg-count>")
	}
	addr := args[0]
	msgSize, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Bad msg-size: %v", err)
	}
	msgCount, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalf("Bad msg-count: %v", err)
	}
	ThroughputServer(addr, msgSize, msgCount)
	os.Exit(0)
}

func doInprocLat(args []string) {
	if len(args) < 2 {
		log.Fatalf("Usage: inproc_lat <msg-size> <roundtrip-count>")
	}

	size, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatalf("Bad msg-size: %v", err)
	}
	count, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Bad roundtrip-count: %v", err)
	}
	go LatencyServer("inproc://inproc_lat", size, count)
	LatencyClient("inproc://inproc_lat", size, count)
	os.Exit(0)
}

func doInprocThr(args []string) {
	if len(args) < 2 {
		log.Fatalf("Usage: inproc_thr <msg-size> <msg-count>")
	}

	size, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatalf("Bad msg-size: %v", err)
	}
	count, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Bad msg-count: %v", err)
	}
	go ThroughputClient("inproc://inproc_lat", size, count)
	ThroughputServer("inproc://inproc_lat", size, count)
	os.Exit(0)
}

func main() {
	args := os.Args

	tries := 0
	for tries = 0; tries < 2; tries++ {
		switch path.Base(args[0]) {
		case "latency_client":
			fallthrough
		case "remote_lat":
			doRemoteLatency(args[1:])

		case "latency_server":
			fallthrough
		case "local_lat":
			doLocalLatency(args[1:])

		case "throughput_client":
			fallthrough
		case "remote_thr":
			doRemoteThroughput(args[1:])

		case "througput_server":
			fallthrough
		case "local_thr":
			doLocalThroughput(args[1:])

		case "inproc_thr":
			doInprocThr(args[1:])

		case "inproc_lat":
			doInprocLat(args[1:])

		default:
			args = args[1:]
		}
	}
	usage()
}
