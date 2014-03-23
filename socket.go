// Copyright 2014 Garrett D'Amore
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

package sp

type Socket interface {
	// Close closes the open Socket.  It is an error (panic) to call
	// other operations on the Socket once it has been closed.
	Close()

	// Send puts the message on the outbound send.  It always succeeds,
	// unless the buffer(s) are full.  Once the system takes ownership of
	// the message, it guarantees to deliver the message or keep trying as
	// long as the Socket is open.
	Send([]byte) error

	// Recv receives a complete message.  The entire message is received.
	Recv() ([]byte, error)

	// SendMsg puts the message on the outbound send.  It works like Send,
	// but allows the caller to supply message headers.
	SendMsg(*Message) error

	// RecvMsg receives a complete message, including the message header,
	// which is useful for protocols in raw mode.
	RecvMsg() (*Message, error)

	// Dial connects a remote endpoint to the Socket.  The function
	// returns immediately, and an asynchronous goroutine is started to
	// establish and maintain the connection, reconnecting as needed.
	// If the address is invalid, then an error is returned.
	Dial(addr string) error

	// Listen connects a local endpoint to the Socket.  Remote peers
	// may connect (e.g. with Dial) and will each be "connected" to
	// the Socket.  The accepter logic is run in a separate goroutine.
	// The only error possible is if the address is invalid.
	Listen(addr string) error
}

// NewSocket creates a new Socket using the specified protocol.
func NewSocket(protocol string) (Socket, error) {
	proto := GetProtocol(protocol)
	if proto == nil {
		return nil, EBadProto
	}
	sock := newCoreSocket()
	proto.Init(&sock.hndl)
	sock.proto = proto
	return sock, nil
}
