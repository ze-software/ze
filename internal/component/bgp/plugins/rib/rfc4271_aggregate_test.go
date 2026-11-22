// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 4271 Section 5.1.7, Section 9.2.2.2, and RFC 9774 Section 5.2 route aggregation tests
// Related: aggregate.go -- parseAggregateConfig, synthesizeAggregate, aggregateCheckMED, aggregateOrigin, aggregateNextHop, aggregateBriefASPath, aggregateAtomic, aggregateOwnAggregator, aggregateAggregatorAttributes, formatAggregateAnnounceCommand, formatAggregateWithdrawCommand
// Related: internal/component/bgp/reactor/rfc4271_aggregate_wire_test.go -- end-to-end wire BGP UPDATE encoding for iBGP, eBGP, 2-byte/4-byte AGGREGATOR, and IPv6 MP_REACH_NLRI
// RFC 4271 Sections 5.1.7 and 9.2.2.2; RFC 9774 Section 5.2 -- see rfc/short/rfc4271.md and rfc/short/rfc9774.md

package rib

import (
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/store"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type aggContributorSpec struct {
	origin        attribute.Origin
	asPath        []uint32
	asPathRaw     []byte // overrides asPath when non-nil (for AS_SET boundary tests)
	nextHop       netip.Addr
	hasMED        bool
	med           uint32
	atomicAgg     bool
	aggregator    *[8]byte
	as4Aggregator *[8]byte
}

func buildContributorWireAttrs(spec aggContributorSpec) []byte {
	var out []byte
	out = append(out, byte(attribute.FlagTransitive), byte(attribute.AttrOrigin), 1, byte(spec.origin))
	asVal := spec.asPathRaw
	if asVal == nil {
		asVal = encodeASPath4(spec.asPath)
	}
	out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrASPath), asVal)
	if spec.nextHop.IsValid() && spec.nextHop.Is4() {
		nh4 := spec.nextHop.As4()
		out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrNextHop), nh4[:])
	}
	if spec.hasMED {
		var med [4]byte
		binary.BigEndian.PutUint32(med[:], spec.med)
		out = appendAggregateAttr(out, byte(attribute.FlagOptional), byte(attribute.AttrMED), med[:])
	}
	var lp [4]byte
	binary.BigEndian.PutUint32(lp[:], 100)
	out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrLocalPref), lp[:])
	if spec.atomicAgg {
		out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrAtomicAggregate), nil)
	}
	if spec.aggregator != nil {
		out = appendAggregateAttr(out, byte(attribute.FlagOptional|attribute.FlagTransitive), byte(attribute.AttrAggregator), spec.aggregator[:])
	}
	if spec.as4Aggregator != nil {
		out = appendAggregateAttr(out, byte(attribute.FlagOptional|attribute.FlagTransitive), byte(attribute.AttrAS4Aggregator), spec.as4Aggregator[:])
	}
	return out
}

type aggCapture struct {
	mu       sync.Mutex
	changes  []ribevents.BestChangeEntry
	commands []string
}

func (c *aggCapture) lastChangeFor(pfx netip.Prefix) (ribevents.BestChangeEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.changes) - 1; i >= 0; i-- {
		if c.changes[i].Prefix == pfx {
			return c.changes[i], true
		}
	}
	return ribevents.BestChangeEntry{}, false
}

func (c *aggCapture) lastCommandContaining(substr string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.commands) - 1; i >= 0; i-- {
		if strings.Contains(c.commands[i], substr) {
			return c.commands[i], true
		}
	}
	return "", false
}

func (c *aggCapture) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.changes = nil
	c.commands = nil
}

func newAggregateTestRIB(t *testing.T, localAddrs []netip.Addr, igpNextHops ...netip.Addr) (*RIBManager, *locrib.RIB, *aggCapture) {
	t.Helper()
	bus := newTestEventBus()
	prevBus := getEventBus()
	SetEventBus(bus)
	t.Cleanup(func() { SetEventBus(prevBus) })

	cap := &aggCapture{}
	ribevents.BestChange.Subscribe(bus, func(batch *ribevents.BestChangeBatch) {
		cap.mu.Lock()
		cap.changes = append(cap.changes, batch.Changes...)
		cap.mu.Unlock()
	})

	r := newTestRIBManager(t)
	loc := electionFixtureLocRIB(t, r, igpNextHops...)
	r.localAddrsFn = func() []netip.Addr { return localAddrs }
	r.updateHook = func(cmd string, _ map[string]any) {
		cap.mu.Lock()
		cap.commands = append(cap.commands, cmd)
		cap.mu.Unlock()
	}
	return r, loc, cap
}

func installAggregateTestConfig(t *testing.T, r *RIBManager, bgpCfg map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"bgp": bgpCfg})
	require.NoError(t, err)
	cfg, _, err := parseRIBConfig([]sdk.ConfigSection{{Root: "bgp", Data: string(raw)}})
	require.NoError(t, err)
	require.NoError(t, r.installConfig(cfg, false))
}

func insertSelectedRoute(t *testing.T, r *RIBManager, owner uint64, peerAddr netip.Addr, peerAS, localAS uint32, fam family.Family, pfx netip.Prefix, spec aggContributorSpec) {
	t.Helper()
	r.peerMu.Lock()
	pr := r.bgpPeers[owner]
	if pr == nil {
		var err error
		pr, err = storage.NewAddressPeerRIB(peerAddr)
		require.NoError(t, err)
		r.bgpPeers[owner] = pr
	}
	r.peerUp[owner] = true
	r.peerMeta[owner] = &peerMetadata{
		Address:  peerAddr,
		PeerASN:  peerAS,
		LocalASN: localAS,
	}
	r.peerMu.Unlock()

	attrs := buildContributorWireAttrs(spec)
	var nlriBuf [24]byte
	nlri := store.PrefixToNLRIInto(pfx, nlriBuf[:])
	pr.Insert(fam, attrs, nlri)
	if change, ok := r.checkBestPathChange(fam, nlri, false, nil); ok {
		publishBestChanges([]bestChangeEntry{change}, fam)
	}
}

func withdrawSelectedRoute(t *testing.T, r *RIBManager, owner uint64, fam family.Family, pfx netip.Prefix) {
	t.Helper()
	r.peerMu.RLock()
	pr := r.bgpPeers[owner]
	r.peerMu.RUnlock()
	require.NotNil(t, pr)
	var nlriBuf [24]byte
	nlri := store.PrefixToNLRIInto(pfx, nlriBuf[:])
	pr.Withdraw(fam, nlri)
	if change, ok := r.checkBestPathChange(fam, nlri, false, nil); ok {
		publishBestChanges([]bestChangeEntry{change}, fam)
	}
}

func countAttrs(attrs []byte, want attribute.AttributeCode) ([][]byte, int) {
	var vals [][]byte
	iter := attribute.NewAttrIterator(attrs)
	for code, _, val, ok := iter.Next(); ok; code, _, val, ok = iter.Next() {
		if code == want {
			vals = append(vals, append([]byte(nil), val...))
		}
	}
	return vals, len(vals)
}

func decodeWireASPath4(t *testing.T, attrs []byte) []uint32 {
	t.Helper()
	vals, n := countAttrs(attrs, attribute.AttrASPath)
	require.Equal(t, 1, n, "expected exactly one AS_PATH attribute")
	val := vals[0]
	var out []uint32
	off := 0
	for off < len(val) {
		require.LessOrEqual(t, off+2, len(val))
		segType := attribute.ASPathSegmentType(val[off])
		require.Equal(t, attribute.ASSequence, segType, "brief aggregate AS_PATH must contain only AS_SEQUENCE")
		count := int(val[off+1])
		off += 2
		require.LessOrEqual(t, off+count*4, len(val))
		for i := range count {
			out = append(out, binary.BigEndian.Uint32(val[off+i*4:off+i*4+4]))
		}
		off += count * 4
	}
	return out
}

// TestRFC4271RouteAggregationMEDAndOrigin proves RFC 4271 Section 9.2.2.2 MED
// non-aggregation and ORIGIN precedence (INCOMPLETE > EGP > IGP) across Loc-RIB,
// BestChange wire attributes, and emitted BGP peer UPDATE commands.
//
// RFC requirement: RFC4271-9.2.2.2-1 positive -- routes with differing MED values (50 vs 80, or present vs absent) are NOT aggregated: the aggregate is withdrawn from Loc-RIB, BestChange emits BestChangeWithdraw, and a peer withdraw UPDATE command is dispatched.
// RFC requirement: RFC4271-9.2.2.2-1 negative -- routes with identical MED (both 50, or both absent) ARE aggregated with wire AttrMED=50 (or absent), installed in Loc-RIB, published on BestChange, and dispatched as an announce UPDATE command.
// RFC requirement: RFC4271-9.2.2.2-2 positive -- when contributors include INCOMPLETE, wire ORIGIN is INCOMPLETE (for both {IGP, INCOMPLETE} and {EGP, INCOMPLETE}); when contributors have EGP and IGP (no INCOMPLETE), wire ORIGIN is EGP on both BestChange and dispatched peer UPDATE commands.
// RFC requirement: RFC4271-9.2.2.2-2 negative -- when all contributors have ORIGIN IGP, wire ORIGIN remains IGP (not promoted to EGP or INCOMPLETE) on both BestChange and dispatched peer UPDATE commands.
func TestRFC4271RouteAggregationMEDAndOrigin(t *testing.T) {
	// Distinct local interface IP (198.51.100.10) vs router-id (198.51.100.1).
	localIF := netip.MustParseAddr("198.51.100.10")
	peer1 := netip.MustParseAddr("192.0.2.1")
	r, loc, cap := newAggregateTestRIB(t, []netip.Addr{localIF}, peer1)
	aggPfx := netip.MustParsePrefix("203.0.113.0/24")
	p1 := netip.MustParsePrefix("203.0.113.0/25")
	p2 := netip.MustParsePrefix("203.0.113.128/25")

	installAggregateTestConfig(t, r, map[string]any{
		"router-id": "198.51.100.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"aggregator": "true",
			},
		},
	})

	// RFC4271-9.2.2.2-1 negative & RFC4271-9.2.2.2-2 negative: both IGP, both MED=50 -> aggregated with ORIGIN IGP and wire AttrMED=50.
	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p1, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: true, med: 50,
	})
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: true, med: 50,
	})

	locEntry, inLoc := loc.Lookup(family.IPv4Unicast, aggPfx)
	require.True(t, inLoc, "matching MED=50 aggregate must be installed in Loc-RIB")
	require.Equal(t, uint32(50), locEntry.Paths[locEntry.Best].Metric)

	bc, hasBC := cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	require.NotEqual(t, ribevents.BestChangeWithdraw, bc.Action)
	originVals, originN := countAttrs(bc.Attributes, attribute.AttrOrigin)
	require.Equal(t, 1, originN)
	require.Equal(t, byte(attribute.OriginIGP), originVals[0][0])
	medVals, medN := countAttrs(bc.Attributes, attribute.AttrMED)
	require.Equal(t, 1, medN, "matching MED=50 must appear in wire attributes")
	require.Equal(t, uint32(50), binary.BigEndian.Uint32(medVals[0]))

	cmd, hasCmd := cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "origin igp")
	require.Contains(t, cmd, "med 50")
	require.Contains(t, cmd, "nlri add 203.0.113.0/24")

	// RFC4271-9.2.2.2-2 positive ({IGP, EGP} -> EGP on wire BestChange + UPDATE).
	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginEGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: true, med: 50,
	})
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	originVals, originN = countAttrs(bc.Attributes, attribute.AttrOrigin)
	require.Equal(t, 1, originN)
	require.Equal(t, byte(attribute.OriginEGP), originVals[0][0])
	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "origin egp")

	// RFC4271-9.2.2.2-2 positive ({IGP, INCOMPLETE} -> INCOMPLETE on wire BestChange + UPDATE).
	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIncomplete, asPath: []uint32{65010}, nextHop: peer1, hasMED: true, med: 50,
	})
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	originVals, originN = countAttrs(bc.Attributes, attribute.AttrOrigin)
	require.Equal(t, 1, originN)
	require.Equal(t, byte(attribute.OriginIncomplete), originVals[0][0])
	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "origin incomplete")

	// RFC4271-9.2.2.2-2 positive ({EGP, INCOMPLETE} -> INCOMPLETE on wire BestChange + UPDATE).
	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p1, aggContributorSpec{
		origin: attribute.OriginEGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: true, med: 50,
	})
	agg, active := r.ActiveAggregate(aggPfx)
	require.True(t, active)
	require.Equal(t, attribute.OriginIncomplete, agg.Origin)

	// RFC4271-9.2.2.2-1 positive: differing MED (50 vs 80) MUST NOT be aggregated ->
	// withdrawn from Loc-RIB, BestChangeDelete emitted, peer withdraw UPDATE dispatched.
	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: true, med: 80,
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active, "routes with MED 50 vs 80 must not be aggregated")
	_, inLoc = loc.Lookup(family.IPv4Unicast, aggPfx)
	require.False(t, inLoc, "suppressed aggregate must be removed from Loc-RIB")
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	require.Equal(t, ribevents.BestChangeWithdraw, bc.Action)
	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "nlri del 203.0.113.0/24")

	// RFC4271-9.2.2.2-1 positive (present vs absent MED): MED 50 vs no MED MUST NOT be aggregated.
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: false,
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active, "routes where one has MED and one lacks MED must not be aggregated")
	_, inLoc = loc.Lookup(family.IPv4Unicast, aggPfx)
	require.False(t, inLoc)

	// RFC4271-9.2.2.2-1 negative (both absent MED): aggregated cleanly with no AttrMED on wire.
	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p1, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, hasMED: false,
	})
	_, inLoc = loc.Lookup(family.IPv4Unicast, aggPfx)
	require.True(t, inLoc)
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	_, medN = countAttrs(bc.Attributes, attribute.AttrMED)
	require.Equal(t, 0, medN)
	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.NotContains(t, cmd, " med ")
	require.Contains(t, cmd, "nlri add 203.0.113.0/24")
}

// TestRFC4271RouteAggregationNextHopAtomicAndAggregator proves RFC 4271 Section
// 9.2.2.2 NEXT_HOP (local speaker interface only, no router-id fallback, IPv4 +
// IPv6), ATOMIC_AGGREGATE propagation (zero-length wire attribute), contributor
// AGGREGATOR / AS4_AGGREGATOR exclusion, and Section 5.1.7 speaker-own
// AGGREGATOR across Loc-RIB, BestChange wire attributes, and peer UPDATE commands.
//
// RFC requirement: RFC4271-9.2.2.2-3 positive -- when contributors have different NEXT_HOP addresses, the aggregate's wire NEXT_HOP, Loc-RIB next-hop, and dispatched UPDATE nhop identify a local speaker interface (IPv4 and IPv6); a configured next-hop equal to a remote IP or a non-interface router-id is refused, and absent local interfaces suppress the aggregate.
// RFC requirement: RFC4271-9.2.2.2-3 negative -- when all contributors share the same NEXT_HOP (192.0.2.1), the aggregate preserves that shared NEXT_HOP in Loc-RIB, wire AttrNextHop, and dispatched UPDATE nhop.
// RFC requirement: RFC4271-9.2.2.2-4 positive -- when at least one contributor carries ATOMIC_AGGREGATE, the aggregate emits wire AttrAtomicAggregate of length 0 on BestChange and includes atomic-aggregate in the dispatched peer UPDATE command.
// RFC requirement: RFC4271-9.2.2.2-4 negative -- when no contributor carries ATOMIC_AGGREGATE and no AS numbers are excluded, wire AttrAtomicAggregate is omitted from BestChange and atomic-aggregate is absent from the dispatched peer UPDATE command.
// RFC requirement: RFC4271-9.2.2.2-5 positive -- contributor AGGREGATOR (type 7) and AS4_AGGREGATOR (type 18) attributes are NEVER included in BestChange wire attributes or peer UPDATE commands (zero count when aggregator=false; exactly one count matching only the speaker's own AS+IP when aggregator=true).
// RFC requirement: RFC4271-9.2.2.2-5 negative -- disabling speaker AGGREGATOR (aggregator=false) omits all AGGREGATOR attributes from BestChange and dispatched peer UPDATE commands even when contributors carry both AGGREGATOR and AS4_AGGREGATOR.
// RFC requirement: RFC4271-5.1.7-1 positive -- when aggregator=true, BestChange wire AttrAggregator (length 8) and dispatched peer UPDATE aggregator carry this speaker's own LocalAS (65000) and RouterID (198.51.100.1), never a contributor's AGGREGATOR or a distinct local interface IP.
// RFC requirement: RFC4271-5.1.7-1 negative -- when aggregator=false, no AGGREGATOR attribute is added on BestChange or dispatched peer UPDATE commands; when aggregator=true without speaker identity (local AS or router-id), no forged AGGREGATOR is emitted.
func TestRFC4271RouteAggregationNextHopAtomicAndAggregator(t *testing.T) {
	// Deliberately distinct from router-id (198.51.100.1) to prevent confounding!
	localIFv4 := netip.MustParseAddr("198.51.100.10")
	localIFv6 := netip.MustParseAddr("2001:db8:fe::10")
	routerID := netip.MustParseAddr("198.51.100.1")
	peer1 := netip.MustParseAddr("192.0.2.1")
	peer2 := netip.MustParseAddr("192.0.2.2")
	peerV6a := netip.MustParseAddr("2001:db8::1")
	peerV6b := netip.MustParseAddr("2001:db8::2")

	r, loc, cap := newAggregateTestRIB(t, []netip.Addr{localIFv4, localIFv6}, peer1, peer2, peerV6a, peerV6b)
	aggPfx := netip.MustParsePrefix("198.18.0.0/23")
	p1 := netip.MustParsePrefix("198.18.0.0/24")
	p2 := netip.MustParsePrefix("198.18.1.0/24")

	var foreignAgg [8]byte
	binary.BigEndian.PutUint32(foreignAgg[0:4], 65111)
	copy(foreignAgg[4:8], []byte{192, 0, 2, 111})
	var foreignAS4Agg [8]byte
	binary.BigEndian.PutUint32(foreignAS4Agg[0:4], 4200000111)
	copy(foreignAS4Agg[4:8], []byte{192, 0, 2, 112})

	// Phase 1: aggregator=false, same NEXT_HOP (peer1), no ATOMIC_AGGREGATE, contributors carry BOTH AGGREGATOR and AS4_AGGREGATOR.
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": routerID.String(),
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"aggregator": "false",
			},
		},
	})

	cap.reset()
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p1, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, aggregator: &foreignAgg, as4Aggregator: &foreignAS4Agg,
	})
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer1, aggregator: &foreignAgg,
	})

	// RFC4271-9.2.2.2-3 negative: shared NEXT_HOP preserved on Loc-RIB, BestChange wire AttrNextHop, and UPDATE command.
	best, inLoc := loc.Lookup(family.IPv4Unicast, aggPfx)
	require.True(t, inLoc)
	require.Equal(t, peer1, best.Paths[best.Best].NextHop)

	bc, hasBC := cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	nhVals, nhN := countAttrs(bc.Attributes, attribute.AttrNextHop)
	require.Equal(t, 1, nhN)
	require.Equal(t, peer1.AsSlice(), nhVals[0])
	// RFC4271-9.2.2.2-4 negative: no ATOMIC_AGGREGATE on wire or UPDATE.
	_, atomicN := countAttrs(bc.Attributes, attribute.AttrAtomicAggregate)
	require.Equal(t, 0, atomicN)
	// RFC4271-9.2.2.2-5 & 5.1.7-1 negative: zero AGGREGATOR and zero AS4_AGGREGATOR on wire or UPDATE.
	_, aggN := countAttrs(bc.Attributes, attribute.AttrAggregator)
	require.Equal(t, 0, aggN, "contributor AGGREGATOR must not leak when aggregator=false")
	_, as4AggN := countAttrs(bc.Attributes, attribute.AttrAS4Aggregator)
	require.Equal(t, 0, as4AggN, "contributor AS4_AGGREGATOR must not leak")

	cmd, hasCmd := cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "nhop 192.0.2.1")
	require.NotContains(t, cmd, "atomic-aggregate")
	require.NotContains(t, cmd, "aggregator ")

	// Phase 2: move p2 to peer2 (different NEXT_HOP) AND set ATOMIC_AGGREGATE on p2 ->
	// NEXT_HOP switches to localIFv4 (198.51.100.10, NOT routerID 198.51.100.1!),
	// and wire AttrAtomicAggregate (length 0) + UPDATE atomic-aggregate appear.
	cap.reset()
	withdrawSelectedRoute(t, r, 1, family.IPv4Unicast, p2)
	insertSelectedRoute(t, r, 2, peer2, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peer2, atomicAgg: true, aggregator: &foreignAgg,
	})

	best, inLoc = loc.Lookup(family.IPv4Unicast, aggPfx)
	require.True(t, inLoc)
	require.Equal(t, localIFv4, best.Paths[best.Best].NextHop, "NEXT_HOP must be speaker's local interface, not router-id")

	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	nhVals, nhN = countAttrs(bc.Attributes, attribute.AttrNextHop)
	require.Equal(t, 1, nhN)
	require.Equal(t, localIFv4.AsSlice(), nhVals[0])
	atomicVals, atomicN := countAttrs(bc.Attributes, attribute.AttrAtomicAggregate)
	require.Equal(t, 1, atomicN, "ATOMIC_AGGREGATE must be present on wire BestChange")
	require.Empty(t, atomicVals[0], "RFC 4271 Section 4.3: ATOMIC_AGGREGATE wire attribute must have length 0")
	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "nhop 198.51.100.10")
	require.Contains(t, cmd, "atomic-aggregate")

	// Phase 3: RFC4271-9.2.2.2-3 positive refusal cases:
	// (a) explicit next-hop equal to non-interface router-id (198.51.100.1) MUST be refused!
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": routerID.String(),
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"next-hop":   routerID.String(),
				"aggregator": "true",
			},
		},
	})
	_, active := r.ActiveAggregate(aggPfx)
	require.False(t, active, "non-interface router-id must be rejected as aggregate next-hop")

	// (b) explicit remote next-hop (192.0.2.99) MUST be refused!
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": routerID.String(),
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"next-hop":   "192.0.2.99",
				"aggregator": "true",
			},
		},
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active, "remote next-hop must be rejected when contributors have different NEXT_HOPs")

	// (c) zero local interfaces (despite valid router-id) MUST refuse instead of falling back to router-id!
	r.localAddrsFn = func() []netip.Addr { return nil }
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": routerID.String(),
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"aggregator": "true",
			},
		},
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active, "router-id must never be used as NEXT_HOP fallback when absent from localAddrs")
	r.localAddrsFn = func() []netip.Addr { return []netip.Addr{localIFv4, localIFv6} }

	// Phase 4: RFC4271-5.1.7-1 positive & RFC4271-9.2.2.2-5 positive:
	// aggregator=true emits EXACTLY ONE AttrAggregator carrying speaker's own (65000, 198.51.100.1)
	// and ZERO contributor AGGREGATOR / AS4_AGGREGATOR attributes on wire BestChange AND UPDATE command.
	cap.reset()
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": routerID.String(),
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"next-hop":   localIFv4.String(),
				"aggregator": "true",
			},
		},
	})
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	aggVals, aggN := countAttrs(bc.Attributes, attribute.AttrAggregator)
	require.Equal(t, 1, aggN, "must carry exactly ONE AGGREGATOR (speaker's own, no contributor leak)")
	require.Len(t, aggVals[0], 8)
	require.Equal(t, uint32(65000), binary.BigEndian.Uint32(aggVals[0][0:4]))
	require.Equal(t, routerID.AsSlice(), aggVals[0][4:8])
	_, as4AggN = countAttrs(bc.Attributes, attribute.AttrAS4Aggregator)
	require.Equal(t, 0, as4AggN, "contributor AS4_AGGREGATOR must be stripped")

	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "aggregator 65000:198.51.100.1")
	require.Contains(t, cmd, "nhop 198.51.100.10")
	require.NotContains(t, cmd, "65111")

	// Phase 5: RFC4271-5.1.7-1 negative (aggregator=true without speaker identity suppresses instead of forging).
	installAggregateTestConfig(t, r, map[string]any{
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":     aggPfx.String(),
				"next-hop":   localIFv4.String(),
				"aggregator": "true",
			},
		},
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active, "aggregator=true without speaker local-as/router-id must not forge AGGREGATOR")

	// Phase 6: IPv6 aggregate with differing contributor next-hops resolves to localIFv6
	// and dispatches `nhop 2001:db8:fe::10 mp ipv6/unicast add 2001:db8:100::/48`.
	aggV6 := netip.MustParsePrefix("2001:db8:100::/48")
	v6a := netip.MustParsePrefix("2001:db8:100:1::/64")
	v6b := netip.MustParsePrefix("2001:db8:100:2::/64")

	cap.reset()
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": routerID.String(),
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggV6.String(): map[string]any{
				"prefix":     aggV6.String(),
				"aggregator": "true",
			},
		},
	})
	insertSelectedRoute(t, r, 11, peerV6a, 65010, 65000, family.IPv6Unicast, v6a, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peerV6a,
	})
	insertSelectedRoute(t, r, 12, peerV6b, 65010, 65000, family.IPv6Unicast, v6b, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010}, nextHop: peerV6b,
	})
	bestV6, inLocV6 := loc.Lookup(family.IPv6Unicast, aggV6)
	require.True(t, inLocV6)
	require.Equal(t, localIFv6, bestV6.Paths[bestV6.Best].NextHop)
	cmdV6, hasCmdV6 := cap.lastCommandContaining(aggV6.String())
	require.True(t, hasCmdV6)
	require.Contains(t, cmdV6, "nhop 2001:db8:fe::10")
	require.Contains(t, cmdV6, "mp ipv6/unicast add 2001:db8:100::/48")

	// Verify replayBestPaths (collectBestPaths) includes active aggregates.
	replayed := r.collectBestPaths()
	foundReplayV6 := false
	for _, entry := range replayed[family.IPv6Unicast] {
		if entry.Prefix == aggV6 {
			foundReplayV6 = true
		}
	}
	require.True(t, foundReplayV6, "collectBestPaths must replay active aggregate routes")
}

// TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS proves
// RFC 9774 Section 5.2 consistent brief aggregation: truncating the wire AS_PATH
// after the right-most instance of the configured brief-origin-as (even when
// shared trailing ASes follow it, and stably across single vs multiple
// contributors), stopping at non-AS_SEQUENCE boundaries, and supporting the
// aggregating speaker's own local AS.
//
// RFC requirement: RFC9774-5.2-1 positive -- with brief-origin-as=65010 on contributors sharing [65010, 65020, 65010, 65030, ...], wire AS_PATH on BestChange and dispatched peer UPDATE truncates after the right-most 65010 (removing shared trailing 65030) to [65010, 65020, 65010] with ATOMIC_AGGREGATE, stays identical when one contributor withdraws, stops at AS_SET boundaries, and yields empty AS_PATH when brief-origin-as equals local AS 65000.
// RFC requirement: RFC9774-5.2-1 negative -- when brief-origin-as is absent from the common leading AS_SEQUENCE (65999) or lies behind an AS_SET segment, aggregation is suppressed; without brief-origin-as, ordinary brief aggregation keeps the full shared trailing AS [65010, 65020, 65010, 65030].
func TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS(t *testing.T) {
	localIF := netip.MustParseAddr("198.51.100.10")
	peer1 := netip.MustParseAddr("192.0.2.1")
	r, loc, cap := newAggregateTestRIB(t, []netip.Addr{localIF}, peer1)
	aggPfx := netip.MustParsePrefix("198.51.100.0/24")
	p1 := netip.MustParsePrefix("198.51.100.0/25")
	p2 := netip.MustParsePrefix("198.51.100.128/25")

	// Contributors share [65010, 65020, 65010, 65030] and diverge at 65041 vs 65042.
	// Notice 65030 follows the right-most 65010 in BOTH contributors, so ordinary
	// brief aggregation produces [65010, 65020, 65010, 65030] while consistent brief
	// aggregation with brief-origin-as=65010 MUST truncate 65030 and produce [65010, 65020, 65010]!
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p1, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010, 65020, 65010, 65030, 65041}, nextHop: peer1,
	})
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010, 65020, 65010, 65030, 65042}, nextHop: peer1,
	})

	// Negative 1 (without brief-origin-as): ordinary brief keeps shared trailing 65030.
	cap.reset()
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": "198.51.100.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix": aggPfx.String(),
			},
		},
	})
	bc, hasBC := cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	require.Equal(t, []uint32{65010, 65020, 65010, 65030}, decodeWireASPath4(t, bc.Attributes))

	// Positive 1: brief-origin-as=65010 truncates after the RIGHT-MOST 65010,
	// dropping shared trailing 65030 -> wire AS_PATH == [65010, 65020, 65010] + ATOMIC_AGGREGATE!
	cap.reset()
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": "198.51.100.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":          aggPfx.String(),
				"brief-origin-as": "65010",
			},
		},
	})
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	require.Equal(t, []uint32{65010, 65020, 65010}, decodeWireASPath4(t, bc.Attributes))
	atomicVals, atomicN := countAttrs(bc.Attributes, attribute.AttrAtomicAggregate)
	require.Equal(t, 1, atomicN)
	require.Empty(t, atomicVals[0])
	cmd, hasCmd := cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.Contains(t, cmd, "as-path [65010 65020 65010]")
	require.Contains(t, cmd, "atomic-aggregate")

	// Positive 2 (RFC 9774 Appendix B stability): withdrawing p2 leaves single
	// contributor [65010, 65020, 65010, 65030, 65041]; aggregate STILL truncates
	// to [65010, 65020, 65010] (consistent origin AS 65010!).
	withdrawSelectedRoute(t, r, 1, family.IPv4Unicast, p2)
	agg, active := r.ActiveAggregate(aggPfx)
	require.True(t, active)
	require.Equal(t, []uint32{65010, 65020, 65010}, agg.ASPath)
	require.True(t, agg.AtomicAggregate)

	// Re-insert p2 for remaining checks.
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPath: []uint32{65010, 65020, 65010, 65030, 65042}, nextHop: peer1,
	})

	// Positive 3: brief-origin-as == local AS (65000) -> empty wire AS_PATH
	// (so eBGP advertisement prepends local AS 65000 as the origin AS) + ATOMIC_AGGREGATE.
	cap.reset()
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": "198.51.100.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":          aggPfx.String(),
				"brief-origin-as": "65000",
			},
		},
	})
	bc, hasBC = cap.lastChangeFor(aggPfx)
	require.True(t, hasBC)
	require.Empty(t, decodeWireASPath4(t, bc.Attributes))
	_, atomicN = countAttrs(bc.Attributes, attribute.AttrAtomicAggregate)
	require.Equal(t, 1, atomicN)
	cmd, hasCmd = cap.lastCommandContaining(aggPfx.String())
	require.True(t, hasCmd)
	require.NotContains(t, cmd, "as-path")
	require.Contains(t, cmd, "atomic-aggregate")

	// Negative 2: brief-origin-as not in common leading path (65999) -> aggregate suppressed.
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": "198.51.100.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":          aggPfx.String(),
				"brief-origin-as": "65999",
			},
		},
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active)
	_, inLoc := loc.Lookup(family.IPv4Unicast, aggPfx)
	require.False(t, inLoc)

	// Negative 3: non-AS_SEQUENCE boundary (AS_SEQUENCE[65010] + AS_SET{65099} + AS_SEQUENCE[65020]).
	// parseASPath4ForAggregate MUST stop at AS_SET so 65020 behind AS_SET is NOT treated as part of
	// the leading sequence.
	var rawSetPath []byte
	rawSetPath = append(rawSetPath, byte(attribute.ASSequence), 1, 0, 0, 0xfe, 0x0a) // 65010
	rawSetPath = append(rawSetPath, byte(attribute.ASSet), 1, 0, 0, 0xfe, 0x4b)      // {65099}
	rawSetPath = append(rawSetPath, byte(attribute.ASSequence), 1, 0, 0, 0xfe, 0x14) // 65020
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p1, aggContributorSpec{
		origin: attribute.OriginIGP, asPathRaw: rawSetPath, nextHop: peer1,
	})
	insertSelectedRoute(t, r, 1, peer1, 65010, 65000, family.IPv4Unicast, p2, aggContributorSpec{
		origin: attribute.OriginIGP, asPathRaw: rawSetPath, nextHop: peer1,
	})
	installAggregateTestConfig(t, r, map[string]any{
		"router-id": "198.51.100.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"aggregate": map[string]any{
			aggPfx.String(): map[string]any{
				"prefix":          aggPfx.String(),
				"brief-origin-as": "65020",
			},
		},
	})
	_, active = r.ActiveAggregate(aggPfx)
	require.False(t, active, "ASNs following an AS_SET must not be concatenated onto the leading AS_SEQUENCE")
}
