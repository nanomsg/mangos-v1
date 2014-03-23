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

// Standard errors for SP
type SPError struct {
	err string
	tmo bool
	tmp bool
}

// SPError implements the error interface.
func (e *SPError) Error() string {
	return e.err
}

var (
	EBadAddr       = &SPError{err: "Invalid Address"}
	EBadHeader     = &SPError{err: "Invalid SP Header Received"}
	EBadVersion    = &SPError{err: "Invalid SP Version Received"}
	EShort         = &SPError{err: "Truncated Message"}
	ETooLong       = &SPError{err: "Received Message Length Too Big"}
	EClosed        = &SPError{err: "Connection Closed"}
	ESendTimeout   = &SPError{err: "Send Time Out", tmo: true, tmp: true}
	ERecvTimeout   = &SPError{err: "Receive Time Out", tmo: true, tmp: true}
	EBadTran       = &SPError{err: "Invalid or Unsupported Transport"}
	EBadProto      = &SPError{err: "Invalid or Unsupported Protocol"}
	EProtoMismatch = &SPError{err: "Protocol Mismatch Received"}
	EPipeFull      = &SPError{err: "Pipe Full", tmp: true}
	EPipeEmpty     = &SPError{err: "Pipe Empty", tmp: true}
	EBadOption     = &SPError{err: "Invalid or Unsupported Option"}
	EBadValue      = &SPError{err: "Invalid Option Value"}
	EGarbled       = &SPError{err: "Garbled Message"}
)
