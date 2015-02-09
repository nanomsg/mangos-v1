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

// Package nanomsg is a compatibility wrapper.  It attempts to offer an
// a minimal replacement for the same API as github.com/op/go-nanomsg,
// but does so using the mangos package underneath.  The intent is to to
// facilitate converting existing applications to mangos.
//
// Only the synchronous API is supported -- the Poller/PollItem API is
// not present here.  Applications are encouraged to use Go's native support
// for goroutines and channels to build such features if needed.
//
// New applications should be developed with mangos API directly, rather
// than using this compatibility shim.  Additionally, note that this API
// lacks a number of the performance improvements in the mangos API; very
// specifically it does not support message reuse, which means that a busy
// consumer is going to thrash the garbage collector in Go pretty hard.
//
// Only a subset of the mangos capabilities are exported through this API;
// to get the full feature set (e.g. TLS over TCP) the mangos API should be
// used directly.
package nanomsg

import (
	"errors"
	"time"
)

import (
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/bus"
	"github.com/gdamore/mangos/protocol/pair"
	"github.com/gdamore/mangos/protocol/pub"
	"github.com/gdamore/mangos/protocol/pull"
	"github.com/gdamore/mangos/protocol/push"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/protocol/respondent"
	"github.com/gdamore/mangos/protocol/sub"
	"github.com/gdamore/mangos/protocol/surveyor"
	"github.com/gdamore/mangos/transport/all"
)

// Domain is the socket domain or address family.  We use it to indicate
// either normal or raw mode sockets.
type Domain int

const (
	AF_SP     = Domain(0)
	AF_SP_RAW = Domain(1)
)

// Protocol is the numeric abstraction to the various protocols or patterns
// that Mangos supports.
type Protocol int

const (
	PUSH       = Protocol(mangos.ProtoPush)
	PULL       = Protocol(mangos.ProtoPull)
	PUB        = Protocol(mangos.ProtoPub)
	SUB        = Protocol(mangos.ProtoSub)
	REQ        = Protocol(mangos.ProtoReq)
	REP        = Protocol(mangos.ProtoRep)
	SURVEYOR   = Protocol(mangos.ProtoSurveyor)
	RESPONDENT = Protocol(mangos.ProtoRespondent)
	BUS        = Protocol(mangos.ProtoBus)
	PAIR       = Protocol(mangos.ProtoPair)
)

// DontWait is an (unsupported!) flag option.
const DontWait = 1

var (
	errNotSup    = errors.New("not supported")
	errNoFlag    = errors.New("flags not supported")
	errBadDomain = errors.New("domain invalid or not supported")
)

// Socket is the main connection to the underlying library.
type Socket struct {
	sock  mangos.Socket
	proto Protocol
	dom   Domain
	rto   time.Duration
	sto   time.Duration
}

// Endpoint is a structure that holds the peer address for now.
type Endpoint struct {
	Address string
}

// String just returns the endpoint address for now.
func (ep *Endpoint) String() string {
	return ep.Address
}

// NewSocket allocates a new Socket.  The Socket is the handle used to
// access the underlying library.
func NewSocket(d Domain, p Protocol) (*Socket, error) {

	var s Socket
	var err error

	s.proto = p
	s.dom = d

	switch p {
	case PUB:
		s.sock, err = pub.NewSocket()
	case SUB:
		s.sock, err = sub.NewSocket()
	case PUSH:
		s.sock, err = push.NewSocket()
	case PULL:
		s.sock, err = pull.NewSocket()
	case REQ:
		s.sock, err = req.NewSocket()
	case REP:
		s.sock, err = rep.NewSocket()
	case SURVEYOR:
		s.sock, err = surveyor.NewSocket()
	case RESPONDENT:
		s.sock, err = respondent.NewSocket()
	case PAIR:
		s.sock, err = pair.NewSocket()
	case BUS:
		s.sock, err = bus.NewSocket()
	default:
		err = mangos.ErrBadProto
	}

	if err != nil {
		return nil, err
	}

	switch d {
	case AF_SP:
	case AF_SP_RAW:
		err = s.sock.SetOption(mangos.OptionRaw, true)
	default:
		err = errBadDomain
	}
	if err != nil {
		s.sock.Close()
		return nil, err
	}

	// Compat mode sockets should timeout on send if we don't have any pipes
	if err = s.sock.SetOption(mangos.OptionWriteQLen, 0); err != nil {
		s.sock.Close()
		return nil, err
	}

	s.rto = -1
	s.sto = -1
	all.AddTransports(s.sock)
	return &s, nil
}

// Close shuts down the socket.
func (s *Socket) Close() error {
	if s.sock != nil {
		s.sock.Close()
	}
	return nil
}

// Bind creates sets up to receive incoming connections from remote peers.
// This wraps around mangos' Listen() socket interface.
func (s *Socket) Bind(addr string) (*Endpoint, error) {

	if err := s.sock.Listen(addr); err != nil {
		return nil, err
	}
	return &Endpoint{Address: addr}, nil
}

// Connect establishes (asynchronously) a client side connection
// to a remote peer.  The client will attempt to keep reconnecting.
// This wraps around mangos' Dial() socket inteface.
func (s *Socket) Connect(addr string) (*Endpoint, error) {
	d, err := s.sock.NewDialer(addr, nil)
	if err != nil {
		return nil, err
	}
	if err := d.Dial(); err != nil {
		return nil, err
	}
	return &Endpoint{Address: addr}, nil
}

// Recv receives a message.  For AF_SP_RAW messages the header data will
// be included at he start of the returned byte slice (otherwise it will
// be stripped).  At this time no flags are supported.
func (s *Socket) Recv(flags int) ([]byte, error) {
	var b []byte
	var m *mangos.Message
	var err error

	if flags != 0 {
		return nil, errNoFlag
	}

	// Legacy nanomsg uses the opposite semantic for negative and
	// zero values than mangos.  A bit unfortunate.
	switch {
	case s.rto > 0:
		s.sock.SetOption(mangos.OptionRecvDeadline, s.rto)
	case s.rto == 0:
		s.sock.SetOption(mangos.OptionRecvDeadline, -1)
	case s.rto < 0:
		s.sock.SetOption(mangos.OptionRecvDeadline, 0)
	}

	if m, err = s.sock.RecvMsg(); err != nil {
		return nil, err
	}

	if s.dom == AF_SP_RAW {
		b = make([]byte, 0, len(m.Body)+len(m.Header))
		b = append(b, m.Header...)
		b = append(b, m.Body...)
	} else {
		b = make([]byte, 0, len(m.Body))
		b = append(b, m.Body...)
	}
	m.Free()
	return b, nil
}

// Send sends a message.  For AF_SP_RAW messages the header must be
// included in the argument.  At this time, no flags are supported.
func (s *Socket) Send(b []byte, flags int) (int, error) {

	if flags != 0 {
		return -1, errNoFlag
	}

	m := mangos.NewMessage(len(b))
	m.Body = append(m.Body, b...)

	// Legacy nanomsg uses the opposite semantic for negative and
	// zero values than mangos.  A bit unfortunate.
	switch {
	case s.sto > 0:
		s.sock.SetOption(mangos.OptionSendDeadline, s.sto)
	case s.sto == 0:
		s.sock.SetOption(mangos.OptionSendDeadline, -1)
	case s.sto < 0:
		s.sock.SetOption(mangos.OptionSendDeadline, 0)
	}

	return len(b), s.sock.SendMsg(m)
}

// Protocol returns the numeric value of the sockets protocol, such as
// REQ, REP, SUB, PUB, etc.
func (s *Socket) Protocol() (Protocol, error) {
	return s.proto, nil
}

// Domain returns the socket domain, either AF_SP or AF_SP_RAW.
func (s *Socket) Domain() (Domain, error) {
	return s.dom, nil
}

// RecvFd is not supported.
func (s *Socket) RecvFd() (uintptr, error) {
	return 0, errNotSup
}

// SendFd is not supported.
func (s *Socket) SendFd() (uintptr, error) {
	return 0, errNotSup
}

// SendPrio is intended to set send priorities.  Mangos does not support
// send priorities at present.
func (s *Socket) SendPrio() (int, error) {
	return 0, errNotSup
}

// SetSendPrio is not supported.
func (s *Socket) SetSendPrio(int) error {
	return errNotSup
}

// Linger should set the TCP linger time, but at present is not supported.
func (s *Socket) Linger() (time.Duration, error) {
	var t time.Duration
	return t, errNotSup
}

// SetLinger is not supported.
func (s *Socket) SetLinger(time.Duration) error {
	return errNotSup
}

// SendTimeout retrieves the send timeout.  Negative values indicate
// an infinite timeout.
func (s *Socket) SendTimeout() (time.Duration, error) {
	return s.sto, nil
}

// SetSendTimeout sets the send timeout.  Negative values indicate
// an infinite timeout.  The Send() operation will return an error if
// a message cannot be sent within this time.
func (s *Socket) SetSendTimeout(d time.Duration) error {
	s.sto = d
	return nil
}

// RecvTimeout retrieves the receive timeout.  Negative values indicate
// an infinite timeout.
func (s *Socket) RecvTimeout() (time.Duration, error) {
	return s.rto, nil
}

// SetRecvTimeout sets a timeout for receive operations.  The Recv()
// function will return an error if no message is received within this time.
func (s *Socket) SetRecvTimeout(d time.Duration) error {
	s.rto = d
	return nil
}

// Shutdown should shut down a particular endpoint.  Mangos lacks the
// underlying functionality to support this at present.
func (s *Socket) Shutdown(*Endpoint) error {
	return errNotSup
}

// BusSocket is a socket associated with the BUS protocol.
type BusSocket struct {
	*Socket
}

// NewBusSocket creates a BUS socket.
func NewBusSocket() (*BusSocket, error) {
	s, err := NewSocket(AF_SP, BUS)
	return &BusSocket{s}, err
}

// PairSocket is a socket associated with the PAIR protocol.
type PairSocket struct {
	*Socket
}

// NewPairSocket creates a PAIR socket.
func NewPairSocket() (*PairSocket, error) {
	s, err := NewSocket(AF_SP, PAIR)
	return &PairSocket{s}, err
}

// PubSocket is a socket associated with the PUB protocol.
type PubSocket struct {
	*Socket
}

// NewPubSocket creates a PUB socket.
func NewPubSocket() (*PubSocket, error) {
	s, err := NewSocket(AF_SP, PUB)
	return &PubSocket{s}, err
}

// PullSocket is a socket associated with the PULL protocol.
type PullSocket struct {
	*Socket
}

// NewPullSocket creates a PULL socket.
func NewPullSocket() (*PullSocket, error) {
	s, err := NewSocket(AF_SP, PULL)
	return &PullSocket{s}, err
}

// PushSocket is a socket associated with the PUSH protocol.
type PushSocket struct {
	*Socket
}

// NewPushSocket creates a PUSH socket.
func NewPushSocket() (*PushSocket, error) {
	s, err := NewSocket(AF_SP, PUSH)
	return &PushSocket{s}, err
}

// RepSocket is a socket associated with the REP protocol.
type RepSocket struct {
	*Socket
}

// NewRepSocket creates a REP socket.
func NewRepSocket() (*RepSocket, error) {
	s, err := NewSocket(AF_SP, REP)
	return &RepSocket{s}, err
}

// ReqSocket is a socket associated with the REQ protocol.
type ReqSocket struct {
	*Socket
}

// NewReqSocket creates a REQ socket.
func NewReqSocket() (*ReqSocket, error) {
	s, err := NewSocket(AF_SP, REQ)
	return &ReqSocket{s}, err
}

// RespondentSocket is a socket associated with the RESPONDENT protocol.
type RespondentSocket struct {
	*Socket
}

// NewRespondentSocket creates a RESPONDENT socket.
func NewRespondentSocket() (*RespondentSocket, error) {
	s, err := NewSocket(AF_SP, RESPONDENT)
	return &RespondentSocket{s}, err
}

// SubSocket is a socket associated with the SUB protocol.
type SubSocket struct {
	*Socket
}

// Subscribe registers interest in a topic.
func (s *SubSocket) Subscribe(topic string) error {
	return s.sock.SetOption(mangos.OptionSubscribe, topic)
}

// Unsubscribe unregisters interest in a topic.
func (s *SubSocket) Unsubscribe(topic string) error {
	return s.sock.SetOption(mangos.OptionUnsubscribe, topic)
}

// NewSubSocket creates a SUB socket.
func NewSubSocket() (*SubSocket, error) {
	s, err := NewSocket(AF_SP, SUB)
	return &SubSocket{s}, err
}

// SurveyorSocket is a socket associated with the SURVEYOR protocol.
type SurveyorSocket struct {
	*Socket
}

// Deadline returns the survey deadline on the socket.  After this time,
// responses from a survey will be discarded.
func (s *SurveyorSocket) Deadline() (time.Duration, error) {
	var d time.Duration
	v, err := s.sock.GetOption(mangos.OptionSurveyTime)
	if err == nil {
		d = v.(time.Duration)
	}
	return d, err
}

// SetDeadline sets the survey deadline on the socket.  After this time,
// responses from a survey will be discarded.
func (s *SurveyorSocket) SetDeadline(d time.Duration) error {
	return s.sock.SetOption(mangos.OptionSurveyTime, d)
}

// NewSurveyorSocket creates a SURVEYOR socket.
func NewSurveyorSocket() (*SurveyorSocket, error) {
	s, err := NewSocket(AF_SP, SURVEYOR)
	return &SurveyorSocket{s}, err
}
