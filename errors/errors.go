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

// Package errors just defines some constant error codes, and is intended
// to be directly imported.  It is safe to import using ".", so that
// short names can be used without concern about unrelated namespace
// pollution.
package errors

type err string

func (e err) Error() string {
	return string(e)
}

// Predefined error values.
const (
	ErrBadAddr     = err("invalid address")
	ErrBadHeader   = err("invalid header received")
	ErrBadVersion  = err("invalid protocol version")
	ErrTooShort    = err("message is too short")
	ErrTooLong     = err("message is too long")
	ErrClosed      = err("object closed")
	ErrConnRefused = err("connection refused")
	ErrSendTimeout = err("send time out")
	ErrRecvTimeout = err("receive time out")
	ErrProtoState  = err("incorrect protocol state")
	ErrProtoOp     = err("invalid operation for protocol")
	ErrBadTran     = err("invalid or unsupported transport")
	ErrBadProto    = err("invalid or unsupported protocol")
	ErrBadOption   = err("invalid or unsupported option")
	ErrBadValue    = err("invalid option value")
	ErrGarbled     = err("message garbled")
	ErrAddrInUse   = err("address in use")
	ErrBadProperty = err("invalid property name")
	ErrTLSNoConfig = err("missing TLS configuration")
	ErrTLSNoCert   = err("missing TLS certificates")
	ErrNotRaw      = err("socket not raw")
	ErrCanceled    = err("operation canceled")
	ErrNoContext   = err("protocol does not support contexts")
)
