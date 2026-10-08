//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- Path MTU and label overhead
// Related: mplsentry_linux.go -- addMPLSSwap, transitRouteMTU
//
// The transit route's MTU follows the kernel's answer, with the netlink probe
// faked so both answers run on any host. The live install on a stock kernel is
// TestMPLSIntegration_TransitPathMTUFollowsTheProbe.

package fibkernel

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

// withTransitMTUProbe fakes the kernel's answer for the duration of a test and
// returns how many times it was asked.
func withTransitMTUProbe(t *testing.T, state kernelcap.State) *int {
	t.Helper()
	original := probeMPLSTransitMTU
	t.Cleanup(func() { probeMPLSTransitMTU = original })

	asked := 0
	probeMPLSTransitMTU = func() kernelcap.Result {
		asked++
		if state == kernelcap.StatePresent {
			return kernelcap.Result{State: state}
		}
		return kernelcap.Result{State: state, Reason: errors.New("faked kernel answer")}
	}
	return &asked
}

// VALIDATES: an AF_MPLS swap or pop carries the path MTU only on a kernel that
// accepts it, and the kernel is asked once, at the first route with an MTU.
// PREVENTS: the stock-kernel failure in plan/journal/kernel-refuses-what-the-
// installer-sends.md, where every transit install carrying an ADSPEC MTU failed
// with EINVAL and no transit LSP came up; and a patched kernel silently losing
// its enforcement.
func TestTransitRouteMTUFollowsTheProbe(t *testing.T) {
	for _, tc := range []struct {
		state kernelcap.State
		want  int
	}{
		{kernelcap.StatePresent, 1480},
		{kernelcap.StateAbsent, 0},
		{kernelcap.StateUnknown, 0},
		{kernelcap.StateUnspecified, 0},
	} {
		t.Run(tc.state.String(), func(t *testing.T) {
			asked := withTransitMTUProbe(t, tc.state)
			backend := &netlinkBackend{}

			if got := backend.transitRouteMTU(0); got != 0 {
				t.Errorf("a route with no path MTU carries %d", got)
			}
			if *asked != 0 {
				t.Errorf("a route with no path MTU asked the kernel %d times", *asked)
			}
			for range 2 {
				if got := backend.transitRouteMTU(1480); got != tc.want {
					t.Errorf("kernel answer %v: the route carries MTU %d, want %d", tc.state, got, tc.want)
				}
			}
			if *asked != 1 {
				t.Errorf("the kernel was asked %d times, want once", *asked)
			}
		})
	}
}
