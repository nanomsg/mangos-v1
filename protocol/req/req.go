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

// Package req implements the REQ protocol, which is the request side of
// the request/response pattern.  (REP is the response.)
package req

import (
	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/xreq"
)

// XXX: REMOVE THIS WHEN WE FINISH CONVERTING THE OTHER RAW PROTOCOLS
// TO X FORMS.

// NewRawSocket allocates a raw Socket using the REQ protocol.
func NewRawSocket() (mangos.Socket, error) {
	return xreq.NewSocket()
}
