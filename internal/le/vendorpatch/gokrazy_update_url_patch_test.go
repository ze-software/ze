package vendorpatch

import "testing"

func TestGokrazyUpdaterURLUserinfoPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, Dir+"/gokrazy-update-url-userinfo.patch", "./le setup install")
}
