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

// Endpoint represents the handle that a Protocol implementation has
// to the underlying stream transport.  It can be thought of as one side
// of a TCP, IPC, or other type of connection.
type Endpoint interface {
	// GetID returns a unique 31-bit value associated with the Endpoint.
	// The value is unique for a given socket, at a given time.
	GetID() uint32

	// Close does what you think.
	Close() error

	// SendMsg sends a message.  On success it returns nil. This is a
	// blocking call.
	SendMsg(*Message) error

	// RecvMsg receives a message.  It blocks until the message is
	// received.  On error, the pipe is closed and nil is returned.
	RecvMsg() *Message
}

// Protocol implementations handle the "meat" of protocol processing.  Each
// protocol type will implement one of these.  For protocol pairs (REP/REQ),
// there will be one for each half of the protocol.
type Protocol interface {

	// Init is called by the core to allow the protocol to perform
	// any initialization steps it needs.  It should save the handle
	// for future use, as well.
	Init(ProtocolSocket)

	// AddEndpoint is called when a new Endpoint is added to the socket.
	// Typically this is as a result of connect or accept completing.
	AddEndpoint(Endpoint)

	// RemoveEndpoint is called when an Endpoint is removed from the socket.
	// Typically this indicates a disconnected or closed connection.
	RemoveEndpoint(Endpoint)

	// ProtocolNumber returns a 16-bit value for the protocol number,
	// as assigned by the SP governing body. (IANA?)
	Number() uint16

	// ValidPeer returns true of the argument protocol number is a valid
	// peer for this protocol, false otherwise.  For example, REP is a
	// valid peer for REQ and XREQ, but not for SUB or PUB.  We only match
	// based on protocol number.
	ValidPeer(uint16) bool

	// GetOption is used to retrieve the current value of an option.
	// If the protocol doesn't recognize the option, EBadOption should
	// be returned.
	GetOption(string) (interface{}, error)

	// SetOption is used to set an option.  EBadOption is returned if
	// the option name is not recognized, EBadValue if the value is
	// invalid.
	SetOption(string, interface{}) error
}

// The follow are optional interfaces that a Protocol can choose to implement.

// ProtocolRecvHook is intended to be an additional extension
// to the Protocol interface.
type ProtocolRecvHook interface {
	// RecvHook is called just before the message is handed to the
	// application.  The message may be modified.  If false is returned,
	// then the message is dropped.
	RecvHook(*Message) bool
}

// ProtocolSendHook is intended to be an additional extension
// to the Protocol interface.
type ProtocolSendHook interface {
	// SendHook is called when the application calls Send.
	// If false is returned, the message will be silently dropped.
	// Note that the message may be dropped for other reasons,
	// such as if backpressure is applied.
	SendHook(*Message) bool
}

// ProtocolSocket is the "handle" given to protocols to interface with the
// socket.  The Protocol implementation should not access any sockets or pipes
// except by using functions made available on the ProtocolSocket.  Note
// that all functions listed here are non-blocking.
type ProtocolSocket interface {
	// SendChannel represents the channel used to send messages.  The
	// application injects messages to it, and the protocol consumes
	// messages from it.
	SendChannel() <-chan *Message

	// RecvChannel is the channel used to receive messages.  The protocol
	// should inject messages to it, and the application will consume them
	// later.
	RecvChannel() chan<- *Message

	// The protocol can wait on this channel to close.  When it is closed,
	// it indicates that the application has closed the upper read socket,
	// and the protocol should stop any further read operations on this
	// instance.
	CloseChannel() chan struct{}

	// The protocol can wait on this channel to close.  When it is closed,
	// it indicates that the application has closed the upper write socket,
	// and the protocol should stop any further write operations on this
	// instance.
	DrainChannel() chan struct{}

	// GetOption may be used by the protocol to retrieve an option from
	// the socket.  This can ultimately wind up calling into the socket's
	// own GetOption handler, so care should be used!
	GetOption(string) (interface{}, error)

	// SetOption is used by the Protocol to set an option on the socket.
	// Note that this may set transport options, or even call back down
	// into the protocol's own SetOption interface!
	SetOption(string, interface{}) error
}

// Useful constants for protocol numbers.  Note that the major protocol number
// is stored in the upper 12 bits, and the minor (subprotocol) is located in
// the bottom 4 bits.
const (
	ProtoPair       = (1 * 16)
	ProtoPub        = (2 * 16)
	ProtoSub        = (2 * 16) + 1
	ProtoReq        = (3 * 16)
	ProtoRep        = (3 * 16) + 1
	ProtoPush       = (5 * 16)
	ProtoPull       = (5 * 16) + 1
	ProtoSurveyor   = (6 * 16)
	ProtoRespondent = (6 * 16) + 1
	ProtoBus        = (7 * 16)

	// Experimental Protocols - Use at Risk

	ProtoStar = (100 * 16)
)
