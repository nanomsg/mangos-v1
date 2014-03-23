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

package sp

import (
	"net"
	"strings"
)

// TCPDialer implements the PipeDialer interface.
type TCPDialer struct {
	addr  *net.TCPAddr
	proto uint16
}

// Dial implements the the Dialer Dial method.
func (d *TCPDialer) Dial() (Pipe, error) {

	conn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	conn.SetLinger(-1) // Close returns immediately, but OS send data
	return NewConnPipe(conn, d.proto)
}

// TCPAccepter implements the PipeAccepter interface.
// In addition to handling Accept(), it also arranges to set up a TCP
// listener.
type TCPAccepter struct {
	addr     *net.TCPAddr
	proto    uint16
	listener *net.TCPListener
}

// Accept implements the the PipeAccepter Accept method.
func (a *TCPAccepter) Accept() (Pipe, error) {

	conn, err := a.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	conn.SetLinger(-1) // Close returns immediately, but OS sends data
	return NewConnPipe(conn, a.proto)
}

// Close implements the PipeAccepter Close method.
func (a *TCPAccepter) Close() error {
	a.listener.Close()
	return nil
}

// TCPTransport implements the Transport interface.
type TCPTransport struct {
}

// Scheme implements the Transport Scheme method.
func (t *TCPTransport) Scheme() string {
	return "tcp://"
}

// NewDialer implements the Transport NewDialer method.
func (t *TCPTransport) NewDialer(url string, proto uint16) (PipeDialer, error) {
	var err error
	if !strings.HasPrefix(url, t.Scheme()) {
		return nil, EBadAddr
	}
	url = url[len(t.Scheme()):]
	d := new(TCPDialer)
	d.proto = proto
	if d.addr, err = net.ResolveTCPAddr("tcp", url); err != nil {
		return nil, err
	}
	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t *TCPTransport) NewAccepter(url string, proto uint16) (PipeAccepter, error) {
	var err error
	if !strings.HasPrefix(url, t.Scheme()) {
		return nil, EBadAddr
	}
	a := new(TCPAccepter)
	a.proto = proto

	// skip over the tcp:// scheme prefix
	url = url[len(t.Scheme()):]
	if a.addr, err = net.ResolveTCPAddr("tcp", url); err != nil {
		return nil, err
	}

	if a.listener, err = net.ListenTCP("tcp", a.addr); err != nil {
		return nil, err
	}

	return a, nil
}
