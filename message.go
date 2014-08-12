// Copyright 2014 The Mangos Authors
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

package mangos

import (
	"sync/atomic"
)

// Message encapsulates the messages that we exchange back and forth.  The
// meaning of the Header and Body fields, and where the splits occur, will
// vary depending on the protocol.  Note however that any headers applied by
// transport layers (including TCP/ethernet headers, and SP protocol
// independent length headers), are *not* included in the Header.
type Message struct {
	Header []byte
	Body   []byte
	bbuf   []byte
	hbuf   []byte
	bsize  int
	refcnt int32
}

type msgCacheInfo struct {
	maxbody int
	cache   chan *Message
}

// We can tweak these!
var messageCache = []msgCacheInfo{
	{maxbody: 64, cache: make(chan *Message, 2048)},   // 128K
	{maxbody: 128, cache: make(chan *Message, 1024)},  // 128K
	{maxbody: 1024, cache: make(chan *Message, 1024)}, // 1 MB
	{maxbody: 8192, cache: make(chan *Message, 256)},  // 2 MB
	{maxbody: 65536, cache: make(chan *Message, 64)},  // 4 MB
}

// Free decrements the reference count on a message, and releases its
// resources if no further references remain.  While this is not
// strictly necessary thanks to GC, doing so allows for the resources to
// be recycled without engaging GC.  This can have rather substantial
// benefits for performance.
func (m *Message) Free() {
	var ch chan *Message
	if v := atomic.AddInt32(&m.refcnt, -1); v > 0 {
		return
	}
	for i := range messageCache {
		if m.bsize == messageCache[i].maxbody {
			ch = messageCache[i].cache
			break
		}
	}
	select {
	case ch <- m:
	default:
	}
}

// Dup creates a "duplicate" message.  What it really does is simply
// increment the reference count on the message.  Note that since the
// underlying message is actually shared, consumers must take care not
// to modify the message.  (We might revise this API in the future to
// add a copy-on-write facility, but for now modification is neither
// needed nor supported.)  Applications should *NOT* make use of this
// function -- it is intended for Protocol, Transport and internal use only.
func (m *Message) Dup() *Message {
	atomic.AddInt32(&m.refcnt, 1)
	return m
}

// NewMessage is the supported way to obtain a new Message.  This makes
// use of a "cache" which greatly reduces the load on the garbage collector.
func NewMessage(sz int) *Message {
	var m *Message
	var ch chan *Message
	for i := range messageCache {
		if sz < messageCache[i].maxbody {
			ch = messageCache[i].cache
			sz = messageCache[i].maxbody
			break
		}
	}
	select {
	case m = <-ch:
	default:
		m = &Message{}
		m.bbuf = make([]byte, 0, sz)
		m.hbuf = make([]byte, 0, 32)
		m.bsize = sz
	}

	m.refcnt = 1
	m.Body = m.bbuf
	m.Header = m.hbuf
	return m
}
