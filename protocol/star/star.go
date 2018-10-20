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

// Package star implements a new, experimental protocol called "STAR".
// This is like the BUS protocol, except that each member of the network
// automatically forwards any message it receives to any other peers.
// In a star network, this means that all members should receive all messages,
// assuming that there is a central server.  Its important to ensure that
// the topology is free from cycles, as there is no protection against
// that, and cycles can lead to infinite message storms.  (TODO: Add a TTL,
// and basic message ID / anti-replay protection.)
package star

import (
	"nanomsg.org/go/mangos/v2/protocol"
	"nanomsg.org/go/mangos/v2/protocol/xstar"
)

type socket struct {
	protocol.Protocol
}

func (s *socket) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionRaw:
		return false, nil
	}
	return s.Protocol.GetOption(name)
}

func (s *socket) SendMsg(m *protocol.Message) error {
	m.Header = make([]byte, 4)
	err := s.Protocol.SendMsg(m)
	if err != nil {
		m.Header = m.Header[:0]
	}
	return err
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	m, err := s.Protocol.RecvMsg()
	if err == nil && m != nil {
		m.Header = m.Header[:0]
	}
	return m, err
}

// Info returns protocol info for star.
func Info() protocol.Info {
	return xstar.Info()
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		Protocol: xstar.NewProtocol(),
	}
	return s
}

// NewSocket allocates a new Socket using the STAR protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
