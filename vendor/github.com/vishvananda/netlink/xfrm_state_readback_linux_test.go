package netlink

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestXfrmStateReadbackESN parses a state message as the kernel answers it for
// an ESN state: XFRM_STATE_ESN in the flags, a legacy replay_window of 0, and
// the window in XFRMA_REPLAY_ESN_VAL.
func TestXfrmStateReadbackESN(t *testing.T) {
	msg := nl.XfrmUsersaInfo{Family: nl.FAMILY_V4, Flags: nl.XFRM_STATE_ESN}
	// struct xfrm_replay_state_esn: bmp_len, oseq, seq, oseq_hi, seq_hi and
	// replay_window as u32, then bmp_len u32 words of bitmap.
	esn := make([]byte, 24+2*4)
	native.PutUint32(esn[0:], 2)
	native.PutUint32(esn[20:], 64)
	m := append(msg.Serialize(), nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, esn).Serialize()...)

	state, err := parseXfrmState(m, FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if !state.ESN {
		t.Errorf("ESN = false, want true")
	}
	if state.ReplayWindow != 64 {
		t.Errorf("ReplayWindow = %d, want 64", state.ReplayWindow)
	}
}
