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

type star struct {
	xstar
}

func (*star) Name() string {
	return StarName
}

func (*star) IsRaw() bool {
	return false
}

type starFactory int

func (starFactory) NewProtocol() Protocol {
	s := &star{}
	s.xstar.redist = true
	return s
}

var StarFactory starFactory
