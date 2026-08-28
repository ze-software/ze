package vendorpatch

import "testing"

const (
	netlinkXFRMPatchPath = "internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch"
	netlinkXFRMRecovery  = "git apply internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch"
)

// TestNetlinkXFRMPatchApplied keeps go mod vendor from removing the XFRM fixes.
func TestNetlinkXFRMPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, netlinkXFRMPatchPath, netlinkXFRMRecovery)
}
