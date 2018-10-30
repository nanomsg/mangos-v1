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

// Package ws implements a simple WebSocket transport for mangos.
// This transport is considered EXPERIMENTAL.
package ws

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

// Some special options
const (
	// OptionWebSocketMux is a retrieve-only property used to obtain
	// the *http.ServeMux instance associated with the server.  This
	// can be used to subsequently register additional handlers for
	// different URIs.  This option is only valid on a Listener.
	// Generally you use this option when you want to use the standard
	// mangos Listen() method to start up the server.
	OptionWebSocketMux = "WEBSOCKET-MUX"

	// OptionWebSocketHandler is used to obtain the underlying
	// http.Handler (websocket.Server) object, so you can use this
	// on your own http.Server instances.  It is a gross error to use
	// the value returned by this method on an http server if the
	// server is also started with mangos Listen().  This means that you
	// will use at most either this option, or OptionWebSocketMux, but
	// never both.  This option is only valid on a listener.
	OptionWebSocketHandler = "WEBSOCKET-HANDLER"

	// OptionWebSocketCheckOrigin controls the check of the origin of the
	// underlying Listener (websocket.Upgrader).
	// Excerpt from https://godoc.org/github.com/gorilla/websocket:
	// Web browsers allow Javascript applications to open a WebSocket
	// connection to any host. It's up to the server to enforce an origin
	// policy using the Origin request header sent by the browser. The
	// Upgrader calls the function specified in the CheckOrigin field to
	// check the origin. If the CheckOrigin function returns false, then
	// the Upgrade method fails the WebSocket handshake with HTTP status
	// 403. If the CheckOrigin field is nil, then the Upgrader uses a safe
	// default: fail the handshake if the Origin request header is present
	// and not equal to the Host request header. An application can allow
	// connections from any origin by specifying a function that always
	// returns true:
	//
	// var upgrader = websocket.Upgrader{
	//         CheckOrigin: func(r *http.Request) bool { return true },
	// }
	//
	// The deprecated Upgrade function does not enforce an origin policy.
	// It's the application's responsibility to check the Origin header
	// before calling Upgrade.
	OptionWebSocketCheckOrigin = "WEBSOCKET-CHECKORIGIN"
)

type options map[string]interface{}

// GetOption retrieves an option value.
func (o options) get(name string) (interface{}, error) {
	if o == nil {
		return nil, mangos.ErrBadOption
	}
	v, ok := o[name]
	if !ok {
		return nil, mangos.ErrBadOption
	}
	return v, nil
}

// SetOption sets an option.  We have none, so just ErrBadOption.
func (o options) set(name string, val interface{}) error {
	switch name {
	case mangos.OptionNoDelay:
		fallthrough
	case mangos.OptionKeepAlive:
		fallthrough
	case OptionWebSocketCheckOrigin:
		if v, ok := val.(bool); ok {
			o[name] = v
			return nil
		}
	case mangos.OptionTLSConfig:
		if v, ok := val.(*tls.Config); ok {
			o[name] = v
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionMaxRecvSize:
		if v, ok := val.(int); ok {
			o[name] = v
			return nil
		}
		return mangos.ErrBadValue
	}
	return mangos.ErrBadOption
}

// wsPipe implements the Pipe interface on a websocket
type wsPipe struct {
	ws      *websocket.Conn
	proto   transport.ProtocolInfo
	addr    string
	open    bool
	wg      sync.WaitGroup
	options map[string]interface{}
	iswss   bool
	dtype   int
	sync.Mutex
}

type wsTran int

func (w *wsPipe) Recv() (*mangos.Message, error) {

	// We ignore the message type for receive.
	_, body, err := w.ws.ReadMessage()
	if err != nil {
		return nil, err
	}
	msg := mangos.NewMessage(0)
	msg.Body = body
	return msg, nil
}

func (w *wsPipe) Send(m *mangos.Message) error {

	var buf []byte

	if len(m.Header) > 0 {
		buf = make([]byte, 0, len(m.Header)+len(m.Body))
		buf = append(buf, m.Header...)
		buf = append(buf, m.Body...)
	} else {
		buf = m.Body
	}
	if err := w.ws.WriteMessage(w.dtype, buf); err != nil {
		return err
	}
	m.Free()
	return nil
}

func (w *wsPipe) LocalProtocol() uint16 {
	return w.proto.Self
}

func (w *wsPipe) RemoteProtocol() uint16 {
	return w.proto.Peer
}

func (w *wsPipe) Close() error {
	w.Lock()
	defer w.Unlock()
	if w.open {
		w.open = false
		w.ws.Close()
		w.wg.Done()
	}
	return nil
}

func (w *wsPipe) GetOption(name string) (interface{}, error) {
	if v, ok := w.options[name]; ok {
		return v, nil
	}
	return nil, mangos.ErrBadOption
}

type dialer struct {
	addr  string // url
	proto mangos.ProtocolInfo
	opts  options
	iswss bool
}

func (d *dialer) Dial() (transport.Pipe, error) {
	var w *wsPipe

	wd := &websocket.Dialer{}

	wd.Subprotocols = []string{d.proto.PeerName + ".sp.nanomsg.org"}
	if v, ok := d.opts[mangos.OptionTLSConfig]; ok {
		wd.TLSClientConfig = v.(*tls.Config)
	}

	w = &wsPipe{
		addr:    d.addr,
		proto:   d.proto,
		open:    true,
		dtype:   websocket.BinaryMessage,
		options: make(map[string]interface{}),
	}

	maxrx := 0
	v, err := d.opts.get(mangos.OptionMaxRecvSize)
	if err == nil {
		maxrx, _ = v.(int)
	}
	if w.ws, _, err = wd.Dial(d.addr, nil); err != nil {
		return nil, err
	}
	w.ws.SetReadLimit(int64(maxrx))
	w.options[mangos.OptionLocalAddr] = w.ws.LocalAddr()
	w.options[mangos.OptionRemoteAddr] = w.ws.RemoteAddr()
	if tlsConn, ok := w.ws.UnderlyingConn().(*tls.Conn); ok {
		w.options[mangos.OptionTLSConnState] = tlsConn.ConnectionState()
	}

	w.wg.Add(1)
	return w, nil
}

func (d *dialer) SetOption(n string, v interface{}) error {
	return d.opts.set(n, v)
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.opts.get(n)
}

type listener struct {
	pending  []*wsPipe
	lock     sync.Mutex
	cv       sync.Cond
	running  bool
	noserve  bool
	addr     string
	ug       websocket.Upgrader
	htsvr    *http.Server
	mux      *http.ServeMux
	url      *url.URL
	listener net.Listener
	proto    transport.ProtocolInfo
	opts     options
	iswss    bool
}

func (l *listener) SetOption(n string, v interface{}) error {
	switch n {
	case OptionWebSocketCheckOrigin:
		if v, ok := v.(bool); ok {
			if !v {
				l.ug.CheckOrigin = func(r *http.Request) bool { return true }
			}
		}
	}
	return l.opts.set(n, v)
}

func (l *listener) GetOption(n string) (interface{}, error) {
	switch n {
	case OptionWebSocketMux:
		return l.mux, nil
	case OptionWebSocketHandler:
		// Caller intends to use use in his own server, so mark
		// us running.  If he didn't mean this, the side effect is
		// that Accept() will appear to hang, even though Listen()
		// is not called yet.
		l.running = true
		l.noserve = true
		return l, nil
	case OptionWebSocketCheckOrigin:
		if v, err := l.opts.get(n); err == nil {
			if v, ok := v.(bool); ok {
				return v, nil
			}
		}
		return true, nil

	}
	return l.opts.get(n)
}

func (l *listener) Listen() error {
	var taddr *net.TCPAddr
	var err error
	var tcfg *tls.Config

	if l.noserve {
		// The HTTP framework is going to call us, so we use that rather than
		// listening on our own.  We just fake this out.
		return nil
	}
	if l.iswss {
		v, ok := l.opts[mangos.OptionTLSConfig]
		if !ok || v == nil {
			return mangos.ErrTLSNoConfig
		}
		tcfg = v.(*tls.Config)
		if tcfg.Certificates == nil || len(tcfg.Certificates) == 0 {
			return mangos.ErrTLSNoCert
		}
	}

	// We listen separately, that way we can catch and deal with the
	// case of a port already in use.  This also lets us configure
	// properties of the underlying TCP connection.

	if taddr, err = transport.ResolveTCPAddr(l.url.Host); err != nil {
		return err
	}

	if tlist, err := net.ListenTCP("tcp", taddr); err != nil {
		return err
	} else if l.iswss {
		l.listener = tls.NewListener(tlist, tcfg)
	} else {
		l.listener = tlist
	}
	l.pending = nil
	l.running = true

	l.htsvr = &http.Server{Addr: l.url.Host, Handler: l.mux}

	go l.htsvr.Serve(l.listener)

	return nil
}

func (l *listener) Accept() (transport.Pipe, error) {
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

func (l *listener) handler(ws *websocket.Conn, req *http.Request) {
	l.lock.Lock()

	if !l.running {
		ws.Close()
		l.lock.Unlock()
		return
	}

	if ws.Subprotocol() != l.proto.SelfName+".sp.nanomsg.org" {
		ws.Close()
		l.lock.Unlock()
		return
	}

	w := &wsPipe{
		ws:      ws,
		addr:    l.addr,
		proto:   l.proto,
		open:    true,
		dtype:   websocket.BinaryMessage,
		iswss:   l.iswss,
		options: make(map[string]interface{}),
	}
	maxrx := 0
	v, err := l.opts.get(mangos.OptionMaxRecvSize)
	if err == nil {
		maxrx, _ = v.(int)
	}

	w.ws.SetReadLimit(int64(maxrx))
	w.options[mangos.OptionLocalAddr] = ws.LocalAddr()
	w.options[mangos.OptionRemoteAddr] = ws.RemoteAddr()

	if req.TLS != nil {
		w.options[mangos.OptionTLSConnState] = *req.TLS
	}

	w.wg.Add(1)
	l.pending = append(l.pending, w)
	l.cv.Broadcast()
	l.lock.Unlock()

	// We must not return before the socket is closed, because
	// our caller will close the websocket on our return.
	w.wg.Wait()
}

func (l *listener) Handle(pattern string, handler http.Handler) {
	l.mux.Handle(pattern, handler)
}

func (l *listener) HandleFunc(pattern string, handler http.HandlerFunc) {
	l.mux.HandleFunc(pattern, handler)
}

func (l *listener) Close() error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.running {
		return mangos.ErrClosed
	}
	if l.listener != nil {
		l.listener.Close()
	}
	l.running = false
	l.cv.Broadcast()
	for _, ws := range l.pending {
		ws.Close()
	}
	l.pending = nil
	return nil
}

func (l *listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := l.ug.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	l.handler(ws, r)
}

func (l *listener) Address() string {
	return l.url.String()
}

func (wsTran) Scheme() string {
	return "ws"
}

func (wsTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	iswss := strings.HasPrefix(addr, "wss://")
	opts := make(map[string]interface{})

	opts[mangos.OptionNoDelay] = true
	opts[mangos.OptionKeepAlive] = true

	d := &dialer{
		addr:  addr,
		proto: sock.Info(),
		iswss: iswss,
		opts:  opts,
	}
	return d, nil
}

func (t wsTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	l, e := t.listener(addr, sock)
	if e == nil {
		l.mux.Handle(l.url.Path, l)
	}
	return l, e
}

func (wsTran) listener(addr string, sock mangos.Socket) (*listener, error) {
	var err error
	l := &listener{
		addr:  addr,
		proto: sock.Info(),
		opts:  make(map[string]interface{}),
	}
	l.cv.L = &l.lock
	l.ug.Subprotocols = []string{l.proto.SelfName + ".sp.nanomsg.org"}

	if strings.HasPrefix(addr, "wss://") {
		l.iswss = true
	}
	l.url, err = url.ParseRequestURI(addr)
	if err != nil {
		return nil, err
	}
	if len(l.url.Path) == 0 {
		l.url.Path = "/"
	}
	l.mux = http.NewServeMux()

	l.htsvr = &http.Server{Addr: l.url.Host, Handler: l.mux}

	return l, nil
}

// NewTransport allocates a new ws:// transport.
func NewTransport() mangos.Transport {
	return wsTran(0)
}
