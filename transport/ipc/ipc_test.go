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

package ipc

import (
	"runtime"
	"testing"

	"github.com/go-mangos/mangos/test"
)

var tt = test.NewTranTest(NewTransport(), "ipc:///tmp/test1234")

func TestIpcListenAndAccept(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("IPC not supported on Windows")
	case "plan9":
		t.Skip("IPC not supported on Plan9")
	default:
		tt.TestListenAndAccept(t)
	}
}

func TestIpcDuplicateListen(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("IPC not supported on Windows")
	case "plan9":
		t.Skip("IPC not supported on Plan9")
	default:
		tt.TestDuplicateListen(t)
	}
}

func TestIpcConnRefused(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("IPC not supported on Windows")
	case "plan9":
		t.Skip("IPC not supported on Plan9")
	default:
		tt.TestConnRefused(t)
	}
}

func TestIpcSendRecv(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("IPC not supported on Windows")
	case "plan9":
		t.Skip("IPC not supported on Plan9")
	default:
		tt.TestSendRecv(t)
	}
}

func TestIpcAll(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("IPC not supported on Windows")
	case "plan9":
		t.Skip("IPC not supported on Plan9")
	default:
		tt.TestAll(t)
	}
}
