//go:build linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- raw-socket probe test (Linux)

package rsvpte

import "testing"

func TestRSVPRawSocketAvailableDeterministic(t *testing.T) {
	// VALIDATES: the raw-socket probe is callable and stable -- it must not panic
	// and must return the same answer on repeated calls in the same environment
	// (with CAP_NET_RAW it is true; without it, false). The check logic that maps
	// this to a diagnostic is covered by doctor_test.go via the seam.
	first := rsvpRawSocketAvailable()
	second := rsvpRawSocketAvailable()
	if first != second {
		t.Fatalf("raw-socket probe not deterministic: %v then %v", first, second)
	}
}
