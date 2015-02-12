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

package test

import (
	"bytes"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
)

type TranTest struct {
	addr     string
	t        *testing.T
	tran     mangos.Transport
	cliCfg   *tls.Config
	srvCfg   *tls.Config
	protoRep mangos.Protocol
	protoReq mangos.Protocol
}

func NewTranTest(tran mangos.Transport, addr string) *TranTest {
	tt := &TranTest{addr: addr, tran: tran}
	if strings.HasPrefix(tt.addr, "tls+tcp://") || strings.HasPrefix(tt.addr, "wss://") {
		tt.cliCfg, _ = GetTlsConfig(false)
		tt.srvCfg, _ = GetTlsConfig(true)
	}
	tt.protoRep = rep.NewProtocol()
	tt.protoReq = req.NewProtocol()
	return tt
}

func (tt *TranTest) TranTestListenAndAccept(t *testing.T) {
	t.Logf("Establishing listener for %s", tt.addr)
	l, err := tt.tran.NewListener(tt.addr, tt.protoRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l.Close()
	if tt.srvCfg != nil {
		if err = l.SetOption(mangos.OptionTlsConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		t.Logf("Connecting on %s", tt.addr)
		d, err := tt.tran.NewDialer(tt.addr, tt.protoReq)
		if err != nil {
			t.Errorf("NewDialer failed: %v", err)
			return
		}
		if tt.cliCfg != nil {
			if err = d.SetOption(mangos.OptionTlsConfig, tt.cliCfg); err != nil {
				t.Errorf("Failed setting TLS config: %v", err)
				return
			}
		}
		client, err := d.Dial()
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
		}
		if v, err := client.GetProp(mangos.PropLocalAddr); err == nil {
			addr := v.(net.Addr)
			t.Logf("Dialed on local net %s addr %s", addr.Network(), addr.String())
		} else {
			t.Logf("err is ", err.Error())
		}
		if v, err := client.GetProp(mangos.PropRemoteAddr); err == nil {
			addr := v.(net.Addr)
			t.Logf("Dialed remote peer %s addr %s", addr.Network(), addr.String())
		}
		t.Logf("Connected client: %d (server %d)",
			client.LocalProtocol(), client.RemoteProtocol())
		t.Logf("Client open: %t", client.IsOpen())
		if !client.IsOpen() {
			t.Error("Client is closed")
			return
		}
	}()

	server, err := l.Accept()
	if err != nil {
		t.Errorf("Accept failed: %v", err)
		return
	}
	if v, err := server.GetProp(mangos.PropLocalAddr); err == nil {
		addr := v.(net.Addr)
		t.Logf("Accepted on local net %s addr %s", addr.Network(), addr.String())
	}
	if v, err := server.GetProp(mangos.PropRemoteAddr); err == nil {
		addr := v.(net.Addr)
		t.Logf("Accepted remote peer %s addr %s", addr.Network(), addr.String())
	}
	defer server.Close()

	t.Logf("Connected server: %d (client %d)",
		server.LocalProtocol(), server.RemoteProtocol())
	t.Logf("Server open: %t", server.IsOpen())
	if !server.IsOpen() {
		t.Error("Server is closed")
	}
	wg.Wait()
}

func (tt *TranTest) TranTestDuplicateListen(t *testing.T) {
	var err error
	t.Logf("Testing Duplicate Listen on %s", tt.addr)
	l1, err := tt.tran.NewListener(tt.addr, tt.protoRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l1.Close()
	if tt.srvCfg != nil {
		if err = l1.SetOption(mangos.OptionTlsConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l1.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	l2, err := tt.tran.NewListener(tt.addr, tt.protoReq)
	if err != nil {
		t.Errorf("NewListener faield: %v", err)
		return
	}
	defer l2.Close()
	if tt.srvCfg != nil {
		if err = l2.SetOption(mangos.OptionTlsConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l2.Listen(); err == nil {
		t.Errorf("Duplicate listen should not be permitted!")
		return
	}
	t.Logf("Got expected error: %v", err)
}

func (tt *TranTest) TranTestConnRefused(t *testing.T) {
	d, err := tt.tran.NewDialer(tt.addr, tt.protoReq)
	if err != nil || d == nil {
		t.Errorf("New Dialer failed: %v", err)
	}
	if tt.cliCfg != nil {
		if err = d.SetOption(mangos.OptionTlsConfig, tt.cliCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	c, err := d.Dial()
	if err == nil || c != nil {
		t.Errorf("Connection not refused (%s)!", tt.addr)
		return
	}
	t.Logf("Got expected error: %v", err)
}

func (tt *TranTest) TranTestSendRecv(t *testing.T) {
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	ch := make(chan *mangos.Message)

	t.Logf("Establishing REP listener on %s", tt.addr)
	l, err := tt.tran.NewListener(tt.addr, tt.protoRep)
	if err != nil {
		t.Errorf("NewListener failed: %v", err)
		return
	}
	defer l.Close()
	if tt.srvCfg != nil {
		if err = l.SetOption(mangos.OptionTlsConfig, tt.srvCfg); err != nil {
			t.Errorf("Failed setting TLS config: %v", err)
			return
		}
	}
	if err = l.Listen(); err != nil {
		t.Errorf("Listen failed: %v", err)
		return
	}

	go func() {
		defer close(ch)

		// Client side
		t.Logf("Connecting REQ on %s", tt.addr)
		d, err := tt.tran.NewDialer(tt.addr, tt.protoReq)
		if tt.cliCfg != nil {
			if err = d.SetOption(mangos.OptionTlsConfig, tt.cliCfg); err != nil {
				t.Errorf("Failed setting TLS config: %v", err)
				return
			}
		}

		client, err := d.Dial()
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
		}
		t.Logf("Connected client: %t", client.IsOpen())
		defer client.Close()

		req := mangos.NewMessage(len(ping))
		req.Body = append(req.Body, ping...)

		// Now try to send data
		t.Logf("Sending %d bytes", len(req.Body))

		err = client.Send(req)
		if err != nil {
			t.Errorf("Client send error: %v", err)
			return
		}
		t.Logf("Client sent")

		rep, err := client.Recv()
		if err != nil {
			t.Errorf("Client receive error: %v", err)
			return
		}

		if !bytes.Equal(rep.Body, ack) {
			t.Errorf("Reply mismatch: %v, %v", rep.Body, ack)
			return
		}
		if len(rep.Header) != 0 {
			t.Errorf("Client reply non-empty header: %v",
				rep.Header)
			return
		}
		select {
		case ch <- rep:
			t.Log("Client reply forwarded")
		case <-time.After(5 * time.Second): // 5 secs should be plenty
			t.Error("Client timeout forwarding reply")
			return
		}
	}()

	server, err := l.Accept()
	if err != nil {
		t.Errorf("Accept failed: %v", err)
		return
	}
	t.Logf("Connected server: %t", server.IsOpen())
	defer server.Close()

	// Now we can try to send and receive
	req, err := server.Recv()
	if err != nil {
		t.Errorf("Server receive error: %v", err)
		return
	}
	t.Logf("Server received %d bytes", len(req.Body))
	if !bytes.Equal(req.Body, ping) {
		t.Errorf("Request mismatch: %v, %v", req.Body, ping)
		return
	}

	if len(req.Header) != 0 {
		t.Errorf("Server request non-empty header: %v", req.Header)
		return
	}

	// Now reply
	rep := mangos.NewMessage(len(ack))
	rep.Body = append(rep.Body, ack...)

	t.Logf("Server sending %d bytes", len(rep.Body))

	err = server.Send(rep)
	if err != nil {
		t.Errorf("Server send error: %v", err)
		return
	}
	t.Log("Server reply sent")

	// Wait for client to ack reply over back channel.
	select {
	case nrep := <-ch:
		if !bytes.Equal(nrep.Body, ack) {
			t.Errorf("Client forward mismatch: %v, %v", ack, rep)
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Client timeout?")
		return
	}
}
