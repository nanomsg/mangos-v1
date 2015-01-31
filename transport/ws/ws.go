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
	"strconv"
	"strings"
	"golang.org/x/net/websocket"

	"github.com/gdamore/mangos"
	"sync"
)

// wsPipe implements the Pipe interface on a websocket
type wsPipe struct {
	ws     *websocket.Conn
	rlock  sync.Mutex
	wlock  sync.Mutex
	lproto uint16
	rproto uint16
	addr   string
	open   bool
	wg     sync.WaitGroup
}

type wsTran struct{}

func (w *wsPipe) handshake() error {
	//
	// Some browsers can only deal with strings.  So to keep
	// it simple, we use a string based handshake, that takes
	// the following form (each side sends):
	//
	// SP<version>:<proto>:<zero>
	// <version> is value "0" for now.
	// <proto> is the 16-bit numeric protocol value
	// <zero> is the value 0 (for future use)
	//
	// All values are expressed in decimal, ASCII.
	//
	send := "SP0:" + strconv.Itoa(int(w.lproto)) + ":0"
	if err := websocket.Message.Send(w.ws, send); err != nil {
		w.ws.Close()
		return err
	}

	var recv string
	if err := websocket.Message.Receive(w.ws, &recv); err != nil {
		w.ws.Close()
		return err
	}

	parts := strings.Split(recv, ":")
	if len(parts) != 3 || parts[0] != "SP0" {
		w.ws.Close()
		return mangos.ErrBadHeader
	}
	if parts[2] != "0" {
		w.ws.Close()
		return mangos.ErrBadVersion
	}

	if rproto, err := strconv.ParseUint(parts[1], 10, 16); err != nil {
		w.ws.Close()
		return mangos.ErrBadHeader
	} else {
		w.rproto = uint16(rproto)
	}
	w.open = true
	return nil
}

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
	return w.lproto
}

func (w *wsPipe) RemoteProtocol() uint16 {
	return w.rproto
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
	proto  uint16
	origin string
}

func (d *wsDialer) Dial() (mangos.Pipe, error) {
	d.origin = "http://localhost/"
	ws, err := websocket.Dial(d.addr, "x-nanomsg", d.origin)
	if err != nil {
		return nil, err
	}
	w := &wsPipe{ws: ws, lproto: d.proto, addr: d.addr}
	w.wg.Add(1)
	if err = w.handshake(); err != nil {
		return nil, err
	}
	return w, nil
}

type wsListener struct {
	pending	 []*wsPipe
	lock	 sync.Mutex
	cv	 sync.Cond
	running  bool
	addr     string
	wssvr	 websocket.Handler
	htsvr	 *http.Server
	url_	 *url.URL
	listener *net.TCPListener
	lproto   uint16
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

func (l *wsListener) Handler(ws *websocket.Conn) {
	l.lock.Lock()

	if !l.running {
		ws.Close()
		l.lock.Unlock()
		return
	}

	w := &wsPipe{ws: ws, addr: l.addr, lproto: l.lproto}
	if err := w.handshake(); err != nil {
		l.lock.Unlock()
		return
	}

	w.wg.Add(1)
	l.pending = append(l.pending, w)
	l.cv.Broadcast()
	l.lock.Unlock()

	// We must not return before the socket is closed, because
	// our caller will close the websocket on our return.
	w.wg.Wait()
}

func (l *wsListener) Close() error  {
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

func (t *wsTran) NewDialer(addr string, proto uint16) (mangos.PipeDialer, error) {
	return &wsDialer{addr: addr, proto: proto}, nil
}

func (t *wsTran) NewAccepter(addr string, proto uint16) (mangos.PipeAccepter, error) {
	var err   error
	var taddr *net.TCPAddr
	l := &wsListener{}
	l.cv.L = &l.lock

	l.lproto = proto
	l.url_, err = url.ParseRequestURI(addr)
	if err != nil {
		return nil, err
	}

        if taddr, err = net.ResolveTCPAddr("tcp", l.url_.Host); err != nil {
		return nil, err
	}

	if l.listener, err = net.ListenTCP("tcp", taddr); err != nil {
		return nil, err
	}
	l.pending = make([]*wsPipe, 0, 5)
	l.running = true

	l.htsvr = &http.Server{Addr: l.url_.Host, Handler: l}
	l.wssvr = l.Handler

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
