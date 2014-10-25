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

// Package push implements the PUSH protocol, which is the write side of
// the pipeline pattern.  (PULL is the reader.)
package push

import (
	"github.com/gdamore/mangos"
)

type push struct {
	sock mangos.ProtocolSocket
	raw  bool
}

func (x *push) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
}

func (x *push) sender(ep mangos.Endpoint) {
	var m *mangos.Message

	for {
		select {
		case m = <-x.sock.SendChannel():
		case <-x.sock.DrainChannel():
			return
		}

		err := ep.SendMsg(m)
		if err != nil {
			select {
			case <-x.sock.DrainChannel():
				m.Free()
			}
			return
		}
	}
}

func (*push) Number() uint16 {
	return mangos.ProtoPush
}

func (*push) ValidPeer(peer uint16) bool {
	if peer == mangos.ProtoPull {
		return true
	}
	return false
}

func (x *push) AddEndpoint(ep mangos.Endpoint) {
	go x.sender(ep)
}

func (x *push) RemoveEndpoint(ep mangos.Endpoint) {}

func (x *push) SetOption(name string, v interface{}) error {
	switch name {
	case mangos.OptionRaw:
		x.raw = v.(bool)
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *push) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PUSH protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&push{}), nil
}
