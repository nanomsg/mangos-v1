// Copyright 2014 The Mangos Authors
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

// Package tcp implements the TCP transport for mangos.
package tcp

import (
	"net"

	"github.com/gdamore/mangos"
)

type tcpDialer struct {
	addr  *net.TCPAddr
	proto uint16
}

func (d *tcpDialer) Dial() (mangos.Pipe, error) {

	conn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	return mangos.NewConnPipe(conn, d.proto)
}

type tcpAccepter struct {
	addr     *net.TCPAddr
	proto    uint16
	listener *net.TCPListener
}

func (a *tcpAccepter) Accept() (mangos.Pipe, error) {

	conn, err := a.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return mangos.NewConnPipe(conn, a.proto)
}

func (a *tcpAccepter) Close() error {
	a.listener.Close()
	return nil
}

type tcpTran struct {
	localAddr net.Addr
}

func (t *tcpTran) Scheme() string {
	return "tcp"
}

func (t *tcpTran) NewDialer(addr string, proto uint16) (mangos.PipeDialer, error) {
	var err error
	d := &tcpDialer{}
	d.proto = proto
	if d.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}
	return d, nil
}

func (t *tcpTran) NewAccepter(addr string, proto uint16) (mangos.PipeAccepter, error) {
	var err error
	a := &tcpAccepter{}
	a.proto = proto

	if a.addr, err = net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	}

	if a.listener, err = net.ListenTCP("tcp", a.addr); err != nil {
		return nil, err
	}

	t.localAddr = a.listener.Addr()
	return a, nil
}

func (*tcpTran) SetOption(string, interface{}) error {
	// Likely we should support some options here...
	return mangos.ErrBadOption
}

func (t *tcpTran) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionLocalAddress:
		if t.localAddr == nil {
			return nil, mangos.ErrBadOption
		}
		return t.localAddr.String(), nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewTransport allocates a new TCP transport.
func NewTransport() mangos.Transport {
	return &tcpTran{}
}
