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
	"encoding/binary"
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
	refcnt int32
}

// Consider making more of thes routines "public" for use by protocol
// implementations.  Applications should definitely *not* be making use
// of any of these.  (Device applications not withstanding.)

// getUint32 retrieves a 32-bit value from the message header.
func (m *Message) getUint32() (uint32, error) {
	if len(m.Header) < 4 {
		return 0, ErrTooShort
	}
	v := binary.BigEndian.Uint32(m.Header)
	m.Header = m.Header[4:]
	return v, nil
}

// putUint32 inserts a 32-bit value into the message header.
func (m *Message) putUint32(v uint32) {
	m.Header = append(m.Header,
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// trimUint32 moves a 32-bit value from the front of the body to the end of
// the header.  No check of the value is done.
func (m *Message) trimUint32() error {
	if len(m.Body) < 4 {
		return ErrGarbled
	}
	m.Header = append(m.Header, m.Body[:4]...)
	m.Body = m.Body[4:]
	return nil
}

// trimBackTrace modifies the message moving the backtrace from the body
// to the headers.  (Any existing header is unchanged - usually this is the
// pipe key.)  The end of the backtrace is a 32-bit value with
// the high-order bit set (the request id usually).
func (m *Message) trimBackTrace() error {
	for {
		if err := m.trimUint32(); err != nil {
			return err
		}
		// Check for high order bit set (0x80000000, big endian)
		if m.Header[len(m.Header)-4]&0x80 != 0 {
			return nil
		}
	}
}

type MessageCache struct {
	maxbody int
	cache   chan *Message
}

// We can tweak these
var messageCache = []MessageCache{
	{maxbody: 1024, cache: make(chan *Message, 1024)}, // 1 MB
	{maxbody: 8192, cache: make(chan *Message, 256)},  // 2 MB
	{maxbody: 65536, cache: make(chan *Message, 64)},  // 4 MB
}

// ReleaseMessage releases the resources for a message.  While this is not
// strictly necessary thanks to GC, doing so allows for the resources to
// be recycled without engaging GC.  This can have rather substantial
// benefits for performance.
func (m *Message) Free() {
	var ch chan *Message
	if v := atomic.AddInt32(&m.refcnt, -1); v > 0 {
		return
	}
	m.Body = m.Body[0:0]
	m.Header = m.Header[0:0]
	for i := range messageCache {
		if cap(m.Body) == messageCache[i].maxbody {
			ch = messageCache[i].cache
			break
		}
	}
	select {
	case ch <- m:
	default:
	}
}

// AddRef increments the reference count on the message.  This allows it
// to be used in multiple places.  Note that it is only safe to do this in
// cases where the caller is sure that the contents are not modified (e.g. as
// part of a multiple distribution fanout.) So this is appropriate for some
// protocol or transport implementations.  It is not appropriate for
// applications.
func (m *Message) AddRef() {
	atomic.AddInt32(&m.refcnt, 1)
}

// DecRef is an alias for Free.  It only actually free's the message when
// the reference count drops to zero.
func (m *Message) DecRef() {
	m.Free()
}

// NewMessage is the supported way to obtain a new Message.  This makes
// use of a "cache" which greatly reduces the load on the garbage collector.
func NewMessage(sz int) *Message {
	var m *Message
	var ch chan *Message
	for i := range messageCache {
		if sz < messageCache[i].maxbody {
			ch = messageCache[i].cache
			break
		}
	}
	select {
	case m = <-ch:
	default:
		m = &Message{}
		m.Body = make([]byte, 0, sz)
	}

	m.refcnt = 1
	m.Header = m.Header[:0]
	if cap(m.Header) < 32 {
		m.Header = make([]byte, 0, 32)
	}
	return m
}
