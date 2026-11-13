package vendorpatch

import "testing"

// TestNetlinkIPv4RouteProbePatchApplied preserves the live identity operation
// when the upstream module is vendored again.
func TestNetlinkIPv4RouteProbePatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, "internal/le/vendorpatch/patches/netlink-route-probe-ipv4.patch",
		"git apply internal/le/vendorpatch/patches/netlink-route-probe-ipv4.patch")
}
