//go:build !linux

// VALIDATES: on non-Linux platforms the attacher reports unsupported and never
// pretends to attach, and its answer reads as a missing TCX attach point.
// PREVENTS: a darwin/other build silently no-op'ing as if it were counting
// traffic (AC-14), or reporting its platform as a permission refusal.

package trafficusage

import "testing"

func TestUnsupportedAttacher(t *testing.T) {
	a := newAttacher()
	availability := a.Available()
	if availability == nil {
		t.Error("Available() = nil on non-Linux, want unsupported error")
	}
	att, err := a.Attach(1, "eth0", 1024, false)
	if err == nil {
		t.Error("Attach() error = nil on non-Linux, want unsupported error")
	}
	if att != nil {
		t.Error("Attach() attachment != nil on non-Linux, want nil")
	}
	// The monitor records either answer as the interface's outcome, and the
	// doctor reports it as absent, the plugin's inert case.
	if got := classifyAttachError(availability); got != attachStateAbsent {
		t.Errorf("Available() classifies as %v on non-Linux, want absent", got)
	}
	if got := classifyAttachError(err); got != attachStateAbsent {
		t.Errorf("Attach() classifies as %v on non-Linux, want absent", got)
	}
}
