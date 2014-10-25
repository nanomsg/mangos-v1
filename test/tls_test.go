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
// WITHOUT WARRANTIES O R CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/rep"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/transport/tlstcp"
	"bytes"
	"crypto/tls"
	"testing"
	"time"
)

// Certificates & Keys for Testing use ONLY.  These are just
// simple self signed.  Not suitable for real use.

var tlsTestServerKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIICXgIBAAKBgQDXW+SaVRSk5he6jr4ksTimYKWwa8Xn+v2GK8u4zNi6rNV/HTXM
YrRBUcX/HwOAyUbPYKWhJU/KuMad0uuFpG64FvoMlWmIsX8nultqGCBY7b6oltIV
RC479wwWv8+Y1yl4QBZI1u8UgJZ3kQgAd7UWovUtSU6h9152rO4ru0aLMwIDAQAB
AoGBAICozJULOU8ei5SPzLb9DXwQh0wzxsNFpoquxYG9F8nGkbHkUIyvd0MCyIgX
Di+1j9E6yxjPwrC43SfSp5Rq3R2SyV+KI2zGnvCrUQgNs3vB7MvchSNHpJKLxQJW
TiWrSVdwJx5J4ISzB9ynpVKsPiOTcKjMsu//+7TOk0Cl427pAkEA+RZ0UpQSC5sx
lv6anRMfRlQ+185kfEzATVlhno51ifdTGUrm25UPC9RrhQYrrWHMLmyn54b81d92
g5A9RWTvfwJBAN1V02/vskCSjkyfQ87Z+vlEVaNqZdn/83VJ6+7itbHUQTfHGOAd
HKNJ2k46joZBOEFB2GelhdhJdxTDjPb2fk0CQF7mcDEaGvnzCeS2Yh/gLjU0WbEN
AHnfIBEYMbogGqYS5cUoJWaZlt7x8nj/DdsD/K/fU+VBJ8kwV03uwXlT6G8CQQCk
xCZxVruYjEE4Uvt0ehr2AuPJkgQeRAZl0tC69bQSnJKsRh+Dfsh52hmUUM0WrmiF
U9IYXkUEHLR0FZrToe2lAkEA3cFvC+tK4q7ElcGIxIMFRMcDcjf8SgAwjRQ/cZ/0
sGzCwGV3VljIguImpl1S0Sbcw+VDce8QmV8AvCEMYy9Qzw==
-----END RSA PRIVATE KEY-----
`

var tlsTestServerCertPEM = `-----BEGIN CERTIFICATE-----
MIIC2zCCAkSgAwIBAgIJAM7DRUne6nfZMA0GCSqGSIb3DQEBBQUAMFMxCzAJBgNV
BAYTAlVTMRYwFAYDVQQIEw1TdGF0ZSBvZiBNaW5kMRIwEAYDVQQHEwlIYXBwaW5l
c3MxGDAWBgNVBAoTD0ZlZWwgR29vZCwgSW5jLjAeFw0xNDAzMjUwMzU4MTFaFw0z
NDAzMjAwMzU4MTFaMFMxCzAJBgNVBAYTAlVTMRYwFAYDVQQIEw1TdGF0ZSBvZiBN
aW5kMRIwEAYDVQQHEwlIYXBwaW5lc3MxGDAWBgNVBAoTD0ZlZWwgR29vZCwgSW5j
LjCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA11vkmlUUpOYXuo6+JLE4pmCl
sGvF5/r9hivLuMzYuqzVfx01zGK0QVHF/x8DgMlGz2CloSVPyrjGndLrhaRuuBb6
DJVpiLF/J7pbahggWO2+qJbSFUQuO/cMFr/PmNcpeEAWSNbvFICWd5EIAHe1FqL1
LUlOofdedqzuK7tGizMCAwEAAaOBtjCBszAdBgNVHQ4EFgQU3mEJlXXC9yyCewHa
226sMLxFxcowgYMGA1UdIwR8MHqAFN5hCZV1wvcsgnsB2tturDC8RcXKoVekVTBT
MQswCQYDVQQGEwJVUzEWMBQGA1UECBMNU3RhdGUgb2YgTWluZDESMBAGA1UEBxMJ
SGFwcGluZXNzMRgwFgYDVQQKEw9GZWVsIEdvb2QsIEluYy6CCQDOw0VJ3up32TAM
BgNVHRMEBTADAQH/MA0GCSqGSIb3DQEBBQUAA4GBAA0gbvJu4eeXsHjuuFbYJJnM
EQgzKx3SIgXE79LOL3r9SJbhke0zRnOm5wk6vwUMf/5X6Y2bfxOtC1ApIlX6GBUd
BM2pgQooI69Oj69/4s1ouoMXFDb06DWCYA1qh2BECkZspHL7f7e7NIN93wvIFnjm
w18UjqzUqsn9QusTf6rP
-----END CERTIFICATE-----
`

var tlsTestClientKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgQCpbrtgC9TWzBB4Pg1xg69HD56NaB/rH5DOhSI8yqeX4Tse8Qdm
qXGp5MoQdl0RxQPZiM5KAtp7H2LOFEN32OMSGf6jpzfH6Yiz/KxV510JecA6yVyx
GsrJX28Gl/cTvufRR5hu8x+U9XN0MtSp1Hb16EMGq1xL7w1dsBeujocPKQIDAQAB
AoGAZyc/dO4/GrcKn+pHjQC7Sew8f6MRK7kAFHwBqDlJZ7J8qA3ej6ZByUm9q+Ak
MZldCqe70FuEYMlvAkBcAy9Mrs6vZhnguMsle4lfkZIT57ic2SXLUBGuw7KOwlBi
6kjgyV/sVVpsI7g5L8/qFVMfhbuUZA9HOrTZGQjPFeD/CHECQQDbMTUxNyRTbP5C
MR0lVWJIT3R988jX5XAbLB36rm8ILwW1dmkq/HYVTAPdPsg1G1rOWZDiVkx2UC8L
SvMwvCXTAkEAxeJrczZwiot1x9uspY8SQrwJSWewaOLm2OsHares0zB6d7AXWBPw
ePPhGHeJ0VINpWig2hGpkrzwa+1cBgPtkwJAVjFMdHT1kOS8OvUrO+IOitbLvTef
E97CLb00cL4lJTewbAILKv8pxAgnQNoOSmveUmSAB7Dd0myHg05OwSxLRwJAZWYG
RT1KIdQggE7SgutzIfsUjyawwK40OEcGv+pqhrU6rAXxkFJ3UKM3XsAyQK5ZC783
XUbbq7NhRwyTsQlmPQJARL0NloUEG+M83R60DxPLoC8xjBUeH0TKGvI3zlltCm0h
KlJJX0BpQr80rVl62AT7+Pzy4gMy7mGnvm2QI5HQkg==
-----END RSA PRIVATE KEY-----
`

var tlsTestClientCertPEM = `-----BEGIN CERTIFICATE-----
MIICvDCCAiWgAwIBAgIJAKG00YYTtIcFMA0GCSqGSIb3DQEBBQUAMEkxCzAJBgNV
BAYTAlVTMREwDwYDVQQIEwhJbnNhbml0eTEPMA0GA1UEBxMGQXN5bHVtMRYwFAYD
VQQKEw1GcmVha3MgQXJlIFVzMB4XDTE0MDMyNTA0MzM0MFoXDTM0MDMyMDA0MzM0
MFowSTELMAkGA1UEBhMCVVMxETAPBgNVBAgTCEluc2FuaXR5MQ8wDQYDVQQHEwZB
c3lsdW0xFjAUBgNVBAoTDUZyZWFrcyBBcmUgVXMwgZ8wDQYJKoZIhvcNAQEBBQAD
gY0AMIGJAoGBAKluu2AL1NbMEHg+DXGDr0cPno1oH+sfkM6FIjzKp5fhOx7xB2ap
cankyhB2XRHFA9mIzkoC2nsfYs4UQ3fY4xIZ/qOnN8fpiLP8rFXnXQl5wDrJXLEa
yslfbwaX9xO+59FHmG7zH5T1c3Qy1KnUdvXoQwarXEvvDV2wF66Ohw8pAgMBAAGj
gaswgagwHQYDVR0OBBYEFAhSBOpvwg6eeyAdVVlAywek5QxhMHkGA1UdIwRyMHCA
FAhSBOpvwg6eeyAdVVlAywek5QxhoU2kSzBJMQswCQYDVQQGEwJVUzERMA8GA1UE
CBMISW5zYW5pdHkxDzANBgNVBAcTBkFzeWx1bTEWMBQGA1UEChMNRnJlYWtzIEFy
ZSBVc4IJAKG00YYTtIcFMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADgYEA
B0Dguox9iHjouoRLuZfdMQA+HT1BCK7bbMo/z9tRJbwVAAwTkcVholivIJmlz5iS
hVMvNUpq/dBUhdmeUfwu19ZxAYeeEkqgHMazyVpVCP0fdgVZ1ZGbfr7GIWf2oXYa
gYiuigwixOW006p90YD+k9NPvro1usEMwEUiqatuYIE=
-----END CERTIFICATE-----
`

// Wrap this in a function so we can use it later.
func SetTLSTest(t testing.TB, sock mangos.Socket) bool {
	cfg := new(tls.Config)
	cert, err := tls.X509KeyPair([]byte(tlsTestClientCertPEM),
		[]byte(tlsTestClientKeyPEM))
	if err != nil {
		t.Errorf("Failed loading TLS certificate: %v", err)
		return false
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
		cert, err = tls.X509KeyPair([]byte(tlsTestServerCertPEM),
			[]byte(tlsTestServerKeyPEM))
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
		cert, err = tls.X509KeyPair([]byte(tlsTestClientCertPEM),
			[]byte(tlsTestClientKeyPEM))
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
