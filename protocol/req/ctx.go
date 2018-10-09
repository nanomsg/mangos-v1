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
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/impl"
)

type reqPipe struct {
	ep     mangos.Endpoint
	r      *req2
	closed bool
}

type reqCtx struct {
	r          *req2
	cond       *sync.Cond
	resendTime time.Duration   // tunable resend time
	sendExpire time.Duration   // how long to wait in send
	recvExpire time.Duration   // how long to wait in recv
	sendTimer  *time.Timer     // send timer
	recvTimer  *time.Timer     // recv timer
	resender   *time.Timer     // resend timeout
	reqMsg     *mangos.Message // message for transmit
	repMsg     *mangos.Message // received reply
	sendMsg    *mangos.Message // messaging waiting for send
	lastPipe   *reqPipe        // last pipe used for transmit
	reqID      uint32          // request ID
	sendID     uint32          // sent id (cleared after first send)
	recvID     uint32          // recv id (set after first send)
	recvWait   bool            // true if a thread is blocked in RecvMsg
	bestEffort bool            // if true, don't block waiting in send
	wantw      bool            // true if we need to send a message
	closed     bool            // true if we are closed
}

type req2 struct {
	sync.Mutex
	defCtx  *reqCtx              // default context
	ctxs    map[*reqCtx]struct{} // all contexts (set)
	ctxByID map[uint32]*reqCtx   // contexts by request ID
	nextID  uint32               // next request ID
	closed  bool                 // true if we are closed
	sendq   []*reqCtx            // contexts waiting to send
	readyq  []*reqPipe           // pipes available for sending
	pipes   map[uint32]*reqPipe  // all pipes for the socket (by pipe ID)
}

func (r *req2) Init() {
	r.pipes = make(map[uint32]*reqPipe)      // maybe sync.Map is better?
	r.nextID = uint32(time.Now().UnixNano()) // quasi-random
	r.defCtx = &reqCtx{resendTime: time.Minute}
	r.defCtx.cond = sync.NewCond(r)
	r.ctxs = make(map[*reqCtx]struct{})
	r.ctxByID = make(map[uint32]*reqCtx)
}

func (r *req2) send() {
	for len(r.sendq) != 0 && len(r.readyq) != 0 {
		c := r.sendq[0]
		r.sendq = r.sendq[1:]
		c.wantw = false

		p := r.readyq[0]
		r.readyq = r.readyq[1:]

		if c.sendID != 0 {
			c.reqMsg = c.sendMsg
			c.sendMsg = nil
			c.recvID = c.sendID
			r.ctxByID[c.recvID] = c
			c.sendID = 0
			c.cond.Broadcast()
		}
		m := c.reqMsg.Dup()

		// Schedule a retransmit for the future.
		c.lastPipe = p
		if c.resendTime > 0 {
			c.resender = time.AfterFunc(c.resendTime, func() {
				c.resendMessage(m)
			})
		}
		go p.sendCtx(c, m)
	}
}

func (p *reqPipe) sendCtx(c *reqCtx, m *mangos.Message) {
	r := p.r

	// Send this message.  If an error occurs, we examine the
	// error.  If it is ErrClosed, we don't schedule ourself.
	if err := p.ep.SendMsg(m); err != nil {
		m.Free()
		if err == mangos.ErrClosed {
			return
		}
	}
	r.Lock()
	if !c.closed && !p.closed {
		r.readyq = append(r.readyq, p)
		r.send()
	}
	r.Unlock()
}

func (p *reqPipe) receiver() {
	r := p.r
	for {
		m := p.ep.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		id := binary.BigEndian.Uint32(m.Header)

		r.Lock()
		if c, ok := r.ctxByID[id]; ok {
			c.unscheduleSend()
			c.reqMsg.Free()
			c.reqMsg = nil
			c.repMsg = m
			delete(r.ctxByID, id)
			if c.resender != nil {
				c.resender.Stop()
				c.resender = nil
			}
			c.cond.Broadcast()
		} else {
			// No matching receiver so just drop it.
			m.Free()
		}
		r.Unlock()
	}

	// XXX: This would be an excllent time to close the pipe and remove
	// it form the socket alogether.
	p.ep.Close()
}

func (c *reqCtx) resendMessage(m *mangos.Message) {
	r := c.r
	r.Lock()
	defer r.Unlock()
	if c.reqMsg != m {
		return
	}
	if c.wantw {
		return // already scheduled for some reason?
	}
	c.wantw = true
	r.sendq = append(r.sendq, c)
	r.send()
}

func (c *reqCtx) unscheduleSend() {
	r := c.r
	if c.wantw {
		c.wantw = false
		for i, c2 := range r.sendq {
			if c2 == c {
				r.sendq = append(r.sendq[0:i-1],
					r.sendq[i+1:]...)
				return
			}
		}
	}

}
func (c *reqCtx) cancel() {
	r := c.r
	c.unscheduleSend()
	if c.reqID != 0 {
		delete(r.ctxByID, c.reqID)
		c.reqID = 0
	}
	if c.repMsg != nil {
		c.repMsg.Free()
		c.repMsg = nil
	}
	if c.reqMsg != nil {
		c.reqMsg.Free()
		c.reqMsg = nil
	}
	if c.resender != nil {
		c.resender.Stop()
		c.resender = nil
	}
	if c.sendTimer != nil {
		c.sendTimer.Stop()
		c.sendTimer = nil
	}
	if c.recvTimer != nil {
		c.recvTimer.Stop()
		c.recvTimer = nil
	}
	c.sendID = 0
	c.recvID = 0
	c.cond.Broadcast()
}

// SendMsg implements the ProtocolContext SendMsg method.
func (c *reqCtx) SendMsg(m *mangos.Message) error {

	r := c.r

	id := atomic.AddUint32(&r.nextID, 1)
	id |= 0x80000000

	// cooked mode, we stash the header
	m.Header = append([]byte{},
		byte(id>>24), byte(id>>16), byte(id>>8), byte(id))

	r.Lock()
	defer r.Unlock()
	if r.closed || c.closed {
		return mangos.ErrClosed
	}

	c.cancel() // this cancels any pending send or recv calls

	c.reqID = id
	r.ctxByID[id] = c
	c.wantw = true
	r.sendq = append(r.sendq, c)

	if c.bestEffort {
		// for best effort case, we just immediately go the
		// reqMsg, and schedule it as a send.  No waiting.
		// This means that if the message cannot be delivered
		// immediately, it will still get a chance later.
		c.reqMsg = m
		c.recvID = id
		r.send()
		return nil
	}

	expired := false
	c.sendID = id
	c.sendMsg = m
	if c.sendExpire > 0 {
		c.sendTimer = time.AfterFunc(c.sendExpire, func() {
			r.Lock()
			if c.sendID == id {
				expired = true
				c.cancel() // also does a wake up
			}
			r.Unlock()
		})
	}

	r.send()

	// This sleeps until someone picks us up for scheduling.
	// It is responsible for providing the blocking semantic and
	// ultimately backpressure.  Note that we will "continue" if
	// the send is canceled by a subsequent send.
	for c.sendID == id {
		c.cond.Wait()
	}
	if c.sendMsg == m {
		c.sendMsg = nil
		if expired {
			return mangos.ErrSendTimeout
		}
		if c.closed {
			return mangos.ErrClosed
		}
		return mangos.ErrCanceled
	}
	return nil
}

// RecvMsg implements the ProtocolContext RecvMsg method.
func (c *reqCtx) RecvMsg() (*mangos.Message, error) {
	r := c.r
	r.Lock()
	defer r.Unlock()
	if c.recvWait || c.recvID == 0 {
		return nil, mangos.ErrProtoState
	}
	c.recvWait = true
	id := c.recvID
	expired := false

	if c.recvExpire > 0 {
		c.recvTimer = time.AfterFunc(c.recvExpire, func() {
			r.Lock()
			if c.recvID == id {
				expired = true
				c.cancel()
			}
			r.Unlock()
		})
	}

	for id == c.recvID && c.repMsg == nil {
		c.cond.Wait()
	}

	m := c.repMsg
	c.repMsg = nil
	c.recvWait = false
	c.cond.Broadcast()

	if m == nil {
		if expired {
			return nil, mangos.ErrRecvTimeout
		}
		if c.closed {
			return nil, mangos.ErrClosed
		}
		return nil, mangos.ErrCanceled
	}
	return m, nil
}

func (c *reqCtx) SetOption(name string, value interface{}) error {
	switch name {
	case mangos.OptionRetryTime:
		if v, ok := value.(time.Duration); ok {
			c.r.Lock()
			c.resendTime = v
			c.r.Unlock()
			return nil
		}
		return mangos.ErrBadValue

	case mangos.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			c.r.Lock()
			c.recvExpire = v
			c.r.Unlock()
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			c.r.Lock()
			c.sendExpire = v
			c.r.Unlock()
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionBestEffort:
		if v, ok := value.(bool); ok {
			c.r.Lock()
			c.bestEffort = v
			c.r.Unlock()
			return nil
		}
		return mangos.ErrBadValue

		// We don't support these
		// case OptionLinger:
		// case OptionWriteQLen:
		// case OptionReadQLen:

		// Arguably these need to be suppored *somewhere*,  maybe in
		// the parent socket?
		// case mangos.OptionMaxRecvSize:
		// case OptionReconnectTime:
		// case OptionMaxReconnectTime:
	}

	return mangos.ErrBadOption
}

func (c *reqCtx) GetOption(option string) (interface{}, error) {
	switch option {
	case mangos.OptionRaw:
		return false, nil
	case mangos.OptionRetryTime:
		c.r.Lock()
		v := c.resendTime
		c.r.Unlock()
		return v, nil
	case mangos.OptionRecvDeadline:
		c.r.Lock()
		v := c.recvExpire
		c.r.Unlock()
		return v, nil
	case mangos.OptionSendDeadline:
		c.r.Lock()
		v := c.sendExpire
		c.r.Unlock()
		return v, nil
	case mangos.OptionBestEffort:
		c.r.Lock()
		v := c.bestEffort
		c.r.Unlock()
		return v, nil
	}

	return nil, mangos.ErrBadOption
}

func (c *reqCtx) Close() error {
	r := c.r
	c.r.Lock()
	defer c.r.Unlock()
	if c.closed {
		return mangos.ErrClosed
	}
	c.closed = true
	c.cancel()
	delete(r.ctxs, c)
	return nil
}

func (r *req2) GetOption(option string) (interface{}, error) {
	switch option {
	case mangos.OptionRaw:
		return false, nil
	default:
		return r.defCtx.GetOption(option)
	}
}
func (r *req2) SetOption(option string, value interface{}) error {
	// XXX: options for pipes?
	return r.defCtx.SetOption(option, value)
}

func (r *req2) SendMsg(m *mangos.Message) error {
	return r.defCtx.SendMsg(m)
}

func (r *req2) RecvMsg() (*mangos.Message, error) {
	return r.defCtx.RecvMsg()
}

func (r *req2) Close() error {
	r.Lock()
	defer r.Unlock()

	if r.closed {
		return mangos.ErrClosed
	}
	r.closed = true
	for c := range r.ctxs {
		c.closed = true
		c.cancel()
		delete(r.ctxs, c)
	}
	// close and remove each and every pipe
	for id, p := range r.pipes {
		p.ep.Close()
		delete(r.pipes, id)
	}
	return nil
}

func (r *req2) OpenContext() (mangos.ProtocolContext, error) {
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return nil, mangos.ErrClosed
	}
	c := &reqCtx{
		r:          r,
		cond:       sync.NewCond(r),
		bestEffort: r.defCtx.bestEffort,
		resendTime: r.defCtx.resendTime,
		sendExpire: r.defCtx.sendExpire,
		recvExpire: r.defCtx.recvExpire,
	}
	r.ctxs[c] = struct{}{}
	return c, nil
}

func (r *req2) AddPipe(ep mangos.Endpoint) error {
	p := &reqPipe{ep: ep, r: r}
	r.Lock()
	defer r.Unlock()
	if r.closed {
		return mangos.ErrClosed
	}
	r.pipes[ep.GetID()] = p
	r.readyq = append(r.readyq, p)
	r.send()
	go p.receiver()
	return nil
}

func (r *req2) RemovePipe(ep mangos.Endpoint) {
	r.Lock()
	defer r.Unlock()
	p := r.pipes[ep.GetID()]
	if p != nil && p.ep == ep {
		p.closed = true
		ep.Close()
		delete(r.pipes, ep.GetID())
		for c := range r.ctxs {
			if c.lastPipe == p {
				// We are closing this pipe, so we need to
				// immediately reschedule it.
				c.lastPipe = nil
				if m := c.reqMsg; m != nil {
					go c.resendMessage(m)
				}
			}
		}
	}
}

func (*req2) Info() mangos.ProtocolInfo {
	return mangos.ProtocolInfo{
		Self:     mangos.ProtoReq,
		Peer:     mangos.ProtoRep,
		SelfName: "req",
		PeerName: "rep",
	}
}

func newProto() mangos.ProtocolBase {
	r := &req2{
		pipes:   make(map[uint32]*reqPipe),     // maybe sync.Map is better?
		nextID:  uint32(time.Now().UnixNano()), // quasi-random
		ctxs:    make(map[*reqCtx]struct{}),
		ctxByID: make(map[uint32]*reqCtx),
	}
	r.defCtx = &reqCtx{
		r:          r,
		cond:       sync.NewCond(r),
		resendTime: time.Minute,
	}
	return r
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (mangos.Socket, error) {
	return impl.MakeSocket(newProto()), nil
}
