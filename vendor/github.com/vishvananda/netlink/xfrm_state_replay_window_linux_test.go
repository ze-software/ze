package netlink

import "testing"

// TestXfrmStateReplayWindowInESN checks where the state writer puts the replay
// window: xfrm_usersa_info holds a 32-bit bitmap, so a wider window, or any
// window of an ESN state, must travel in XFRMA_REPLAY_ESN_VAL.
func TestXfrmStateReplayWindowInESN(t *testing.T) {
	for _, tc := range []struct {
		esn    bool
		window int
		want   bool
	}{
		{esn: false, window: 0, want: false},
		{esn: false, window: 32, want: false},
		{esn: false, window: 33, want: true},
		{esn: false, window: 64, want: true},
		{esn: false, window: 4096, want: true},
		{esn: true, window: 32, want: true},
		{esn: true, window: 64, want: true},
	} {
		state := &XfrmState{ESN: tc.esn, ReplayWindow: tc.window}
		if got := xfrmReplayWindowInESN(state); got != tc.want {
			t.Errorf("ESN %v, ReplayWindow %d: in XFRMA_REPLAY_ESN_VAL = %v, want %v", tc.esn, tc.window, got, tc.want)
		}
	}
}
