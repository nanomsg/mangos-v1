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

// Package bus implements the BUS protocol.  In this protocol, participants
// send a message to each of their peers.
package bus

import (
	"nanomsg.org/go/mangos/v2/protocol"
	"nanomsg.org/go/mangos/v2/protocol/xbus"
)

type socket struct {
	protocol.Protocol
}

// Info returns protocol information.
func Info() protocol.Info {
	return xbus.Info()
}

func (s *socket) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionRaw:
		return false, nil
	}
	return s.Protocol.GetOption(name)
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	m, e := s.Protocol.RecvMsg()
	if m != nil {
		// Strip the raw mode header, as we don't use it in cooked mode
		m.Header = m.Header[:0]
	}
	return m, e
}

func (s *socket) SendMsg(m *protocol.Message) error {
	if len(m.Header) > 0 {
		m.Header = m.Header[:0]
	}
	return s.Protocol.SendMsg(m)
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {

	s := &socket{
		Protocol: xbus.NewProtocol(),
	}
	return s
}

// NewSocket allocates a raw Socket using the BUS protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
