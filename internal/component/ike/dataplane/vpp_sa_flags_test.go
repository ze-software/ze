// VALIDATES: vppSAFlags sets USE_ANTI_REPLAY exactly when the SA carries a window, so
// the inbound SA keeps the check and the outbound SA, which carries none, sets no flag.
// PREVENTS: a flag on the sending SA standing in for a check VPP performs anyway: VPP
// v26.06 refuses a cycled sequence counter on every outbound SA (esp_seq_advance).

//go:build ze_vpp

package dataplane

import (
	"testing"

	"go.fd.io/govpp/binapi/ipsec_types"
)

// TestVPPSAFlagsAntiReplayPerDirection proves the anti-replay flag follows the window.
// Method: derive the flags for each direction with and without a window.
func TestVPPSAFlagsAntiReplayPerDirection(t *testing.T) {
	cases := []struct {
		name   string
		dir    SADir
		window uint8
		want   bool
	}{
		{"outbound without a window", SADirOut, 0, false},
		// The window, not the direction, decides: a direction test in its place
		// would pass the three other cases and fail this one.
		{"outbound with a window", SADirOut, 64, true},
		{"inbound with a window", SADirIn, 64, true},
		{"inbound without a window", SADirIn, 0, false},
	}
	for _, tc := range cases {
		p := testSAParams()
		p.Dir, p.ReplayWin = tc.dir, tc.window
		got := vppSAFlags(p)&ipsec_types.IPSEC_API_SAD_FLAG_USE_ANTI_REPLAY != 0
		if got != tc.want {
			t.Errorf("%s: USE_ANTI_REPLAY = %v, want %v", tc.name, got, tc.want)
		}
	}
}
