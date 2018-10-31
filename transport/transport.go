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

package transport

import (
	"net"
	"strings"
	"sync"

	"nanomsg.org/go/mangos/v2"
)

// Currently these types are just aliases, in order to avoid
// circular dependencies.  When mangos is broken up into a separate
// implementation package, we can move the definitions here (except
// for Message of course).

// Message is an alias for the mangos.Message
type Message = mangos.Message

// ProtocolInfo is stuff that describes a protocol.
type ProtocolInfo = mangos.ProtocolInfo

// Pipe is a transport pipe.
type Pipe = mangos.TranPipe

// Dialer is a factory that creates Pipes by connecting to remote listeners.
type Dialer = mangos.TranDialer

// Listener is a factory that creates Pipes by listening to inbound dialers.
type Listener = mangos.TranListener

// Transport is our transport operations.
type Transport = mangos.Transport

// StripScheme removes the leading scheme (such as "http://") from an address
// string.  This is mostly a utility for benefit of transport providers.
func StripScheme(t Transport, addr string) (string, error) {
	if !strings.HasPrefix(addr, t.Scheme()+"://") {
		return addr, mangos.ErrBadTran
	}
	return addr[len(t.Scheme()+"://"):], nil
}

// ResolveTCPAddr is like net.ResolveTCPAddr, but it handles the
// wildcard used in nanomsg URLs, replacing it with an empty
// string to indicate that all local interfaces be used.
func ResolveTCPAddr(addr string) (*net.TCPAddr, error) {
	if strings.HasPrefix(addr, "*") {
		addr = addr[1:]
	}
	return net.ResolveTCPAddr("tcp", addr)
}

var lock sync.RWMutex
var transports = map[string]Transport{}

// RegisterTransport is used to register the transport globally,
// after which it will be available for all sockets.  The
// transport will override any others registered for the same
// scheme.
func RegisterTransport(t Transport) {
	lock.Lock()
	transports[t.Scheme()] = t
	lock.Unlock()
}

// GetTransport is used by a socket to lookup the transport
// for a given scheme.
func GetTransport(scheme string) Transport {
	lock.RLock()
	defer lock.RUnlock()
	if t, ok := transports[scheme]; ok {
		return t
	}
	return nil
}
