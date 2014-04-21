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

package mangos

import (
	"crypto/tls"
	"net"
)

// TLSOptionConfig is the name of the configuration option.  The value
// associated with it is a *tls.Config which provides all the information
// to configure the TLS security parameters.
const TLSOptionConfig = "TLS.CONFIG"

type tlsDialer struct {
	addr   *net.TCPAddr
	config *tls.Config
	proto  uint16
}

// Dial implements the the Dialer Dial method.
func (d *tlsDialer) Dial() (Pipe, error) {

	tconn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	conn := tls.Client(tconn, d.config)
	return NewConnPipe(conn, d.proto)
}

type tlsAccepter struct {
	addr     *net.TCPAddr
	proto    uint16
	listener net.Listener
}

// Accept implements the the PipeAccepter Accept method.
func (a *tlsAccepter) Accept() (Pipe, error) {

	conn, err := a.listener.Accept()
	if err != nil {
		return nil, err
	}

	return NewConnPipe(conn, a.proto)
}

// Close implements the PipeAccepter Close method.
func (a *tlsAccepter) Close() error {
	a.listener.Close()
	return nil
}

// tlsTran implements the Transport interface.
type tlsTran struct {
	config *tls.Config
}

// Scheme implements the Transport Scheme method.
func (t *tlsTran) Scheme() string {
	return "tls+tcp"
}

// NewDialer implements the Transport NewDialer method.
func (t *tlsTran) NewDialer(addr string, proto uint16) (PipeDialer, error) {
	var err error
	d := new(tlsDialer)
	d.proto = proto
	d.config = t.config
	if d.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t *tlsTran) NewAccepter(addr string, proto uint16) (PipeAccepter, error) {
	var err error
	a := new(tlsAccepter)
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
	switch {
	case name == TLSOptionConfig:
		switch v := val.(type) {
		case *tls.Config:
			t.config = v
			return nil
		default:
			return ErrBadValue
		}
	default:
		return ErrBadOption
	}
}

// SetOption implements the Transport SetOption method. We support a single
// option, TLSOptionConfig, which takes a single value, a *tls.Config.
func (t *tlsTran) GetOption(name string) (interface{}, error) {
	switch {
	case name == TLSOptionConfig:
		return t.config, nil
	default:
		return nil, ErrBadOption
	}
}

type tlsFactory int

func (tlsFactory) NewTransport() Transport {
	t := new(tlsTran)
	return t
}

// TLSFactory is used by the core to create new TLS Transport instances.
// These instances are used with the "tls+tcp://" scheme.
var TLSFactory tlsFactory
