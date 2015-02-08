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

// Package sub implements the SUB protocol.  This protocol receives messages
// from publishers (PUB peers).  The messages are filtered based on
// subscription, such that only subscribed messages (see OptionSubscribe) are
// received.
package sub

import (
	"bytes"
	"sync"
	"time"

	"github.com/gdamore/mangos"
)

type sub struct {
	sock mangos.ProtocolSocket
	subs [][]byte
	raw  bool
	sync.Mutex
}

func (s *sub) Init(sock mangos.ProtocolSocket) {
	s.sock = sock
	s.subs = [][]byte{}

	go s.sender()
}

func (*sub) Shutdown(time.Duration) {} // No sender to drain.

func (s *sub) sender() {
	sq := s.sock.SendChannel()
	for {
		if m := <-sq; m == nil {
			break
		} else {
			m.Free()
		}
	}
}

func (s *sub) receiver(ep mangos.Endpoint) {
	for {
		var matched = false

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		s.Lock()
		for _, sub := range s.subs {
			if bytes.HasPrefix(m.Body, sub) {
				// Matched, send it up.  Best effort.
				matched = true
				break
			}
		}
		s.Unlock()

		if !matched {
			m.Free()
			continue
		}

		select {
		case s.sock.RecvChannel() <- m:
		case <-s.sock.CloseChannel():
			m.Free()
			return
		default: // no room, drop it
			m.Free()
		}
	}
}

func (*sub) Number() uint16 {
	return mangos.ProtoSub
}

func (*sub) PeerNumber() uint16 {
	return mangos.ProtoPub
}

func (*sub) Name() string {
	return "sub"
}

func (*sub) PeerName() string {
	return "pub"
}

func (s *sub) AddEndpoint(ep mangos.Endpoint) {
	go s.receiver(ep)
}

func (*sub) RemoveEndpoint(mangos.Endpoint) {}

func (s *sub) SetOption(name string, value interface{}) error {
	s.Lock()
	defer s.Unlock()

	var vb []byte

	// Check names first, because type check below is only valid for
	// subscription options.
	switch name {
	case mangos.OptionRaw:
		s.raw = value.(bool)
		return nil
	case mangos.OptionSubscribe:
	case mangos.OptionUnsubscribe:
	default:
		return mangos.ErrBadOption
	}

	switch value.(type) {
	case []byte:
		vb = value.([]byte)
	default:
		return mangos.ErrBadValue
	}
	switch name {
	case mangos.OptionSubscribe:
		for _, sub := range s.subs {
			if bytes.Equal(sub, vb) {
				// Already present
				return nil
			}
		}
		s.subs = append(s.subs, vb)
		return nil

	case mangos.OptionUnsubscribe:
		for i, sub := range s.subs {
			if bytes.Equal(sub, vb) {
				s.subs[i] = s.subs[len(s.subs)-1]
				s.subs = s.subs[:len(s.subs)-1]
				return nil
			}
		}
		// Subscription not present
		return mangos.ErrBadValue

	default:
		return mangos.ErrBadOption
	}
}

func (s *sub) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return s.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewProtocol returns a new SUB protocol object.
func NewProtocol() mangos.Protocol {
	return &sub{}
}

// NewSocket allocates a new Socket using the SUB protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&sub{}), nil
}
