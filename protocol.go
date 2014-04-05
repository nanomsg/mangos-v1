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

import (
	"strings"
	"sync"
)

// PipeKey is the "key" or identifier used to match to an underlying
// connection channel.  For example, a given key will correspond to a single
// TCP or UNIX domain connected socket.  PipeKey's are always non-zero if
// associated with an actual Pipe; the zero value may be used as an invalid
// value.
type PipeKey uint32

type Endpoint interface {
	// GetID returns a unique 31-bit value associated with the Endpoint.
	// The value is unique for a given socket, at a given time.
	GetID() uint32

	// Close does what you think.
	Close() error

	// SendMsg sends a message.  On success it returns nil.  If the
	// pipe is full, EPipeFull is returned, and the caller may attempt
	// again later.  Otherwise the error is fatal.
	SendMsg(*Message) error

	// RecvMsg receives a message.  If one cannot be obtained
	// for any reason (not present, or closed) nil is returned.
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

	// ProcessSend implements the sender process.  The protocol
	// must attempt to process all of the send activity that it
	// can without blocking.
	ProcessSend()

	// ProcessRecv implements the receiver process.  The protocol
	// must attempt to process all of the receive activity that it
	// can without blocking.
	ProcessRecv()

	// AddEndpoint is called when a new Endpoint is added to the socket.
	// Typically this is as a result of connect or accept completing.
	AddEndpoint(Endpoint)

	// RemEndpoint is called when an Endpoint is removed from the socket.
	// Typically this indicates a disconnected or closed connection.
	RemEndpoint(Endpoint)

	// Name returns the protocol name as a string.  or example, "REP"
	// or "XREP".  (Note that this allows us to provide for different
	// handling semantics using the same protocol number.)
	Name() string

	// ProtocolNumber returns a 16-bit value for the protocol number,
	// as assigned by the SP governing body. (IANA?)
	Number() uint16

	// IsRaw returns true when the protocol handler is a raw mode
	// protocol (such as XReq rather than Req).  This is useful to
	// the Device framework.
	IsRaw() bool

	// ValidPeer returns true of the argument protocol number is a valid
	// peer for this protocol, false otherwise.  For example, REP is a
	// valid peer for REQ and XREQ, but not for SUB or PUB.  We only match
	// based on protocol number.
	ValidPeer(uint16) bool
}

// The follow are optional interfaces that a Protocol can choose to implement.

// ProtocolGetOptionHandler is intended to be an additional extension
// to the Protocol interface.
type ProtocolGetOptionHandler interface {
	// GetOption is used to retrieve the current value of an option.
	// If the protocol doesn't recognize the option, EBadOption should
	// be returned.
	GetOption(string) (interface{}, error)
}

// ProtocolSetOptionHandler is intended to be an additional extension
// to the Protocol interface.
type ProtocolSetOptionHandler interface {
	// SetOption is used to set an option.  EBadOption is returned if
	// the option name is not recognized, EBadValue if the value is
	// invalid.
	SetOption(string, interface{}) error
}

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
	// PullDown gets a message from the user, if one is queued.  If none
	// are available, nil is returned.
	PullDown() *Message

	// PushUp attempts to push a message to the user.  It returns false
	// if it was unable to do so, perhaps because the user has messages
	// he has not received yet.
	PushUp(*Message) bool

	// WakeUp wakes the protocol handler (resulting in the Process()
	// function being called.  Unlike the other functions,
	// this can be executed asynchronously from a goroutine.  The
	// purpose is to permit waking up from a retry timer established
	// by the protocol handler.
	WakeUp()

	// NextSendEndpoint returns the next Endpoint capable of sending
	// messages.  Note that the Endpoint may still be subject to
	// backpressure.
	NextSendEndpoint() Endpoint

	// NextRecvEndpoint returns the next Endpoint with messages pending
	// for receive.  It is possible that there won't actually be a message
	// pending, so the protocol should check.
	NextRecvEndpoint() Endpoint
}

var protocolsL sync.Mutex
var protocols map[string]ProtocolFactory

func registerProtocolFactory(name string, f ProtocolFactory) {
	// This version assumes the lock is already held
	protocols[strings.ToLower(name)] = f
}

func init() {
	protocols = make(map[string]ProtocolFactory)

	// Lets go ahead and pre-register the stock transports.
	registerProtocolFactory(XReqName, XReqFactory)
	registerProtocolFactory(XRepName, XRepFactory)
	registerProtocolFactory(ReqName, ReqFactory)
	registerProtocolFactory(RepName, RepFactory)
	registerProtocolFactory(XPubName, XPubFactory)
	registerProtocolFactory(XSubName, XSubFactory)
	registerProtocolFactory(PubName, PubFactory)
	registerProtocolFactory(SubName, SubFactory)
	registerProtocolFactory(XPairName, XPairFactory)
	registerProtocolFactory(PairName, PairFactory)
}

// ProtocolFactory implements the factory pattern for Protocol instances.
type ProtocolFactory interface {
	// NewProtocol creates a new instance of the Protocol.
	NewProtocol() Protocol
}

// RegisterProtocolFactory registers a new ProtocolFactory.
// Note that the ProtocolFactory might already be registered.
// We don't warn about this as an error.  You can override a built-in
// protocol this way.  Use this at your own risk!
// (The name is used as the lookup key for
// protocols, but is converted to lower case first.)
func RegisterProtocolFactory(name string, f ProtocolFactory) {
	protocolsL.Lock()
	registerProtocolFactory(name, f)
	protocolsL.Unlock()
}

// getProtocol instantiates a Protocol by name.  The lookup is case-insensitive.
func getProtocol(name string) Protocol {
	protocolsL.Lock()
	f := protocols[strings.ToLower(name)]
	protocolsL.Unlock()

	return f.NewProtocol()
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
)

// Protocol names.  These correlate to specific Protocol implementations.
const (
	PairName  = "PAIR"  // Pair Protocol
	ReqName   = "REQ"   // Request Protocol
	RepName   = "REP"   // Reply Protocol
	PubName   = "PUB"   // Publish Protocol
	SubName   = "SUB"   // Subscribe Protocol
	BusName   = "BUS"   // Bus Protocol
	XPairName = "XPAIR" // Raw Pair Protocol
	XReqName  = "XREQ"  // Raw Request Protocol
	XRepName  = "XREP"  // Raw Reply Protocol
	XPubName  = "XPUB"  // Raw Publish Protocol
	XSubName  = "XSUB"  // Raw Subscribe Protocol
	XBusName  = "XBUS"  // Raw Bus Protocol
)
