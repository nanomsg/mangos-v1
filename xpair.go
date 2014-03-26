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

// xrep is an implementation of the XREP Protocol.
type xpair struct {
	sock   ProtocolSocket
	key    PipeKey
	sndmsg *Message // Pending message for outbound delivery
	rcvmsg *Message // Pending message for inbound delivery
}

// Init implements the Protocol Init method.
func (p *xpair) Init(sock ProtocolSocket) {
	p.sock = sock
}

// Process implements the Protocol Process method.
// For XPair/Pair, we try hard to avoid dropping messages.  It still isn't
// perfect, because the message can be dropped if the pipe we were using for
// it disconnects after accepting the pipe.  But without an acknowledgement,
// this is the best we can do.
func (p *xpair) Process() {

	sock := p.sock

	// Generally we only have one connection alive at a time.  We
	// reject all others.
	if !sock.IsOpen(p.key) {
		sock.ClosePipe(p.key)
		p.key = 0
	}
	pipes := sock.OpenPipes()
	if p.key != 0 {
		// Close any other open pipes.  (Too bad we negotiated the
		// SP layer already, but ... good bye.)
		for i := 0; i < len(pipes); i++ {
			if pipes[i] != p.key {
				sock.ClosePipe(pipes[i])
			}
		}
	} else {
		// Select the the first Pipe
		pipes := sock.OpenPipes()
		if len(pipes) > 0 {
			p.key = pipes[0]
		}
	}

	if p.key != 0 && p.sndmsg == nil {
		p.sndmsg = sock.PullDown()
	}

	if p.key != 0 && p.sndmsg != nil {
		switch sock.SendToPipe(p.sndmsg, p.key) {
		case nil:
			// sent it
			p.sndmsg = nil
		case ErrPipeFull:
			// just save it for later delivery, when backpressure
			// eases.
		default:
			// we had some other worse error
			sock.ClosePipe(p.key)
			p.key = 0
		}
	}

	if p.rcvmsg == nil {
		// Get a new message if one is available.  Discard
		// any that are not from our expected peer.
		msg, key, err := sock.RecvAnyPipe()
		if msg != nil && err == nil && key == p.key {
			p.rcvmsg = msg
		}
	}

	if p.rcvmsg != nil && sock.PushUp(p.rcvmsg) {
		// Sent it up!
		p.rcvmsg = nil
	}
}

// Name implements the Protocol Name method.  It returns "XRep".
func (*xpair) Name() string {
	return XPairName
}

// Number implements the Protocol Number method.
func (*xpair) Number() uint16 {
	return ProtoPair
}

// IsRaw implements the Protocol IsRaw method.
func (*xpair) IsRaw() bool {
	return true
}

// ValidPeer implements the Protocol ValidPeer method.
func (*xpair) ValidPeer(peer uint16) bool {
	if peer == ProtoPair {
		return true
	}
	return false
}

type xpairFactory int

func (xpairFactory) NewProtocol() Protocol {
	return new(xpair)
}

// XPairFactory implements the Protocol Factory for the XPAIR protocol.
// The XPAIR Protocol is the raw form of the PAIR (Pair) protocol.
var XPairFactory xpairFactory
