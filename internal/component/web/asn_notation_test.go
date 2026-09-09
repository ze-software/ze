package web

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// TestConfigPeerTableFollowsTheConfiguredNotation proves the AS number the web
// BGP peer and group pages show carries the configured notation. The method
// renders one stored value under each notation.
//
// VALIDATES: asnText renders through asn.Text.
// PREVENTS: the config pages showing 65546 beside a route table showing 1.10.
// The tree stores the decimal form whatever the operator typed.
func TestConfigPeerTableFollowsTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })

	configureNotation(t, asn.NotationPlain)
	if got := asnText("65546"); got != "65546" {
		t.Errorf("asplain = %q, want %q", got, "65546")
	}

	configureNotation(t, asn.NotationDot)
	if got := asnText("65546"); got != "1.10" {
		t.Errorf("asdot = %q, want %q", got, "1.10")
	}

	// A stored value that names no AS number is shown as it is stored. A view
	// that hides an unreadable value tells the operator less.
	if got := asnText("not-an-as"); got != "not-an-as" {
		t.Errorf("unreadable = %q, want it shown as stored", got)
	}
}

// configureNotation records one notation for the rest of a test.
func configureNotation(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}
