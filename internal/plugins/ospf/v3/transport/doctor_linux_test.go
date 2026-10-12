// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- Linux raw-socket probe tests

package transport

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// probeTree builds the smallest config that makes the doctor check run.
func probeTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("ospfv3")
	return tree
}

// TestCheckRawSocketRefusalKeepsItsCause proves the Linux probe reports why
// the host refused the OSPFv3 raw socket, and gives the CAP_NET_RAW hint only
// for a permission refusal.
//
// VALIDATES: for EPERM and EACCES the doctor warning names CAP_NET_RAW; for
// EAFNOSUPPORT, EPROTONOSUPPORT and EMFILE it does not; every refusal wraps
// the socket error and the warning carries its text; a socket that opens
// gives no warning.
// PREVENTS: one fixed "needs CAP_NET_RAW" text for every errno, which sends
// the operator after a capability the process already holds.
//
// Method: swap the rawSocketOpen seam for one returning each errno, or a real
// descriptor on /dev/null for the success branch, and run the doctor check
// through the default rawSocketProbe.
func TestCheckRawSocketRefusalKeepsItsCause(t *testing.T) {
	old := rawSocketOpen
	t.Cleanup(func() { rawSocketOpen = old })
	cases := []struct {
		errno   unix.Errno
		capHint bool
	}{
		{unix.EPERM, true},
		{unix.EACCES, true},
		{unix.EAFNOSUPPORT, false},
		{unix.EPROTONOSUPPORT, false},
		{unix.EMFILE, false},
	}
	for _, tc := range cases {
		t.Run(unix.ErrnoName(tc.errno), func(t *testing.T) {
			rawSocketOpen = func() (int, error) { return -1, tc.errno }
			if err := checkRawSocket(); !errors.Is(err, tc.errno) {
				t.Fatalf("probe error %v does not wrap %v", err, tc.errno)
			}
			diags := checkOSPFv3RawSocket(diagnostic.DoctorCheckContext{Tree: probeTree()})
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics, want 1", len(diags))
			}
			message := diags[0].Message
			if !strings.Contains(message, tc.errno.Error()) {
				t.Errorf("warning %q does not carry the socket error %q", message, tc.errno.Error())
			}
			if got := strings.Contains(message, "CAP_NET_RAW"); got != tc.capHint {
				t.Errorf("warning %q: CAP_NET_RAW hint present=%v, want %v", message, got, tc.capHint)
			}
		})
	}
	t.Run("opens", func(t *testing.T) {
		rawSocketOpen = func() (int, error) {
			return unix.Open("/dev/null", unix.O_RDONLY|unix.O_CLOEXEC, 0)
		}
		if err := checkRawSocket(); err != nil {
			t.Fatalf("a socket that opens was reported refused: %v", err)
		}
		if diags := checkOSPFv3RawSocket(diagnostic.DoctorCheckContext{Tree: probeTree()}); len(diags) != 0 {
			t.Errorf("a socket that opens produced %d diagnostics", len(diags))
		}
	})
}
