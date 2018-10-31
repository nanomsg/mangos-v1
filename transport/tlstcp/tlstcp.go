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

// Package tlstcp implements the TLS over TCP transport for mangos.
// To enable it simply import it.
package tlstcp

import (
	"crypto/tls"
	"net"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

type options map[string]interface{}

// Transport is a transport.Transport for TLS over TCP.
const Transport = tlsTran(0)

func init() {
	transport.RegisterTransport(Transport)
}

func (o options) get(name string) (interface{}, error) {
	if v, ok := o[name]; ok {
		return v, nil
	}
	return nil, mangos.ErrBadOption
}

func (o options) set(name string, val interface{}) error {
	switch name {
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
	case mangos.OptionNoDelay:
		fallthrough
	case mangos.OptionKeepAlive:
		if v, ok := val.(bool); ok {
			o[name] = v
			return nil
		}
		return mangos.ErrBadValue

	case mangos.OptionKeepAliveTime:
		if v, ok := val.(time.Duration); ok && v.Nanoseconds() > 0 {
			o[name] = v
			return nil
		}
		return mangos.ErrBadValue
	}

	return mangos.ErrBadOption
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
	if v, ok := o[mangos.OptionKeepAliveTime]; ok {
		if err := conn.SetKeepAlivePeriod(v.(time.Duration)); err != nil {
			return err
		}
	}

	return nil
}

func newOptions(t tlsTran) options {
	o := make(map[string]interface{})
	o[mangos.OptionTLSConfig] = nil
	o[mangos.OptionKeepAlive] = true
	o[mangos.OptionNoDelay] = true
	o[mangos.OptionMaxRecvSize] = 0
	return options(o)
}

type dialer struct {
	addr  string
	proto transport.ProtocolInfo
	opts  options
}

func (d *dialer) Dial() (transport.Pipe, error) {
	var (
		addr   *net.TCPAddr
		config *tls.Config
		err    error
	)

	if addr, err = transport.ResolveTCPAddr(d.addr); err != nil {
		return nil, err
	}

	tconn, err := net.DialTCP("tcp", nil, addr)
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
	opts := make(map[string]interface{})
	for n, v := range d.opts {
		opts[n] = v
	}
	opts[mangos.OptionTLSConnState] = conn.ConnectionState()
	return transport.NewConnPipe(conn, d.proto, opts)
}

func (d *dialer) SetOption(n string, v interface{}) error {
	return d.opts.set(n, v)
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.opts.get(n)
}

type listener struct {
	addr     *net.TCPAddr
	bound    net.Addr
	listener *net.TCPListener
	proto    transport.ProtocolInfo
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

func (l *listener) Accept() (transport.Pipe, error) {

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
	opts := make(map[string]interface{})
	for n, v := range l.opts {
		opts[n] = v
	}
	opts[mangos.OptionTLSConnState] = conn.ConnectionState()
	return transport.NewConnPipe(conn, l.proto, opts)
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

type tlsTran int

func (t tlsTran) Scheme() string {
	return "tls+tcp"
}

func (t tlsTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	var err error

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// check to ensure the provided addr resolves correctly.
	if _, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	d := &dialer{
		proto: sock.Info(),
		opts:  newOptions(t),
		addr:  addr,
	}

	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t tlsTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	l := &listener{
		proto: sock.Info(),
		opts:  newOptions(t),
	}

	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}
	if l.addr, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	return l, nil
}
