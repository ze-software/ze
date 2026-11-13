package vendorpatch

import "testing"

// netlinkXFRMSeries is the XFRM patch series against the vendored
// vishvananda/netlink, in set order. TestEveryVendorPatchIsApplied cannot
// notice a patch file that is gone, so each name here pins that its patch is
// present.
var netlinkXFRMSeries = []string{
	"netlink-xfrm-01-state-readback-esn-replay.patch",
	"netlink-xfrm-02-state-replay-window-above-32.patch",
	"netlink-xfrm-03-template-family-no-destination.patch",
	"netlink-xfrm-04-readback-message-family.patch",
	"netlink-xfrm-05-state-truncated-header.patch",
	"netlink-xfrm-06-selector-port-masks.patch",
	"netlink-xfrm-07-state-af-unspec.patch",
	"netlink-xfrm-08-state-no-pmtu-disc.patch",
	"netlink-xfrm-09-icmp-flag.patch",
	"netlink-xfrm-10-selector-prefix-family.patch",
	"netlink-xfrm-90-state-extra-attributes.patch",
}

// TestNetlinkXFRMPatchApplied keeps a re-vendor or a deletion from dropping
// any patch of the XFRM series.
func TestNetlinkXFRMPatchApplied(t *testing.T) {
	for _, name := range netlinkXFRMSeries {
		t.Run(name, func(t *testing.T) {
			assertVendorPatchApplied(t, Dir+"/"+name, vendorPatchRecovery)
		})
	}
}
