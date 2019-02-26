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

// Package tcp implements the TCP transport for mangos. To enable it simply
// import it.
package tcp

import (
	"net"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

const (
	// Transport is a transport.Transport for TCP.
	Transport = tcpTran(0)
)

func init() {
	transport.RegisterTransport(Transport)
}

// options is used for shared GetOption/SetOption logic.
type options map[string]interface{}

// GetOption retrieves an option value.
func (o options) get(name string) (interface{}, error) {
	v, ok := o[name]
	if !ok {
		return nil, mangos.ErrBadOption
	}
	return v, nil
}

// SetOption sets an option.
func (o options) set(name string, val interface{}) error {
	switch name {
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

	case mangos.OptionMaxRecvSize:
		if v, ok := val.(int); ok {
			o[name] = v
			return nil
		}
		return mangos.ErrBadValue
	}
	return mangos.ErrBadOption
}

func newOptions() options {
	o := make(map[string]interface{})
	o[mangos.OptionNoDelay] = true
	o[mangos.OptionKeepAlive] = true
	o[mangos.OptionMaxRecvSize] = 0
	return options(o)
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

type dialer struct {
	addr  string
	proto transport.ProtocolInfo
	opts  options
}

func (d *dialer) Dial() (_ transport.Pipe, err error) {
	var (
		addr *net.TCPAddr
	)

	if addr, err = transport.ResolveTCPAddr(d.addr); err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	if err = d.opts.configTCP(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return transport.NewConnPipe(conn, d.proto, d.opts)
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
	proto    transport.ProtocolInfo
	listener *net.TCPListener
	opts     options
}

func (l *listener) Accept() (transport.Pipe, error) {

	if l.listener == nil {
		return nil, mangos.ErrClosed
	}
	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if err = l.opts.configTCP(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return transport.NewConnPipe(conn, l.proto, l.opts)
}

func (l *listener) Listen() (err error) {
	l.listener, err = net.ListenTCP("tcp", l.addr)
	if err == nil {
		l.bound = l.listener.Addr()
	}
	return
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return "tcp://" + b.String()
	}
	return "tcp://" + l.addr.String()
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

type tcpTran int

func (t tcpTran) Scheme() string {
	return "tcp"
}

func (t tcpTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// check to ensure the provided addr resolves correctly.
	if _, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	d := &dialer{addr: addr, proto: sock.Info(), opts: newOptions()}

	return d, nil
}

func (t tcpTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	var err error
	l := &listener{proto: sock.Info(), opts: newOptions()}

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	if l.addr, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	return l, nil
}
