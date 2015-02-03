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

// Package inproc implements an simple inproc transport for mangos.
package inproc

import (
	"github.com/gdamore/mangos"
	"sync"
)

// inproc implements the Pipe interface on top of channels.
type inproc struct {
	rq     chan *mangos.Message
	wq     chan *mangos.Message
	closeq chan struct{}
	readyq chan struct{}
	proto  mangos.Protocol
	addr   string
	peer   *inproc
}

type listener struct {
	addr      string
	proto     mangos.Protocol
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

func (p *inproc) Recv() (*mangos.Message, error) {

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
	}
}

func (p *inproc) LocalProtocol() uint16 {
	return p.proto.Number()
}

func (p *inproc) RemoteProtocol() uint16 {
	return p.proto.PeerNumber()
}

func (p *inproc) Close() error {
	close(p.closeq)
	return nil
}

func (p *inproc) IsOpen() bool {
	select {
	case <-p.closeq:
		return false
	default:
		return true
	}
}

type inprocDialer struct {
	addr  string
	proto mangos.Protocol
}

func (d *inprocDialer) Dial() (mangos.Pipe, error) {

	var server *inproc
	client := &inproc{proto: d.proto, addr: d.addr}
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

		if !mangos.ValidPeers(client.proto, l.proto) {
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

	server.wq = make(chan *mangos.Message)
	server.rq = make(chan *mangos.Message)
	client.rq = server.wq
	client.wq = server.rq
	server.peer = client
	client.peer = server

	close(server.readyq)
	close(client.readyq)
	return client, nil
}

func (l *listener) Accept() (mangos.Pipe, error) {
	server := &inproc{proto: l.proto, addr: l.addr}
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

func (l *listener) Close() error {
	listeners.mx.Lock()
	if listeners.byAddr[l.addr] == l {
		delete(listeners.byAddr, l.addr)
	}
	servers := l.accepters
	l.accepters = make([]*inproc, 0, 5)
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

func (t *inprocTran) NewDialer(addr string, proto mangos.Protocol) (mangos.PipeDialer, error) {
	return &inprocDialer{addr: addr, proto: proto}, nil
}

func (t *inprocTran) NewAccepter(addr string, proto mangos.Protocol) (mangos.PipeAccepter, error) {
	listeners.mx.Lock()
	if l, ok := listeners.byAddr[addr]; l != nil || ok {
		listeners.mx.Unlock()
		return nil, mangos.ErrAddrInUse
	}
	l := &listener{addr: addr, proto: proto}
	l.accepters = make([]*inproc, 0, 5)
	listeners.byAddr[addr] = l
	listeners.cv.Broadcast()
	listeners.mx.Unlock()
	return l, nil
}

func (*inprocTran) SetOption(string, interface{}) error {
	return mangos.ErrBadOption
}

func (*inprocTran) GetOption(string) (interface{}, error) {
	return nil, mangos.ErrBadOption
}

// NewTransport allocates a new inproc:// transport.
func NewTransport() mangos.Transport {
	return &inprocTran{}
}
