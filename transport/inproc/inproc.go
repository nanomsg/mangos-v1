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

// Package inproc implements an simple inproc transport for mangos.
package inproc

import (
	"strings"
	"sync"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

// inproc implements the Pipe interface on top of channels.
type inproc struct {
	rq        chan *transport.Message
	wq        chan *transport.Message
	closeq    chan struct{}
	readyq    chan struct{}
	selfProto uint16
	peerProto uint16
	addr      addr
	peer      *inproc
	sync.Mutex
}

type addr string

func (a addr) String() string {
	s := string(a)
	if strings.HasPrefix(s, "inproc://") {
		s = s[len("inproc://"):]
	}
	return s
}

func (addr) Network() string {
	return "inproc"
}

type listener struct {
	addr      string
	selfProto uint16
	peerProto uint16
	accepters []*inproc
}

type inprocTran struct{}

var listeners struct {
	// Who is listening, on which "address"?
	byAddr map[string]*listener
	cv     sync.Cond
	mx     sync.Mutex
}

func init() {
	listeners.byAddr = make(map[string]*listener)
	listeners.cv.L = &listeners.mx
}

func (p *inproc) Recv() (*transport.Message, error) {

	if p.peer == nil {
		return nil, mangos.ErrClosed
	}
	select {
	case m, ok := <-p.rq:
		if m == nil || !ok {
			return nil, mangos.ErrClosed
		}
		// Upper protocols expect to have to pick header and
		// body part.  So mush them back together.
		//msg.Body = append(msg.Header, msg.Body...)
		//msg.Header = make([]byte, 0, 32)
		return m, nil
	case <-p.closeq:
		return nil, mangos.ErrClosed
	case <-p.peer.closeq:
		return nil, mangos.ErrClosed
	}
}

func (p *inproc) Send(m *mangos.Message) error {

	if p.peer == nil {
		return mangos.ErrClosed
	}

	// Upper protocols expect to have to pick header and body part.
	// Also we need to have a fresh copy of the message for receiver, to
	// break ownership.
	nmsg := mangos.NewMessage(len(m.Header) + len(m.Body))
	nmsg.Body = append(nmsg.Body, m.Header...)
	nmsg.Body = append(nmsg.Body, m.Body...)
	select {
	case p.wq <- nmsg:
		return nil
	case <-p.closeq:
		nmsg.Free()
		return mangos.ErrClosed
	case <-p.peer.closeq:
		nmsg.Free()
		return mangos.ErrClosed
	}
}

func (p *inproc) LocalProtocol() uint16 {
	return p.selfProto
}

func (p *inproc) RemoteProtocol() uint16 {
	return p.peerProto
}

func (p *inproc) Close() error {
	p.Lock()
	select {
	case <-p.closeq: // If already closed, don't do it again.
	default:
		close(p.closeq)
	}
	p.Unlock()
	return nil
}

func (p *inproc) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRemoteAddr:
		return p.addr, nil
	case mangos.OptionLocalAddr:
		return p.addr, nil
	}
	// We have no special properties
	return nil, mangos.ErrBadProperty
}

type dialer struct {
	addr      string
	selfProto uint16
	peerProto uint16
}

func (d *dialer) Dial() (transport.Pipe, error) {

	var server *inproc
	client := &inproc{
		selfProto: d.selfProto,
		peerProto: d.peerProto,
		addr:      addr(d.addr),
	}
	client.readyq = make(chan struct{})
	client.closeq = make(chan struct{})

	listeners.mx.Lock()

	// NB: No timeouts here!
	for {
		var l *listener
		var ok bool
		if l, ok = listeners.byAddr[d.addr]; !ok || l == nil {
			listeners.mx.Unlock()
			return nil, mangos.ErrConnRefused
		}

		if (client.selfProto != l.peerProto) ||
			(client.peerProto != l.selfProto) {
			return nil, mangos.ErrBadProto
		}

		if len(l.accepters) != 0 {
			server = l.accepters[len(l.accepters)-1]
			l.accepters = l.accepters[:len(l.accepters)-1]
			break
		}

		listeners.cv.Wait()
		continue
	}

	listeners.mx.Unlock()

	server.wq = make(chan *transport.Message)
	server.rq = make(chan *transport.Message)
	client.rq = server.wq
	client.wq = server.rq
	server.peer = client
	client.peer = server

	close(server.readyq)
	close(client.readyq)
	return client, nil
}

func (*dialer) SetOption(string, interface{}) error {
	return mangos.ErrBadOption
}

func (*dialer) GetOption(string) (interface{}, error) {
	return nil, mangos.ErrBadOption
}

func (l *listener) Listen() error {
	listeners.mx.Lock()
	if _, ok := listeners.byAddr[l.addr]; ok {
		listeners.mx.Unlock()
		return mangos.ErrAddrInUse
	}

	listeners.byAddr[l.addr] = l
	listeners.cv.Broadcast()
	listeners.mx.Unlock()
	return nil
}

func (l *listener) Address() string {
	return l.addr
}

func (l *listener) Accept() (mangos.TranPipe, error) {
	server := &inproc{
		selfProto: l.selfProto,
		peerProto: l.peerProto,
		addr:      addr(l.addr),
	}
	server.readyq = make(chan struct{})
	server.closeq = make(chan struct{})

	listeners.mx.Lock()
	l.accepters = append(l.accepters, server)
	listeners.cv.Broadcast()
	listeners.mx.Unlock()

	select {
	case <-server.readyq:
		return server, nil
	case <-server.closeq:
		return nil, mangos.ErrClosed
	}
}

func (*listener) SetOption(string, interface{}) error {
	return mangos.ErrBadOption
}

func (*listener) GetOption(string) (interface{}, error) {
	return nil, mangos.ErrBadOption
}

func (l *listener) Close() error {
	listeners.mx.Lock()
	if listeners.byAddr[l.addr] == l {
		delete(listeners.byAddr, l.addr)
	}
	servers := l.accepters
	l.accepters = nil
	listeners.cv.Broadcast()
	listeners.mx.Unlock()

	for _, s := range servers {
		close(s.closeq)
	}

	return nil
}

func (t *inprocTran) Scheme() string {
	return "inproc"
}

func (t *inprocTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	if _, err := transport.StripScheme(t, addr); err != nil {
		return nil, err
	}
	d := &dialer{
		addr:      addr,
		selfProto: sock.Info().Self,
		peerProto: sock.Info().Peer,
	}
	return d, nil
}

func (t *inprocTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	if _, err := transport.StripScheme(t, addr); err != nil {
		return nil, err
	}
	l := &listener{
		addr:      addr,
		selfProto: sock.Info().Self,
		peerProto: sock.Info().Peer,
	}
	return l, nil
}

// NewTransport allocates a new inproc:// transport.
func NewTransport() transport.Transport {
	return &inprocTran{}
}
