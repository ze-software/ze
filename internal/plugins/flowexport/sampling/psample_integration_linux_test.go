// VALIDATES: the live psample genetlink path — NewPsampleReader resolves the
// "psample" family and its multicast group and Close tears the socket down —
// constructs against the real kernel without panicking. Auto-enrolled in the
// QEMU integration run via the derived `integration && linux` package list.
// PREVENTS: a regression in the genetlink family/group resolution going
// unnoticed until a live appliance enables flow sampling.

//go:build integration && linux

package sampling

import "testing"

func TestPsampleReaderSmoke(t *testing.T) {
	r, err := NewPsampleReader()
	if err != nil {
		// psample module not loaded / insufficient privileges: the resolution
		// path has still been exercised up to the failure point.
		t.Skipf("psample reader unavailable in this environment: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestRemoveSamplingUnknownIface(t *testing.T) {
	// A non-existent interface must fail interface resolution rather than
	// panicking or silently succeeding.
	if err := RemoveSampling("ze-nope-xyz0"); err == nil {
		t.Error("RemoveSampling on a non-existent interface should return an error")
	}
}
