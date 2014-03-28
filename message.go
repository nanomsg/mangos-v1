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
)

// Message encapsulates the messages that we exchange back and forth.  The
// meaning of the Header and Body fields, and where the splits occur, will
// vary depending on the protocol.  Note however that any headers applied by
// transport layers (including TCP/ethernet headers, and SP protocol
// independent length headers), are *not* included in the Header.
type Message struct {
	Header []byte
	Body   []byte
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

// putPipeKey inserts a 32-bit pipe key into the front of the header.
func (m *Message) putPipeKey(v PipeKey) {
	m.putUint32(uint32(v))
}

// getPipeKey retrieves a 32-bit pipe key from the front of the header.
func (m *Message) getPipeKey() (PipeKey, error) {
	if len(m.Header) < 4 {
		// oops!
		return 0, ErrTooShort
	}
	key := PipeKey(getUint32(m.Header))
	m.Header = m.Header[4:]
	return key, nil
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
