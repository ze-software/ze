// Design: docs/contributing/patching-the-standard-library.md -- vendored netlink fixes

package vendorpatch

import "testing"

// TestNetlinkSubscriptionUnicastPatchApplied keeps read-only subscription dumps
// unprivileged after a re-vendor, including on hosts without Linux netlink.
func TestNetlinkSubscriptionUnicastPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, "internal/le/vendorpatch/patches/netlink-subscription-unicast.patch",
		"run ./le setup install to reapply the netlink subscription unicast patch")
}
