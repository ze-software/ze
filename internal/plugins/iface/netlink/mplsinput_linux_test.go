package ifacenetlink

import "testing"

// VALIDATES: AC-27 -- the netlink backend accepts any MPLS input set and
// programs nothing, because net.mpls.conf.<dev>.input carries the leaf.
// Method: a backend with no netlink handle; a call that reached the kernel
// would need one.
func TestNetlinkReplaceMPLSInputInstallsNothing(t *testing.T) {
	b := &netlinkBackend{}
	if err := b.ReplaceMPLSInput([]string{"eth0", "eth1"}); err != nil {
		t.Fatalf("ReplaceMPLSInput(set) = %v, want nil", err)
	}
	if err := b.ReplaceMPLSInput(nil); err != nil {
		t.Fatalf("ReplaceMPLSInput(nil) = %v, want nil", err)
	}
}
