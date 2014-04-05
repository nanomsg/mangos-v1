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

import (
	"container/list"
	"math/rand"
	"sync"
	"time"
)

var pipes struct {
	byid   map[uint32]*pipe
	nextid uint32
	sync.Mutex
}

// pipe wraps the Pipe data structure with the stuff we need to keep
// for the core.  It implements the Endpoint interface.
type pipe struct {
	pipe    Pipe
	rq      chan *Message // messages sent to user
	wq      chan *Message // messages sent to wire
	closeq  chan struct{} // only closed, never passes data
	id      uint32
	alllist *list.Element // Used by socket
	cansend *list.Element // linkage to socket cansend
	canrecv *list.Element // linkage to socket canrecv

	sock    *socket
	closing bool // true if we were closed
	sync.Mutex
}

func init() {
	pipes.byid = make(map[uint32]*pipe)
	pipes.nextid = uint32(rand.NewSource(time.Now().UnixNano()).Int63())
}

func newPipe(tranpipe Pipe) *pipe {
	p := &pipe{pipe: tranpipe}
	// queue depths are kind of arbitrary.  deep enough to avoid
	// stalls, but hopefully shallow enough to avoid latency.
	p.rq = make(chan *Message, 10)
	p.wq = make(chan *Message, 10)
	p.closeq = make(chan struct{})
	for {
		pipes.Lock()
		p.id = pipes.nextid & 0x7fffffff
		pipes.nextid++
		if p.id != 0 && pipes.byid[p.id] == nil {
			pipes.byid[p.id] = p
			pipes.Unlock()
			break
		}
		pipes.Unlock()
	}
	return p
}

func (p *pipe) start(sock *socket) {
	p.sock = sock
	go p.sender()
	go p.receiver()
}

func (p *pipe) GetID() uint32 {
	return p.id
}

func (p *pipe) Close() error {
	p.Lock()
	if p.closing {
		return nil
	}
	p.closing = true
	p.Unlock()
	close(p.closeq)
	p.sock.remPipe(p)
	p.pipe.Close()
	pipes.Lock()
	delete(pipes.byid, p.id)
	pipes.Unlock()
	p.id = 0 // safety
	return nil
}

func (p *pipe) SendMsg(msg *Message) error {
	select {
	case p.wq <- msg:
		p.sock.notifySendReady(p)
		return nil
	case <-p.closeq:
		return ErrClosed
	default:
		return ErrPipeFull
	}
}

func (p *pipe) RecvMsg() *Message {
	select {
	case msg := <-p.rq:
		p.sock.notifyRecvReady(p)
		return msg
	case <-p.closeq:
		return nil
	default:
		return nil
	}
}

func (p *pipe) receiver() {
	var msg *Message
	var err error
	for {
		if msg, err = p.pipe.Recv(); err != nil {
			p.Close()
			return
		}

		select {
		case p.rq <- msg:
			p.sock.notifyRecvReady(p)

		case <-p.closeq:
			return
		}
	}
}

func (p *pipe) sender() {

	for {
		select {
		case msg, ok := <-p.wq:
			p.sock.notifySendReady(p)
			if msg != nil {
				if err := p.pipe.Send(msg); err != nil {
					p.Close()
					return
				}
			}
			if ok == false {
				p.Close()
				return
			}

		case <-p.closeq:
			return
		}
	}
}
