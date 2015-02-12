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

package test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/pair"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/transport/all"
)

func benchmarkReq(t *testing.B, url string, size int) {

	if strings.HasPrefix(url, "ipc://") && runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows")
		return
	}

	srvopts := make(map[string]interface{})
	cliopts := make(map[string]interface{})

	if strings.HasPrefix(url, "wss://") || strings.HasPrefix(url, "tls+tcp://") {
		srvopts[mangos.OptionTlsConfig] = srvCfg
		cliopts[mangos.OptionTlsConfig] = cliCfg
	}
	srvrdy := make(chan struct{})
	srvsock, err := rep.NewSocket()
	if err != nil || srvsock == nil {
		t.Errorf("Failed creating server socket: %v", err)
		return
	}
	defer srvsock.Close()

	all.AddTransports(srvsock)

	clisock, err := req.NewSocket()
	if err != nil || clisock == nil {
		t.Errorf("Failed creating client socket: %v", err)
		return
	}
	defer clisock.Close()
	all.AddTransports(clisock)

	go func() {
		var err error
		var msg *mangos.Message

		if err = srvsock.ListenOptions(url, srvopts); err != nil {
			t.Errorf("Server listen failed: %v", err)
			return
		}

		close(srvrdy)
		// echo server

		for {
			if msg, err = srvsock.RecvMsg(); err != nil {
				return
			}
			if err = srvsock.SendMsg(msg); err != nil {
				t.Errorf("Server send failed: %v", err)
				return
			}
		}

	}()

	if err = clisock.DialOptions(url, cliopts); err != nil {
		t.Errorf("Client dial failed: %v", err)
		return
	}
	<-srvrdy

	time.Sleep(time.Millisecond * 1000)
	t.ResetTimer()
	msg := make([]byte, size)

	for i := 0; i < t.N; i++ {
		if err = clisock.Send(msg); err != nil {
			t.Errorf("Client send failed: %v", err)
			return
		}
		if msg, err = clisock.Recv(); err != nil {
			t.Errorf("Client receive failed: %v", err)
			return
		}
	}
	if size > 128 {
		t.SetBytes(int64(size))
	}
	t.StopTimer()
}

func benchmarkPair(t *testing.B, url string, size int) {

	if strings.HasPrefix(url, "ipc://") && runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows")
		return
	}

	srvopts := make(map[string]interface{})
	cliopts := make(map[string]interface{})

	if strings.HasPrefix(url, "wss://") || strings.HasPrefix(url, "tls+tcp://") {
		srvopts[mangos.OptionTlsConfig] = srvCfg
		cliopts[mangos.OptionTlsConfig] = cliCfg
	}

	finish := make(chan struct{})
	ready := make(chan struct{})
	srvsock, err := pair.NewSocket()
	if err != nil || srvsock == nil {
		t.Errorf("Failed creating server socket: %v", err)
		return
	}
	all.AddTransports(srvsock)
	defer srvsock.Close()

	clisock, err := pair.NewSocket()
	if err != nil || clisock == nil {
		t.Errorf("Failed creating client socket: %v", err)
		return
	}
	all.AddTransports(clisock)
	defer clisock.Close()

	go func() {
		var err error
		var m *mangos.Message

		if err = srvsock.ListenOptions(url, srvopts); err != nil {
			t.Errorf("Server listen failed: %v", err)
			return
		}
		close(ready)
		for i := 0; i < t.N; i++ {
			if m, err = srvsock.RecvMsg(); err != nil {
				t.Errorf("Error receiving %d: %v", i, err)
				return
			}
			m.Free()
		}
		close(finish)

	}()
	<-ready

	if err = clisock.DialOptions(url, cliopts); err != nil {
		t.Errorf("Client dial failed: %v", err)
		return
	}

	time.Sleep(700 * time.Millisecond)
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		msg := mangos.NewMessage(size)
		if err = clisock.SendMsg(msg); err != nil {
			t.Errorf("Client send failed: %v", err)
			return
		}
	}
	<-finish
	t.StopTimer()
	if size > 128 {
		t.SetBytes(int64(size))
	}
}

var benchInpAddr = "inproc://benchmark_test"
var benchTCPAddr = "tcp://127.0.0.1:33833"
var benchIPCAddr = "ipc:///tmp/benchmark_test"
var benchTLSAddr = "tls+tcp://127.0.0.1:44844"
var benchWSAddr = "ws://127.0.0.1:55855/BENCHMARK"
var benchWSSAddr = "wss://127.0.0.1:55856/BENCHMARK"

func BenchmarkLatencyInp(t *testing.B) {
	benchmarkReq(t, benchInpAddr, 0)
}
func BenchmarkLatencyIPC(t *testing.B) {
	benchmarkReq(t, benchIPCAddr, 0)
}
func BenchmarkLatencyTCP(t *testing.B) {
	benchmarkReq(t, benchTCPAddr, 0)
}
func BenchmarkLatencyTLS(t *testing.B) {
	benchmarkReq(t, benchTLSAddr, 0)
}
func BenchmarkLatencyWS(t *testing.B) {
	benchmarkReq(t, benchWSAddr, 0)
}
func BenchmarkLatencyWSS(t *testing.B) {
	benchmarkReq(t, benchWSSAddr, 0)
}

func BenchmarkTPut4kInp(t *testing.B) {
	benchmarkPair(t, benchInpAddr, 4096)
}
func BenchmarkTPut4kIPC(t *testing.B) {
	benchmarkPair(t, benchIPCAddr, 4096)
}
func BenchmarkTPut4kTCP(t *testing.B) {
	benchmarkPair(t, benchTCPAddr, 4096)
}
func BenchmarkTPut4kTLS(t *testing.B) {
	benchmarkPair(t, benchTLSAddr, 4096)
}
func BenchmarkTPut4kWS(t *testing.B) {
	benchmarkPair(t, benchWSAddr, 4096)
}
func BenchmarkTPut4kWSS(t *testing.B) {
	benchmarkPair(t, benchWSSAddr, 4096)
}

func BenchmarkTPut64kInp(t *testing.B) {
	benchmarkPair(t, benchInpAddr, 65536)
}
func BenchmarkTPut64kIPC(t *testing.B) {
	benchmarkPair(t, benchIPCAddr, 65536)
}
func BenchmarkTPut64kTCP(t *testing.B) {
	benchmarkPair(t, benchTCPAddr, 65536)
}
func BenchmarkTPut64kTLS(t *testing.B) {
	benchmarkPair(t, benchTLSAddr, 65536)
}
func BenchmarkTPut64kWS(t *testing.B) {
	benchmarkPair(t, benchWSAddr, 65536)
}
func BenchmarkTPut64kWSS(t *testing.B) {
	benchmarkPair(t, benchWSSAddr, 65536)
}
