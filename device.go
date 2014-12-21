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

import (
	"sync"
)

// Device is used to create a forwarding loop between two sockets.  If the
// same socket is listed (or either socket is nil), then a loopback device
// is established instead.  Note that the single socket case is only valid
// for protocols where the underlying protocol can peer for itself (e.g. PAIR,
// or BUS, but not REQ/REP or PUB/SUB!)  Both sockets will be placed into RAW
// mode.
//
// If the plumbing is successful, nil will be returned.  Two threads will be
// established to forward messages in each direction.  If either socket returns
// error on receive or send, the goroutine doing the forwarding will exit.
// This means that closing either socket will generally cause the goroutines
// to exit.  Apart from closing the socket(s), no futher operations should be
// performed against the socket.

type Device interface {
	// Start starts up the Device, forwarding between sockets.
	Start() error

	// Stop stops forwarding.  Note that it does so cleanly, by aborting
	// operations on the receive side.  If a pending send is in progress
	// when Stop is called, that send will complete first, before the
	// operation is stopped.  Applications wishing to have a harder
	// abort may either Abort() or Close() the underlying sockets.  Note
	// that once an underlying socket is closed, the device can no longer
	// be used.
	Stop() error

	// IsStopped returns true if the device is stopped.
	IsStopped() bool

	// LastError returns the last error from either socket.  If all is
	// well, then nil is returned.
	LastError() error

	// StopChan returns the channel used for stopping.  The channel will
	// be closed when the device is fully stopped.  If the device has
	// never had start called, then nil is returned.
	StopChan() chan struct{}
}

type device struct {
	s1 Socket
	s2 Socket

	stopped bool
	lasterr error

	active int
	stopq  chan struct{}

	sync.Mutex
}

// NewDevice creates the Device.  Forwarding is not started until Start().
func NewDevice(s1 Socket, s2 Socket) (Device, error) {
	d := &device{}

	// Is one of the sockets nil?
	if s1 == nil {
		d.s1 = s2
	} else {
		d.s1 = s1
	}
	if s2 == nil {
		d.s2 = s1
	} else {
		d.s2 = s2
	}
	if (d.s1 == nil) {
		d.lasterr = ErrClosed
		return nil, ErrClosed
	}

	p1 := d.s1.GetProtocol()
	p2 := d.s2.GetProtocol()

	if !p1.ValidPeer(p2.Number()) {
		d.lasterr = ErrBadProto
		return nil, ErrBadProto
	}
	if !p2.ValidPeer(p1.Number()) {
		d.lasterr = ErrBadProto
		return nil, ErrBadProto
	}
	return d, nil
}

// Start starts the forwarding process.
func (d *device) Start() error {
	if err := d.s1.SetOption(OptionRaw, true); err != nil {
		d.lasterr = err
		return err
	}
	if err := d.s2.SetOption(OptionRaw, true); err != nil {
		d.lasterr = err
		return err
	}

	d.Lock()
	d.stopped = false
	d.lasterr = nil
	d.s1.Reset()
	d.s2.Reset()
	d.active = 2
	d.stopq = make(chan struct{})
	go d.forwarder(d.s1, d.s2)
	go d.forwarder(d.s2, d.s1)
	d.Unlock()
	return nil
}

// Stop stops the Device, closing the underlying sockets.
func (d *device) Stop() error {
	d.Lock()
	d.s1.Abort()
	d.s2.Abort()
	d.stopped = true
	d.Unlock()
	return nil
}

func (d *device) IsStopped() bool {
	return d.stopped
}

func (d *device) LastError() error {
	return d.lasterr
}

func (d *device) StopChan() chan struct{} {
	return d.stopq
}

// forwarder takes messages from one socket, and sends them to the other.
// The sockets must be of compatible types, and must be in Raw mode.
func (d *device) forwarder(src Socket, dst Socket) {

	for {
		m, err := src.RecvMsg()
		switch err {
		case nil:
			break
		case ErrAborted:
			d.Lock()
			dst.Abort()
			d.stopped = true
			d.lasterr = err
			d.active--
			if d.active <= 0 {
				close(d.stopq)
			}
			d.Unlock()
			return
		default:
			d.Lock()
			d.lasterr = err
			d.active--
			if d.active <= 0 {
				close(d.stopq)
			}
			d.Unlock()
			return
		}

		// Note that we should never have an aborted socket here,
		// because we only abort the Recv socket.  This avoids
		// the problem of stop/start causing a dropped message.
		err = dst.SendMsg(m)
		switch err {
		case nil:
			break
		default:
			d.Lock()
			d.lasterr = err
			d.active--
			if d.active <= 0 {
				close(d.stopq)
			}
			d.Unlock()
			return
		}
	}
}
