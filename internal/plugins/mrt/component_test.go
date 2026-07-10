// VALIDATES: MRT record gating — New() precomputes the peer set and direction
// filter from config, and shouldRecord honors direction (received/sent/both)
// and peer-filter membership.
// PREVENTS: MRT dumps capturing the wrong direction or peers the operator
// explicitly excluded (or dropping peers they wanted).
package mrt

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func peerAt(addr string) *plugin.PeerInfo {
	return &plugin.PeerInfo{Address: netip.MustParseAddr(addr)}
}

func TestShouldRecordBothDirectionsNoFilter(t *testing.T) {
	c := New(Config{}, nil)
	p := peerAt("10.0.0.1")
	if !c.shouldRecord(p, true) || !c.shouldRecord(p, false) {
		t.Error("default config should record both sent and received")
	}
}

func TestShouldRecordReceivedOnly(t *testing.T) {
	c := New(Config{Direction: "received"}, nil)
	p := peerAt("10.0.0.1")
	if c.shouldRecord(p, true) {
		t.Error("received-only recorded a sent message")
	}
	if !c.shouldRecord(p, false) {
		t.Error("received-only dropped a received message")
	}
}

func TestShouldRecordSentOnly(t *testing.T) {
	c := New(Config{Direction: "sent"}, nil)
	p := peerAt("10.0.0.1")
	if !c.shouldRecord(p, true) {
		t.Error("sent-only dropped a sent message")
	}
	if c.shouldRecord(p, false) {
		t.Error("sent-only recorded a received message")
	}
}

func TestShouldRecordPeerFilter(t *testing.T) {
	c := New(Config{PeerFilter: []string{"10.0.0.1", "2001:db8::1"}}, nil)
	if !c.shouldRecord(peerAt("10.0.0.1"), false) {
		t.Error("filtered-in IPv4 peer dropped")
	}
	if !c.shouldRecord(peerAt("2001:db8::1"), false) {
		t.Error("filtered-in IPv6 peer dropped")
	}
	if c.shouldRecord(peerAt("10.0.0.99"), false) {
		t.Error("peer outside the filter was recorded")
	}
}

func TestShouldRecordDirectionAndPeerCombined(t *testing.T) {
	c := New(Config{Direction: "received", PeerFilter: []string{"10.0.0.1"}}, nil)
	// Right peer, wrong direction → dropped.
	if c.shouldRecord(peerAt("10.0.0.1"), true) {
		t.Error("combined filter recorded a sent message from the allowed peer")
	}
	// Right peer, right direction → kept.
	if !c.shouldRecord(peerAt("10.0.0.1"), false) {
		t.Error("combined filter dropped an allowed received message")
	}
	// Wrong peer, right direction → dropped.
	if c.shouldRecord(peerAt("10.0.0.2"), false) {
		t.Error("combined filter recorded a disallowed peer")
	}
}

// TestNewIgnoresMalformedPeerFilter proves a bad address in the filter is
// skipped (not fatal) while valid entries still gate.
func TestNewIgnoresMalformedPeerFilter(t *testing.T) {
	c := New(Config{PeerFilter: []string{"not-an-ip", "10.0.0.1"}}, nil)
	if !c.shouldRecord(peerAt("10.0.0.1"), false) {
		t.Error("valid peer dropped when filter also had a malformed entry")
	}
	if c.shouldRecord(peerAt("10.0.0.2"), false) {
		t.Error("filter with a valid entry still recorded a non-member")
	}
}
