// Copyright 2015 The Mangos Authors
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

// Package push implements the PUSH protocol, which is the write side of
// the pipeline pattern.  (PULL is the reader.)
package push

import (
	"sync"
	"time"

	"github.com/go-mangos/mangos"
)

type push struct {
	sock mangos.ProtocolSocket
	raw  bool
	w    mangos.Waiter
	eps  map[uint32]*pushEp
	sync.Mutex
}

type pushEp struct {
	ep mangos.Endpoint
	cq chan struct{}
}

func (x *push) Init(sock mangos.ProtocolSocket) {
	x.sock = sock
	x.w.Init()
	x.sock.SetRecvError(mangos.ErrProtoOp)
	x.eps = make(map[uint32]*pushEp)
}

func (x *push) Shutdown(expire time.Time) {
	x.w.WaitAbsTimeout(expire)
}

func (x *push) sender(ep *pushEp) {
	defer x.w.Done()
	sq := x.sock.SendChannel()
	cq := x.sock.CloseChannel()

	for {
		select {
		case <-cq:
			return
		case <-ep.cq:
			return
		case m := <-sq:
			if m == nil {
				sq = x.sock.SendChannel()
				continue
			}
			if ep.ep.SendMsg(m) != nil {
				m.Free()
				return
			}
		}
	}
}

func (*push) Number() uint16 {
	return mangos.ProtoPush
}

func (*push) PeerNumber() uint16 {
	return mangos.ProtoPull
}

func (*push) Name() string {
	return "push"
}

func (*push) PeerName() string {
	return "pull"
}

func (x *push) AddEndpoint(ep mangos.Endpoint) {
	pe := &pushEp{ep: ep, cq: make(chan struct{})}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	x.w.Add()
	go x.sender(pe)
	go mangos.NullRecv(ep)
}

func (x *push) RemoveEndpoint(ep mangos.Endpoint) {
	id := ep.GetID()
	x.Lock()
	pe := x.eps[id]
	delete(x.eps, id)
	x.Unlock()
	if pe != nil {
		close(pe.cq)
	}
}

func (x *push) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case mangos.OptionRaw:
		if x.raw, ok = v.(bool); !ok {
			return mangos.ErrBadValue
		}
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (x *push) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRaw:
		return x.raw, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PUSH protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&push{}), nil
}
