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
//

// Package protocol implements some common things protocol implementors
// need.  Only protocol implementations should import this package.
package protocol

import (
	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/errors"
	"nanomsg.org/go/mangos/v2/internal/core"
)

// Protocol numbers
const (
	ProtoPair       = mangos.ProtoPair
	ProtoPub        = mangos.ProtoPub
	ProtoSub        = mangos.ProtoSub
	ProtoReq        = mangos.ProtoReq
	ProtoRep        = mangos.ProtoRep
	ProtoPush       = mangos.ProtoPush
	ProtoPull       = mangos.ProtoPull
	ProtoSurveyor   = mangos.ProtoSurveyor
	ProtoRespondent = mangos.ProtoRespondent
	ProtoBus        = mangos.ProtoBus
	ProtoStar       = mangos.ProtoStar // Experimental
)

// Pipe is a single connection -- provided by the transport layer.
type Pipe = mangos.ProtocolPipe

// Info describes a protocol and it's peer.
type Info = mangos.ProtocolInfo

// Context describes a protocol context.
type Context = mangos.ProtocolContext

// Protocol is the main ops vector for a protocol.
type Protocol = mangos.ProtocolBase

// Socket is the interface definition of a mangos.Socket.
// We need this for creating new ones.
type Socket = mangos.Socket

// Message is an alias for the common mangos.Message.
type Message = mangos.Message

// Borrow common error codes for convenience.
const (
	ErrClosed      = errors.ErrClosed
	ErrSendTimeout = errors.ErrSendTimeout
	ErrRecvTimeout = errors.ErrRecvTimeout
	ErrBadValue    = errors.ErrBadValue
	ErrBadOption   = errors.ErrBadOption
	ErrProtoOp     = errors.ErrProtoOp
	ErrProtoState  = errors.ErrProtoState
	ErrCanceled    = errors.ErrCanceled
)

// Common option definitions
// We have elided transport-specific options here.
const (
	OptionRaw          = mangos.OptionRaw
	OptionRecvDeadline = mangos.OptionRecvDeadline
	OptionSendDeadline = mangos.OptionSendDeadline
	OptionRetryTime    = mangos.OptionRetryTime
	OptionSubscribe    = mangos.OptionSubscribe
	OptionUnsubscribe  = mangos.OptionUnsubscribe
	OptionSurveyTime   = mangos.OptionSurveyTime
	OptionWriteQLen    = mangos.OptionWriteQLen
	OptionReadQLen     = mangos.OptionReadQLen
	OptionLinger       = mangos.OptionLinger // Remove?
	OptionTTL          = mangos.OptionTTL
	OptionBestEffort   = mangos.OptionBestEffort
)

// MakeSocket creates a Socket on top of a Protocol.
func MakeSocket(proto Protocol) Socket {
	return core.MakeSocket(proto)
}
