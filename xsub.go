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
	"container/list"
	"strings"
	"sync"
)

// XSub is an implementation of the XSub protocol.
type XSub struct {
	handle ProtocolHandle
	L      sync.Mutex
	subs   *list.List
}

// Init implements the Protocol Init method.
func (p *XSub) Init(handle ProtocolHandle) {
	p.handle = handle
	p.subs = list.New()
	handle.RegisterOptionHandler(p)
}

// Process implements the Protocol Process method.
func (p *XSub) Process() {

	h := p.handle

	// We never send data to peers, so we just discard any that we see.
	h.PullDown()

	// For inbound messages, we compare the payload (the first part)
	// with our list of subscriptions.  A typical subscriber will only
	// have a few subscriptions, so for now we are using an inefficient
	// linked list.  If we have very many subscriptions, a trie would be
	// a better structure.
	m, _, err := h.Recv()
	if m != nil && err == nil {
		p.L.Lock()
		for e := p.subs.Front(); e != nil; e = e.Next() {
			if strings.HasPrefix(string(m.Body), e.Value.(string)) {
				// Matched, send it up
				h.PushUp(m)
				break
			}
		}
		p.L.Unlock()
	}
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*XSub) Name() string {
	return "XSub"
}

// Number implements the Protocol Number method.
func (*XSub) Number() uint16 {
	return ProtoSub
}

// IsRaw implements the Protocol IsRaw method.
func (*XSub) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*XSub) ValidPeer(peer uint16) bool {
	if peer == ProtoSub {
		return true
	}
	return false
}

// RecvHook implements the Protocol RecvHook method.  It is a no-op.
func (*XSub) RecvHook(*Message) bool {
	return true
}

// SendHook implements the Protocol SendHook method.  It is a no-op.
func (*XSub) SendHook(*Message) bool {
	return true
}

// SetOption implements the ProtocolOptionHandler SetOption method.
func (p *XSub) SetOption(name string, value interface{}) error {
	p.L.Lock()
	defer p.L.Unlock()

	var vstr string

	switch value.(type) {
	case string:
		vstr = value.(string)
	default:
		return EBadValue
	}
	switch {
	case name == "XSub.SUB":
		for e := p.subs.Front(); e != nil; e = e.Next() {
			if e.Value.(string) == vstr {
				// Already present
				return nil
			}
		}
		p.subs.PushBack(vstr)
		return nil

	case name == "XSub.UNSUB":
		for e := p.subs.Front(); e != nil; e = e.Next() {
			if e.Value.(string) == vstr {
				p.subs.Remove(e)
				return nil
			}
		}
		// Subscription not present
		return EBadValue

	default:
		return EBadOption
	}
}

// GetOption well, we don't really support this at present.
// XXX: What would it mean to "GetOption" the list of subscriptions.
// Probably this is some sort of list that should be returned?
func (p *XSub) GetOption(name string) (interface{}, error) {
	return nil, EBadOption
}
