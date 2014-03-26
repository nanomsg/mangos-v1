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
	lk   sync.Mutex
	subs *list.List
}

// Init implements the Protocol Init method.
func (p *xsub) Init(sock ProtocolSocket) {
	p.sock = sock
	p.subs = list.New()
	sock.RegisterOptionHandler(p)
}

// Process implements the Protocol Process method.
func (p *xsub) Process() {

	h := p.sock

	// We never send data to peers, so we just discard any that we see.
	h.PullDown()

	// For inbound messages, we compare the payload (the first part)
	// with our list of subscriptions.  A typical subscriber will only
	// have a few subscriptions, so for now we are using an inefficient
	// linked list.  If we have very many subscriptions, a trie would be
	// a better structure.
	m, _, err := h.RecvAnyPipe()
	if m != nil && err == nil {
		p.lk.Lock()
		for e := p.subs.Front(); e != nil; e = e.Next() {
			if bytes.HasPrefix(m.Body, e.Value.([]byte)) {
				// Matched, send it up
				h.PushUp(m)
				break
			}
		}
		p.lk.Unlock()
	}
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*xsub) Name() string {
	return XSubName
}

// Number implements the Protocol Number method.
func (*xsub) Number() uint16 {
	return ProtoSub
}

// IsRaw implements the Protocol IsRaw method.
func (*xsub) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*xsub) ValidPeer(peer uint16) bool {
	if peer == ProtoSub {
		return true
	}
	return false
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*xsub) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*xsub) SendHook(*Message) bool {
	return true
}

const (
	// XSubOptionSubscribe is the name of the subscribe option.
	XSubOptionSubscribe = "XSUB.SUBSCRIBE"

	// XSubOptionUnsubscribe is the name of the unsubscribe option
	XSubOptionUnsubscribe = "XSUB.UNSUBSCRIBE"
)

// SetOption implements the ProtocolOptionHandler SetOption method.
func (p *xsub) SetOption(name string, value interface{}) error {
	p.lk.Lock()
	defer p.lk.Unlock()

	var vb []byte

	switch value.(type) {
	case []byte:
		vb = value.([]byte)
	default:
		return ErrBadValue
	}
	switch {
	case name == XSubOptionSubscribe:
		for e := p.subs.Front(); e != nil; e = e.Next() {
			if bytes.Equal(e.Value.([]byte), vb) {
				// Already present
				return nil
			}
		}
		p.subs.PushBack(vb)
		return nil

	case name == XSubOptionUnsubscribe:
		for e := p.subs.Front(); e != nil; e = e.Next() {
			if bytes.Equal(e.Value.([]byte), vb) {
				p.subs.Remove(e)
				return nil
			}
		}
		// Subscription not present
		return ErrBadValue

	default:
		return ErrBadOption
	}
}

// GetOption well, we don't really support this at present.
// XXX: What would it mean to "GetOption" the list of subscriptions.
// Probably this is some sort of list that should be returned?
func (p *xsub) GetOption(name string) (interface{}, error) {
	return nil, ErrBadOption
}

type xsubFactory int

func (xsubFactory) NewProtocol() Protocol {
	return new(xsub)
}

// XSubFactory implements the Protocol Factory for the XSUB protocol.
// The XSUB Protocol is the raw form of the SUB (Subscribe) protocol.
var XSubFactory xsubFactory
