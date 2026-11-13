package netlink

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestXfrmStateAFUnspecFlag checks that AFUnspec sets XFRM_STATE_AF_UNSPEC, which is
// 0x20 in include/uapi/linux/xfrm.h, and is read back from it.
func TestXfrmStateAFUnspecFlag(t *testing.T) {
	if got := xfrmUsersaInfoFromXfrmState(&XfrmState{AFUnspec: true}).Flags; got != 0x20 {
		t.Errorf("flags with AFUnspec = %#x, want 0x20", got)
	}
	if got := xfrmUsersaInfoFromXfrmState(&XfrmState{}).Flags; got != 0 {
		t.Errorf("flags without AFUnspec = %#x, want 0", got)
	}
	if !xfrmStateFromXfrmUsersaInfo(&nl.XfrmUsersaInfo{Flags: 0x20}).AFUnspec {
		t.Errorf("AFUnspec = false for flags 0x20, want true")
	}
	if xfrmStateFromXfrmUsersaInfo(&nl.XfrmUsersaInfo{}).AFUnspec {
		t.Errorf("AFUnspec = true for flags 0, want false")
	}
}
