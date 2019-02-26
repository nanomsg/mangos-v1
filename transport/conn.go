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

package transport

import (
	"encoding/binary"
	"io"
	"net"
	"sync"

	"nanomsg.org/go/mangos/v2"
)

// conn implements the Pipe interface on top of net.Conn.  The
// assumption is that transports using this have similar wire protocols,
// and conn is meant to be used as a building block.
type conn struct {
	c       net.Conn
	proto   ProtocolInfo
	open    bool
	options map[string]interface{}
	maxrx   int
	sync.Mutex
}

// connipc is *almost* like a regular conn, but the IPC protocol insists
// on stuffing a leading byte (valued 1) in front of messages.  This is for
// compatibility with nanomsg -- the value cannot ever be anything but 1.
type connipc struct {
	conn
}

// Recv implements the TranPipe Recv method.  The message received is expected
// as a 64-bit size (network byte order) followed by the message itself.
func (p *conn) Recv() (*Message, error) {

	var sz int64
	var err error
	var msg *Message

	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	// Limit messages to the maximum receive value, if not
	// unlimited.  This avoids a potential denaial of service.
	if sz < 0 || (p.maxrx > 0 && sz > int64(p.maxrx)) {
		return nil, mangos.ErrTooLong
	}
	msg = mangos.NewMessage(int(sz))
	msg.Body = msg.Body[0:sz]
	if _, err = io.ReadFull(p.c, msg.Body); err != nil {
		msg.Free()
		return nil, err
	}
	return msg, nil
}

// Send implements the Pipe Send method.  The message is sent as a 64-bit
// size (network byte order) followed by the message itself.
func (p *conn) Send(msg *Message) error {
	var buff = net.Buffers{}

	// Serialize the length header
	l := uint64(len(msg.Header) + len(msg.Body))
	lbyte := make([]byte, 8)
	binary.BigEndian.PutUint64(lbyte, l)

	// Attach the length header along with the actual header and body
	buff = append(buff, lbyte, msg.Header, msg.Body)

	if _, err := buff.WriteTo(p.c); err != nil {
		return err
	}

	msg.Free()
	return nil
}

// LocalProtocol returns our local protocol number.
func (p *conn) LocalProtocol() uint16 {
	return p.proto.Self
}

// RemoteProtocol returns our peer's protocol number.
func (p *conn) RemoteProtocol() uint16 {
	return p.proto.Peer
}

// Close implements the Pipe Close method.
func (p *conn) Close() error {
	p.Lock()
	defer p.Unlock()
	if p.open {
		p.open = false
		return p.c.Close()
	}
	return nil
}

func (p *conn) GetOption(n string) (interface{}, error) {
	if v, ok := p.options[n]; ok {
		return v, nil
	}
	return nil, mangos.ErrBadProperty
}

// NewConnPipe allocates a new Pipe using the supplied net.Conn, and
// initializes it.  It performs the handshake required at the SP layer,
// only returning the Pipe once the SP layer negotiation is complete.
//
// Stream oriented transports can utilize this to implement a Transport.
// The implementation will also need to implement PipeDialer, PipeAccepter,
// and the Transport enclosing structure.   Using this layered interface,
// the implementation needn't bother concerning itself with passing actual
// SP messages once the lower layer connection is established.
func NewConnPipe(c net.Conn, proto ProtocolInfo, options map[string]interface{}) (Pipe, error) {
	p := &conn{
		c:       c,
		proto:   proto,
		options: make(map[string]interface{}),
	}

	p.options[mangos.OptionMaxRecvSize] = int(0)
	p.options[mangos.OptionLocalAddr] = p.c.LocalAddr()
	p.options[mangos.OptionRemoteAddr] = p.c.RemoteAddr()
	for n, v := range options {
		p.options[n] = v
	}
	p.maxrx = p.options[mangos.OptionMaxRecvSize].(int)

	if err := p.handshake(); err != nil {
		return nil, err
	}

	return p, nil
}

// connHeader is exchanged during the initial handshake.
type connHeader struct {
	Zero    byte // must be zero
	S       byte // 'S'
	P       byte // 'P'
	Version byte // only zero at present
	Proto   uint16
	Rsvd    uint16 // always zero at present
}

// handshake establishes an SP connection between peers.  Both sides must
// send the header, then both sides must wait for the peer's header.
// As a side effect, the peer's protocol number is stored in the conn.
// Also, various properties are initialized.
func (p *conn) handshake() error {
	var err error

	h := connHeader{S: 'S', P: 'P', Proto: p.proto.Self}
	if err = binary.Write(p.c, binary.BigEndian, &h); err != nil {
		return err
	}
	if err = binary.Read(p.c, binary.BigEndian, &h); err != nil {
		p.c.Close()
		return err
	}
	if h.Zero != 0 || h.S != 'S' || h.P != 'P' || h.Rsvd != 0 {
		p.c.Close()
		return mangos.ErrBadHeader
	}
	// The only version number we support at present is "0", at offset 3.
	if h.Version != 0 {
		p.c.Close()
		return mangos.ErrBadVersion
	}

	// The protocol number lives as 16-bits (big-endian) at offset 4.
	if h.Proto != p.proto.Peer {
		p.c.Close()
		return mangos.ErrBadProto
	}
	p.open = true
	return nil
}
