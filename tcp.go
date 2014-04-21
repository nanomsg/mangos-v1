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
	"net"
)

// TCPDialer implements the PipeDialer interface.
type tcpDialer struct {
	addr  *net.TCPAddr
	proto uint16
}

// Dial implements the the Dialer Dial method.
func (d *tcpDialer) Dial() (Pipe, error) {

	conn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	conn.SetLinger(-1) // Close returns immediately, but OS send data
	return NewConnPipe(conn, d.proto)
}

// tcpAccepter implements the PipeAccepter interface.
// In addition to handling Accept(), it also arranges to set up a TCP
// listener.
type tcpAccepter struct {
	addr     *net.TCPAddr
	proto    uint16
	listener *net.TCPListener
}

// Accept implements the the PipeAccepter Accept method.
func (a *tcpAccepter) Accept() (Pipe, error) {

	conn, err := a.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	conn.SetLinger(-1) // Close returns immediately, but OS sends data
	return NewConnPipe(conn, a.proto)
}

// Close implements the PipeAccepter Close method.
func (a *tcpAccepter) Close() error {
	a.listener.Close()
	return nil
}

// tcpTran implements the Transport interface.
type tcpTran struct {
}

// Scheme implements the Transport Scheme method.
func (t *tcpTran) Scheme() string {
	return "tcp"
}

// NewDialer implements the Transport NewDialer method.
func (t *tcpTran) NewDialer(addr string, proto uint16) (PipeDialer, error) {
	var err error
	d := new(tcpDialer)
	d.proto = proto
	if d.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t *tcpTran) NewAccepter(addr string, proto uint16) (PipeAccepter, error) {
	var err error
	a := new(tcpAccepter)
	a.proto = proto

	if a.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}

	if a.listener, err = net.ListenTCP("tcp", a.addr); err != nil {
		return nil, err
	}

	return a, nil
}

// SetOption implements the Transport SetOption method.  No options are
// supported at this time.
func (*tcpTran) SetOption(string, interface{}) error {
	// Likely we should support some options here...
	return ErrBadOption
}

// GetOption implements the Transport GetOption method.  No options are
// supported at this time.
func (*tcpTran) GetOption(string) (interface{}, error) {
	return nil, ErrBadOption
}

type tcpFactory int

func (tcpFactory) NewTransport() Transport {
	return new(tcpTran)
}

// TCPFactory is used by the core to create TCP Transport instances.
var TCPFactory tcpFactory
