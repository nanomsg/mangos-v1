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
	"github.com/go-mangos/mangos/protocol/surveyor"
	"github.com/go-mangos/mangos/transport/tcp"
)

func testSurvNonBlock(t *testing.T, addr string, tran mangos.Transport) {
	maxqlen := 2
	timeout := time.Second/2

	rp, err := surveyor.NewSocket()
	if err != nil {
		t.Errorf("Failed to make PUB: %v", err)
		return
	}
	defer rp.Close()
	rp.AddTransport(tran)

	// Now try setting the option
	err = rp.SetOption(mangos.OptionWriteQLen, maxqlen)
	if err != nil {
		t.Errorf("Failed set WriteQLen: %v", err)
		return
	}
	// At this point, we can issue requests on rq, and read them from rp.
	if err = rp.SetOption(mangos.OptionSendDeadline, timeout); err != nil {
		t.Errorf("Failed set recv deadline")
		return
	}
	if err = rp.Listen(addr); err != nil {
		t.Errorf("Failed listen: %v", err)
		return
	}

	msg := []byte{'A', 'B', 'C'}

	for i := 0; i < maxqlen * 2; i++ {
		t.Logf("Sending #%d", i)
		if err := rp.Send(msg); err != nil {
			t.Errorf("Failed to send: %v", err)
		}
	}
}

func TestSurvNonBlockTCP(t *testing.T) {
	testSurvNonBlock(t, AddrTestTCP, tcp.NewTransport())
}
