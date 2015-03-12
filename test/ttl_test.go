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
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/transport/inproc"
)

func TestTtlInvalidZero(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionTtl, 0)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted zero TTL")
	default:
		t.Errorf("Negative test fail (0), wrong error %v")
	}
}

func TestTtlInvalidNegative(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionTtl, 0)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted negative TTL")
	default:
		t.Errorf("Negative test fail (-1), wrong error %v")
	}
}

func TestTtlInvalidTooBig(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionTtl, 256)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted too large TTL")
	default:
		t.Errorf("Negative test fail (256), wrong error %v")
	}
}

func TestTtlInvalidNotInt(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionTtl, "garbage")
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted non-int value")
	default:
		t.Errorf("Negative test fail (garbage), wrong error %v")
	}
}

func TestTtlSet(t *testing.T) {
	srep, err := rep.NewSocket()
	if err != nil {
		t.Errorf("Failed to make REP: %v", err)
		return
	}
	defer srep.Close()

	err = srep.SetOption(mangos.OptionTtl, 2)
	if err != nil {
		t.Errorf("Failed SetOption: %v", err)
		return
	}

	v, err := srep.GetOption(mangos.OptionTtl)
	if err != nil {
		t.Errorf("Failed GetOption: %v", err)
		return
	}
	if val, ok := v.(int); !ok {
		t.Errorf("Returned value not type int")
	} else if val != 2 {
		t.Errorf("Returned value %d not %d", val, 2)
	}
}

func TestTtlDrop(t *testing.T) {
	nhop := 3
	srep := make([]mangos.Socket, 0, nhop)
	sreq := make([]mangos.Socket, 0, nhop)
	inp := inproc.NewTransport()

	for i := 0; i < nhop; i++ {
		s, err := rep.NewSocket()
		if err != nil {
			t.Errorf("Failed to make REP: %v", err)
			return
		}
		defer s.Close()

		s.AddTransport(inp)

		err = s.Listen(AddrTestInp + fmt.Sprintf("HOP%d", i))
		if err != nil {
			t.Errorf("Failed listen: %v", err)
			return
		}

		err = s.SetOption(mangos.OptionRaw, true)
		if err != nil {
			t.Errorf("Failed set raw mode: %v", err)
			return
		}

		srep = append(srep, s)
	}

	for i := 0; i < nhop; i++ {
		s, err := req.NewSocket()
		if err != nil {
			t.Errorf("Failed to make REQ: %v", err)
			return
		}
		defer s.Close()

		s.AddTransport(inp)

		err = s.Dial(AddrTestInp + fmt.Sprintf("HOP%d", i))
		if err != nil {
			t.Errorf("Failed dial: %v", err)
			return
		}

		sreq = append(sreq, s)
	}

	// Now make the device chain
	for i := 0; i < nhop-1; i++ {
		err := mangos.Device(srep[i], sreq[i+1])
		if err != nil {
			t.Errorf("Device failed: %v", err)
			return
		}
	}

	// At this point, we can issue requests on sreq[0], and read them from
	// srep[nhop-1].

	rq := sreq[0]
	rp := srep[nhop-1]

	err := rp.SetOption(mangos.OptionRecvDeadline, time.Millisecond*20)
	if err != nil {
		t.Errorf("Failed set recv deadline")
		return
	}

	if err = rq.Send([]byte("GOOD")); err != nil {
		t.Errorf("Failed first send: %v", err)
		return
	}
	v, err := rp.Recv()
	if err != nil {
		t.Errorf("Failed first recv: %v", err)
		return
	} else if !bytes.Equal(v, []byte("GOOD")) {
		t.Errorf("Got wrong message: %v", v)
		return
	} else {
		t.Logf("Got good message: %v", v)
	}

	// Now try setting the option
	err = rp.SetOption(mangos.OptionTtl, nhop-1)
	if err != nil {
		t.Errorf("Failed set TTL: %v", err)
		return
	}

	if err = rq.Send([]byte("DROP")); err != nil {
		t.Errorf("Failed send drop: %v", err)
		return
	}

	v, err = rp.Recv()
	switch err {
	case mangos.ErrRecvTimeout: // expected
		t.Logf("TTL honored")
	case nil:
		t.Errorf("Message not dropped: %v", v)
	default:
		t.Errorf("Got unexpected error: %v", err)
	}
}
