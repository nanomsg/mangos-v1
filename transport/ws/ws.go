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

// Package ws implements an simple websocket transport for mangos.
// This transport is considered EXPERIMENTAL.
package ws

import (
	"net"
	"net/url"
	"net/http"
	"golang.org/x/net/websocket"

	"github.com/gdamore/mangos"
	"sync"
)

// wsPipe implements the Pipe interface on a websocket
type wsPipe struct {
	ws     *websocket.Conn
	rlock  sync.Mutex
	wlock  sync.Mutex
	proto  mangos.Protocol
	addr   string
	open   bool
	wg     sync.WaitGroup
}

type wsTran struct{}

func (w *wsPipe) Recv() (*mangos.Message, error) {

	var buf []byte

	// prevent interleaved reads
	w.rlock.Lock()
	defer w.rlock.Unlock()

	if err := websocket.Message.Receive(w.ws, &buf); err != nil {
		return nil, err
	}
	msg := mangos.NewMessage(len(buf))
	// This is kind of suboptimal copying...
	msg.Body = append(msg.Body, buf...)
	return msg, nil
}

func (w *wsPipe) Send(m *mangos.Message) error {

	var buf []byte

	w.wlock.Lock()
	defer w.wlock.Unlock()

	if len(m.Header) > 0 {
		buf = make([]byte, 0, len(m.Header) + len(m.Body))
		buf = append(buf, m.Header...)
		buf = append(buf, m.Body...)
	} else {
		buf = m.Body
	}
	if err := websocket.Message.Send(w.ws, buf); err != nil {
		return err
	}
	m.Free()
	return nil
}

func (w *wsPipe) LocalProtocol() uint16 {
	return w.proto.Number()
}

func (w *wsPipe) RemoteProtocol() uint16 {
	return w.proto.PeerNumber()
}

func (w *wsPipe) Close() error {
	w.open = false
	w.ws.Close()
	w.wg.Done()
	return nil
}

func (w *wsPipe) IsOpen() bool {
	return w.open
}

type wsDialer struct {
	addr   string	// url
	proto  mangos.Protocol
	origin string
}

func (d *wsDialer) Dial() (mangos.Pipe, error) {
	pname := d.proto.PeerName() + ".sp.nanomsg.org"
	// We have to supply an origin because Go's websocket
	// implementation seems to require it.  We fake a garbage one.
	// Perhaps we should allow applications to fake this out.
	d.origin = "x://"
	ws, err := websocket.Dial(d.addr, pname, d.origin)
	if err != nil {
		return nil, err
	}
	w := &wsPipe{ws: ws, proto: d.proto, addr: d.addr, open: true}
	w.wg.Add(1)
	return w, nil
}

type wsListener struct {
	pending	 []*wsPipe
	lock	 sync.Mutex
	cv	 sync.Cond
	running  bool
	addr     string
	//wssvr	 websocket.Handler
	wssvr	 websocket.Server
	htsvr	 *http.Server
	url_	 *url.URL
	listener *net.TCPListener
	proto    mangos.Protocol
}

func (l *wsListener) Accept() (mangos.Pipe, error) {
	var w *wsPipe

	l.lock.Lock()
	defer l.lock.Unlock()

	for {
		if !l.running {
			return nil, mangos.ErrClosed	
		}
		if len(l.pending) == 0 {
			l.cv.Wait()
			continue
		}
		w = l.pending[len(l.pending)-1]
		l.pending = l.pending[:len(l.pending)-1]
		break
	}

	return w, nil
}

func (l *wsListener) handler(ws *websocket.Conn) {
	l.lock.Lock()

	if !l.running {
		ws.Close()
		l.lock.Unlock()
		return
	}

	w := &wsPipe{ws: ws, addr: l.addr, proto: l.proto, open: true}

	w.wg.Add(1)
	l.pending = append(l.pending, w)
	l.cv.Broadcast()
	l.lock.Unlock()

	// We must not return before the socket is closed, because
	// our caller will close the websocket on our return.
	w.wg.Wait()
}

func (l *wsListener) handshake (c *websocket.Config, _ *http.Request) error {
	pname := l.proto.Name() + ".sp.nanomsg.org"
	for _, p := range c.Protocol {
		if p == pname {
			c.Protocol = append([]string{}, p)
			return nil
		}
	}
	return websocket.ErrBadWebSocketProtocol
}

func (l *wsListener) Close() error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.running {
		return mangos.ErrClosed
	}
	l.listener.Close()
	l.running = false
	l.cv.Broadcast()
	for _, ws := range(l.pending) {
		ws.Close()
	}
	l.pending = l.pending[0:0]
	return nil
}

func (l *wsListener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.wssvr.ServeHTTP(w, r)
}

func (w *wsTran) Scheme() string {
	return "ws"
}

func (t *wsTran) NewDialer(addr string, proto mangos.Protocol) (mangos.PipeDialer, error) {
	return &wsDialer{addr: t.Scheme() + "://" + addr, proto: proto}, nil
}

func (t *wsTran) NewAccepter(addr string, proto mangos.Protocol) (mangos.PipeAccepter, error) {
	var err   error
	var taddr *net.TCPAddr
	l := &wsListener{}
	l.cv.L = &l.lock

	l.proto = proto
	l.url_, err = url.ParseRequestURI(t.Scheme() + "://" + addr)
	if err != nil {
		return nil, err
	}

	// We listen separately, that way we can catch and deal with the
	// case of a port already in use.

        if taddr, err = net.ResolveTCPAddr("tcp", l.url_.Host); err != nil {
		return nil, err
	}

	if l.listener, err = net.ListenTCP("tcp", taddr); err != nil {
		return nil, err
	}
	l.pending = make([]*wsPipe, 0, 5)
	l.running = true

	l.wssvr.Handler = l.handler
	l.wssvr.Handshake = l.handshake
	l.htsvr = &http.Server{Addr: l.url_.Host, Handler: l.wssvr}

	go l.htsvr.Serve(l.listener)

	return l, nil
}

func (*wsTran) SetOption(string, interface{}) error {
	return mangos.ErrBadOption
}

func (*wsTran) GetOption(string) (interface{}, error) {
	return nil, mangos.ErrBadOption
}

// NewTransport allocates a new inproc:// transport.
func NewTransport() mangos.Transport {
	return &wsTran{}
}
