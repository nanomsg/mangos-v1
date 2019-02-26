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

// Pipe represents the high level interface to a low level communications
// channel.  There is one of these associated with a given TCP connection,
// for example.  This interface is intended for application use.
//
// Note that applications cannot send or receive data on a Pipe directly.
type Pipe interface {

	// ID returns the numeric ID for this Pipe.  This will be a
	// 31 bit (bit 32 is clear) value for the Pipe, which is unique
	// across all other Pipe instances in the application, while
	// this Pipe exists.  (IDs are recycled on Close, but only after
	// all other Pipe values are used.)
	ID() uint32

	// Address returns the address (URL form) associated with the Pipe.
	// This matches the string passed to Dial() or Listen().
	Address() string

	// GetOption returns an arbitrary option.  The details will vary
	// for different transport types.
	GetOption(name string) (interface{}, error)

	// Listener returns the Listener for this Pipe, or nil if none.
	Listener() Listener

	// Dialer returns the Dialer for this Pipe, or nil if none.
	Dialer() Dialer

	// Close closes the Pipe.  This does a disconnect, or something similar.
	// Note that if a dialer is present and active, it will redial.
	Close() error
}

// PipeEvent determines what is actually transpiring on the Pipe.
type PipeEvent int

const (
	// PipeEventAttaching is called before the Pipe is registered with the
	// socket.  The intention is to permit the application to reject
	// a pipe before it is attached.
	PipeEventAttaching = iota

	// PipeEventAttached occurs after the Pipe is attached.
	// Consequently, it is possible to use the Pipe for delivering
	// events to sockets, etc.
	PipeEventAttached

	// PipeEventDetached occurs after the Pipe has been detached
	// from the socket.
	PipeEventDetached
)

// PipeEventHook is an application supplied function to be called when
// events occur relating to a Pipe.
type PipeEventHook func(PipeEvent, Pipe)
