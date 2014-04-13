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
	"sync"
)

type resp struct {
	surveyid uint32
	surveyok bool
	xresp
	sync.Mutex
}

func (*resp) Name() string {
	return RespondentName
}

func (*resp) IsRaw() bool {
	return false
}

func (r *resp) RecvHook(msg *Message) bool {
	var err error
	r.Lock()
	defer r.Unlock()

	if r.surveyid, err = msg.getUint32(); err != nil {
		return false
	}
	r.surveyok = true
	return true
}

func (r *resp) SendHook(msg *Message) bool {
	r.Lock()
	defer r.Unlock()
	if !r.surveyok {
		return false
	}
	msg.putUint32(r.surveyid)
	r.surveyok = false
	return true
}

type respFactory int

func (respFactory) NewProtocol() Protocol {
	return &resp{}
}

var RespondentFactory respFactory
