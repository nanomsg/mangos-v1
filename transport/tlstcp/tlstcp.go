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

// Package tlstcp implements the TLS / SSL over TCP transport for mangos.
package tlstcp

import (
	"bitbucket.org/gdamore/mangos"
	"crypto/tls"
	"net"
)

type tlsDialer struct {
	addr   *net.TCPAddr
	config *tls.Config
	proto  uint16
}

func (d *tlsDialer) Dial() (mangos.Pipe, error) {

	tconn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	conn := tls.Client(tconn, d.config)
	return mangos.NewConnPipe(conn, d.proto)
}

type tlsAccepter struct {
	addr     *net.TCPAddr
	proto    uint16
	listener net.Listener
}

func (a *tlsAccepter) Accept() (mangos.Pipe, error) {

	conn, err := a.listener.Accept()
	if err != nil {
		return nil, err
	}

	return mangos.NewConnPipe(conn, a.proto)
}

func (a *tlsAccepter) Close() error {
	a.listener.Close()
	return nil
}

type tlsTran struct {
	config *tls.Config
}

func (t *tlsTran) Scheme() string {
	return "tls+tcp"
}

func (t *tlsTran) NewDialer(addr string, proto uint16) (mangos.PipeDialer, error) {
	var err error
	d := &tlsDialer{}
	d.proto = proto
	d.config = t.config
	if d.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t *tlsTran) NewAccepter(addr string, proto uint16) (mangos.PipeAccepter, error) {
	var err error
	a := &tlsAccepter{}
	a.proto = proto

	if a.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}

	var tlistener net.Listener
	if tlistener, err = net.ListenTCP("tcp", a.addr); err != nil {
		return nil, err
	}
	a.listener = tls.NewListener(tlistener, t.config)

	return a, nil
}

// SetOption implements the Transport SetOption method. We support a single
// option, TLSOptionConfig, which takes a single value, a *tls.Config.
func (t *tlsTran) SetOption(name string, val interface{}) error {
	switch name {
	case mangos.OptionTLSConfig:
		switch v := val.(type) {
		case *tls.Config:
			t.config = v
			return nil
		default:
			return mangos.ErrBadValue
		}
	default:
		return mangos.ErrBadOption
	}
}

// SetOption implements the Transport SetOption method. We support a single
// option, TLSOptionConfig, which takes a single value, a *tls.Config.
func (t *tlsTran) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionTLSConfig:
		return t.config, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewTransport allocates a new inproc transport.
func NewTransport() mangos.Transport {
	return &tlsTran{}
}
