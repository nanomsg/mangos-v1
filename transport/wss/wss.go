// Copyright 2015 The Mangos Authors
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
// This transport is considered EXPERIMENTAL.
package wss

import (
	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/transport/ws"
)

type wssTran struct {
	w mangos.Transport
}

func (wssTran) Scheme() string {
	return "wss"
}

func (w *wssTran) NewDialer(addr string, sock mangos.Socket) (mangos.PipeDialer, error) {
	return w.w.NewDialer(addr, sock)
}

func (w *wssTran) NewListener(addr string, sock mangos.Socket) (mangos.PipeListener, error) {
	return w.w.NewListener(addr, sock)
}

// NewTransport allocates a new wss:// transport.
func NewTransport() mangos.Transport {
	w := &wssTran{w: ws.NewTransport()}
	return w
}
