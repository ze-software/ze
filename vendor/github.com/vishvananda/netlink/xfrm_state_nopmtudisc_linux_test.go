package netlink

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestXfrmStateNoPMTUDiscFlag checks that NoPMTUDisc sets XFRM_STATE_NOPMTUDISC, which is
// 0x04 in include/uapi/linux/xfrm.h, and is read back from it.
func TestXfrmStateNoPMTUDiscFlag(t *testing.T) {
	if got := xfrmUsersaInfoFromXfrmState(&XfrmState{NoPMTUDisc: true}).Flags; got != 0x04 {
		t.Errorf("flags with NoPMTUDisc = %#x, want 0x04", got)
	}
	if got := xfrmUsersaInfoFromXfrmState(&XfrmState{}).Flags; got != 0 {
		t.Errorf("flags without NoPMTUDisc = %#x, want 0", got)
	}
	if !xfrmStateFromXfrmUsersaInfo(&nl.XfrmUsersaInfo{Flags: 0x04}).NoPMTUDisc {
		t.Errorf("NoPMTUDisc = false for flags 0x04, want true")
	}
	if xfrmStateFromXfrmUsersaInfo(&nl.XfrmUsersaInfo{}).NoPMTUDisc {
		t.Errorf("NoPMTUDisc = true for flags 0, want false")
	}
}
