//go:build !linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- non-Linux raw-socket probe test

package transport

import (
	"runtime"
	"strings"
	"testing"
)

// TestRawSocketProbeOffLinuxNamesTheOS proves the probe on a host that is not
// Linux reports the OSPF raw transport unavailable, naming Linux and the
// host's OS, so the doctor check warns instead of reporting healthy.
//
// VALIDATES: checkRawSocket returns an error carrying "Linux-only" and
// runtime.GOOS.
// PREVENTS: the stub answering available while backend_other.go refuses to
// open the transport.
//
// Method: call the platform probe directly.
func TestRawSocketProbeOffLinuxNamesTheOS(t *testing.T) {
	err := checkRawSocket()
	if err == nil {
		t.Fatal("the probe reports the raw transport available on a host that is not Linux")
	}
	for _, want := range []string{"Linux-only", runtime.GOOS} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("reason %q does not carry %q", err, want)
		}
	}
}
