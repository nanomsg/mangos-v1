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

// Package pull implements the PULL protocol, which is the read side of
// the pipeline pattern.  (PUSH is the reader.)
package pull

import (
	"time"

	"github.com/gdamore/mangos"
)

type pull struct {
	sock mangos.ProtocolSocket
	raw  bool
}

func (x *pull) Init(sock mangos.ProtocolSocket) {
	x.sock = sock

	go x.sender()
}

func (x *pull) Shutdown(time.Duration) {} // No sender to drain

func (x *pull) receiver(ep mangos.Endpoint) {
	rq := x.sock.RecvChannel()
	cq := x.sock.CloseChannel()
	for {

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		select {
		case rq <- m:
		case <-cq:
			return
		}
	}
}

func (x *pull) sender() {
	sq := x.sock.SendChannel()
	for {
		if m := <-sq; m == nil {
			break
		} else {
			m.Free()
		}
	}
}

func (*pull) Number() uint16 {
	return mangos.ProtoPull
}

func (*pull) PeerNumber() uint16 {
	return mangos.ProtoPush
}

func (*pull) Name() string {
	return "pull"
}

func (*pull) PeerName() string {
	return "push"
}

func (x *pull) AddEndpoint(ep mangos.Endpoint) {
	go x.receiver(ep)
}

func (x *pull) RemoveEndpoint(ep mangos.Endpoint) {}

func (*pull) SendHook(msg *mangos.Message) bool {
	return false
}

func (x *pull) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *pull) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewProtocol() allocates a new PULL protocol object.
func NewProtocol() mangos.Protocol {
	return &pull{}
}

// NewSocket allocates a new Socket using the PULL protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&pull{}), nil
}
