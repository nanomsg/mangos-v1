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
// TCP or UNIX domain connected socket.
type PipeKey uint32

// ProtocolOptionHandler provides a standard interface for setting or getting
// protocol specific options.
type ProtocolOptionHandler interface {

	// SetOption is used to set an option.  EBadOption is returned if
	// the option name is not recognized, EBadValue if the value is
	// invalid.
	SetOption(string, interface{}) error

	// GetOption is used to retrieve the current value of an option.
	// If the protocol doesn't recognize the option, EBadOption should
	// be returned.
	GetOption(string) (interface{}, error)
}

// Protocol implementations handle the "meat" of protocol processing.  Each
// protocol type will implement one of these.  For protocol pairs (REP/REQ),
// there will be one for each half of the protocol.
type Protocol interface {

	// Init is called by the core to allow the protocol to perform
	// any initialization steps it needs.  It should save the handle
	// for future use, as well.
	Init(ProtocolHandle)

	// Process is called by the implementation core.  It is used to
	// perform all protocol processing.  IT MUST NOT BLOCK.  It runs one
	// iteration of the processing, and then should return.  It will be
	// called automatically when new messages arrive for handling, or
	// when previously "full" queues become available.  It is also called
	// when new pipes are added, or when existing pipes are closed.
	// It should try to perform all non-blocking actions that it can,
	// before returning.
	//
	// A trivial Protocol (one that just acts as a unicast passthru) might
	// have a Process handler like the following.  Note that this example
	// drops messages that encounter backpressure or errors.
	//
	//	func (p *Passthru) Process() bool {
	//		var err error
	//		h = p.protocolHandle
	//		if msg := h.PullDown(); msg != nil {
	//			h.Send(p.send)  // drop if backpressure
	//		}
	//
	//		if msg, err := h.Recv() {
	//			h.PushUp(p.recv) // drop if backpressure
	//		}
	//	}
	//
	Process()

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

	// RecvHook is called just before the message is handed to the
	// application.  The message may be modified.  If false is returned,
	// then the message is dropped.
	RecvHook(*Message) bool

	// SendHook is called when the application calls Send.
	// If false is returned, the message will be silently dropped.
	// Note that the message may be dropped for other reasons,
	// such as if backpressure is applied.
	SendHook(*Message) bool
}

// ProtocolHandle is the "handle" given to protocols to interface with the
// core.  The Protocol implementation should not access any sockets or pipes
// except by using functions made available on the ProtocolHandle.  Note
// that all functions listed here are non-blocking.
type ProtocolHandle interface {
	// PullDown gets a message from the user, if one is queued.  If none
	// are available, nil is returned.
	PullDown() *Message

	// PushUp attempts to push a message to the user.  It returns false
	// if it was unable to do so, perhaps because the user has messages
	// he has not received yet.
	PushUp(*Message) bool

	// Send sends a message out the wire.
	// the message can't be sent or queued (all pipes/buffers full, or
	// no peers are connected), EPipeFull is returned.  This sends a
	// message to just one remote peer, which is chosen by the core.
	// The pipe chosen for transmit is given back, in case the caller
	// wants to later check if it disconnected (to expedite resends on
	// closed channels.)
	Send(*Message) (PipeKey, error)

	// Send sends to an explicit peer.  If the Pipe is not open, then
	// EClosed is returned.  If the Pipe simply cannot accept more
	// messages, then EPipeFull is returned.  Other errors are fatal.
	SendTo(*Message, PipeKey) error

	// Send attempts to broadcast the message to all Pipes.
	// Its quite possible that there are Pipes that cannot accept it.
	// The message will only be delivered to those that are capable
	// of sending or queueing it.  No status is returned.
	SendAll(*Message)

	// Recv receives a message, but includes the Pipe from which
	// it was received.  Note that the Protocol must treat the Pipe as
	// an opaque endpoint.  If no message was available for receiving,
	// then the error will be EPipeEmpty. The message is received as
	// a contiguous byte array -- the protocol will carve off any
	// header(s) it cares about.
	Recv() (*Message, PipeKey, error)

	// IsOpen checks to see if the associated Pipe is open.  This way
	// an immediate retransmit can be scheduled if an underlying connection
	// is closed.
	IsOpen(PipeKey) bool

	// WakeUp wakes the protocol handler (resulting in the Process()
	// function being called.  Unlike the other functions,
	// this can be executed asynchronously from a goroutine.  The
	// purpose is to permit waking up from a retry timer established
	// by the protocol handler.
	WakeUp()

	// RegisterOptionHandler is intended to give protocols a chance to
	// register handlers for option setting, or retrieval.  It should be
	// executed during the protocol's Init handler.  This allows protocols
	// to create option handlers, without requiring all protocols to
	// cope with it.
	RegisterOptionHandler(ProtocolOptionHandler)
}

var protocolsL sync.Mutex
var protocols map[string]ProtocolFactory

func registerProtocolFactory(name string, f ProtocolFactory) {
	// This version assumes the lock is already held
	protocols[strings.ToLower(name)] = f
}

func initProtocols() {
	protocolsL.Lock()
	defer protocolsL.Unlock()
	if protocols == nil {
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
	}
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

	initProtocols()

	protocolsL.Lock()
	registerProtocolFactory(name, f)
	protocolsL.Unlock()
}

// getProtocol instantiates a Protocol by name.  The lookup is case-insensitive.
func getProtocol(name string) Protocol {
	initProtocols()

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
	PairName = "PAIR" // Pair Protoco
	ReqName  = "REQ"  // Request Protocol
	RepName  = "REP"  // Reply Protocol
	PubName  = "PUB"  // Publish Protocol
	SubName  = "SUB"  // Subscribe Protocol
	BusName  = "BUS"  // Bus Protocol
	XReqName = "XREQ" // Raw Request Protocol
	XRepName = "XREP" // Raw Reply Protocol
	XPubName = "XPUB" // Raw Publish Protocol
	XSubName = "XSUB" // Raw Subscribe Protocol
	XBusName = "XBUS" // Raw Bus Protocol
)
