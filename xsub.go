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
	"bytes"
	"sync"
)

// XSub is an implementation of the XSub protocol.
type xsub struct {
	sock ProtocolSocket
	subs [][]byte
	sync.Mutex
}

func (x *xsub) Init(sock ProtocolSocket) {
	x.sock = sock
	x.subs = [][]byte{}
}

func (x *xsub) receiver(ep Endpoint) {
	for {
		var matched = false

		msg := ep.RecvMsg()
		if msg == nil {
			return
		}

		x.Lock()
		for _, sub := range x.subs {
			if bytes.HasPrefix(msg.Body, sub) {
				// Matched, send it up.  Best effort.
				matched = true
				break
			}
		}
		x.Unlock()

		if !matched {
			msg.Free()
			continue
		}

		select {
		case x.sock.RecvChannel() <- msg:
		case <-x.sock.CloseChannel():
			msg.Free()
			return
		default: // no room, drop it
			msg.Free()
		}
	}
}

func (*xsub) Name() string {
	return XSubName
}

func (*xsub) Number() uint16 {
	return ProtoSub
}

func (*xsub) IsRaw() bool {
	return true
}

func (*xsub) ValidPeer(peer uint16) bool {
	if peer == ProtoSub {
		return true
	}
	return false
}

func (x *xsub) AddEndpoint(ep Endpoint) {
	go x.receiver(ep)
}

func (*xsub) RemEndpoint(Endpoint) {}

func (x *xsub) SetOption(name string, value interface{}) error {
	x.Lock()
	defer x.Unlock()

	var vb []byte

	// Check names first, because type check below is only valid for
	// subscription options.
	switch name {
	case OptionSubscribe:
	case OptionUnsubscribe:
	default:
		return ErrBadOption
	}

	switch value.(type) {
	case []byte:
		vb = value.([]byte)
	default:
		return ErrBadValue
	}
	switch {
	case name == OptionSubscribe:
		for _, sub := range x.subs {
			if bytes.Equal(sub, vb) {
				// Already present
				return nil
			}
		}
		x.subs = append(x.subs, vb)
		return nil

	case name == OptionUnsubscribe:
		for i, sub := range x.subs {
			if bytes.Equal(sub, vb) {
				x.subs[i] = x.subs[len(x.subs)-1]
				x.subs = x.subs[:len(x.subs)-1]
				return nil
			}
		}
		// Subscription not present
		return ErrBadValue

	default:
		return ErrBadOption
	}
}

type xsubFactory int

func (xsubFactory) NewProtocol() Protocol {
	return &xsub{}
}

// XSubFactory implements the Protocol Factory for the XSUB protocol.
// The XSUB Protocol is the raw form of the SUB (Subscribe) protocol.
var XSubFactory xsubFactory
