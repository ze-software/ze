// Design: docs/architecture/core-design.md -- egress route decisions on the forward rails
// Related: forward_local_pref.go -- applyFactsLocalPref, the LOCAL_PREF obligation under test
// Related: reactor_a2_rfc8950_forward_test.go -- a2Forward and a2Parts, the harness

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// a2InlineASPath is the received AS_PATH value: one AS_SEQUENCE of the 4-octet
// AS 65001.
var a2InlineASPath = []byte{0x02, 0x01, 0, 0, 0xFD, 0xE9}

// a2InlinePrefix is 203.0.113.0/24, the IPv4 unicast route the inline payload
// announces.
var a2InlinePrefix = []byte{24, 203, 0, 113}

// a2InlinePayload builds an UPDATE, as an external peer sends it, that announces
// a2InlinePrefix with ORIGIN, AS_PATH and NEXT_HOP 192.0.2.254, and no
// LOCAL_PREF.
func a2InlinePayload() []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, byte(len(a2InlineASPath)), // AS_PATH header
	}
	attrs = append(attrs, a2InlineASPath...)
	attrs = append(attrs, 0x40, 0x03, 0x04, 192, 0, 2, 254) // NEXT_HOP
	return buildUpdatePayload(attrs, a2InlinePrefix)
}

// TestRFC4271ForwardAddsLocalPrefTowardInternalPeer drives RFC 4271 Section
// 5.1.5 on the two forward rails: "LOCAL_PREF is a well-known attribute that
// SHALL be included in all UPDATE messages that a given BGP speaker sends to
// other internal peers."
//
// Method: a route learned from an external peer carries no LOCAL_PREF (Section
// 5.1.5 forbids the external peer from sending one). It is forwarded, on the
// general rail and on the route-server rail, to an internal and an external
// destination in one fan-out.
//
// VALIDATES: the internal destination is sent LOCAL_PREF 100, the value every
// originating rail writes; the external destination is sent none.
// PREVENTS: the forward rails relaying an eBGP-learned route to an internal peer
// without the attribute, which applyFactsLocalPref only ever stripped.
//
// RFC requirement: RFC4271-5.1.5-1 positive -- a route relayed from an external peer without LOCAL_PREF reaches an internal destination carrying LOCAL_PREF 100, on the general and route-server rails.
// RFC requirement: RFC4271-5.1.5-1 negative -- the obligation is confined to internal peers: the external destination in the same fan-out is sent no LOCAL_PREF.
func TestRFC4271ForwardAddsLocalPrefTowardInternalPeer(t *testing.T) {
	for _, tc := range []struct {
		name string
		rs   bool
	}{
		{"general rail", false},
		{"route-server rail", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			internal := a2Dest(t, "192.0.2.31", 65000, netip.Addr{}, false)
			external := a2Dest(t, "192.0.2.32", 65032, netip.Addr{}, false)

			got := a2Forward(t, tc.rs, a2InlinePayload(), internal, external)

			toInternal, ok := got[netip.MustParseAddr("192.0.2.31")]
			require.True(t, ok, "the internal destination is owed the route")
			require.Equal(t, a2InlinePrefix, toInternal.nlri)
			require.Equal(t, []byte{0, 0, 0, 100}, toInternal.localPref, "LOCAL_PREF SHALL be included toward an internal peer")

			toExternal, ok := got[netip.MustParseAddr("192.0.2.32")]
			require.True(t, ok, "the external destination is owed the route")
			require.Equal(t, a2InlinePrefix, toExternal.nlri)
			require.Nil(t, toExternal.localPref, "no LOCAL_PREF toward an external peer")
		})
	}
}

// TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer drives RFC 4271 Section
// 5.1.2 a) on the general forward rail: "When a given BGP speaker advertises the
// route to an internal peer, the advertising speaker SHALL NOT modify the
// AS_PATH attribute associated with the route."
//
// Method: a route learned from an external peer is forwarded to two internal
// destinations, one of them configured with as-override, which rewrites AS_PATH
// toward an external peer. An external destination in the same fan-out is the
// control that the rail modifies AS_PATH when it is allowed to.
//
// VALIDATES: both internal destinations are sent the received AS_PATH value
// byte for byte; the external one is sent it with 65000 prepended.
// PREVENTS: a prepend or an override reaching an internal peer's AS_PATH.
//
// RFC requirement: RFC4271-5.1.2-2 positive -- a route learned from an external peer and forwarded to an internal peer carries the received AS_PATH value byte for byte.
// RFC requirement: RFC4271-5.1.2-2 negative -- an internal destination configured with as-override, the setting that rewrites AS_PATH, is still sent the received AS_PATH unmodified, while the external destination in the same fan-out is sent it with 65000 prepended.
func TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer(t *testing.T) {
	internal := a2Dest(t, "192.0.2.41", 65000, netip.Addr{}, false)
	overridden := a2Dest(t, "192.0.2.42", 65000, netip.Addr{}, false)
	overridden.settings.ASOverride = true
	overridden.refreshForwardFacts()
	external := a2Dest(t, "192.0.2.43", 65043, netip.Addr{}, false)

	got := a2Forward(t, false, a2InlinePayload(), internal, overridden, external)

	for _, addr := range []string{"192.0.2.41", "192.0.2.42"} {
		sent, ok := got[netip.MustParseAddr(addr)]
		require.True(t, ok, "internal destination %s is owed the route", addr)
		require.Equal(t, a2InlinePrefix, sent.nlri)
		require.Equal(t, a2InlineASPath, sent.asPath, "AS_PATH toward internal %s SHALL NOT be modified", addr)
	}

	sent, ok := got[netip.MustParseAddr("192.0.2.43")]
	require.True(t, ok, "the external destination is owed the route")
	require.Equal(t, []byte{0x02, 0x02, 0, 0, 0xFD, 0xE8, 0, 0, 0xFD, 0xE9}, sent.asPath, "65000 is prepended toward an external peer")
}
