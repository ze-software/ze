package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// Attribute PRESERVATION across the two announce rails.
//
// reactor_api_batch_attr_order_test.go pins the ORDER the two rails emit, and its
// TestAnnounceRailsAgreeByteForByte pins that they agree. It could not see this
// defect: every case in its table carries only attributes the queued rail's type
// switch listed, so the switch's DEFAULT behavior -- silently dropping anything
// else -- was never exercised.
//
// buildRIBRouteUpdate gated each stored optional attribute through a type switch
// naming eight types. *attribute.AIGP (code 26, RFC 7311, produced by
// Builder.SetAIGP) and OpaqueAttribute (what EVERY unknown transitive attribute
// decodes to, attribute/wire.go) matched neither case and were dropped. The batch
// rail copies the caller's block verbatim and kept them. Which rail runs is
// decided by Peer.ShouldQueue(), i.e. by scheduling -- so the same route reached
// the peer with or without its AIGP depending on whether the destination had
// finished its initial sync.
//
// Dropping an unrecognized TRANSITIVE attribute is also a conformance defect in
// its own right: RFC 4271 Section 5 requires a speaker to pass one on.
//
// These tests live in their own file for the same reason the capacity tests do:
// the ordering file's cases carry `RFC requirement:` tags for RFC 4271 Section 5
// attribute ordering, and this is an attribute-preservation concern, not an
// ordering one.

// preserveCase is one announce carrying attributes the queued rail used to drop.
type preserveCase struct {
	name      string
	fam       family.Family
	nlriHex   string
	nextHop   string
	isIBGP    bool
	packedHex string // the caller's attribute block, verbatim wire bytes
	wantCodes []int
}

// unknownTransitiveAttr is an optional-transitive attribute with an unassigned
// type code, which AttributesWire.All decodes to an OpaqueAttribute. 40 is chosen
// so it sorts above AIGP (26) and LARGE_COMMUNITIES (32), proving the last range
// [17,255) carries an attribute this builder has never heard of.
const unknownTransitiveAttr = "c02804deadbeef" // flags C0, code 40 (0x28), len 4

// aigpMetric1234 is an AIGP (RFC 7311) carrying the metric TLV for 1234.
const aigpMetric1234 = "c01a0b" + "01000b" + "00000000000004d2" // flags C0, code 26, len 11

func preserveCases() []preserveCase {
	const originIGP = "400101 00"
	const asPath65000 = "4002 06 02010000fde8" // AS_SEQUENCE [65000], 4-octet
	const community = "c0080465000064"         // COMMUNITIES 65000:100

	return []preserveCase{
		{
			// AIGP (26) between MP_REACH (14) and the unknown transitive (40).
			name:      "aigp-and-unknown-transitive-ipv6",
			fam:       family.IPv6Unicast,
			nlriHex:   "202001 0db8",
			nextHop:   "2001:db8::1",
			isIBGP:    true,
			packedHex: originIGP + asPath65000 + community + aigpMetric1234 + unknownTransitiveAttr,
			wantCodes: []int{1, 2, 5, 8, 14, 26, 40},
		},
		{
			// The same on IPv4 unicast, where there is no MP_REACH to anchor the
			// middle of the block.
			name:      "aigp-and-unknown-transitive-ipv4",
			fam:       family.IPv4Unicast,
			nlriHex:   "180a0000",
			nextHop:   "10.0.0.1",
			isIBGP:    true,
			packedHex: originIGP + asPath65000 + community + aigpMetric1234 + unknownTransitiveAttr,
			wantCodes: []int{1, 2, 3, 5, 8, 26, 40},
		},
	}
}

// buildPreserveBatchRail encodes one preserveCase through buildBatchAnnounceUpdate.
func buildPreserveBatchRail(t *testing.T, c preserveCase) []byte {
	t.Helper()
	wireNLRI, err := hex.DecodeString(stripSpaces(c.nlriHex))
	require.NoError(t, err)
	wn, err := nlri.NewWireNLRI(c.fam, wireNLRI, false)
	require.NoError(t, err)

	packed, err := hex.DecodeString(stripSpaces(c.packedHex))
	require.NoError(t, err)

	batch := bgptypes.NLRIBatch{
		Family:  c.fam,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(c.nextHop)),
		Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
	}

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	update := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
		batch, netip.MustParseAddr(c.nextHop), c.isIBGP, false /*rsClient*/, true /*asn4*/, false /*addPath*/, 65000)
	require.NotNil(t, update)
	return update.PathAttributes
}

// buildPreserveQueuedRail encodes the SAME preserveCase through
// buildRIBRouteUpdate, exactly as AnnounceNLRIBatch's queue path does: the wire
// block is decoded with All() and the resulting attributes are stored on the
// rib.Route.
func buildPreserveQueuedRail(t *testing.T, c preserveCase) []byte {
	t.Helper()
	wireNLRI, err := hex.DecodeString(stripSpaces(c.nlriHex))
	require.NoError(t, err)
	wn, err := nlri.NewWireNLRI(c.fam, wireNLRI, false)
	require.NoError(t, err)

	packed, err := hex.DecodeString(stripSpaces(c.packedHex))
	require.NoError(t, err)
	aw := attribute.NewAttributesWire(packed, bgpctx.APIContextID)
	attrs, err := aw.All()
	require.NoError(t, err)

	// Mirror AnnounceNLRIBatch's queue path exactly: it takes the caller's AS_PATH
	// from the wire block and hands the whole attribute to buildBatchASPathAttr.
	// Passing nil here instead would let the harness synthesize a path the
	// production queue never builds, and the rails comparison would be measuring
	// the harness.
	var userASPath *attribute.ASPath
	if asPathAttr, err := aw.Get(attribute.AttrASPath); err == nil {
		if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
			userASPath = asp
		}
	}

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	asPath := adapter.buildBatchASPathAttr(userASPath, 0, c.isIBGP, false /*rsClient*/, 65000)
	route := rib.NewRouteWithASPath(wn, netip.MustParseAddr(c.nextHop), attrs, asPath)

	update := buildRIBRouteUpdate(make([]byte, message.MaxMsgLen), route, 65000, c.isIBGP, true /*asn4*/, false /*addPath*/)
	require.NotNil(t, update)
	return update.PathAttributes
}

// TestAnnounceRailsPreserveUnlistedAttributes is the invariant
// TestAnnounceRailsAgreeByteForByte was meant to establish, driven with the
// attributes that table could not reach.
//
// VALIDATES: an AIGP and an unknown transitive attribute survive BOTH rails, at
// their type-code position, and the two rails encode them to identical bytes.
// PREVENTS: the queued rail silently dropping any attribute its type switch does
// not name -- one route encoding two ways according to Peer.ShouldQueue(), and an
// unrecognized transitive attribute discarded against RFC 4271 Section 5.
func TestAnnounceRailsPreserveUnlistedAttributes(t *testing.T) {
	for _, c := range preserveCases() {
		t.Run(c.name, func(t *testing.T) {
			batch := buildPreserveBatchRail(t, c)
			queued := buildPreserveQueuedRail(t, c)

			assert.Equal(t, c.wantCodes, attrCodes(t, batch), "batch rail must carry every attribute")
			assert.Equal(t, c.wantCodes, attrCodes(t, queued), "queued rail must carry every attribute")
			assertAscending(t, attrCodes(t, queued))

			assert.Equal(t, hex.EncodeToString(queued), hex.EncodeToString(batch),
				"the two announce rails must encode the same route to the same bytes")
		})
	}
}

// TestAnnounceRailsPreserveMultiSegmentASPath is the THIRD way the two rails
// disagreed about the same route, and the one with routing consequences.
//
// AnnounceNLRIBatch used to store `asp.Segments[0].ASNs` for the queue, so a
// caller-supplied AS_PATH reached a queued peer as ONE AS_SEQUENCE: an AS_SET
// (RFC 4271 Section 5.1.2, what aggregation produces) became a sequence, and every
// segment after the first vanished. The established rail copies the block
// verbatim. A flattened AS_SET is not cosmetic -- an AS_SET counts as ONE hop for
// best-path length (Section 9.1.2.2) whereas the sequence it became counts as N,
// and the dropped segments take their ASNs out of loop detection entirely.
//
// It is reachable: `update ... attr set <hex>` hands arbitrary attribute bytes
// straight through as batch.Wire (plugins/cmd/update/update_wire.go
// parseWireAttrSection), multi-segment AS_PATH included.
//
// VALIDATES: a two-segment AS_PATH (AS_SEQUENCE then AS_SET) survives the queued
// rail intact, and both rails encode it identically -- including the eBGP prepend,
// which must go in front as a new segment rather than into the existing one.
// PREVENTS: an AS_SET silently becoming an AS_SEQUENCE, and trailing segments
// being dropped, whenever the destination peer happens to still be syncing.
func TestAnnounceRailsPreserveMultiSegmentASPath(t *testing.T) {
	// AS_PATH: AS_SEQUENCE [65001] then AS_SET [65002 65003], 4-octet ASNs.
	// Value length 0x10 = 6 (sequence: type+count+1 ASN) + 10 (set: type+count+2 ASNs).
	const multiSegASPath = "400210" + "0201 0000fde9" + "0102 0000fdea 0000fdeb"
	c := preserveCase{
		name:      "as-set-preserved-ebgp",
		fam:       family.IPv4Unicast,
		nlriHex:   "180a0000",
		nextHop:   "10.0.0.1",
		isIBGP:    false, // eBGP: RFC 4271 Section 5.1.2 owes a prepend
		packedHex: "400101 00" + multiSegASPath,
		wantCodes: []int{1, 2, 3},
	}

	batch := buildPreserveBatchRail(t, c)
	queued := buildPreserveQueuedRail(t, c)

	assert.Equal(t, hex.EncodeToString(queued), hex.EncodeToString(batch),
		"a multi-segment AS_PATH must encode identically on both rails")

	// Decode the emitted AS_PATH and assert the shape, so the test still means
	// something if both rails were to break the same way.
	_, value, ok := findPathAttr(queued, byte(attribute.AttrASPath))
	require.True(t, ok, "AS_PATH must be present")
	asp, err := attribute.ParseASPath(value, true /*asn4*/)
	require.NoError(t, err)

	require.Len(t, asp.Segments, 2, "both segments must survive; flattening drops the second")
	assert.Equal(t, attribute.ASSequence, asp.Segments[0].Type)
	assert.Equal(t, []uint32{65000, 65001}, asp.Segments[0].ASNs, "local AS prepended to the leading sequence")
	assert.Equal(t, attribute.ASSet, asp.Segments[1].Type, "an AS_SET must not become an AS_SEQUENCE")
	assert.Equal(t, []uint32{65002, 65003}, asp.Segments[1].ASNs)
}

// TestQueuedRailPreservesUnknownTransitiveValue checks the VALUE, not just the
// type code: an attribute can be present and still be re-encoded wrongly, and a
// code-only assertion would not notice.
//
// VALIDATES: the unknown transitive attribute's flags and value bytes survive the
// queued rail unchanged.
// PREVENTS: a future "handle unknown attributes" edit that preserves the code but
// normalises the flags or truncates the value.
func TestQueuedRailPreservesUnknownTransitiveValue(t *testing.T) {
	c := preserveCases()[1] // IPv4 unicast
	queued := buildPreserveQueuedRail(t, c)

	flags, value, ok := findPathAttr(queued, 40)
	require.True(t, ok, "the unknown transitive attribute must survive the queued rail")
	assert.Equal(t, byte(0xC0), flags, "optional-transitive flags must be preserved verbatim")
	assert.Equal(t, "deadbeef", hex.EncodeToString(value), "value must be preserved verbatim")

	_, aigpValue, ok := findPathAttr(queued, byte(attribute.AttrAIGP))
	require.True(t, ok, "AIGP must survive the queued rail")
	assert.Equal(t, "01000b00000000000004d2", hex.EncodeToString(aigpValue),
		"the AIGP metric TLV must be preserved verbatim")
}
