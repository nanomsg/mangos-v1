package mangos

import (
	"bytes"
	"fmt"
	"sync"
)

type logger struct {
	sync.Mutex
	buf bytes.Buffer
}

func (l *logger) Log(a ...interface{}) {
	l.Lock()
	defer l.Unlock()
	l.buf.WriteString(fmt.Sprint(a...))
	l.buf.WriteByte('\n')
}

func (l *logger) Logf(format string, a ...interface{}) {
	l.Lock()
	defer l.Unlock()
	l.buf.WriteString(fmt.Sprintf(format, a...))
	l.buf.WriteByte('\n')
}

func (l *logger) String() string {
	l.Lock()
	defer l.Unlock()
	return l.buf.String()
}

func (l *logger) Clear() {
	l.Lock()
	defer l.Unlock()
	l.buf.Reset()
}
