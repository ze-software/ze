// Design: plan/learned/1024-installer-initrd-pure-go.md -- multi-console fan-out writer

//go:build linux && ze_installer

package disk

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type writerEntry struct {
	w io.Writer
}

type consoleWriter struct {
	mu      sync.Mutex
	writers []writerEntry
}

func (cw *consoleWriter) Write(p []byte) (int, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	for _, e := range cw.writers {
		e.w.Write(p) //nolint:errcheck // best-effort fan-out; a dead console must not block the others
	}
	return len(p), nil
}

func setupConsoles() *consoleWriter {
	data, err := os.ReadFile("/sys/class/tty/console/active")
	if err != nil {
		return &consoleWriter{writers: []writerEntry{{w: os.Stdout}}}
	}

	var tb textbuf.Buffer
	var writers []writerEntry
	for name := range strings.FieldsSeq(strings.TrimSpace(string(data))) {
		path := tb.Reset().Str("/dev/").Str(name).String()
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			continue
		}
		writers = append(writers, writerEntry{w: f})
	}

	if len(writers) == 0 {
		writers = append(writers, writerEntry{w: os.Stdout})
	}
	return &consoleWriter{writers: writers}
}
