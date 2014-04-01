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
	"container/list"
	"sync"
)

// XSub is an implementation of the XSub protocol.
type xsub struct {
	sock ProtocolSocket
	subs *list.List
	sync.Mutex
}

func (p *xsub) Init(sock ProtocolSocket) {
	p.sock = sock
	p.subs = list.New()
}

func (x *xsub) Process() {
	x.ProcessSend()
	x.ProcessRecv()
}

func (x *xsub) ProcessRecv() {
	sock := x.sock
	var m *Message
	var err error
	for {
		if m, _, err = sock.RecvAnyPipe(); m == nil || err != nil {
			break
		}

		x.Lock()
		for e := x.subs.Front(); e != nil; e = e.Next() {
			if bytes.HasPrefix(m.Body, e.Value.([]byte)) {
				// Matched, send it up.  Best effort.
				sock.PushUp(m)
				break
			}
		}
		x.Unlock()
	}
}

func (x *xsub) ProcessSend() {
	// This is a an error!  Just leave the packets at the
	// "stream head".
	x.sock.PullDown()
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

const (
	// XSubOptionSubscribe is the name of the subscribe option.
	XSubOptionSubscribe = "XSUB.SUBSCRIBE"

	// XSubOptionUnsubscribe is the name of the unsubscribe option
	XSubOptionUnsubscribe = "XSUB.UNSUBSCRIBE"
)

func (*xsub) AddEndpoint(Endpoint) {}
func (*xsub) RemEndpoint(Endpoint) {}

func (x *xsub) SetOption(name string, value interface{}) error {
	x.Lock()
	defer x.Unlock()

	var vb []byte

	switch value.(type) {
	case []byte:
		vb = value.([]byte)
	default:
		return ErrBadValue
	}
	switch {
	case name == XSubOptionSubscribe:
		for e := x.subs.Front(); e != nil; e = e.Next() {
			if bytes.Equal(e.Value.([]byte), vb) {
				// Already present
				return nil
			}
		}
		x.subs.PushBack(vb)
		return nil

	case name == XSubOptionUnsubscribe:
		for e := x.subs.Front(); e != nil; e = e.Next() {
			if bytes.Equal(e.Value.([]byte), vb) {
				x.subs.Remove(e)
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
