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

// Package wss implements a secure WebSocket transport for mangos.
// To enable it simply import it.
package wss

import (
	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
	"nanomsg.org/go/mangos/v2/transport/ws"
)

type wssTran int

// Transport is a transport.Transport for WebSocket over TLS.
const Transport = wssTran(0)

func init() {
	transport.RegisterTransport(Transport)
}

func (wssTran) Scheme() string {
	return "wss"
}

func (w wssTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	return ws.Transport.NewDialer(addr, sock)
}

func (w wssTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	return ws.Transport.NewListener(addr, sock)
}
