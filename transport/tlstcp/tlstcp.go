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

// Package tlstcp implements the TLS over TCP transport for mangos.
package tlstcp

import (
	"crypto/tls"
	"net"

	"github.com/go-mangos/mangos"
)

type options map[string]interface{}

func (o options) get(name string) (interface{}, error) {
	if v, ok := o[name]; ok {
		return v, nil
	}
	return nil, mangos.ErrBadOption
}

func (o options) set(name string, val interface{}) error {
	switch name {
	case mangos.OptionTLSConfig:
		switch v := val.(type) {
		case *tls.Config:
			o[name] = v
		default:
			return mangos.ErrBadValue
		}
	default:
		return mangos.ErrBadOption
	}
	return nil
}

func (o options) configTCP(conn *net.TCPConn) error {
	if v, ok := o[mangos.OptionNoDelay]; ok {
		if err := conn.SetNoDelay(v.(bool)); err != nil {
			return err
		}
	}
	if v, ok := o[mangos.OptionKeepAlive]; ok {
		if err := conn.SetKeepAlive(v.(bool)); err != nil {
			return err
		}
	}

	return nil
}

func newOptions(t *tlsTran) options {
	o := make(map[string]interface{})
	o[mangos.OptionTLSConfig] = t.config
	return options(o)
}

type dialer struct {
	addr *net.TCPAddr
	sock mangos.Socket
	opts options
}

func (d *dialer) Dial() (mangos.Pipe, error) {

	var config *tls.Config
	tconn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	if err = d.opts.configTCP(tconn); err != nil {
		tconn.Close()
		return nil, err
	}
	if v, ok := d.opts[mangos.OptionTLSConfig]; ok {
		config = v.(*tls.Config)
	}
	conn := tls.Client(tconn, config)
	if err = conn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	return mangos.NewConnPipe(conn, d.sock,
		mangos.PropTLSConnState, conn.ConnectionState())
}

func (d *dialer) SetOption(n string, v interface{}) error {
	return d.opts.set(n, v)
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.opts.get(n)
}

type listener struct {
	sock     mangos.Socket
	addr     *net.TCPAddr
	bound    net.Addr
	listener *net.TCPListener
	opts     options
	config   *tls.Config
}

func (l *listener) Listen() error {
	var err error
	v, ok := l.opts[mangos.OptionTLSConfig]
	if !ok {
		return mangos.ErrTLSNoConfig
	}
	l.config = v.(*tls.Config)
	if l.config == nil {
		return mangos.ErrTLSNoConfig
	}
	if l.config.Certificates == nil || len(l.config.Certificates) == 0 {
		return mangos.ErrTLSNoCert
	}

	if l.listener, err = net.ListenTCP("tcp", l.addr); err != nil {
		return err
	}

	l.bound = l.listener.Addr()

	return nil
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return "tls+tcp://" + b.String()
	}
	return "tls+tcp://" + l.addr.String()
}

func (l *listener) Accept() (mangos.Pipe, error) {

	tconn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}

	if err = l.opts.configTCP(tconn); err != nil {
		tconn.Close()
		return nil, err
	}

	conn := tls.Server(tconn, l.config)
	if err = conn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	return mangos.NewConnPipe(conn, l.sock,
		mangos.PropTLSConnState, conn.ConnectionState())
}

func (l *listener) Close() error {
	l.listener.Close()
	return nil
}

func (l *listener) SetOption(n string, v interface{}) error {
	return l.opts.set(n, v)
}

func (l *listener) GetOption(n string) (interface{}, error) {
	return l.opts.get(n)
}

type tlsTran struct {
	config    *tls.Config
	localAddr net.Addr
}

func (t *tlsTran) Scheme() string {
	return "tls+tcp"
}

func (t *tlsTran) NewDialer(addr string, sock mangos.Socket) (mangos.PipeDialer, error) {
	var err error

	if addr, err = mangos.StripScheme(t, addr); err != nil {
		return nil, err
	}

	d := &dialer{sock: sock, opts: newOptions(t)}
	if d.addr, err = mangos.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t *tlsTran) NewListener(addr string, sock mangos.Socket) (mangos.PipeListener, error) {
	var err error
	l := &listener{sock: sock, opts: newOptions(t)}

	if addr, err = mangos.StripScheme(t, addr); err != nil {
		return nil, err
	}
	if l.addr, err = mangos.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	return l, nil
}

// NewTransport allocates a new inproc transport.
func NewTransport() mangos.Transport {
	return &tlsTran{}
}
