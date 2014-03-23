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

// Package sp provides a pure Go implementation of the Scalability
// Protocols.  These are more familiarily known as "nanomsg" which is the
// C-based software package that is also their reference implementation.
//
// These protocols facilitate the rapid creation of applications which
// rely on multiple participants in sometimes complex communications
// topologies, including Request/Reply, Publish/Subscribe, Push/Pull,
// Surveyor/Respondant, etc.
//
// For more information, see www.nanomsg.org.
//
// This package is very very preliminary, and should probably not be put
// into any real use at this point.  Use at your own risk!
//
package sp

import ()

// SPAddr stores just the address, which will normally be something
// like a path, but any valid string can be used as a key.  This implements
// the net.Addr interface.  SP addresses look like URLs, and are independent
// of the SP protocol.  For example, tcp://127.0.0.1:5050 represents TCP
// port 5050 on localhost, and ipc:///var/run/mysock  is a reference to the
// path /var/run/mysock which (on UNIX systems) would be a UNIX domain socket.
// The inproc:// scheme is also supported, and the string that follows is just
// an arbitrary key.
type SPAddr struct {
	name string
}

// String returns the name of the end point -- the listen address.  This
// is just an arbitrary string used as a lookup key.
func (a *SPAddr) String() string {
	return a.name
}

// Network returns the scheme such as "ipc", "tcp", "inproc", etc.
func (a *SPAddr) Network() string {
	return "TBD"
}
