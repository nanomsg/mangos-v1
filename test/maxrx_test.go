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
	"testing"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/protocol/req"
	"github.com/go-mangos/mangos/transport/tcp"
	"github.com/go-mangos/mangos/transport/ws"
)

func TestMaxRxSizeInvalidNegative(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionMaxRecvSize, -1)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted negative max recv size")
	default:
		t.Errorf("Negative test fail (-1), wrong error %v", err)
	}
}

func TestMaxRxSizeInvalidNotInt(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionMaxRecvSize, "garbage")
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted non-int value")
	default:
		t.Errorf("Negative test fail (garbage), wrong error %v", err)
	}
}

func TestMaxRxSizeSet(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionMaxRecvSize, 100)
	if err != nil {
		t.Errorf("Failed SetOption: %v", err)
		return
	}

	v, err := srep.GetOption(mangos.OptionMaxRecvSize)
	if err != nil {
		t.Errorf("Failed GetOption: %v", err)
		return
	}
	if val, ok := v.(int); !ok {
		t.Errorf("Returned value not type int")
	} else if val != 100 {
		t.Errorf("Returned value %d not %d", val, 100)
	}
}

func TestMaxRxSizeDefault(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	v, err := srep.GetOption(mangos.OptionMaxRecvSize)
	if err != nil {
		t.Errorf("Failed GetOption: %v", err)
		return
	}
	if val, ok := v.(int); !ok {
		t.Errorf("Returned value not type int")
	} else if val != 1024*1024 {
		t.Errorf("Returned value %d not %d", val, 1024*1024)
	}
}

func testMaxRx(t *testing.T, addr string, tran mangos.Transport) {
	maxrx := 100

	rp, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer rp.Close()
	rp.AddTransport(tran)

	// Now try setting the option
	err = rp.SetOption(mangos.OptionMaxRecvSize, maxrx)
	if err != nil {
		t.Errorf("Failed set MaxRecvSize: %v", err)
		return
	}
	// At this point, we can issue requests on rq, and read them from rp.
	if err = rp.SetOption(mangos.OptionRecvDeadline, time.Millisecond*20); err != nil {
		t.Errorf("Failed set recv deadline")
		return
	}
	if err = rp.Listen(addr); err != nil {
		t.Errorf("Failed listen: %v", err)
		return
	}

	rq, err := req.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REQ: %v", err)
		return
	}
	defer rq.Close()
	rq.AddTransport(tran)

	if err = rq.Dial(addr); err != nil {
		t.Errorf("Failed dial: %v", err)
		return
	}

	time.Sleep(time.Millisecond * 10)

	msg1 := make([]byte, maxrx+1)
	msg2 := make([]byte, maxrx+1)
	for i := 0; i < len(msg1); i++ {
		msg1[i] = 'A' + byte(i%26)
		msg2[i] = 'A' + byte(i%26)
	}

	// NB: maxrx *includes* the header.
	if err = rq.Send(msg1[:maxrx-8]); err != nil {
		t.Errorf("Failed first send: %v", err)
		return
	}
	v, err := rp.Recv()
	if err != nil {
		t.Errorf("Failed first recv: %v", err)
		return
	} else if !bytes.Equal(v, msg2[:maxrx-8]) {
		t.Errorf("Got wrong message: %v", v)
		return
	} else {
		t.Logf("Got good message")
	}

	if err = rq.Send(msg1); err != nil {
		t.Errorf("Failed send drop: %v", err)
		return
	}

	v, err = rp.Recv()
	switch err {
	case mangos.ErrRecvTimeout: // expected
		t.Logf("MaxRx honored")
	case nil:
		t.Errorf("Message not dropped: %v", v)
	default:
		t.Errorf("Got unexpected error: %v", err)
	}
}

func TestMaxRxTCP(t *testing.T) {
	testMaxRx(t, AddrTestTCP, tcp.NewTransport())
}

func TestMaxRxWS(t *testing.T) {
	testMaxRx(t, AddrTestWS, ws.NewTransport())
}
