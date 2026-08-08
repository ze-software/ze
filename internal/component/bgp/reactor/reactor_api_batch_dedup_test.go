package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countAttr counts how many times an attribute type code appears in a raw
// PathAttributes block. RFC 4271 Section 4.3 attribute framing: flags, type,
// then a 1- or 2-octet length depending on the Extended Length flag.
func countAttr(t *testing.T, attrs []byte, code attribute.AttributeCode) int {
	t.Helper()
	n, pos := 0, 0
	for pos+2 <= len(attrs) {
		flags := attrs[pos]
		tc := attribute.AttributeCode(attrs[pos+1])
		var attrLen int
		if flags&0x10 != 0 {
			require.LessOrEqual(t, pos+4, len(attrs))
			attrLen = 4 + int(binary.BigEndian.Uint16(attrs[pos+2:]))
		} else {
			require.LessOrEqual(t, pos+3, len(attrs))
			attrLen = 3 + int(attrs[pos+2])
		}
		if tc == code {
			n++
		}
		pos += attrLen
	}
	require.Equal(t, len(attrs), pos, "attribute block did not parse cleanly")
	return n
}

// RFC 7606 Section 3(g): "if the same attribute (as identified by the attribute
// type) appears more than once within an UPDATE message, then ... the UPDATE
// message ... SHALL be handled using the approach of 'treat-as-withdraw'."
// FRR and other daemons discard such a route. The route-server replay path
// re-encodes a stored route via `update hex attr set <attrs> nhop set <nh> ...`,
// where <attrs> is the full received attribute block INCLUDING its NEXT_HOP, and
// the builder then adds a NEXT_HOP from <nh> -- producing the attribute twice.

// VALIDATES: an IPv4 wire-mode announce whose wire attributes already carry a
// NEXT_HOP yields exactly ONE NEXT_HOP, carrying the resolved value.
// PREVENTS: the duplicate-NEXT_HOP UPDATE the route server replayed to FRR, which
// FRR discarded (0 prefixes installed, no NOTIFICATION).
func TestBuildBatchAnnounce_WireMode_IPv4_NoDuplicateNextHop(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// ORIGIN + AS_PATH + NEXT_HOP 10.0.0.9 -- exactly what adj-rib-in stores and
	// replays for a received IPv4 route.
	wireAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x09, // NEXT_HOP 10.0.0.9
	}
	require.Equal(t, 1, countAttr(t, wireAttrs, attribute.AttrNextHop), "fixture guard")

	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.9")),
		Wire:    attribute.NewAttributesWire(wireAttrs, 0),
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.MustParseAddr("10.0.0.9"), false, false, true, false, 65000)
	require.NotNil(t, update)

	assert.Equal(t, 1, countAttr(t, update.PathAttributes, attribute.AttrNextHop),
		"a relayed UPDATE must carry exactly one NEXT_HOP; RFC 7606 Section 3(g) treats a "+
			"duplicate as a withdraw")
}

// VALIDATES: with an INVALID resolved next-hop, the builder fails closed -- it leaves the
// wire block's own NEXT_HOP untouched instead of stripping it and writing a malformed one.
// The assertion is on the EXACT output bytes, because an invalid Addr encodes as a
// zero-LENGTH NEXT_HOP value (attribute/simple.go): a weaker "count == 1 and contains
// 10.0.0.9" assertion passes even without the guard (mutation-verified), so it must be the
// whole attribute block, unchanged.
// PREVENTS: the de-duplication strip dropping a good stored next-hop when a caller reaches
// the builder with an invalid explicit next-hop -- resolveNextHop deliberately passes those
// through (TestResolveNextHop_ExplicitInvalid), so the builder is where it must fail closed.
func TestBuildBatchAnnounce_WireMode_IPv4_InvalidNextHopKeepsBlock(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// ORIGIN + AS_PATH + stored NEXT_HOP 10.0.0.9 + MED. Both mandatory attrs present, so
	// writeMandatoryAttrs copies the block verbatim and the output must equal it. The MED
	// AFTER the NEXT_HOP is load-bearing: it makes a broken strip observable. If the guard
	// is removed, stripping the NEXT_HOP shifts the MED left and the malformed zero-length
	// re-insert corrupts the block -- with the NEXT_HOP last, the malformed rewrite would
	// coincidentally reproduce the same bytes and the test could not tell.
	wireAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x09, // NEXT_HOP 10.0.0.9
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED 100
	}
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs:  []nlri.NLRI{wn},
		Wire:   attribute.NewAttributesWire(wireAttrs, 0),
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	// isIBGP=false so no LOCAL_PREF is appended; an invalid (zero) resolved next-hop.
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.Addr{}, false, false, true, false, 65000)
	require.NotNil(t, update)

	assert.Equal(t, wireAttrs, update.PathAttributes,
		"an invalid next-hop must leave the attribute block byte-for-byte unchanged, not "+
			"strip the stored NEXT_HOP and write a malformed zero-length one")
}

// VALIDATES: next-hop-self also yields exactly one NEXT_HOP, and it is the
// rewritten value, not the stored one.
// PREVENTS: a route-server or eBGP peer that rewrites next-hop shipping both the
// original and the rewrite.
func TestBuildBatchAnnounce_WireMode_IPv4_NextHopSelfReplaces(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	wireAttrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x09, // original NEXT_HOP 10.0.0.9
	}
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
		Wire:    attribute.NewAttributesWire(wireAttrs, 0),
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.MustParseAddr("192.0.2.1"), false, false, true, false, 65000)
	require.NotNil(t, update)

	require.Equal(t, 1, countAttr(t, update.PathAttributes, attribute.AttrNextHop))
	// The surviving NEXT_HOP must be the rewritten address 192.0.2.1, not 10.0.0.9.
	assert.Contains(t, string(update.PathAttributes), string([]byte{0xc0, 0x00, 0x02, 0x01}),
		"the surviving NEXT_HOP must be the rewritten 192.0.2.1")
	assert.NotContains(t, string(update.PathAttributes), string([]byte{0x0a, 0x00, 0x00, 0x09}),
		"the stored 10.0.0.9 NEXT_HOP must not survive alongside the rewrite")
}

// VALIDATES: an IPv6 wire-mode announce whose wire attributes already carry an
// MP_REACH_NLRI yields exactly ONE MP_REACH_NLRI.
// PREVENTS: the same duplicate-attribute defect for MP families, where the stored
// block carries an MP_REACH and the builder appends a second one.
func TestBuildBatchAnnounce_WireMode_IPv6_NoDuplicateMPReach(t *testing.T) {
	r := &Reactor{config: &Config{LocalAS: 65000}}
	adapter := &reactorAPIAdapter{r: r}

	// ORIGIN + a stored MP_REACH_NLRI (AFI 2 / SAFI 1, 16-byte next hop, one /32).
	mpValue := []byte{0x00, 0x02, 0x01, 0x10}
	mpValue = append(mpValue,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x00, 0x01, // next hop
		0x00,                         // reserved
		0x20, 0x20, 0x01, 0x0d, 0xb8) // 2001:db8::/32
	wireAttrs := []byte{0x40, 0x01, 0x01, 0x00}
	wireAttrs = append(wireAttrs, 0x80, 0x0e, byte(len(mpValue)))
	wireAttrs = append(wireAttrs, mpValue...)
	require.Equal(t, 1, countAttr(t, wireAttrs, attribute.AttrMPReachNLRI), "fixture guard")

	wn, err := nlri.NewWireNLRI(family.IPv6Unicast, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}, false)
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		Wire:    attribute.NewAttributesWire(wireAttrs, 0),
	}

	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.MustParseAddr("2001:db8::1"), false, false, true, false, 65000)
	require.NotNil(t, update)

	assert.Equal(t, 1, countAttr(t, update.PathAttributes, attribute.AttrMPReachNLRI),
		"a relayed IPv6 UPDATE must carry exactly one MP_REACH_NLRI")
}
