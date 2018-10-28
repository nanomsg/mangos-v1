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

package impl

import (
	"math/rand"
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2/errors"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

type dialer struct {
	sync.Mutex
	d             transport.Dialer
	s             *socket
	addr          string
	closed        bool
	active        bool
	dialing       bool
	asynch        bool
	redialer      *time.Timer
	reconnTime    time.Duration
	reconnMinTime time.Duration
	reconnMaxTime time.Duration
	closeq        chan struct{}
}

func (d *dialer) Dial() error {
	d.Lock()
	if d.active {
		d.Unlock()
		return mangos.ErrAddrInUse
	}
	if d.closed {
		d.Unlock()
		return mangos.ErrClosed
	}
	d.closeq = make(chan struct{})
	d.active = true
	d.reconnTime = d.reconnMinTime
	if d.asynch {
		go d.redial()
		d.Unlock()
		return nil
	}
	d.Unlock()
	return d.dial(false)
}

func (d *dialer) Close() error {
	d.Lock()
	defer d.Unlock()
	if d.closed {
		return mangos.ErrClosed
	}
	d.closed = true
	return nil
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	switch n {
	case mangos.OptionReconnectTime:
		d.Lock()
		v := d.reconnMinTime
		d.Unlock()
		return v, nil
	case mangos.OptionMaxReconnectTime:
		d.Lock()
		v := d.reconnMaxTime
		d.Unlock()
		return v, nil
	case mangos.OptionDialAsynch:
		d.Lock()
		v := d.asynch
		d.Unlock()
		return v, nil
	}
	if val, err := d.d.GetOption(n); err != mangos.ErrBadOption {
		return val, err
	}
	// Pass it up to the socket
	return d.s.GetOption(n)
}

func (d *dialer) SetOption(n string, v interface{}) error {
	switch n {
	case mangos.OptionReconnectTime:
		if v, ok := v.(time.Duration); ok {
			d.Lock()
			d.reconnMinTime = v
			d.Unlock()
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionMaxReconnectTime:
		if v, ok := v.(time.Duration); ok {
			d.Lock()
			d.reconnMaxTime = v
			d.Unlock()
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionDialAsynch:
		if v, ok := v.(bool); ok {
			d.Lock()
			d.asynch = v
			d.Unlock()
			return nil
		}
	}
	// Transport specific options passed down.
	return d.d.SetOption(n, v)
}

func (d *dialer) Address() string {
	return d.addr
}

// Socket calls this after the pipe is fully accepted (we got a good
// SP layer connection) -- this way we still get the full backoff if
// we achieve a TCP connect, but the upper layer protocols are mismatched,
// or the remote peer just rejects us (such as if an already connected
// pair pipe.)
func (d *dialer) pipeConnected() {
	d.Lock()
	d.reconnTime = d.reconnMinTime
	d.Unlock()
}

func (d *dialer) pipeClosed() {
	// We always want to sleep a little bit after the pipe closed down,
	// to avoid spinning hard.  This can happen if we connect, but the
	// peer refuses to accept our protocol.  Injecting at least a little
	// delay should help.
	d.Lock()
	time.AfterFunc(d.reconnTime, d.redial)
	d.Unlock()
}

func (d *dialer) dial(redial bool) error {
	d.Lock()
	if d.asynch {
		redial = true
	}
	if d.dialing || d.closed {
		// If we already have a dial in progress, then stop.
		// This really should never occur (see comments below),
		// but having multiple dialers create multiple pipes is
		// probably bad.  So be paranoid -- I mean "defensive" --
		// for now.
		d.Unlock()
		return errors.ErrAddrInUse
	}
	if d.redialer != nil {
		d.redialer.Stop()
	}
	d.dialing = true
	d.Unlock()

	p, err := d.d.Dial()
	if err == nil {
		d.s.addPipe(p, d, nil)

		d.Lock()
		d.dialing = false
		d.Unlock()
		return nil
	}

	d.Lock()
	defer d.Unlock()

	// We're no longer dialing, so let another reschedule happen, if
	// appropriate.   This is quite possibly paranoia.  We should only
	// be in this routine in the following circumstances:
	//
	// 1. Initial dialing (via Dial())
	// 2. After a previously created pipe fails and is closed due to error.
	// 3. After timing out from a failed connection attempt.
	//
	// The above cases should be mutually exclusive.  But paranoia.
	// Consider removing the d.dialing logic later if we can prove
	// that this never occurs.
	d.dialing = false

	if !redial {
		return err
	}
	switch err {
	case mangos.ErrClosed:
		// Stop redialing, no further action.

	default:
		// Exponential backoff, and jitter.  Our backoff grows at
		// about 1.3x on average, so we don't penalize a failed
		// connection too badly.
		minfact := float64(1.1)
		maxfact := float64(1.5)
		actfact := rand.Float64()*(maxfact-minfact) + minfact
		rtime := d.reconnTime
		if d.reconnMaxTime != 0 {
			d.reconnTime = time.Duration(actfact * float64(d.reconnTime))
			if d.reconnTime > d.reconnMaxTime {
				d.reconnTime = d.reconnMaxTime
			}
		}
		d.redialer = time.AfterFunc(rtime, d.redial)
	}
	return err
}

func (d *dialer) redial() {
	d.dial(true)
}

func (d *dialer) xredial() {
	d.Lock()
	if d.dialing || d.closed {
		// If we already have a dial in progress, then stop.
		// This really should never occur (see comments below),
		// but having multiple dialers create multiple pipes is
		// probably bad.  So be paranoid -- I mean "defensive" --
		// for now.
		d.Unlock()
		return
	}
	if d.redialer != nil {
		d.redialer.Stop()
	}
	d.dialing = true
	d.Unlock()

	p, err := d.d.Dial()
	if err == nil {
		d.s.addPipe(p, d, nil)

		d.Lock()
		d.dialing = false
		d.Unlock()
		return
	}

	d.Lock()
	defer d.Unlock()

	// We're no longer dialing, so let another reschedule happen, if
	// appropriate.   This is quite possibly paranoia.  We should only
	// be in this routine in the following circumstances:
	//
	// 1. Initial dialing (via Dial())
	// 2. After a previously created pipe fails and is closed due to error.
	// 3. After timing out from a failed connection attempt.
	//
	// The above cases should be mutually exclusive.  But paranoia.
	// Consider removing the d.dialing logic later if we can prove
	// that this never occurs.
	d.dialing = false

	switch err {
	case nil:
		return

	case mangos.ErrClosed:
		// Stop redialing, no further action.
		return

	default:
		// Exponential backoff, and jitter.  Our backoff grows at
		// about 1.3x on average, so we don't penalize a failed
		// connection too badly.
		minfact := float64(1.1)
		maxfact := float64(1.5)
		actfact := rand.Float64()*(maxfact-minfact) + minfact
		rtime := d.reconnTime
		if d.reconnMaxTime != 0 {
			d.reconnTime = time.Duration(actfact * float64(d.reconnTime))
			if d.reconnTime > d.reconnMaxTime {
				d.reconnTime = d.reconnMaxTime
			}
		}
		d.redialer = time.AfterFunc(rtime, d.redial)
	}
}
