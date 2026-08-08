package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// peerTreeWith builds the smallest config tree parsePeerFromTree accepts, with
// the named leaves overridden.
func peerTreeWith(localIP, nextHop string) map[string]any {
	session := map[string]any{"asn": map[string]any{"remote": "65001"}}
	if nextHop != "" {
		session["next-hop"] = nextHop
	}
	return map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "2001:db8::2"},
			"local":  map[string]any{"ip": localIP},
		},
		"session": session,
	}
}

// TestParsePeerFromTreeRejectsLinkLocalGlobalNextHop drives the RFC 2545 Section 3
// next-hop form guard from the two CONFIG leaves that reach the field, rather than
// from the helper alone.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." `connection > local >
// ip` reaches that first slot through `next-hop self` and through the
// default-originate rail; `session > next-hop` reaches it directly.
//
// VALIDATES: both leaves refuse a link-local address, and the error names the leaf.
// PREVENTS: a link-local reaching the first slot of the Next Hop field by a route
// the three route-level guards do not cover.
func TestParsePeerFromTreeRejectsLinkLocalGlobalNextHop(t *testing.T) {
	tests := []struct {
		name     string
		tree     map[string]any
		wantLeaf string
	}{
		{"local ip", peerTreeWith("fe80::1", ""), "local ip"},
		{"next-hop", peerTreeWith("2001:db8::1", "fe80::cafe"), "next-hop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePeerFromTree("peer1", tt.tree, 65000, 0x0a000001)
			require.Error(t, err)
			assert.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
			assert.Contains(t, err.Error(), tt.wantLeaf)
		})
	}
}

// TestParsePeerFromTreeAcceptsGlobalNextHop is the other side of the guard.
//
// VALIDATES: a global IPv6 address, an IPv4 address, "auto" and the three next-hop
// MODES all parse. Without these rows the guard could refuse everything and still
// pass the negative test.
func TestParsePeerFromTreeAcceptsGlobalNextHop(t *testing.T) {
	for _, localIP := range []string{"2001:db8::1", "::1", "192.0.2.1", "auto"} {
		t.Run("local ip "+localIP, func(t *testing.T) {
			ps, err := parsePeerFromTree("peer1", peerTreeWith(localIP, ""), 65000, 0x0a000001)
			require.NoError(t, err)
			require.NotNil(t, ps)
		})
	}
	for _, nextHop := range []string{"2001:db8::ffff", "192.0.2.1", "self", "unchanged", "auto"} {
		t.Run("next-hop "+nextHop, func(t *testing.T) {
			ps, err := parsePeerFromTree("peer1", peerTreeWith("2001:db8::1", nextHop), 65000, 0x0a000001)
			require.NoError(t, err)
			require.NotNil(t, ps)
		})
	}
}

// TestParsePeerFromTreeNextHopModesUnchanged pins the mode parsing that moved into
// applyNextHopMode with the guard.
//
// VALIDATES: each keyword still maps to its mode, and an explicit global address
// still lands in NextHopAddress.
func TestParsePeerFromTreeNextHopModesUnchanged(t *testing.T) {
	tests := []struct {
		value    string
		wantMode uint8
		wantAddr netip.Addr
	}{
		{"self", NextHopSelf, netip.Addr{}},
		{"unchanged", NextHopUnchanged, netip.Addr{}},
		{"auto", NextHopAuto, netip.Addr{}},
		{"2001:db8::ffff", NextHopExplicit, netip.MustParseAddr("2001:db8::ffff")},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			ps, err := parsePeerFromTree("peer1", peerTreeWith("2001:db8::1", tt.value), 65000, 0x0a000001)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, ps.NextHopMode)
			assert.Equal(t, tt.wantAddr, ps.NextHopAddress)
		})
	}
}
