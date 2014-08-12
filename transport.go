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

package mangos

// Pipe behaves like a full-duplex message-oriented connection between two
// peers.  Callers may call operations on a Pipe simultaneously from
// different goroutines.  (These are different from net.Conn because they
// provide message oriented semantics.)
//
// Pipe is only intended for use by transport implementors, and should
// not be directly used in applications.
type Pipe interface {

	// Send sends a complete message.  In the event of a partial send,
	// the Pipe will be closed, and an error is returned.  For reasons
	// of efficiency, we allow the message to be sent in a scatter/gather
	// list.
	Send(*Message) error

	// Recv receives a complete message.  In the event that either a
	// complete message could not be received, an error is returned
	// to the caller and the Pipe is closed.
	//
	// To mitigate Denial-of-Service attacks, we limit the max message
	// size to 1M.
	Recv() (*Message, error)

	// Close closes the underlying transport.  Further operations on
	// the Pipe will result in errors.  Note that messages that are
	// queued in transport buffers may still be received by the remote
	// peer.
	Close() error

	// LocalProtocol returns the 16-bit SP protocol number used by the
	// local side.  This will normally be sent to the peer during
	// connection establishment.
	LocalProtocol() uint16

	// RemoteProtocol returns the 16-bit SP protocol number used by the
	// remote side.  This will normally be received from the peer during
	// connection establishment.
	RemoteProtocol() uint16

	// IsOpen returns true if the underlying connection is open.
	IsOpen() bool
}

// PipeDialer represents the client side of a connection.  Clients initiate
// the connection.
//
// PipeDialer is only intended for use by transport implementors, and should
// not be directly used in applications.
type PipeDialer interface {
	// Dial is used to initiate a connection to a remote peer.
	Dial() (Pipe, error)
}

// PipeAccepter represents the server side of a connection.  Servers respond
// to a connection request from clients.
//
// PipeAccepter is only intended for use by transport implementors, and should
// not be directly used in applications.
type PipeAccepter interface {

	// Accept completes the server side of a connection.  Once the
	// connection is established and initial handshaking is complete,
	// the resulting connection is returned to the client.
	Accept() (Pipe, error)

	// Close ceases any listening activity, and will specifically close
	// any underlying file descriptor.  Once this is done, the only way
	// to resume listening is to create a new Server instance.  Presumably
	// this function is only called when the last reference to the server
	// is about to go away.
	Close() error
}

// Transport is the interface for transport suppliers to implement.
type Transport interface {
	// Scheme returns a string used as the prefix for SP "addresses".
	// This is similar to a URI scheme.  For example, schemes can be
	// "tcp" (for "tcp://xxx..."), "ipc", "inproc", etc.
	Scheme() string

	// NewDialer creates a new Dialer for this Transport.
	NewDialer(url string, protocol uint16) (PipeDialer, error)

	// NewAccepter creates a new Accepter for this Transport.
	// This generally also arranges for an OS-level file descriptor to be
	// opened, and bound to the the given address, as well as establishing
	// any "listen" backlog.
	NewAccepter(url string, protocol uint16) (PipeAccepter, error)

	// SetOption allows for transports to handle transport specific
	// options.  EBadOption should be returned if the Transport doesn't
	// recognize the option.
	SetOption(string, interface{}) error

	// GetOption is used to retrieve the current value of an option.
	// If the protocol doesn't recognize the option, EBadOption should
	// be returned.
	GetOption(string) (interface{}, error)
}
