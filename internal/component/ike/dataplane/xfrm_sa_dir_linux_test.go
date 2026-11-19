// VALIDATES: xfrmStateFromParams hands the kernel the direction Ze gave each SA as
// XFRMA_SA_DIR (netlink.XfrmState.SADir), leaves the shared RFC 4552 state without one,
// and refuses, before the kernel, the two shapes only a Ze defect builds: an
// anti-replay window on an outbound SA, and an SA in the forward direction.
// PREVENTS: an SA that `show` cannot place on a side, an outbound state the kernel
// refuses ("Replay window should be 0 for output SA") reported as a kernel error
// rather than as the Ze defect it is, and a both-directions state flagged one way.

//go:build linux

package dataplane

import (
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestXFRMStateCarriesTheSADirection proves each direction maps to its kernel value
// and the zero Dir maps to no attribute. Method: build one state per Dir and read
// SADir, which the vendored netlink writes as XFRMA_SA_DIR when it is nonzero.
func TestXFRMStateCarriesTheSADirection(t *testing.T) {
	cases := []struct {
		name string
		dir  SADir
		want netlink.SADir
	}{
		{"inbound", SADirIn, netlink.XFRM_SA_DIR_IN},
		{"outbound", SADirOut, netlink.XFRM_SA_DIR_OUT},
		{"shared by both directions", 0, 0},
	}
	for _, tc := range cases {
		params := boundarySA(0x1000)
		params.Dir = tc.dir
		state, err := xfrmStateFromParams(params)
		if err != nil {
			t.Fatalf("%s: xfrmStateFromParams: %v", tc.name, err)
		}
		if state.SADir != tc.want {
			t.Errorf("%s: SADir = %d, want %d", tc.name, state.SADir, tc.want)
		}
	}
}

// TestXFRMStateReplayWindowOnlyOnTheReceivingSA proves the window reaches an inbound
// and a shared state, and that an outbound one carrying a window is refused as a Ze
// defect with no state built. Method: the same window on each direction.
func TestXFRMStateReplayWindowOnlyOnTheReceivingSA(t *testing.T) {
	for _, dir := range []SADir{SADirIn, 0} {
		params := boundarySA(0x1000)
		params.Dir, params.ReplayWin = dir, 64
		state, err := xfrmStateFromParams(params)
		if err != nil {
			t.Fatalf("dir %d: xfrmStateFromParams: %v", dir, err)
		}
		if state.ReplayWindow != 64 {
			t.Errorf("dir %d: replay window %d, want 64", dir, state.ReplayWindow)
		}
	}

	outbound := boundarySA(0x1000)
	state, err := xfrmStateFromParams(outbound)
	if err != nil {
		t.Fatalf("outbound without a window: xfrmStateFromParams: %v", err)
	}
	if state.ReplayWindow != 0 {
		t.Errorf("outbound without a window: replay window %d, want 0", state.ReplayWindow)
	}

	outbound.ReplayWin = 64
	state, err = xfrmStateFromParams(outbound)
	if err == nil || state != nil {
		t.Fatalf("outbound with window 64: state %v, err %v, want a refusal and no state", state, err)
	}
	if !strings.Contains(err.Error(), "BUG:") || !strings.Contains(err.Error(), "outbound SA") {
		t.Errorf("outbound with window 64: error %q, want a BUG naming the outbound SA", err)
	}
}

// TestXFRMStateRefusesADirectionNoStateCarries proves the forward direction, which
// names a policy, and a value outside the enum are refused rather than installed
// without a direction. Method: build a state for each and expect no state.
func TestXFRMStateRefusesADirectionNoStateCarries(t *testing.T) {
	for _, dir := range []SADir{SADirFwd, SADirFwd + 1} {
		params := boundarySA(0x1000)
		params.Dir = dir
		state, err := xfrmStateFromParams(params)
		if err == nil || state != nil {
			t.Fatalf("dir %d: state %v, err %v, want a refusal and no state", dir, state, err)
		}
		if !strings.Contains(err.Error(), "BUG:") {
			t.Errorf("dir %d: error %q, want a BUG naming the Ze defect", dir, err)
		}
	}
}
