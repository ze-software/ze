//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- Linux unit mode errors.
// Related: devppp_linux.go -- PPPIOCSNPMODE boundary.

package ppp

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// TestSetIPv4ModeFailure proves valid modes reach the ioctl and preserve errno,
// while invalid modes fail before it. Positive kernel state is integration work.
func TestSetIPv4ModeFailure(t *testing.T) {
	for _, mode := range []NPMode{NPModePass, NPModeDrop} {
		if err := SetIPv4Mode(-1, mode); !errors.Is(err, unix.EBADF) {
			t.Fatalf("mode %d lost ioctl error: %v", mode, err)
		}
	}
	for _, mode := range []NPMode{NPModeUnspecified, 255} {
		err := SetIPv4Mode(-1, mode)
		if err == nil {
			t.Fatalf("invalid mode %d admitted", mode)
		}
		if errors.Is(err, unix.EBADF) {
			t.Fatalf("invalid mode %d reached ioctl", mode)
		}
	}
}
