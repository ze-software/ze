package netlink

import (
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestXfrmStateICMPFlag checks that ICMP sets XFRM_STATE_ICMP, which is
// 0x10 in include/uapi/linux/xfrm.h, and is read back from it.
func TestXfrmStateICMPFlag(t *testing.T) {
	if got := xfrmUsersaInfoFromXfrmState(&XfrmState{ICMP: true}).Flags; got != 0x10 {
		t.Errorf("flags with ICMP = %#x, want 0x10", got)
	}
	if got := xfrmUsersaInfoFromXfrmState(&XfrmState{}).Flags; got != 0 {
		t.Errorf("flags without ICMP = %#x, want 0", got)
	}
	if !xfrmStateFromXfrmUsersaInfo(&nl.XfrmUsersaInfo{Flags: 0x10}).ICMP {
		t.Errorf("ICMP = false for flags 0x10, want true")
	}
	if xfrmStateFromXfrmUsersaInfo(&nl.XfrmUsersaInfo{}).ICMP {
		t.Errorf("ICMP = true for flags 0, want false")
	}
}

// TestXfrmPolicyICMPFlag checks that ICMP sets XFRM_POLICY_ICMP, which is 2 in
// include/uapi/linux/xfrm.h, and is read back from it.
func TestXfrmPolicyICMPFlag(t *testing.T) {
	if got := xfrmUserpolicyFlags(&XfrmPolicy{ICMP: true}); got != 0x02 {
		t.Errorf("flags with ICMP = %#x, want 0x02", got)
	}
	if got := xfrmUserpolicyFlags(&XfrmPolicy{}); got != 0 {
		t.Errorf("flags without ICMP = %#x, want 0", got)
	}
	info := nl.XfrmUserpolicyInfo{Flags: 0x02}
	policy, err := parseXfrmPolicy(info.Serialize(), FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.ICMP {
		t.Errorf("ICMP = false for flags 0x02, want true")
	}
}
