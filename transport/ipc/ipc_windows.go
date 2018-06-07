// +build windows

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

// Package ipc implements the IPC transport on top of Windows Named Pipes.
package ipc

import (
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/go-mangos/mangos"
)

// The options here are pretty specific to Windows Named Pipes.

const (
	// OptionSecurityDescriptor represents a Windows security
	// descriptor in SDDL format (string).  This can only be set on
	// a Listener, and must be set before the Listen routine
	// is called.
	OptionSecurityDescriptor = "WIN-IPC-SECURITY-DESCRIPTOR"

	// OptionInputBufferSize represents the Windows Named Pipe
	// input buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionInputBufferSize = "WIN-IPC-INPUT-BUFFER-SIZE"

	// OptionOutputBufferSize represents the Windows Named Pipe
	// output buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionOutputBufferSize = "WIN-IPC-OUTPUT-BUFFER-SIZE"
)

type pipeAddr string

func (p pipeAddr) Network() string {
	return "ipc"
}

func (p pipeAddr) String() string {
	return string(p)
}

type dialer struct {
	path string
	sock mangos.Socket
}

// Dial implements the PipeDialer Dial method.
func (d *dialer) Dial() (mangos.Pipe, error) {

	conn, err := winio.DialPipe("\\\\.\\pipe\\"+d.path, nil)
	if err != nil {
		return nil, err
	}
	addr := pipeAddr(d.path)
	return mangos.NewConnPipeIPC(conn, d.sock,
		mangos.PropLocalAddr, addr, mangos.PropRemoteAddr, addr)
}

// SetOption implements a stub PipeDialer SetOption method.
func (d *dialer) SetOption(n string, v interface{}) error {
	return mangos.ErrBadOption
}

// GetOption implements a stub PipeDialer GetOption method.
func (d *dialer) GetOption(n string) (interface{}, error) {
	return nil, mangos.ErrBadOption
}

// listenerOptions is used for shared GetOption/SetOption logic for listeners.
// We don't have dialer specific options at this point.
type listenerOptions map[string]interface{}

// GetOption retrieves an option value.
func (o listenerOptions) get(name string) (interface{}, error) {
	if o == nil {
		return nil, mangos.ErrBadOption
	}
	v, ok := o[name]
	if !ok {
		return nil, mangos.ErrBadOption
	}
	return v, nil
}

// SetOption sets an option.  We have none, so just ErrBadOption.
func (o listenerOptions) set(string, interface{}) error {
	return mangos.ErrBadOption
}

type listener struct {
	path     string
	sock     mangos.Socket
	listener net.Listener
	config   winio.PipeConfig
}

// Listen implements the PipeListener Listen method.
func (l *listener) Listen() error {
	listener, err := winio.ListenPipe("\\\\.\\pipe\\"+l.path, &l.config)
	if err != nil {
		return err
	}
	l.listener = listener
	return nil
}

func (l *listener) Address() string {
	return "ipc://" + l.path
}

// Accept implements the the PipeListener Accept method.
func (l *listener) Accept() (mangos.Pipe, error) {

	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	addr := pipeAddr(l.path)
	return mangos.NewConnPipeIPC(conn, l.sock,
		mangos.PropLocalAddr, addr, mangos.PropRemoteAddr, addr)
}

// Close implements the PipeListener Close method.
func (l *listener) Close() error {
	if l.listener != nil {
		l.listener.Close()
	}
	return nil
}

// SetOption implements a stub PipeListener SetOption method.
func (l *listener) SetOption(name string, val interface{}) error {
	switch name {
	case OptionInputBufferSize:
		switch v := val.(type) {
		case int32:
			l.config.InputBufferSize = v
			return nil
		default:
			return mangos.ErrBadValue
		}
	case OptionOutputBufferSize:
		switch v := val.(type) {
		case int32:
			l.config.OutputBufferSize = v
			return nil
		default:
			return mangos.ErrBadValue
		}
	case OptionSecurityDescriptor:
		switch v := val.(type) {
		case string:
			l.config.SecurityDescriptor = v
			return nil
		default:
			return mangos.ErrBadValue
		}
	default:
		return mangos.ErrBadOption
	}
}

// GetOption implements a stub PipeListener GetOption method.
func (l *listener) GetOption(name string) (interface{}, error) {
	switch name {
	case OptionInputBufferSize:
		return l.config.InputBufferSize, nil
	case OptionOutputBufferSize:
		return l.config.OutputBufferSize, nil
	case OptionSecurityDescriptor:
		return l.config.SecurityDescriptor, nil
	}
	return nil, mangos.ErrBadOption
}

type ipcTran struct{}

// Scheme implements the Transport Scheme method.
func (t *ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t *ipcTran) NewDialer(addr string, sock mangos.Socket) (mangos.PipeDialer, error) {
	var err error

	if addr, err = mangos.StripScheme(t, addr); err != nil {
		return nil, err
	}

	d := &dialer{sock: sock}
	d.path = addr
	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t *ipcTran) NewListener(addr string, sock mangos.Socket) (mangos.PipeListener, error) {
	var err error
	l := &listener{sock: sock}
	l.config.OutputBufferSize = 4096
	l.config.InputBufferSize = 4096
	l.config.SecurityDescriptor = ""
	l.config.MessageMode = false

	if addr, err = mangos.StripScheme(t, addr); err != nil {
		return nil, err
	}

	l.path = addr

	return l, nil
}

// NewTransport allocates a new IPC transport.
func NewTransport() mangos.Transport {
	return &ipcTran{}
}
