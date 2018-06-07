// +build windows

// Copyright 2018 The Mangos Authors
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
	"encoding/binary"
	"io"
	"net"
)

// NewConnPipeIPC allocates a new Pipe using the IPC exchange protocol.
func NewConnPipeIPC(c net.Conn, sock Socket, props ...interface{}) (Pipe, error) {
	p := &connipc{conn: conn{c: c, proto: sock.GetProtocol(), sock: sock}}

	if err := p.handshake(props); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *connipc) Send(msg *Message) error {

	l := uint64(len(msg.Header) + len(msg.Body))
	var err error

	// On Windows, we have to put everything into a contiguous buffer.
	// This is to workaround bugs in legacy libnanomsg.  Eventually we
	// might do away with this logic, but only when legacy libnanomsg
	// has been fixed.  This puts some pressure on the gc too, which
	// makes me pretty sad.

	// send length header
	buf := make([]byte, 9, 9+l)
	buf[0] = 1
	binary.BigEndian.PutUint64(buf[1:], l)
	buf = append(buf, msg.Header...)
	buf = append(buf, msg.Body...)

	if _, err = p.c.Write(buf[:]); err != nil {
		return err
	}
	msg.Free()
	return nil
}

func (p *connipc) Recv() (*Message, error) {

	var sz int64
	var err error
	var msg *Message
	var one [1]byte

	if _, err = p.c.Read(one[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	// Limit messages to the maximum receive value, if not
	// unlimited.  This avoids a potential denaial of service.
	if sz < 0 || (p.maxrx > 0 && sz > p.maxrx) {
		return nil, ErrTooLong
	}
	msg = NewMessage(int(sz))
	msg.Body = msg.Body[0:sz]
	if _, err = io.ReadFull(p.c, msg.Body); err != nil {
		msg.Free()
		return nil, err
	}
	return msg, nil
}
