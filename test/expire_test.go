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
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pair"
	"github.com/go-mangos/mangos/transport/inproc"
)

func TestExpireDrop(t *testing.T) {
	inp := inproc.NewTransport()
	addr := AddrTestInp + "EXPIRE"

	if runtime.GOOS == "windows" {
		t.Skip("Windows clock resolution too coarse")
		return
	}
	srv, err := pair.NewSocket()
	if err != nil {
		t.Errorf("Failed to make server: %v", err)
		return
	}
	defer srv.Close()
	srv.AddTransport(inp)

	err = srv.Listen(addr)
	if err != nil {
		t.Errorf("Failed listen: %v", err)
		return
	}

	cli, err := pair.NewSocket()
	if err != nil {
		t.Errorf("Failed to make client: %v", err)
		return
	}
	defer cli.Close()

	cli.AddTransport(inp)

	err = cli.Dial(addr)
	if err != nil {
		t.Errorf("Failed dial: %v", err)
		return
	}

	time.Sleep(time.Millisecond * 100)

	err = srv.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10)
	if err != nil {
		t.Errorf("Failed set recv deadline")
		return
	}

	if err = cli.Send([]byte("GOOD")); err != nil {
		t.Errorf("Failed first send: %v", err)
		return
	}
	// Now try setting the option.  We set it to a very short window.
	// Note that we default to have a non-zero queue depth, so we will
	// still queue the message, but then when we retrieve the message
	// it will be found to be stale.
	err = cli.SetOption(mangos.OptionSendDeadline, time.Nanosecond*1)
	if err != nil {
		t.Errorf("Failed set send deadline")
		return
	}

	if err = cli.Send([]byte("DROP")); err != nil {
		t.Errorf("Failed send drop: %v", err)
		return
	}

	v, err := srv.Recv()
	if err != nil {
		t.Errorf("Failed first recv: %v", err)
		return
	} else if !bytes.Equal(v, []byte("GOOD")) {
		t.Errorf("Got wrong message: %v", v)
		return
	} else {
		t.Logf("Got good message: %v", v)
	}

	v, err = srv.Recv()
	switch err {
	case mangos.ErrRecvTimeout: // expected
		t.Logf("Deadline honored")
	case nil:
		t.Errorf("Message not dropped: %v", v)
	default:
		t.Errorf("Got unexpected error: %v", err)
	}
}
