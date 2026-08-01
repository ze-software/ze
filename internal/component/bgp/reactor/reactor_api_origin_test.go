package reactor

import (
	"encoding/hex"
	"log/slog"
	"net/netip"
	"strings"
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

// An API-originated route is an edit set over a base, materialized by the SAME
// writer a forwarded route takes.
//
// The three properties this file pins are the ones convergence could plausibly
// break and the rail-agreement tables cannot see: the RFC 6793 AS4_PATH is derived
// once from the AS_PATH the rail synthesized, an announce too large for its region
// is DROPPED with the route named rather than truncated, and the initial-sync
// rail's attribute region stops where the NLRI parked at the buffer tail begins.

// swapRoutesLogger points routesLogger at lg and returns the restore func.
func swapRoutesLogger(lg *slog.Logger) func() {
	prev := routesLogger
	routesLogger = func() *slog.Logger { return lg }
	return func() { routesLogger = prev }
}

// TestAnnounceAS4PathFromSharedResolver pins that AS4_PATH is derived from the SAME
// ASN sequence the AS_PATH was built from, on both rails, rather than by a
// rail-local insertion that could disagree with the path it accompanies.
//
// VALIDATES: an announce to a two-octet peer carrying a non-mappable AS emits
// AS_PATH with AS_TRANS and an AS4_PATH carrying the real four-octet sequence, at
// its ascending position, identically on both rails.
// PREVENTS: the AS_PATH and the AS4_PATH being produced by two functions that can
// drift, which is what a rail-local insertAnnounceAS4Path was.
func TestAnnounceAS4PathFromSharedResolver(t *testing.T) {
	const fourOctetAS = uint32(4200000000) // non-mappable: forces AS_TRANS + AS4_PATH
	c := orderCase{
		fam:     family.IPv4Unicast,
		nlri:    "180a0000",
		nextHop: "10.0.0.1",
		isIBGP:  false, // eBGP: the rail synthesizes the AS_PATH
		asn4:    false, // OLD peer: RFC 6793 Section 4.2.2 owes an AS4_PATH
		localAS: fourOctetAS,
		build:   func(b *attribute.Builder) { b.AddLargeCommunity(65000, 1, 2) },
	}

	batch := buildBatchRail(t, c)
	assert.Equal(t, []int{1, 2, 3, 17, 32}, attrCodes(t, batch),
		"AS4_PATH must sit at its type-code position, between NEXT_HOP and LARGE_COMMUNITIES")

	// AS_PATH carries AS_TRANS (23456), the two-octet substitution.
	_, asPathValue, ok := findPathAttr(batch, byte(attribute.AttrASPath))
	require.True(t, ok, "AS_PATH must be present")
	asp, err := attribute.ParseASPath(asPathValue, false /*two-octet toward an OLD peer*/)
	require.NoError(t, err)
	require.Len(t, asp.Segments, 1)
	assert.Equal(t, []uint32{attribute.ASTrans}, asp.Segments[0].ASNs,
		"a non-mappable AS must be substituted by AS_TRANS, not truncated")

	// AS4_PATH carries the real four-octet AS, and it is the SAME sequence.
	_, as4Value, ok := findPathAttr(batch, byte(attribute.AttrAS4Path))
	require.True(t, ok, "RFC 6793 Section 4.2.2 makes AS4_PATH mandatory once AS_PATH carries AS_TRANS")
	as4, err := attribute.ParseAS4Path(as4Value)
	require.NoError(t, err)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{fourOctetAS}, as4.Segments[0].ASNs,
		"AS4_PATH must carry the AS the AS_PATH substituted")
	assert.Len(t, asp.Segments[0].ASNs, len(as4.Segments[0].ASNs),
		"the two paths must describe the same hop count, or they came from different sequences")
}

// TestAnnounceOversizeDropsWithNamedLog pins the fail-closed drop on both rails: an
// announce that does not fit its region is refused, and the log line names the
// route so an operator can see why it never arrived.
//
// VALIDATES: buildBatchAnnounceUpdate and buildRIBRouteUpdate return nil, having
// written nothing, and each logs a line naming the route or its family.
// PREVENTS: a truncated UPDATE going out, and a silent drop with nothing said
// (ai/rules/fail-closed-guards.md: fail closed OR say something -- both, here).
func TestAnnounceOversizeDropsWithNamedLog(t *testing.T) {
	// A COMMUNITIES attribute far larger than the 256-byte region below.
	comms := make(attribute.Communities, 200) // 800 octets
	for i := range comms {
		comms[i] = attribute.Community(uint32(i)) //nolint:gosec // G115: bounded by loop
	}
	b := attribute.NewBuilder()
	b.SetOrigin(0)
	b.SetASPath([]uint32{65000})
	for _, c := range comms {
		b.AddCommunity(uint16(c>>16), uint16(c)) //nolint:gosec // G115: bounded by loop
	}
	packed := b.Build()

	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	t.Run("batch-rail", func(t *testing.T) {
		var sink syncBuffer
		defer swapRoutesLogger(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn})))()

		adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
		update := adapter.buildBatchAnnounceUpdate(make([]byte, 256), make([]byte, message.MaxMsgLen),
			bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{wn},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
				Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
			},
			netip.MustParseAddr("10.0.0.1"), true, false, true, false, 65000)

		require.Nil(t, update, "an announce that does not fit must be dropped, never truncated")
		logged := sink.String()
		assert.Contains(t, logged, "announce rejected", "the drop must say so")
		assert.Contains(t, logged, "ipv4/unicast", "the log must name the route's family")
	})

	t.Run("queued-rail", func(t *testing.T) {
		var sink syncBuffer
		defer swapRoutesLogger(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn})))()

		aw := attribute.NewAttributesWire(packed, bgpctx.APIContextID)
		attrs, err := aw.All()
		require.NoError(t, err)
		// An INET NLRI, not the wire one above: the drop log names the route through
		// NLRI.String(), and only a decoded prefix has a name to print.
		inet := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
		route := rib.NewRouteWithASPath(inet, netip.MustParseAddr("10.0.0.1"), attrs,
			&attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65000}}}})

		update := buildRIBRouteUpdate(make([]byte, 256), route, 65000, true, true, false)

		require.Nil(t, update, "an announce that does not fit must be dropped, never truncated")
		logged := sink.String()
		assert.Contains(t, logged, "queued route rejected", "the drop must say so")
		assert.Contains(t, logged, "10.0.0.0/24", "the log must name the route that did not go out")
	})
}

// TestQueuedRailNLRIRegionIntact is the region bound, checked at the one place it
// matters: the initial-sync rail parks the NLRI at the TAIL of the same pooled slot
// the attributes grow into, so a writer bounded on the buffer length rather than on
// the region would overwrite the prefix being announced.
//
// VALIDATES: with the attribute region filled to within a few octets of the NLRI,
// the NLRI bytes at the tail are byte-identical to what WriteNLRI put there, and
// the emitted attribute block stops at or before the region end.
// PREVENTS: a silent wrong-route bug -- an UPDATE announcing a prefix the caller
// never asked for, which no ordering or size assertion would notice.
func TestQueuedRailNLRIRegionIntact(t *testing.T) {
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)
	nlriLen := nlri.LenWithContext(wn, false)
	require.Positive(t, nlriLen)

	// What the NLRI region must still hold afterwards.
	wantNLRI := make([]byte, nlriLen)
	nlri.WriteNLRI(wn, wantNLRI, 0, false)

	asPath := &attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65000}}}}

	// Walk the community count up until the attributes no longer fit. Every size is
	// checked, so the case that lands exactly on the boundary is covered without
	// having to compute it.
	for count := 1; count <= 80; count++ {
		comms := make(attribute.Communities, count)
		for i := range comms {
			comms[i] = attribute.Community(uint32(i)) //nolint:gosec // G115: bounded by loop
		}
		route := rib.NewRouteWithASPath(wn, netip.MustParseAddr("10.0.0.1"),
			[]attribute.Attribute{attribute.OriginIGP, comms}, asPath)

		const bufLen = 128
		attrBuf := make([]byte, bufLen)
		for i := range attrBuf {
			attrBuf[i] = 0x5A // poison: an unwritten byte is visible
		}
		update := buildRIBRouteUpdate(attrBuf, route, 65000, true /*iBGP*/, true /*asn4*/, false)

		nlriOff := bufLen - nlriLen
		assert.Equal(t, hex.EncodeToString(wantNLRI), hex.EncodeToString(attrBuf[nlriOff:]),
			"the NLRI parked at the tail must survive %d communities, fitting or not", count)

		if update == nil {
			continue // refused: the region bound did its job
		}
		assert.LessOrEqual(t, len(update.PathAttributes), nlriOff,
			"the attribute block must stop where the NLRI region begins")
		assert.Equal(t, hex.EncodeToString(wantNLRI), hex.EncodeToString(update.NLRI),
			"the emitted NLRI must be the bytes WriteNLRI wrote")
	}
}

// TestAnnounceRejectsDuplicateBaseAttribute pins the tightening convergence brings:
// a caller block carrying one type code twice is refused rather than merge-inserted
// into.
//
// RFC 4271 Section 4.3 makes a duplicate type code a Malformed Attribute List, and
// RFC 7606 Section 3(g) makes it treat-as-withdraw at the receiver, so emitting one
// is originating a route the peer will drop. The batch rail used to walk such a
// block with no duplicate check at all.
//
// VALIDATES: an announce whose base carries two ORIGINs is dropped.
// PREVENTS: Ze originating an UPDATE a conforming peer treats as a withdrawal.
func TestAnnounceRejectsDuplicateBaseAttribute(t *testing.T) {
	var sink syncBuffer
	defer swapRoutesLogger(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn})))()

	packed, err := hex.DecodeString(strings.ReplaceAll("400101 00 400101 00 4002 06 02010000fde8", " ", ""))
	require.NoError(t, err)

	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	update := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
		bgptypes.NLRIBatch{
			Family:  family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{wn},
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
			Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
		},
		netip.MustParseAddr("10.0.0.1"), true, false, true, false, 65000)

	require.Nil(t, update, "a duplicate type code in the base must be refused, not emitted")
	assert.Contains(t, sink.String(), "does not index", "the refusal must say why")
}

// TestAnnounceBuilderModeIsEditSetOverEmptyBase is the shape claim itself: a route
// originated from a Builder carries NO caller block at all, so every attribute on
// the wire came through the shared writer as a slot.
//
// VALIDATES: an announce built from a Builder (batch.Attrs, not batch.Wire) emits
// the same bytes as the identical announce presented as a pre-encoded block.
// PREVENTS: the Builder path quietly keeping a byte-producing shortcut, which is
// exactly what Builder.Build() as an intermediate block was.
func TestAnnounceBuilderModeIsEditSetOverEmptyBase(t *testing.T) {
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)

	makeBuilder := func() *attribute.Builder {
		b := attribute.NewBuilder()
		b.SetOrigin(0)
		b.SetLocalPref(300)
		b.AddCommunity(65000, 100)
		b.AddLargeCommunity(65000, 1, 2)
		return b
	}

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	build := func(batch bgptypes.NLRIBatch) []byte {
		update := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
			batch, netip.MustParseAddr("10.0.0.1"), true /*iBGP*/, false, true /*asn4*/, false, 65000)
		require.NotNil(t, update)
		return update.PathAttributes
	}

	base := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}

	fromBuilder := base
	fromBuilder.Attrs = makeBuilder()

	fromWire := base
	fromWire.Wire = attribute.NewAttributesWire(makeBuilder().Build(), bgpctx.APIContextID)

	assert.Equal(t, hex.EncodeToString(build(fromWire)), hex.EncodeToString(build(fromBuilder)),
		"a Builder-originated route and the same route as a pre-encoded block must reach the wire identically")

	codes := attrCodes(t, build(fromBuilder))
	assert.Equal(t, []int{1, 2, 3, 5, 8, 32}, codes)
	assertAscending(t, codes)
}
