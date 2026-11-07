// Design: docs/guide/appliance.md -- the kernel host name follows system host on the appliance
// Related: main_system_hostname.go -- reconcileKernelHostname, driven through this fake

package hub

import (
	"bytes"
	"log/slog"
	"strings"
)

// fakeKernel is a kernel host name a test owns: get answers name, set records
// each call and, unless fail is set, changes name.
type fakeKernel struct {
	name  string
	fail  error
	gets  int
	calls []string
}

func (k *fakeKernel) ops() kernelHostnameOps {
	return kernelHostnameOps{
		get: func() (string, error) { k.gets++; return k.name, nil },
		set: func(name string) error {
			k.calls = append(k.calls, name)
			if k.fail != nil {
				return k.fail
			}
			k.name = name
			return nil
		},
	}
}

// captureLog answers a logger and the buffer its text lines go to.
func captureLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

// warnLines answers the WARN lines of a captured log.
func warnLines(buf *bytes.Buffer) []string {
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if strings.Contains(line, "level=WARN") {
			out = append(out, line)
		}
	}
	return out
}
