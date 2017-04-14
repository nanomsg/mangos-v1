// Copyright 2016 The Mangos Authors
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

// Package udp implements the UDP transport for mangos. It is based on
// M. Sustrik, Ed., "UDP Mapping for Scalability Protocols",
// sp-udp-mapping-01 (March 2014).
package udp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/go-mangos/mangos"
)

// Errors produced by this transport prototocol
var (
	ErrMsgTruncated = errors.New("message truncated due to insufficient buffer space")
)

// connHeader is exchanged during the initial handshake.
// FIXME: stolen from conn.go -- unify this
type connHeader struct {
	Zero    byte // must be zero
	S       byte // 'S'
	P       byte // 'P'
	Version byte // only zero at present
	Proto   uint16
	Rsvd    uint16 // always zero at present
}

const (
	// Maximum read queue depth for listening endpoints
	// FIXME: instead of udpMaxBacklog, use a ListenerOption to set this
	// FIXME: e.g. OptionUdpListenerBacklog
	udpMaxBacklog = 100

	// Maximum expected UDP datagram size.
	// UDPv4 datagrams can be up to 65507 = 2^16-1 - 20 - 8 bytes, due to the 2-byte
	// IPv4 total length field, a minimum IPv4 header size of 20 bytes, and the 8
	// byte UDP header length. IPv6 minimum header size is 40 bytes, so it is slightly
	// less (not considering UDPv6 jumbograms, RFC 2675).
	// These are theoretical limits, since in practice the maximum datagram size
	// is determined by the MTU (typically 1500 .. ~9000 bytes).
	// Hence a buffer size of 64k should suffice for 'regular' networks.
	udpMaxBuf = 1 << 16
)

// udpEndpoint represents attributes and methods common to udp Dialers/Listeners
type udpEndpoint struct {
	addr  *net.UDPAddr           // Dialer: remote address to connect to; Listener: local address
	opts  map[string]interface{} // Dialer or Listener options
	proto mangos.Protocol        // Local protocol
}

func (e *udpEndpoint) SetOption(name string, value interface{}) error {
	// FIXME: nothing at the moment
	return mangos.ErrBadOption
}

func (e *udpEndpoint) GetOption(name string) (interface{}, error) {
	v, ok := e.opts[name]
	if !ok {
		return nil, mangos.ErrBadOption
	}
	return v, nil
}

func (e *udpEndpoint) LocalProtocol() uint16 {
	return e.proto.Number()
}

func (e *udpEndpoint) RemoteProtocol() uint16 {
	return e.proto.PeerNumber()
}

// extractMsgFromPacket validates the SP header in @pkt and extracts it into a Message.
// FIXME: this could be refactored into conn.go
func (e *udpEndpoint) extractMsgFromPacket(pkt []byte) (*mangos.Message, error) {
	var h connHeader
	var r = bytes.NewBuffer(pkt)

	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		return nil, err
	}

	if h.Zero != 0 || h.S != 'S' || h.P != 'P' || h.Rsvd != 0 {
		return nil, mangos.ErrBadHeader
	}

	// The only version number we support at present is "0", at offset 3.
	if h.Version != 0 {
		return nil, mangos.ErrBadVersion
	}

	// The protocol number lives as 16-bits (big-endian) at offset 4.
	if h.Proto != e.proto.PeerNumber() {
		return nil, mangos.ErrBadProto
	}

	msg := mangos.NewMessage(r.Len())
	msg.Body = r.Bytes()

	return msg, nil
}

// udpPipe implements the Pipe interface on top of a UDP datagram connection.
type udpPipe struct {
	udpEndpoint
	*net.UDPConn      // bidirectional or half-open UDP "connection"
	open         bool // whether the net.UDPConn is in a usable state
	props        map[string]interface{}
}

func (p *udpPipe) GetProp(name string) (interface{}, error) {
	if v, ok := p.props[name]; ok {
		return v, nil
	}
	return nil, mangos.ErrBadProperty
}

func (p *udpPipe) Close() error {
	if !p.open {
		return nil
	}
	p.open = false
	return p.UDPConn.Close()
}

func (u *udpPipe) IsOpen() bool {
	return u.open
}

func (p *udpPipe) sendHelper(m *mangos.Message, writePkt func(b []byte) (int, error)) error {
	if !m.Expired() {
		var b bytes.Buffer
		var h = connHeader{S: 'S', P: 'P', Proto: p.proto.Number()}

		if err := binary.Write(&b, binary.BigEndian, &h); err != nil {
			return err
		}
		if _, err := b.Write(m.Header); err != nil {
			return err
		}
		if _, err := b.Write(m.Body); err != nil {
			return err
		}
		if _, err := writePkt(b.Bytes()); err != nil {
			return err
		}
	}
	m.Free()
	return nil
}

// udpFullPipe is a fully connected UDP pipe, as created by the udp PipeDialer.
type udpFullPipe struct {
	*udpPipe
	readBuf []byte // read buffer large enough to hold UDP packets
}

func (uf udpFullPipe) String() string {
	return fmt.Sprintf("udp full pipe: %s -> %s", uf.LocalAddr(), uf.RemoteAddr())
}

func (uf *udpFullPipe) Recv() (*mangos.Message, error) {
	n, err := uf.UDPConn.Read(uf.readBuf)
	if err != nil {
		return nil, err
	}
	msg, err := uf.extractMsgFromPacket(uf.readBuf[:n])
	if err != nil {
		return nil, err
	}
	return msg, err
}

func (uf *udpFullPipe) Send(m *mangos.Message) error {
	return uf.sendHelper(m, uf.UDPConn.Write)
}

// udpHalfPipe is a half-open connection associated with a remote peer, created by the udp PipeListener.
type udpHalfPipe struct {
	*udpPipe
	peer    net.Addr             // the remote endpoint talking to this Listener
	inqueue chan *mangos.Message // buffers incoming messages until Recv() is called (CAVEAT: udpMaxBacklog)
}

func (uh udpHalfPipe) String() string {
	return fmt.Sprintf("udp half pipe: %s -> %s", uh.LocalAddr(), uh.RemoteAddr())
}

// RemoteAddr overrides the function of the same name provided by UDPConn, since for listening endpoints
// it always returns nil. The @peer value is non-nil once a remote endpoint has connected to the listener.
func (uh *udpHalfPipe) RemoteAddr() net.Addr {
	return uh.peer
}

func (uh *udpHalfPipe) Recv() (*mangos.Message, error) {
	if msg, ok := <-uh.inqueue; ok {
		return msg, nil
	}
	return nil, mangos.ErrClosed
}

// Send mimics writing to a connected peer via a udp half-pipe.
func (uh *udpHalfPipe) Send(m *mangos.Message) error {
	return uh.sendHelper(m, func(b []byte) (int, error) {
		return uh.UDPConn.WriteTo(b, uh.peer)
	})
}

func (uh *udpHalfPipe) Close() error {
	halfPipePeers.Lock()
	delete(halfPipePeers.byAddr, uh.peer.String())
	close(uh.inqueue)
	halfPipePeers.Unlock()

	return uh.udpPipe.Close()
}

/*
 * Dialer Methods
 */

// dialer inherits from udpEndpoint
type dialer struct {
	*udpEndpoint
}

func (d *dialer) Dial() (mangos.Pipe, error) {
	conn, err := net.DialUDP("udp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	return &udpFullPipe{
		udpPipe: &udpPipe{
			udpEndpoint: udpEndpoint{
				addr:  d.addr,
				proto: d.proto,
				opts:  d.opts,
			},
			UDPConn: conn,
			open:    true,
			props: map[string]interface{}{
				mangos.PropLocalAddr:  conn.LocalAddr(),
				mangos.PropRemoteAddr: conn.RemoteAddr(),
			},
		},
		readBuf: make([]byte, udpMaxBuf),
	}, nil
}

/*
 * Listener methods
 */
type listener struct {
	*udpEndpoint
	*net.UDPConn // implements listener.Close()
}

func (l *listener) Address() string {
	return "udp://" + l.addr.String()
}

func (l *listener) Listen() (err error) {
	l.UDPConn, err = net.ListenUDP("udp", l.addr)
	return
}

// halfPipePeers records remote endpoints that have connected to this Listener
var halfPipePeers = struct {
	// which peer connected the listening endpoint
	byAddr map[string]*udpHalfPipe
	sync.Mutex
}{byAddr: make(map[string]*udpHalfPipe)}

// Accept sorts messages into the mailboxes of existing peers, until a new
// peer connects; causing a new udpHalfPipe to be returned.
func (l *listener) Accept() (mangos.Pipe, error) {
	for {
		var buf = make([]byte, udpMaxBuf)

		n, addr, err := l.UDPConn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		msg, err := l.extractMsgFromPacket(buf[:n])
		if err != nil {
			return nil, fmt.Errorf("invalid packet from %s: %s", addr, err)
		}

		halfPipePeers.Lock()
		uh, peerExists := halfPipePeers.byAddr[addr.String()]
		if !peerExists {
			uh = &udpHalfPipe{
				udpPipe: &udpPipe{
					udpEndpoint: udpEndpoint{
						addr:  l.addr,
						opts:  l.opts,
						proto: l.proto,
					},
					UDPConn: l.UDPConn,
					open:    true,
					props: map[string]interface{}{
						mangos.PropLocalAddr:  l.addr,
						mangos.PropRemoteAddr: addr,
					},
				},
				peer:    addr,
				inqueue: make(chan *mangos.Message, udpMaxBacklog),
			}
			halfPipePeers.byAddr[addr.String()] = uh
		}
		halfPipePeers.Unlock()

		uh.inqueue <- msg

		if !peerExists {
			return uh, nil
		}
	}
	return nil, mangos.ErrClosed
}

// udpTran implements the Transport interface for udp.
type udpTran struct{}

func (t *udpTran) Scheme() string {
	return "udp"
}

func (t *udpTran) NewDialer(addr string, sock mangos.Socket) (mangos.PipeDialer, error) {
	ep, err := t.newEndpoint(addr, sock)
	if err != nil {
		return nil, err
	}
	return &dialer{ep}, nil
}

func (t *udpTran) NewListener(addr string, sock mangos.Socket) (mangos.PipeListener, error) {
	ep, err := t.newEndpoint(addr, sock)
	if err != nil {
		return nil, err
	}
	return &listener{udpEndpoint: ep}, nil
}

func (t *udpTran) newEndpoint(addr string, sock mangos.Socket) (*udpEndpoint, error) {
	var e = &udpEndpoint{
		proto: sock.GetProtocol(),
		opts:  make(map[string]interface{}),
	}
	var err error

	if addr, err = mangos.StripScheme(t, addr); err != nil {
		return nil, err
	}
	if e.addr, err = net.ResolveUDPAddr("udp", addr); err != nil {
		return nil, err
	}

	if val, err := sock.GetOption(mangos.OptionMaxRecvSize); err == nil {
		if maxrx, ok := val.(int); ok && maxrx < udpMaxBuf {
			return nil, fmt.Errorf("MaxRecvSize(%d) to small for udp - need %d", maxrx, udpMaxBuf)
		}
		// FIXME: maybe set maxrx on socket
	}
	return e, nil
}

// NewTransport allocates a new UDP transport.
func NewTransport() mangos.Transport {
	return &udpTran{}
}
