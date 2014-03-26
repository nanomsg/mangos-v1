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
	"net"
	"sync"
)

// ConnPipe implements the Pipe interface on top of net.Conn.  The
// assumption is that transports using this have similar wire protocols,
// and ConnPipe is meant to be used as a building block.
//
type connPipe struct {
	conn   net.Conn
	rlock  sync.Mutex
	wlock  sync.Mutex
	rproto uint16
	lproto uint16
	open   bool
	cdata  interface{}
	pdata  interface{}
}

// Recv implements the Pipe Recv method.  The message received is expected as
// a 64-bit size (network byte order) followed by the message itself.
func (p *connPipe) Recv() (*Message, error) {

	var sz uint64
	h := make([]byte, 8)

	// prevent interleaved reads
	p.rlock.Lock()
	defer p.rlock.Unlock()

	if err := p.recvAll(h); err != nil {
		return nil, err
	}
	// decode length header
	sz = getUint64(h)

	// TBD: This fixed limit is kind of silly, but it keeps
	// a bogus peer from causing us to try to allocate ridiculous
	// amounts of memory.  If you don't like it, then prealloc
	// a buffer.  But for protocols that only use small messages
	// this can actually be more efficient since we don't allocate
	// any more space than our peer says we need to.
	if sz > 1024*1024 {
		p.conn.Close()
		return nil, ErrTooLong
	}
	b := make([]byte, sz)
	if err := p.recvAll(b); err != nil {
		return nil, err
	}
	msg := new(Message)
	msg.Header = make([]byte, 0, 32) // Header empty, but room to grow
	msg.Body = b                     // The whole payload is the body
	return msg, nil
}

// Send implements the Pipe Send method.  The message is sent as a 64-bit
// size (network byte order) followed by the message itself.
func (p *connPipe) Send(msg *Message) (err error) {

	h := make([]byte, 8)
	l := uint64(len(msg.Header) + len(msg.Body))
	putUint64(h, l)

	// prevent interleaved writes
	p.wlock.Lock()
	defer p.wlock.Unlock()

	// send length header
	err = p.sendAll(h)
	if err != nil {
		return
	}
	err = p.sendAll(msg.Header)
	if err != nil {
		return
	}
	err = p.sendAll(msg.Body)
	if err != nil {
		return
	}
	return
}

// LocalProtocol returns our local protocol number.
func (p *connPipe) LocalProtocol() uint16 {
	return p.lproto
}

// RemoteProtocol returns our peer's protocol number.
func (p *connPipe) RemoteProtocol() uint16 {
	return p.rproto
}

// Close implements the Pipe Close method.
func (p *connPipe) Close() error {
	p.open = false
	return p.conn.Close()
}

// IsOpen implements the PipeIsOpen method.
func (p *connPipe) IsOpen() bool {
	return p.open
}

// SetCoreData implements the Pipe SetCoreData method.
func (p *connPipe) SetCoreData(data interface{}) {
	p.cdata = data
}

// GetCoreData implements the Pipe GetCoreData method.
func (p *connPipe) GetCoreData() interface{} {
	return p.cdata
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
func NewConnPipe(conn net.Conn, lproto uint16) (Pipe, error) {
	p := new(connPipe)
	p.conn = conn
	p.lproto = lproto
	p.rproto = 0

	if err := p.handshake(); err != nil {
		return nil, err
	}

	return p, nil
}

// sendAll sends until the array is sent or an error occurs.
func (p *connPipe) sendAll(b []byte) (err error) {
	sent := 0
	for n := 0; sent < len(b) && err == nil; sent += n {
		n, err = p.conn.Write(b[sent:])
	}
	if err != nil {
		p.conn.Close()
	}
	return
}

// recvAll receives until the array is filled or an error occurs.
func (p *connPipe) recvAll(b []byte) (err error) {
	recd := 0
	for n := 0; recd < len(b) && err == nil; recd += n {
		n, err = p.conn.Read(b[recd:])
	}
	if err != nil {
		p.conn.Close()
	}
	return
}

// handshake establishes an SP connection between peers.  Both sides must
// send the header, then both sides must wait for the peer's header.
// As a side effect, the peer's protocol number is stored in the ConnPipe.
func (p *connPipe) handshake() error {
	h := []byte{0, 'S', 'P', 0, 0, 0, 0, 0}
	// include our protocol number - big endian
	h[4] = byte(p.lproto >> 8) // type (high byte)
	h[5] = byte(p.lproto)      // type (low byte)

	if err := p.sendAll(h); err != nil {
		return err
	}
	if err := p.recvAll(h); err != nil {
		return err
	}
	if h[0] != 0 || h[1] != 'S' || h[2] != 'P' || h[6] != 0 || h[7] != 0 {
		p.conn.Close()
		return ErrBadHeader
	}
	// The only version number we support at present is "0", at offset 3.
	if h[3] != 0 {
		p.conn.Close()
		return ErrBadVersion
	}

	// The protocol number lives as 16-bits (big-endian) at offset 4.
	p.rproto = (uint16(h[4]) << 8) + uint16(h[5])
	p.open = true
	return nil
}
