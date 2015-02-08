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

// Package push implements the PUSH protocol, which is the write side of
// the pipeline pattern.  (PULL is the reader.)
package push

import (
	"time"

	"github.com/gdamore/mangos"
)

type push struct {
	sock    mangos.ProtocolSocket
	raw     bool
	senders mangos.Waiter
}

func (x *push) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.senders.Init()
}

func (x *push) Shutdown(linger time.Duration) {
	x.senders.WaitRelTimeout(linger)
}

func (x *push) sender(ep mangos.Endpoint) {
	sq := x.sock.SendChannel()

	for {
		m := <-sq
		if m == nil {
			break
		}

		if ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}

	x.senders.Done()
}

func (x *push) receiver(ep mangos.Endpoint) {
	// In order for us to detect a dropped connection, we need to poll
	// on the socket.  We don't care about the results and discard them,
	// but this allows the disconnect to be noticed.  Note that we will
	// be blocked in this call forever, until the connection is dropped.
	for {
		if m := ep.RecvMsg(); m == nil {
			break
		}
	}
}

func (*push) Number() uint16 {
	return mangos.ProtoPush
}

func (*push) PeerNumber() uint16 {
	return mangos.ProtoPull
}

func (*push) Name() string {
	return "push"
}

func (*push) PeerName() string {
	return "pull"
}

func (x *push) AddEndpoint(ep mangos.Endpoint) {
	x.senders.Add()
	go x.sender(ep)
	go x.receiver(ep)
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

// NewProtocol returns a new PUSH protocol object.
func NewProtocol() mangos.Protocol {
	return &push{}
}

// NewSocket allocates a new Socket using the PUSH protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&push{}), nil
}
