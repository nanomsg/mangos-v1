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

package test

import (
	"testing"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pair"
	"github.com/go-mangos/mangos/transport/tcp"
)

func testBestEffort(t *testing.T, addr string, tran mangos.Transport) {
	timeout := time.Second/2

	rp, err := pair.NewSocket()
	if err != nil {
		t.Errorf("Failed to make PAIR: %v", err)
		return
	}
	defer rp.Close()
	rp.AddTransport(tran)

	// Now try setting the option
	err = rp.SetOption(mangos.OptionWriteQLen, 0)
	if err != nil {
		t.Errorf("Failed set WriteQLen: %v", err)
		return
	}
	if err = rp.SetOption(mangos.OptionBestEffort, true); err != nil {
		t.Errorf("Failed set best effort")
		return
	}
	if err = rp.SetOption(mangos.OptionSendDeadline, timeout); err != nil {
		t.Errorf("Failed set send deadline")
		return
	}
	if err = rp.Listen(addr); err != nil {
		t.Errorf("Failed listen: %v", err)
		return
	}

	msg := []byte{'A', 'B', 'C'}

	for i := 0; i < 2; i++ {
		t.Logf("Best Effort Send#%d", i)
		if err := rp.Send(msg); err != nil {
			t.Errorf("Failed to send: %v", err)
		}
	}

	// Now reenable blocking (and timeout)
	if err = rp.SetOption(mangos.OptionBestEffort, false); err != nil {
		t.Errorf("Failed disabling best effort")
		return
	}
	for i := 0; i < 2; i++ {
		t.Logf("Blocking Send #%d", i)
		if err := rp.Send(msg); err != mangos.ErrSendTimeout {
			t.Errorf("Send did not timeout: %v", err)
		}
	}
}

func TestBestEffortTCP(t *testing.T) {
	testBestEffort(t, AddrTestTCP, tcp.NewTransport())
}
