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
	"testing"
)

func TestDeviceBadPair(t *testing.T) {
	s1, err := NewSocket(ReqName)
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()
	s2, err := NewSocket(PushName)
	if err != nil {
		t.Errorf("Failed to open S2: %v", err)
		return
	}
	defer s2.Close()

	switch err := Device(s1, s2); err {
	case ErrBadProto:
		t.Logf("Got expected err: %v", err)
		return
	case nil:
		t.Errorf("Matching incompatible types succeeded")
		return
	default:
		t.Errorf("Got unexpected err: %v", err)
		return
	}
}

func TestDeviceBadSingle(t *testing.T) {
	s1, err := NewSocket(ReqName)
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()

	switch err := Device(s1, s1); err {
	case ErrBadProto:
		t.Logf("Got expected err: %v", err)
		return
	case nil:
		t.Errorf("Matching incompatible types succeeded")
		return
	default:
		t.Errorf("Got unexpected err: %v", err)
		return
	}
}

func TestDeviceFirstNil(t *testing.T) {
	s1, err := NewSocket(PairName)
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()

	switch err := Device(nil, s1); err {
	case nil:
		t.Logf("Ok!")
		return
	default:
		t.Errorf("Got unexpected err: %v", err)
		return
	}
}

func TestDeviceSecondNil(t *testing.T) {
	s1, err := NewSocket(PairName)
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()

	switch err := Device(s1, nil); err {
	case nil:
		t.Logf("Ok!")
		return
	default:
		t.Errorf("Got unexpected err: %v", err)
		return
	}
}

func TestDeviceBothNil(t *testing.T) {
	switch err := Device(nil, nil); err {
	case ErrClosed:
		t.Logf("Got expected err: %v", err)
		return
	case nil:
		t.Errorf("Matching incompatible types succeeded")
		return
	default:
		t.Errorf("Got unexpected err: %v", err)
		return
	}
}

func TestDeviceReqRep(t *testing.T) {
	s1, err := NewSocket(ReqName)
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()
	s2, err := NewSocket(RepName)
	if err != nil {
		t.Errorf("Failed to open S2: %v", err)
		return
	}
	defer s2.Close()

	switch err := Device(s1, s2); err {
	case nil:
		t.Logf("Matching req/rep ok!")
		return
	default:
		t.Errorf("Got unexpected err: %v", err)
		return
	}
}

// TODO: Add fanout and concurrency testing.

func deviceCaseClient() []TestCase {
	// We cheat and reuse from pairTest.
	pair := &pairTest{}
	pair.id = 0
	pair.msgsz = 4
	pair.wanttx = 50
	pair.wantrx = 50
	cases := []TestCase{pair}
	return cases
}

func testDevLoop(t *testing.T, addr string) {
	s1, err := NewSocket(PairName)
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()

	SetTLSTest(t, s1)
	if err := s1.Listen(addr); err != nil {
		t.Errorf("Failed listening to AddrTestTCP: %v", err)
		return
	}

	if err := Device(s1, s1); err != nil {
		t.Errorf("Device failed: %v", err)
		return
	}

	RunTests(t, addr, deviceCaseClient())
}

func testDevChain(t *testing.T, addr1 string, addr2 string, addr3 string) {
	// This tests using multiple devices across a few transports.
	// It looks like this:  addr1->addr2->addr3 <==> addr3->addr2->addr1
	var err error
	s := make([]Socket, 5)
	for i := 0; i < 5; i++ {
		if s[i], err = NewSocket(PairName); err != nil {
			t.Errorf("Failed to open S1_1: %v", err)
			return
		}
		defer s[i].Close()
		SetTLSTest(t, s[i])
	}

	if err = s[0].Listen(addr1); err != nil {
		t.Errorf("s[0] Listen: %v", err)
		return
	}
	if err = s[1].Dial(addr2); err != nil {
		t.Errorf("s[1] Dial: %v", err)
		return
	}
	if err = s[2].Listen(addr2); err != nil {
		t.Errorf("s[2] Listen: %v", err)
		return
	}
	if err = s[3].Dial(addr3); err != nil {
		t.Errorf("s[3] Dial: %v", err)
		return
	}
	if err = s[4].Listen(addr3); err != nil {
		t.Errorf("s[4] Listen: %v", err)
		return
	}
	if err = Device(s[0], s[1]); err != nil {
		t.Errorf("s[0],s[1] Device: %v", err)
		return
	}
	if err = Device(s[2], s[3]); err != nil {
		t.Errorf("s[2],s[3] Device: %v", err)
		return
	}
	if err = Device(s[4], nil); err != nil {
		t.Errorf("s[4] Device: %v", err)
		return
	}
	RunTests(t, addr1, deviceCaseClient())
}

func TestDeviceChain(t *testing.T) {
	testDevChain(t, AddrTestTCP, AddrTestTLS, AddrTestInp)
}

func TestDeviceLoopTCP(t *testing.T) {
	testDevLoop(t, AddrTestTCP)
}

func TestDeviceLoopInp(t *testing.T) {
	testDevLoop(t, AddrTestInp)
}

func TestDeviceLoopIPC(t *testing.T) {
	testDevLoop(t, AddrTestIPC)
}

func TestDeviceLoopTLS(t *testing.T) {
	testDevLoop(t, AddrTestTLS)
}
