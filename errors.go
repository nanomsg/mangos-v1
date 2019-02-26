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

import (
	"nanomsg.org/go/mangos/v2/errors"
)

// Various error codes.
const (
	ErrBadAddr     = errors.ErrBadAddr
	ErrBadHeader   = errors.ErrBadHeader
	ErrBadVersion  = errors.ErrBadVersion
	ErrTooShort    = errors.ErrTooShort
	ErrTooLong     = errors.ErrTooLong
	ErrClosed      = errors.ErrClosed
	ErrConnRefused = errors.ErrConnRefused
	ErrSendTimeout = errors.ErrSendTimeout
	ErrRecvTimeout = errors.ErrRecvTimeout
	ErrProtoState  = errors.ErrProtoState
	ErrProtoOp     = errors.ErrProtoOp
	ErrBadTran     = errors.ErrBadTran
	ErrBadProto    = errors.ErrBadProto
	ErrBadOption   = errors.ErrBadOption
	ErrBadValue    = errors.ErrBadValue
	ErrGarbled     = errors.ErrGarbled
	ErrAddrInUse   = errors.ErrAddrInUse
	ErrBadProperty = errors.ErrBadProperty
	ErrTLSNoConfig = errors.ErrTLSNoConfig
	ErrTLSNoCert   = errors.ErrTLSNoCert
	ErrNotRaw      = errors.ErrNotRaw
	ErrCanceled    = errors.ErrCanceled
	ErrNoContext   = errors.ErrNoContext
)
