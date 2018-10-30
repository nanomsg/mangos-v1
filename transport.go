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

package mangos

// XXX: The interfaces listed here will eventually move to the Transport
// package, to be named without the Tran prefix, and then this file will
// go away.

// TranPipe behaves like a full-duplex message-oriented connection between two
// peers.  Callers may call operations on a Pipe simultaneously from
// different goroutines.  (These are different from net.Conn because they
// provide message oriented semantics.)
//
// Pipe is only intended for use by transport implementors, and should
// not be directly used in applications.
type TranPipe interface {

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

	// GetOption returns an arbitrary transport specific option on a
	// pipe.  Options for pipes are read-only and specific to that
	// particular connection. If the property doesn't exist, then
	// ErrBadOption should be returned.
	GetOption(string) (interface{}, error)
}

// TranDialer represents the client side of a connection.  Clients initiate
// the connection.
//
// TranDialer is only intended for use by transport implementors, and should
// not be directly used in applications.
type TranDialer interface {
	// Dial is used to initiate a connection to a remote peer.
	Dial() (TranPipe, error)

	// SetOption sets a local option on the dialer.
	// ErrBadOption can be returned for unrecognized options.
	// ErrBadValue can be returned for incorrect value types.
	SetOption(name string, value interface{}) error

	// GetOption gets a local option from the dialer.
	// ErrBadOption can be returned for unrecognized options.
	GetOption(name string) (value interface{}, err error)
}

// TranListener represents the server side of a connection.  Servers respond
// to a connection request from clients.
//
// TranListener is only intended for use by transport implementors, and should
// not be directly used in applications.
type TranListener interface {

	// Listen actually begins listening on the interface.  It is
	// called just prior to the Accept() routine normally. It is
	// the socket equivalent of bind()+listen().
	Listen() error

	// Accept completes the server side of a connection.  Once the
	// connection is established and initial handshaking is complete,
	// the resulting connection is returned to the client.
	Accept() (TranPipe, error)

	// Close ceases any listening activity, and will specifically close
	// any underlying file descriptor.  Once this is done, the only way
	// to resume listening is to create a new Server instance.  Presumably
	// this function is only called when the last reference to the server
	// is about to go away.  Established connections are unaffected.
	Close() error

	// SetOption sets a local option on the listener.
	// ErrBadOption can be returned for unrecognized options.
	// ErrBadValue can be returned for incorrect value types.
	SetOption(name string, value interface{}) error

	// GetOption gets a local option from the listener.
	// ErrBadOption can be returned for unrecognized options.
	GetOption(name string) (value interface{}, err error)

	// Address gets the local address.  The value may not be meaningful
	// until Listen() has been called.
	Address() string
}

// Transport is the interface for transport suppliers to implement.
type Transport interface {
	// Scheme returns a string used as the prefix for SP "addresses".
	// This is similar to a URI scheme.  For example, schemes can be
	// "tcp" (for "tcp://xxx..."), "ipc", "inproc", etc.
	Scheme() string

	// NewDialer creates a new Dialer for this Transport.
	NewDialer(url string, sock Socket) (TranDialer, error)

	// NewListener creates a new PipeListener for this Transport.
	// This generally also arranges for an OS-level file descriptor to be
	// opened, and bound to the the given address, as well as establishing
	// any "listen" backlog.
	NewListener(url string, sock Socket) (TranListener, error)
}
