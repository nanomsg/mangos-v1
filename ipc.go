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
)

type ipcDialer struct {
	addr  *net.UnixAddr
	proto uint16
}

// Dial implements the PipeDialer Dial method
func (d *ipcDialer) Dial() (Pipe, error) {

	conn, err := net.DialUnix("unix", nil, d.addr)
	if err != nil {
		return nil, err
	}
	return NewConnPipeIPC(conn, d.proto)
}

type ipcAccepter struct {
	addr     *net.UnixAddr
	proto    uint16
	listener *net.UnixListener
}

// Accept implements the the PipeAccepter Accept method.
func (a *ipcAccepter) Accept() (Pipe, error) {

	conn, err := a.listener.AcceptUnix()
	if err != nil {
		return nil, err
	}
	return NewConnPipeIPC(conn, a.proto)
}

// Close implements the PipeAccepter Close method.
func (a *ipcAccepter) Close() error {
	a.listener.Close()
	return nil
}

type ipcTran struct{}

// Scheme implements the Transport Scheme method.
func (t *ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t *ipcTran) NewDialer(addr string, proto uint16) (PipeDialer, error) {
	var err error
	d := new(ipcDialer)
	d.proto = proto
	if d.addr, err = net.ResolveUnixAddr("unix", addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewAccepter implements the Transport NewAccepter method.
func (t *ipcTran) NewAccepter(addr string, proto uint16) (PipeAccepter, error) {
	var err error
	a := new(ipcAccepter)
	a.proto = proto

	if a.addr, err = net.ResolveUnixAddr("unix", addr); err != nil {
		return nil, err
	}

	if a.listener, err = net.ListenUnix("unix", a.addr); err != nil {
		return nil, err
	}

	return a, nil
}

// SetOption implements a stub Transport SetOption method.
func (t *ipcTran) SetOption(string, interface{}) error {
	return ErrBadOption
}

// GetOption implements a stub Transport GetOption method.
func (t *ipcTran) GetOption(string) (interface{}, error) {
	return nil, ErrBadOption
}

type ipcFactory int

func (ipcFactory) NewTransport() Transport {
	return new(ipcTran)
}

// IPCFactory is used by the core to create new IPC Transport instances.
var IPCFactory ipcFactory
