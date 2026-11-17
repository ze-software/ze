// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected attributes.
// Related: rfc4271_partial_session_wire_test.go -- ordinary Session preservation test.
// Related: rfc4271_selected_boundaries_test.go -- real ordinary-speaker fixture.
// Related: rs_departure_reconnect_test.go -- real destination Session replacement.

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// partialSessionWireTarget names one valid recognized optional-transitive value.
// All are short attributes, and the AGGREGATOR uses the negotiated four-octet ASN.
type partialSessionWireTarget struct {
	code  attribute.AttributeCode
	value []byte
}

// partialSessionWireTargets supplies distinct ordinary values for the four
// dedicated pooled optional-transitive attributes, independent of path controls.
func partialSessionWireTargets() []partialSessionWireTarget {
	return []partialSessionWireTarget{
		{attribute.AttrAggregator, []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}},
		{attribute.AttrCommunity, []byte{0, 1, 0, 1}},
		{attribute.AttrExtCommunity, []byte{0, 2, 0xfd, 0xe9, 0, 0, 0, 1}},
		{attribute.AttrLargeCommunity, []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 2, 0, 0, 0, 3}},
	}
}

// requirePartialSessionWireRoute checks the current useful route and every
// announcement, so a later correction cannot hide earlier bad attribute bytes.
func requirePartialSessionWireRoute(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, flags byte) {
	t.Helper()
	require.Len(t, view.routes, 1, "the receiver holds the useful source prefix")
	require.Contains(t, view.routes, prefix)
	require.NotEmpty(t, view.announcements[prefix], "the destination Session receives the useful route")
	require.Zero(t, view.withdrawals[prefix])
	requirePartialSessionWireAttributes(t, view.routes[prefix], flags)
	for _, attrs := range view.announcements[prefix] {
		requirePartialSessionWireAttributes(t, attrs, flags)
	}
}

// requirePartialSessionWireAttributes checks useful path fields and the separate
// iBGP LOCAL_PREF projection before the four optional-transitive attributes.
func requirePartialSessionWireAttributes(t *testing.T, attrs []byte, flags byte) {
	t.Helper()
	for _, control := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{
		{attribute.AttrOrigin, []byte{0}},
		{attribute.AttrASPath, []byte{2, 1, 0, 0, 0xfd, 0xf2}},
		{attribute.AttrNextHop, []byte{198, 18, 249, 1}},
		{attribute.AttrLocalPref, []byte{0, 0, 0, 100}},
	} {
		_, _, value, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "useful attribute %d reaches the peer", control.code)
		require.Equal(t, control.value, value, "attribute %d value", control.code)
	}
	requirePartialSessionWireTargets(t, attrs, flags)
}

// requirePartialSessionWireTargets checks retained values and all flags except
// the independently legal Extended Length bit (RFC 4271 Section 4.3).
func requirePartialSessionWireTargets(t *testing.T, attrs []byte, flags byte) {
	t.Helper()
	for _, target := range partialSessionWireTargets() {
		_, received, value, found := attribute.AttrFind(attrs, target.code)
		require.True(t, found, "recognized attribute %d survives", target.code)
		require.Equal(t, target.value, value, "attribute %d retains its exact value", target.code)
		require.Equal(t, flags, byte(received)&^byte(attribute.FlagExtLength),
			"attribute %d retains its received flags apart from header width", target.code)
	}
}
