//go:build !linux

// VALIDATES: on non-Linux platforms the attacher reports unsupported and never
// pretends to attach.
// PREVENTS: a darwin/other build silently no-op'ing as if it were counting
// traffic (AC-14).

package trafficusage

import "testing"

func TestUnsupportedAttacher(t *testing.T) {
	a := newAttacher()
	if err := a.Available(); err == nil {
		t.Error("Available() = nil on non-Linux, want unsupported error")
	}
	att, err := a.Attach(1, "eth0", 1024, false)
	if err == nil {
		t.Error("Attach() error = nil on non-Linux, want unsupported error")
	}
	if att != nil {
		t.Error("Attach() attachment != nil on non-Linux, want nil")
	}
	// The doctor uses ebpfSupported() (side-effect-free); it must report
	// unsupported on non-Linux.
	if ebpfSupported() == nil {
		t.Error("ebpfSupported() = nil on non-Linux, want unsupported error")
	}
}
