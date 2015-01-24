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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mangos

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// mkTimer creates a timer based upon a duration.  If however
// a zero valued duration is passed, then a nil channel is passed
// i.e. never selectable.  This allows the output to be readily used
// with deadlines in network connections, etc.
func mkTimer(deadline time.Duration) <-chan time.Time {

	if deadline == 0 {
		return nil
	}

	return time.After(deadline)
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
