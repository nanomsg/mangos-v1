// +build !windows,!nacl,!plan9

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

// Package ipc implements the IPC transport on top of UNIX domain sockets.
// To enable it simply import it.
package ipc

import (
	"net"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

const (
	// Transport is a transport.Transport for IPC.
	Transport = ipcTran(0)
)

func init() {
	transport.RegisterTransport(Transport)
}

// options is used for shared GetOption/SetOption logic.
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

// SetOption sets an option.
func (o options) set(name string, val interface{}) error {
	switch name {
	case mangos.OptionMaxRecvSize:
		if v, ok := val.(int); ok {
			o[name] = v
			return nil
		}
		return mangos.ErrBadValue
	}

	return mangos.ErrBadOption
}

type dialer struct {
	addr  *net.UnixAddr
	proto transport.ProtocolInfo
	opts  options
}

// Dial implements the Dialer Dial method
func (d *dialer) Dial() (transport.Pipe, error) {

	conn, err := net.DialUnix("unix", nil, d.addr)
	if err != nil {
		return nil, err
	}
	return transport.NewConnPipeIPC(conn, d.proto, d.opts)
}

// SetOption implements Dialer SetOption method.
func (d *dialer) SetOption(n string, v interface{}) error {
	return d.opts.set(n, v)
}

// GetOption implements Dialer GetOption method.
func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.opts.get(n)
}

type listener struct {
	addr     *net.UnixAddr
	proto    transport.ProtocolInfo
	listener *net.UnixListener
	opts     options
}

// Listen implements the PipeListener Listen method.
func (l *listener) Listen() error {
	listener, err := net.ListenUnix("unix", l.addr)
	if err != nil {
		return err
	}
	l.listener = listener
	return nil
}

func (l *listener) Address() string {
	return "ipc://" + l.addr.String()
}

// Accept implements the the PipeListener Accept method.
func (l *listener) Accept() (transport.Pipe, error) {

	conn, err := l.listener.AcceptUnix()
	if err != nil {
		return nil, err
	}
	return transport.NewConnPipeIPC(conn, l.proto, l.opts)
}

// Close implements the PipeListener Close method.
func (l *listener) Close() error {
	l.listener.Close()
	return nil
}

// SetOption implements a stub PipeListener SetOption method.
func (l *listener) SetOption(n string, v interface{}) error {
	return l.opts.set(n, v)
}

// GetOption implements a stub PipeListener GetOption method.
func (l *listener) GetOption(n string) (interface{}, error) {
	return l.opts.get(n)
}

type ipcTran int

// Scheme implements the Transport Scheme method.
func (ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t ipcTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	var err error

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	d := &dialer{
		proto: sock.Info(),
		opts:  make(map[string]interface{}),
	}
	d.opts[mangos.OptionMaxRecvSize] = 0
	if d.addr, err = net.ResolveUnixAddr("unix", addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t ipcTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	var err error
	l := &listener{
		proto: sock.Info(),
		opts:  make(map[string]interface{}),
	}
	l.opts[mangos.OptionMaxRecvSize] = 0

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	if l.addr, err = net.ResolveUnixAddr("unix", addr); err != nil {
		return nil, err
	}

	return l, nil
}
