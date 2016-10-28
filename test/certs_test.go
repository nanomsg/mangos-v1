// Copyright 2016 The Mangos Authors
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

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewKeys(t *testing.T) {
	Convey("With a new keys", t, func() {
		keys, err := newKeys()
		So(err, ShouldBeNil)
		So(keys, ShouldNotBeNil)

		Convey("Root signature should check", func() {
			err = keys.root.cert.CheckSignatureFrom(keys.root.cert)
			So(err, ShouldBeNil)
		})

		Convey("Server signature should check", func() {
			err = keys.server.cert.CheckSignatureFrom(keys.root.cert)
			So(err, ShouldBeNil)
		})

		Convey("Client signature should check", func() {
			err = keys.client.cert.CheckSignatureFrom(keys.root.cert)
			So(err, ShouldBeNil)
		})

		Convey("Client cert does not check root", func() {
			err = keys.root.cert.CheckSignatureFrom(keys.client.cert)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestNewTLSConfig(t *testing.T) {

	Convey("We can generate a new TLS config", t, func() {
		cfg, err := NewTLSConfig(true)
		So(err, ShouldBeNil)
		So(cfg, ShouldNotBeNil)
	})
}
