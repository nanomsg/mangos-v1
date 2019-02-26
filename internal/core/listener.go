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

package core

import (
	"sync"
	"time"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/transport"
)

type listener struct {
	sync.Mutex
	l      transport.Listener
	s      *socket
	addr   string
	closed bool
}

func (l *listener) GetOption(n string) (interface{}, error) {
	// Listeners do not keep local options; we just pass this down.
	return l.l.GetOption(n)
}

func (l *listener) SetOption(n string, v interface{}) error {
	// Listeners do not keep local options; we just pass this down.
	return l.l.SetOption(n, v)
}

// serve spins in a loop, calling the accepter's Accept routine.
func (l *listener) serve() {
	for {
		l.Lock()
		if l.closed {
			l.Unlock()
			break
		}
		l.Unlock()

		// If the underlying PipeListener is closed, or not
		// listening, we expect to return back with an error.
		if tp, err := l.l.Accept(); err == mangos.ErrClosed {
			return
		} else if err == nil {
			l.s.addPipe(tp, nil, l)
		} else {
			// Debounce a little bit, to avoid thrashing the CPU.
			time.Sleep(time.Second / 100)
		}
	}
}

func (l *listener) Listen() error {
	// This function sets up a goroutine to accept inbound connections.
	// The accepted connection will be added to a list of accepted
	// connections.  The Listener just needs to listen continuously,
	// as we assume that we want to continue to receive inbound
	// connections without limit.

	if err := l.l.Listen(); err != nil {
		return err
	}

	go l.serve()
	return nil
}

func (l *listener) Address() string {
	return l.l.Address()
}

func (l *listener) Close() error {
	l.Lock()
	defer l.Unlock()
	if l.closed {
		return mangos.ErrClosed
	}
	l.closed = true
	return l.l.Close()
}
