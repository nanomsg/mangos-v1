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
	"encoding/binary"
	"fmt"
	"runtime"
	"strings"
	"time"
)

func putUint64(b []byte, v uint64) {
	binary.BigEndian.PutUint64(b, v)
}

func getUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

func putUint32(b []byte, v uint32) {
	binary.BigEndian.PutUint32(b, v)
}

func getUint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}

func appendUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v)>>8, byte(v))
}

// mkTimer creates a timer based upon an absolute time.  If however
// a zero valued time is passed, then then a nil channel is passed
// i.e. never selectable.  This allows the output to be readily used
// with deadlines in network connections, etc.
func mkTimer(deadline time.Time) <-chan time.Time {

	if deadline.IsZero() {
		return nil
	}

	dur := deadline.Sub(time.Now())
	if dur < 0 {
		// a closed channel never blocks
		tm := make(chan time.Time)
		close(tm)
		return tm
	}

	return time.After(dur)
}

var debug = true

func debugf(format string, args ...interface{}) {
	if debug {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "<?>"
			line = 0
		} else {
			if i := strings.LastIndex(file, "/"); i >= 0 {
				file = file[i+1:]
			}
		}
		fmt.Printf("DEBUG: %s:%d [%s]: %s\n", file, line,
			time.Now().String(), fmt.Sprintf(format, args...))
	}
}
