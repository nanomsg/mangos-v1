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

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/transport/inproc"
)

// SetTTLZero tests that a given socket fails to set a TTL of zero.
func SetTTLZero(t *testing.T, f newSockFunc) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, 0)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted zero TTL")
	default:
		t.Errorf("Negative test fail (0), wrong error %v", err)
	}
}

// SetTTLNegative tests that a given socket fails to set a negative TTL.
func SetTTLNegative(t *testing.T, f newSockFunc) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, -1)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted negative TTL")
	default:
		t.Errorf("Negative test fail (-1), wrong error %v", err)
	}
}

// SetTTLTooBig tests that a given socket fails to set a very large TTL.
func SetTTLTooBig(t *testing.T, f newSockFunc) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, 256)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted too large TTL")
	default:
		t.Errorf("Negative test fail (256), wrong error %v", err)
	}
}

// SetTTLNotInt tests that a given socket fails to set a non-integer TTL.
func SetTTLNotInt(t *testing.T, f newSockFunc) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, "garbage")
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted non-int value")
	default:
		t.Errorf("Negative test fail (garbage), wrong error %v", err)
	}
}

// SetTTL tests that we can set a valid TTL, and get the same value back.
func SetTTL(t *testing.T, f newSockFunc) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()

	err = s.SetOption(mangos.OptionTTL, 2)
	if err != nil {
		t.Errorf("Failed SetOption: %v", err)
		return
	}

	v, err := s.GetOption(mangos.OptionTTL)
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

// TTLDropTest is a generic test for dropping based on TTL expiration.
// F1 makes the client socket, f2 makes the server socket.
func TTLDropTest(t *testing.T, cli newSockFunc, srv newSockFunc) {
	nhop := 3
	clis := make([]mangos.Socket, 0, nhop)
	srvs := make([]mangos.Socket, 0, nhop)
	inp := inproc.NewTransport()

	for i := 0; i < nhop; i++ {
		s, err := srv()
		if err != nil {
			t.Errorf("Failed to make server: %v", err)
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

		srvs = append(srvs, s)
	}

	for i := 0; i < nhop; i++ {
		s, err := cli()
		if err != nil {
			t.Errorf("Failed to make client: %v", err)
			return
		}
		defer s.Close()

		s.AddTransport(inp)

		err = s.Dial(AddrTestInp + fmt.Sprintf("HOP%d", i))
		if err != nil {
			t.Errorf("Failed dial: %v", err)
			return
		}

		clis = append(clis, s)
	}

	// Now make the device chain
	for i := 0; i < nhop-1; i++ {
		err := mangos.Device(srvs[i], clis[i+1])
		if err != nil {
			t.Errorf("Device failed: %v", err)
			return
		}
	}

	// At this point, we can issue requests on clis[0], and read them from
	// srvs[nhop-1].

	rq := clis[0]
	rp := srvs[nhop-1]

	err := rp.SetOption(mangos.OptionRecvDeadline, time.Millisecond*20)
	if err != nil {
		t.Errorf("Failed set recv deadline")
		return
	}

	t.Logf("Socket for sending is %s", rq.GetProtocol().Name())
	if err = rq.Send([]byte("GOOD")); err != nil {
		t.Errorf("Failed first send: %v", err)
		return
	}
	t.Logf("Socket for receiving is %s", rp.GetProtocol().Name())
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

	// Now try setting the option.
	err = rp.SetOption(mangos.OptionTTL, nhop-1)
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
