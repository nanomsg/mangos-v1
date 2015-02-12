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

package mangos

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
)

// conn implements the Pipe interface on top of net.Conn.  The
// assumption is that transports using this have similar wire protocols,
// and conn is meant to be used as a building block.
type conn struct {
	c     net.Conn
	rlock sync.Mutex
	wlock sync.Mutex
	proto Protocol
	open  bool
	props map[string]interface{}
}

// connipc is *almost* like a regular conn, but the IPC protocol insists
// on stuffing a leading byte (valued 1) in front of messages.  This is for
// compatibility with nanomsg -- the value cannot ever be anything but 1.
type connipc struct {
	conn
}

// Recv implements the Pipe Recv method.  The message received is expected as
// a 64-bit size (network byte order) followed by the message itself.
func (p *conn) Recv() (*Message, error) {

	var sz int64
	var err error
	var msg *Message

	// prevent interleaved reads
	p.rlock.Lock()
	defer p.rlock.Unlock()

	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	// TBD: This fixed limit is kind of silly, but it keeps
	// a bogus peer from causing us to try to allocate ridiculous
	// amounts of memory.  If you don't like it, then prealloc
	// a buffer.  But for protocols that only use small messages
	// this can actually be more efficient since we don't allocate
	// any more space than our peer says we need to.
	if sz > 1024*1024 || sz < 0 {
		p.c.Close()
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

// Send implements the Pipe Send method.  The message is sent as a 64-bit
// size (network byte order) followed by the message itself.
func (p *conn) Send(msg *Message) error {

	l := uint64(len(msg.Header) + len(msg.Body))

	// prevent interleaved writes
	p.wlock.Lock()
	defer p.wlock.Unlock()

	// send length header
	if err := binary.Write(p.c, binary.BigEndian, l); err != nil {
		return err
	}
	if _, err := p.c.Write(msg.Header); err != nil {
		return err
	}
	// hope this works
	if _, err := p.c.Write(msg.Body); err != nil {
		return err
	}
	msg.Free()
	return nil
}

// LocalProtocol returns our local protocol number.
func (p *conn) LocalProtocol() uint16 {
	return p.proto.Number()
}

// RemoteProtocol returns our peer's protocol number.
func (p *conn) RemoteProtocol() uint16 {
	return p.proto.PeerNumber()
}

// Close implements the Pipe Close method.
func (p *conn) Close() error {
	p.open = false
	return p.c.Close()
}

// IsOpen implements the PipeIsOpen method.
func (p *conn) IsOpen() bool {
	return p.open
}

func (p *conn) GetProp(n string) (interface{}, error) {
	if v, ok := p.props[n]; ok {
		return v, nil
	}
	return nil, ErrBadProperty
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
func NewConnPipe(c net.Conn, proto Protocol, props ...interface{}) (Pipe, error) {
	p := &conn{c: c, proto: proto}

	p.props = make(map[string]interface{})
	p.props[PropLocalAddr] = c.LocalAddr()
	p.props[PropRemoteAddr] = c.RemoteAddr()

	for i := 0; i+1 < len(props); i++ {
		p.props[props[i].(string)] = props[i+1]
	}
	if err := p.handshake(); err != nil {
		return nil, err
	}

	return p, nil
}

// NewConnPipeIPC allocates a new Pipe using the IPC exchange protocol.
func NewConnPipeIPC(c net.Conn, proto Protocol, props ...interface{}) (Pipe, error) {
	p := &connipc{conn: conn{c: c, proto: proto}}

	p.props = make(map[string]interface{})
	p.props[PropLocalAddr] = c.LocalAddr()
	p.props[PropRemoteAddr] = c.RemoteAddr()

	for i := 0; i+1 < len(props); i++ {
		p.props[props[i].(string)] = props[i+1]
	}
	if err := p.handshake(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *connipc) Send(msg *Message) error {

	l := uint64(len(msg.Header) + len(msg.Body))
	one := [1]byte{1}
	var err error

	// prevent interleaved writes
	p.wlock.Lock()
	defer p.wlock.Unlock()

	// send length header
	if _, err = p.c.Write(one[:]); err != nil {
		return err
	}
	if err = binary.Write(p.c, binary.BigEndian, l); err != nil {
		return err
	}
	if _, err = p.c.Write(msg.Header); err != nil {
		return err
	}
	// hope this works
	if _, err = p.c.Write(msg.Body); err != nil {
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

	// prevent interleaved reads
	p.rlock.Lock()
	defer p.rlock.Unlock()

	if _, err = p.c.Read(one[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	// TBD: This fixed limit is kind of silly, but it keeps
	// a bogus peer from causing us to try to allocate ridiculous
	// amounts of memory.  If you don't like it, then prealloc
	// a buffer.  But for protocols that only use small messages
	// this can actually be more efficient since we don't allocate
	// any more space than our peer says we need to.
	if sz > 1024*1024 || sz < 0 {
		p.c.Close()
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
func (p *conn) handshake() error {
	var err error

	h := connHeader{S: 'S', P: 'P', Proto: p.proto.Number()}
	if err = binary.Write(p.c, binary.BigEndian, &h); err != nil {
		return err
	}
	if err = binary.Read(p.c, binary.BigEndian, &h); err != nil {
		p.c.Close()
		return err
	}
	if h.Zero != 0 || h.S != 'S' || h.P != 'P' || h.Rsvd != 0 {
		p.c.Close()
		return ErrBadHeader
	}
	// The only version number we support at present is "0", at offset 3.
	if h.Version != 0 {
		p.c.Close()
		return ErrBadVersion
	}

	// The protocol number lives as 16-bits (big-endian) at offset 4.
	if h.Proto != p.proto.PeerNumber() {
		p.c.Close()
		return ErrBadProto
	}
	p.open = true
	return nil
}
