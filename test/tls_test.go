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
// WITHOUT WARRANTIES O R CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/transport/tlstcp"
)

var rootKey *rsa.PrivateKey
var serverKey *rsa.PrivateKey
var clientKey *rsa.PrivateKey

var tlsTestRootKeyPEM []byte
var tlsTestServerKeyPEM []byte
var tlsTestClientKeyPEM []byte

var tlsTestRootCertDER []byte
var tlsTestServerCertDER []byte
var tlsTestClientCertDER []byte

var tlsTestRootCert *x509.Certificate
var tlsTestServerCert *x509.Certificate
var tlsTestClientCert *x509.Certificate

var tlsTestRootCertPEM []byte
var tlsTestServerCertPEM []byte
var tlsTestClientCertPEM []byte

var rootTmpl = &x509.Certificate{
	SerialNumber: big.NewInt(1),

	Issuer: pkix.Name{
		CommonName:   "issuer.mangos.example.com",
		Organization: []string{"Mangos Issuer Org"},
	},
	Subject: pkix.Name{
		CommonName:   "root.mangos.example.com",
		Organization: []string{"Mangos Root Org"},
	},
	NotBefore:          time.Unix(1000, 0),
	NotAfter:           time.Now().Add(time.Hour),
	IsCA:               true,
	OCSPServer:         []string{"ocsp.mangos.example.com"},
	DNSNames:           []string{"root.mangos.example.com"},
	IPAddresses:        []net.IP{net.ParseIP("127.0.0.1")},
	SignatureAlgorithm: x509.SHA1WithRSA,
	KeyUsage:           x509.KeyUsageCertSign,
	ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
}

var serverTmpl = &x509.Certificate{
	SerialNumber: big.NewInt(2),

	Issuer: pkix.Name{
		CommonName:   "issuer.mangos.example.com",
		Organization: []string{"Mangos Issuer Org"},
	},
	Subject: pkix.Name{
		CommonName:   "server.mangos.example.com",
		Organization: []string{"Mangos Server Org"},
	},
	NotBefore:          time.Unix(1000, 0),
	NotAfter:           time.Now().Add(time.Hour),
	IsCA:               false,
	OCSPServer:         []string{"ocsp.mangos.example.com"},
	DNSNames:           []string{"server.mangos.example.com"},
	IPAddresses:        []net.IP{net.ParseIP("127.0.0.1")},
	SignatureAlgorithm: x509.SHA1WithRSA,
	KeyUsage:           x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
}

var clientTmpl = &x509.Certificate{
	SerialNumber: big.NewInt(3),

	Issuer: pkix.Name{
		CommonName:   "issuer.mangos.example.com",
		Organization: []string{"Mangos Issuer Org"},
	},
	Subject: pkix.Name{
		CommonName:   "client.mangos.example.com",
		Organization: []string{"Mangos Client Org"},
	},
	NotBefore:          time.Unix(1000, 0),
	NotAfter:           time.Now().Add(time.Hour),
	IsCA:               false,
	OCSPServer:         []string{"ocsp.mangos.example.com"},
	DNSNames:           []string{"client.mangos.example.com"},
	IPAddresses:        []net.IP{net.ParseIP("127.0.0.1")},
	SignatureAlgorithm: x509.SHA1WithRSA,
	KeyUsage:           x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
}

var tlslock sync.Mutex

func GenerateKeysAndCerts() (err error) {
	tlslock.Lock()
	defer tlslock.Unlock()
	if rootKey == nil {
		if rootKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			return
		}
	}
	if tlsTestRootKeyPEM == nil {
		tlsTestRootKeyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
		})
	}

	if serverKey == nil {
		if serverKey, err = rsa.GenerateKey(rand.Reader, 1024); err != nil {
			return
		}
	}
	if tlsTestServerKeyPEM == nil {
		tlsTestServerKeyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(serverKey),
		})
	}

	if clientKey == nil {
		if clientKey, err = rsa.GenerateKey(rand.Reader, 1024); err != nil {
			return
		}
	}
	if tlsTestClientKeyPEM == nil {
		tlsTestClientKeyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
		})
	}

	if tlsTestRootCertDER == nil {
		var der_, pem_ []byte
		var cert *x509.Certificate

		der_, err = x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
		if err != nil {
			return
		}
		if cert, err = x509.ParseCertificate(der_); err != nil {
			return
		}
		pem_ = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der_})
		tlsTestRootCertDER = der_
		tlsTestRootCertPEM = pem_
		tlsTestRootCert = cert
	}

	if tlsTestServerCertDER == nil {
		var der_, pem_ []byte
		var cert *x509.Certificate

		der_, err = x509.CreateCertificate(rand.Reader, serverTmpl, tlsTestRootCert, &serverKey.PublicKey, rootKey)
		if err != nil {
			return
		}
		if cert, err = x509.ParseCertificate(der_); err != nil {
			return
		}
		pem_ = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der_})
		tlsTestServerCertDER = der_
		tlsTestServerCertPEM = pem_
		tlsTestServerCert = cert
	}

	if tlsTestClientCertDER == nil {
		var der_, pem_ []byte
		var cert *x509.Certificate

		der_, err = x509.CreateCertificate(rand.Reader, clientTmpl, tlsTestRootCert, &clientKey.PublicKey, rootKey)
		if err != nil {
			return
		}
		if cert, err = x509.ParseCertificate(der_); err != nil {
			return
		}
		pem_ = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der_})
		tlsTestClientCertDER = der_
		tlsTestClientCertPEM = pem_
		tlsTestClientCert = cert
	}

	return nil
}

// Wrap this in a function so we can use it later.
func SetTLSTest(t testing.TB, sock mangos.Socket, server bool) bool {
	if err := GenerateKeysAndCerts(); err != nil {
		t.Errorf("Failed generating keys or certs: %v", err)
		return false
	}
	cfg := new(tls.Config)
	var cert tls.Certificate
	var err error
	if server {
		cert, err = tls.X509KeyPair(tlsTestServerCertPEM, tlsTestServerKeyPEM)
		if err != nil {
			t.Errorf("Failed loading server TLS certificate: %v", err)
			return false
		}
	} else {
		cert, err = tls.X509KeyPair(tlsTestClientCertPEM, tlsTestClientKeyPEM)
		if err != nil {
			t.Errorf("Failed loading client TLS certificate: %v", err)
			return false
		}
	}
	cfg.Certificates = make([]tls.Certificate, 1)
	cfg.Certificates[0] = cert
	cfg.InsecureSkipVerify = true

	err = sock.SetOption(mangos.OptionTLSConfig, cfg)
	if err != nil {
		t.Errorf("Failed setting TLS config: %v", err)
		return false
	}
	return true
}

// Instead of a traditional full transport test using the innards, we will
// test Req/Rep *over* TLS.  Its a simpler test to make.

func TestTLSReqRep(t *testing.T) {
	url := "tls+tcp://127.0.0.1:3737"
	ping := []byte("REQUEST_MESSAGE")
	ack := []byte("RESPONSE_MESSAGE")

	clich := make(chan bool, 1)
	srvch := make(chan bool, 1)

	var srvsock mangos.Socket

	var pass, ok bool

	if err := GenerateKeysAndCerts(); err != nil {
		t.Errorf("Failed to generate keys/certs: %v", err)
		return
	}
	t.Log("Starting server")
	go func() {
		var err error
		var reqm, repm *mangos.Message
		var cfg *tls.Config
		var cert tls.Certificate

		defer close(srvch)
		srvsock, err = rep.NewSocket()
		if err != nil || srvsock == nil {
			t.Errorf("Failed creating server socket: %v", err)
			return
		}
		srvsock.AddTransport(tlstcp.NewTransport())
		// XXX: Closing the server socket too soon causes the
		// underlying connecctions to be closed, which breaks the
		// client.  We really need a shutdown().  For now we just
		// close in the outer handler.
		//defer srvsock.Close()

		cfg = new(tls.Config)
		cert, err = tls.X509KeyPair(tlsTestServerCertPEM, tlsTestServerKeyPEM)
		if err != nil {
			t.Errorf("Failed loading server certificate: %v", err)
			return
		}
		cfg.Certificates = make([]tls.Certificate, 1)
		cfg.Certificates[0] = cert
		cfg.InsecureSkipVerify = true

		err = srvsock.SetOption(mangos.OptionTLSConfig, cfg)
		if err != nil {
			t.Errorf("Failed setting TLS server config: %v", err)
			return
		}

		if err = srvsock.Listen(url); err != nil {
			t.Errorf("Server listen failed: %v", err)
			return
		}
		t.Logf("Server listening")

		if reqm, err = srvsock.RecvMsg(); err != nil {
			t.Errorf("Server receive failed: %v", err)
			return
		}
		t.Logf("Server got message")
		defer reqm.Free()

		if !bytes.Equal(reqm.Body, ping) {
			t.Errorf("Server recd bad message: %v", reqm)
			return
		}

		t.Logf("Server sending reply")
		repm = mangos.NewMessage(len(ack))
		repm.Body = append(repm.Body, ack...)
		if err = srvsock.SendMsg(repm); err != nil {
			t.Errorf("Server send failed: %v", err)
			return
		}

		t.Logf("Server OK")
		// its all good
		srvch <- true
	}()

	// Generally the server takes a little bit of time finish accept()
	// because server key generation takes longer.  Give it some time.
	time.Sleep(time.Millisecond * 500)

	t.Log("Starting client")
	go func() {
		var clisock mangos.Socket
		var err error
		var reqm, repm *mangos.Message
		var cfg *tls.Config
		var cert tls.Certificate

		defer close(clich)
		clisock, err = req.NewSocket()
		if err != nil || clisock == nil {
			t.Errorf("Failed creating client socket: %v", err)
			return
		}
		defer clisock.Close()
		clisock.AddTransport(tlstcp.NewTransport())

		// Load up ROOT CA
		cfg = new(tls.Config)
		cert, err = tls.X509KeyPair(tlsTestClientCertPEM, tlsTestClientKeyPEM)
		if err != nil {
			t.Errorf("Failed loading client certificate: %v", err)
			return
		}
		cfg.Certificates = make([]tls.Certificate, 1)
		cfg.Certificates[0] = cert
		cfg.InsecureSkipVerify = true

		// XXX: Closing the server socket too soon causes the
		// underlying connecctions to be closed, which breaks the
		// client.  We really need a shutdown().  For now we just
		// close in the outer handler.
		//defer srvsock.Close()

		err = clisock.SetOption(mangos.OptionTLSConfig, cfg)
		if err != nil {
			t.Errorf("Failed setting TLS server config: %v", err)
			return
		}

		if err = clisock.Dial(url); err != nil {
			t.Errorf("Client dial failed: %v", err)
			return
		}

		t.Logf("Client dial complete")

		reqm = mangos.NewMessage(len(ping))
		reqm.Body = append(reqm.Body, ping...)
		if err = clisock.SendMsg(reqm); err != nil {
			t.Errorf("Client send failed: %v", err)
			return
		}
		t.Logf("Sent client request")

		if repm, err = clisock.RecvMsg(); err != nil {
			t.Errorf("Client receive failed: %v", err)
			return
		}
		t.Logf("Client received reply")

		if !bytes.Equal(repm.Body, ack) {
			t.Errorf("Client recd bad message: %v", reqm)
			return
		}

		t.Logf("Client OK")

		// its all good
		clich <- true

	}()

	// Check server reported OK
	select {
	case pass, ok = <-srvch:
		if !ok {
			t.Error("Server aborted")
			return
		}
		if !pass {
			t.Error("Server reported failure")
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for server")
		return
	}

	// Check client reported OK
	select {
	case pass, ok = <-clich:
		if !ok {
			t.Error("Client aborted")
			return
		}
		if !pass {
			t.Error("Client reported failure")
			return
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for client")
		return
	}

	srvsock.Close()
}
