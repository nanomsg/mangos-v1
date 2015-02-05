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
	"testing"
)

func TestNewKeys(t *testing.T) {
	t.Logf("Starting key generation")
	keys, err := newKeys()
	if err != nil {
		t.Errorf("Failed key generation: %v", err)
		return
	}
	if keys == nil {
		t.Errorf("Nil keys")
		return
	}
	t.Logf("Keys Generated")
	t.Logf("Checking root signature")
	if err = keys.root.cert.CheckSignatureFrom(keys.root.cert); err != nil {
		t.Errorf("Invalid sig: %v", err)
		//return
	}
	t.Logf("Checking server signature")
	if err = keys.server.cert.CheckSignatureFrom(keys.root.cert); err != nil {
		t.Errorf("Invalid sig: %v", err)
		//return
	}
	t.Logf("Checking client signature")
	if err = keys.client.cert.CheckSignatureFrom(keys.root.cert); err != nil {
		t.Errorf("Invalid sig: %v", err)
		//return
	}
	t.Logf("Checking negative signature")
	if err = keys.root.cert.CheckSignatureFrom(keys.client.cert); err == nil {
		t.Errorf("Whoops! Negative signature test failure")
	}
	t.Logf("Negative test: Got expected error: %v", err)
}

func TestNewTLSConfig(t *testing.T) {
	t.Logf("Creating TLS config")
	cfg, err := NewTlsConfig(true)
	if err != nil {
		t.Errorf("Failed generation: %v", err)
		return
	}
	if cfg == nil {
		t.Errorf("Nil config!")
		return
	}
	t.Logf("Done")
}
