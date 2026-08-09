//go:build !linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- raw-socket probe test (non-Linux)

package rsvpte

import "testing"

func TestRSVPRawSocketAvailableNonLinux(t *testing.T) {
	// VALIDATES: on non-Linux there is no raw-socket transport (transport_other.go
	// is a stub), so the probe reports "available" and the doctor check stays quiet.
	if !rsvpRawSocketAvailable() {
		t.Fatal("non-Linux raw-socket probe must report available (no dependency to warn about)")
	}
}
