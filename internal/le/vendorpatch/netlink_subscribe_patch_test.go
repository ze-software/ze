// Design: docs/contributing/patching-the-standard-library.md -- vendored netlink library fixes

package vendorpatch

import "testing"

// TestNetlinkSubscriptionUnicastPatchApplied keeps read-only subscription dumps
// unprivileged after a re-vendor, including on hosts without Linux netlink sockets.
func TestNetlinkSubscriptionUnicastPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, "internal/le/vendorpatch/patches/netlink-subscription-unicast.patch",
		"run ./le setup install to reapply the netlink subscription unicast patch")
}
