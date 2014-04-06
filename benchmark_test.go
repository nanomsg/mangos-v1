// Copyright 2014 Garrett D'Amore
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

package sp

import (
	"runtime"
	"testing"
	"time"
)

func benchmarkReq(t *testing.B, url string, size int) {

	srvrdy := make(chan struct{})
	srvsock, err := NewSocket(RepName)
	if err != nil || srvsock == nil {
		t.Errorf("Failed creating server socket: %v", err)
		return
	}
	defer srvsock.Close()
	clisock, err := NewSocket(ReqName)
	if err != nil || clisock == nil {
		t.Errorf("Failed creating client socket: %v", err)
		return
	}
	defer clisock.Close()

	go func() {
		var err error
		var msg *Message

		if err = srvsock.Listen(url); err != nil {
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

	if err = clisock.Dial(url); err != nil {
		t.Errorf("Client dial failed: %v", err)
		return
	}
	<-srvrdy

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
}

func benchmarkPair(t *testing.B, url string, size int) {

	finish := make(chan struct{})
	ready := make(chan struct{})
	srvsock, err := NewSocket(PairName)
	if err != nil || srvsock == nil {
		t.Errorf("Failed creating server socket: %v", err)
		return
	}
	defer srvsock.Close()
	clisock, err := NewSocket(PairName)
	if err != nil || clisock == nil {
		t.Errorf("Failed creating client socket: %v", err)
		return
	}
	defer clisock.Close()

	go func() {
		var err error
		var m *Message

		if err = srvsock.Listen(url); err != nil {
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

	if err = clisock.Dial(url); err != nil {
		t.Errorf("Client dial failed: %v", err)
		return
	}

	time.Sleep(700 * time.Millisecond)
	t.ResetTimer()
	//msg := make([]byte, size)

	for i := 0; i < t.N; i++ {
		msg := NewMessage(size)
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

func BenchmarkLatencyInproc(t *testing.B) {
	benchmarkReq(t, "inproc://somename", 0)
}
func BenchmarkLatencyIPC(t *testing.B) {
	if runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows")
		return
	}
	benchmarkReq(t, "ipc:///tmp/benchmark_ipc", 0)
}

func BenchmarkLatencyTCP(t *testing.B) {
	benchmarkReq(t, "tcp://127.0.0.1:3333", 0)
}

func BenchmarkThruputInproc(t *testing.B) {
	benchmarkPair(t, "inproc://anothername", 65536)
}
func BenchmarkThruputIPC(t *testing.B) {
	if runtime.GOOS == "windows" {
		t.Skip("IPC not supported on Windows")
		return
	}
	benchmarkPair(t, "ipc:///tmp/benchmark_ipc", 65536)
}

func BenchmarkThruputTCP(t *testing.B) {
	benchmarkPair(t, "tcp://127.0.0.1:3333", 65536)
}
