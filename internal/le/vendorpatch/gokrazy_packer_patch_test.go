// Design: docs/contributing/patching-the-standard-library.md -- embedded gokrazy.

package vendorpatch

import "testing"

func TestGokrazyPackerReturnsErrorsPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, Dir+"/gokrazy-packer-return-errors.patch", "./le setup install")
}
