package main

import "testing"

const (
	netlinkXFRMPatchPath = "scripts/dev/patches/netlink-xfrm-fixes.patch"
	netlinkXFRMRecovery  = "git apply scripts/dev/patches/netlink-xfrm-fixes.patch"
)

// TestNetlinkXFRMPatchApplied keeps go mod vendor from removing the XFRM fixes.
func TestNetlinkXFRMPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, netlinkXFRMPatchPath, netlinkXFRMRecovery)
}
