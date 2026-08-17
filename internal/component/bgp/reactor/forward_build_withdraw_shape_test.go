package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// The four bodies a relay meets that advertise NOTHING, plus the one that does.
//
// RFC 4271 Section 4.3: "An UPDATE message might advertise only routes that are
// to be withdrawn from service, in which case the message will not include path
// attributes or Network Layer Reachability Information."

// withdrawOnlyBody withdraws 10.0.0.0/24 and carries no attributes: attrLen 0000.
func withdrawOnlyBody() []byte {
	nlri := []byte{24, 10, 0, 0}
	body := make([]byte, 2+len(nlri)+2)
	binary.BigEndian.PutUint16(body[0:2], uint16(len(nlri)))
	copy(body[2:], nlri)
	return body
}

// legacyEORBody is the RFC 4724 Section 2 End-of-RIB marker for IPv4 unicast:
// an UPDATE with no withdrawn routes, no attributes and no NLRI.
func legacyEORBody() []byte { return []byte{0, 0, 0, 0} }

// mpUnreachOnlyBody withdraws one IPv6 prefix through MP_UNREACH_NLRI (RFC 4760
// Section 4) and carries no other attribute.
func mpUnreachOnlyBody() []byte {
	val := []byte{0x00, 0x02, 0x01, 0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01} // AFI 2, SAFI 1, 2001:db8:0:1::/64
	attr := makeAttr(0x80, 15, val)
	body := make([]byte, 4+len(attr))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attr)))
	copy(body[4:], attr)
	return body
}

// advertiseBody announces 10.0.0.0/24 with an ORIGIN. It is the control: every
// assertion below runs over it too, so a gate that refused everything would fail
// rather than pass.
func advertiseBody() []byte {
	return buildModTestPayload(makeAttr(0x40, 1, []byte{0x00}), modTestNLRI)
}

// creatingHandler emits its operation's bytes whatever the source holds. It
// stands in for every handler that CAN create: genericAttrSetHandler
// (filter_delta_handlers.go), originatorIDHandler, clusterListHandler, and
// genericCommunityHandler in the filter_community plugin, which this package
// cannot import. The gate runs before any of them, so one stand-in proves the
// property for all four.
func creatingHandler(flags, code byte) filterapi.AttrModHandler {
	return func(p *filterapi.AttrPlan) {
		p.Op(0)
		p.Emit(flags, code)
	}
}

// VALIDATES: spec-rfc7606-5-1-2-relay-shape follow-on 2 -- no per-destination
// egress rule may stamp an attribute onto a relayed UPDATE that advertises
// nothing.
//
// rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
// correctness-only test edits. A mis-drafted RFC4271-4.3-1 tag, written minutes
// ago in this same session, is REMOVED: that id is the Transitive-bit rule
// (rfc/short/rfc4271.md:698) and has nothing to do with the shape this test
// drives. A wrong tag counts as evidence for an obligation nobody proved.
// No assertion changes.
//
// NO `RFC requirement:` TAG. RFC 4271 Section 4.3's "will not include path
// attributes" is indicative prose with no checklist row, and the MUST that bites
// -- Section 6.3's Missing Well-known Attribute -- has an extracted row
// (RFC4271-6.3-1) that is a RECEIVER obligation this test does not drive.
//
// A withdraw-only UPDATE relayed on still carries no path attributes. RFC 4271
// Section 6.3 makes the alternative a wire error: "If any of the well-known
// mandatory attributes are not present, then the Error Subcode MUST be set to
// Missing Well-known Attribute." FRR 10.3.1 answers the shape with "Missing
// well-known attribute AS_PATH" and "rcvd UPDATE with errors in attr(s)!!
// Withdrawing route" (measured 2026-08-04, interop scenario 53 with the guard
// reverted), so the withdrawal never takes effect at the peer.
//
// PREVENTS: the four producers named in this spec's "Follow-On 2" table.
// applyFactsNextHop (peer_forward_facts.go) records Op(3)/Op(14) whenever a
// next-hop rewrite is configured; forwardUpdateCore records Op(9)/Op(10) for RFC
// 4456 Section 8 reflection; the egress community tag records an operation on
// code 8. None asked whether the UPDATE advertises anything, so a relayed
// withdrawal and a relayed End-of-RIB each gained a lone attribute.
func TestRelayCreatesNoAttributeOnABodyAdvertisingNothing(t *testing.T) {
	nhSelf := &peerForwardFacts{nhMode: nhModeSelf4, nhLegacy: [4]byte{192, 0, 2, 1}, nhMapped: netip.MustParseAddr("192.0.2.1").As16()}
	var clusterID [4]byte
	binary.BigEndian.PutUint32(clusterID[:], 0x0A000001)

	producers := []struct {
		name string
		// record composes the same operations the production producer does.
		record func(mods *filterapi.ModAccumulator)
		// code is the attribute the producer would have created.
		code byte
	}{
		{
			name:   "next-hop-self/applyFactsNextHop",
			record: func(mods *filterapi.ModAccumulator) { applyFactsNextHop(nhSelf, mods) },
			code:   3,
		},
		{
			name: "rfc4456-originator-id/originatorIDHandler",
			record: func(mods *filterapi.ModAccumulator) {
				mods.Op(9, filterapi.AttrModSet, []byte{0x0A, 0x00, 0x00, 0x02})
			},
			code: 9,
		},
		{
			name: "rfc4456-cluster-list/clusterListHandler",
			record: func(mods *filterapi.ModAccumulator) {
				mods.Op(10, filterapi.AttrModPrepend, clusterID[:])
			},
			code: 10,
		},
		{
			name: "egress-community-tag/genericCommunityHandler",
			record: func(mods *filterapi.ModAccumulator) {
				mods.Op(8, filterapi.AttrModSet, []byte{0xFF, 0xFF, 0xFF, 0x01})
			},
			code: 8,
		},
	}

	bodies := []struct {
		name string
		body []byte
	}{
		{"withdraw-only", withdrawOnlyBody()},
		{"legacy-end-of-rib", legacyEORBody()},
		{"mp-unreach-only", mpUnreachOnlyBody()},
	}

	// The production handler map, with every code under test replaced by one that
	// creates on demand. Nothing below can pass merely because a real handler
	// declined, and the codes NOT under test (14, 40) keep their real behavior so
	// the ops that ride along with a next-hop rewrite behave as they do in a
	// deployment.
	handlers := attrModHandlersWithDefaults()
	for code, flags := range map[uint8]byte{3: 0x40, 8: 0xC0, 9: 0x80, 10: 0x80} {
		handlers[code] = creatingHandler(flags, code)
	}

	for _, p := range producers {
		t.Run(p.name, func(t *testing.T) {
			for _, b := range bodies {
				t.Run(b.name, func(t *testing.T) {
					var mods filterapi.ModAccumulator
					p.record(&mods)
					require.NotZero(t, mods.Len(), "guard: the producer must record something, or this proves nothing")

					result, _, fail := buildModifiedPayload(b.body, &mods, handlers, nil, nil)
					require.Equal(t, modifyFailureNone, fail, "refusing to create is not a failure")
					assert.Nil(t, result,
						"nothing landed, so the relay keeps the source bytes and its zero-copy path")
				})
			}

			// The control. Same producer, same handlers, a body that DOES
			// advertise: the attribute must appear. Without this a gate stuck
			// closed would pass every case above.
			t.Run("advertising-body-still-gets-it", func(t *testing.T) {
				var mods filterapi.ModAccumulator
				p.record(&mods)

				result, _, fail := buildModifiedPayload(advertiseBody(), &mods, handlers, nil, nil)
				require.Equal(t, modifyFailureNone, fail)
				require.NotNil(t, result, "an advertisement must still be modified")
				assert.Contains(t, rebuiltAttrs(t, result), p.code,
					"the producer's attribute belongs on a route that advertises something")
			})
		})
	}
}

// VALIDATES: the gate blocks CREATION only. An attribute the source already
// carries is still rewritten on a body that advertises nothing.
//
// rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
// correctness-only test edits. A mis-drafted RFC6793-4.2.2-1 tag, written earlier
// in this same session, is REMOVED. That id is "when sending to an OLD speaker,
// MUST send AS path information in AS_PATH encoded with two-octet AS numbers"
// (rfc/short/rfc6793.md:498). This test involves no OLD speaker: it hand-builds
// `narrowed` and asserts those exact bytes return, so the producer of that
// obligation (ASPathEdit.recordTranscode, wireu/aspath_slot.go) never runs and
// the assertion is a tautology with respect to the requirement. The real positive
// lives in rfc6793_as4_test.go. Removing a false claim strengthens the ledger.
//
// NO `RFC requirement:` TAG. What this proves is the gate's create-versus-modify
// boundary, which no checklist row states.
//
// An AS_PATH that rode along on a withdraw-only UPDATE is still rewritten in
// place. Presence is what a receiver's well-known-mandatory check reads (RFC 4271
// Section 6.3), so rewriting one changes nothing about the shape.
//
// PREVENTS: the over-fire half. A guard placed at the driver -- refusing to
// RECORD the operation -- cannot tell create from modify and would have dropped
// this rewrite too, undoing what commit 79b46ef60 was careful to keep.
func TestRelayStillRewritesAnAttributeAWithdrawalCarries(t *testing.T) {
	asPath := makeAttr(0x40, 2, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE8}) // AS_SEQUENCE [65000], 4-byte
	nlri := []byte{24, 10, 0, 0}
	body := make([]byte, 2+len(nlri)+2+len(asPath))
	binary.BigEndian.PutUint16(body[0:2], uint16(len(nlri)))
	copy(body[2:], nlri)
	binary.BigEndian.PutUint16(body[2+len(nlri):], uint16(len(asPath)))
	copy(body[2+len(nlri)+2:], asPath)

	narrowed := []byte{0x02, 0x01, 0xFD, 0xE8} // the same path, 2-byte width

	var mods filterapi.ModAccumulator
	mods.Op(2, filterapi.AttrModSet, narrowed)

	result, _, fail := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
	require.Equal(t, modifyFailureNone, fail)
	require.NotNil(t, result, "a rewrite of a PRESENT attribute must still happen on a withdrawal")

	got := rebuiltAttrs(t, result)
	require.Contains(t, got, byte(attribute.AttrASPath))
	assert.Equal(t, hex.EncodeToString(makeAttr(0x40, 2, narrowed)), hex.EncodeToString(got[byte(attribute.AttrASPath)]),
		"the AS_PATH is re-encoded, not dropped and not duplicated")
	assert.Len(t, got, 1, "no second attribute was invented alongside it")

	// The withdrawn section survives the rebuild untouched.
	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	assert.Equal(t, nlri, result[2:2+wdLen])
}

// VALIDATES: the gate reads the shape the rebuild WRITES, not the shape it
// reads. An export chain that denies every prefix leaves a body advertising
// nothing, whatever the source carried.
//
// rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
// correctness-only test edits. A mis-drafted RFC4271-4.3-1 tag, written minutes
// ago in this same session, is REMOVED: that id is the Transitive-bit rule
// (rfc/short/rfc4271.md:698) and has nothing to do with the shape this test
// drives. A wrong tag counts as evidence for an obligation nobody proved.
// No assertion changes.
//
// NO `RFC requirement:` TAG. RFC 4271 Section 4.3's "will not include path
// attributes" is indicative prose with no checklist row, and the MUST that bites
// -- Section 6.3's Missing Well-known Attribute -- has an extracted row
// (RFC4271-6.3-1) that is a RECEIVER obligation this test does not drive.
//
// Same obligation, reached down the per-prefix filter path
// (buildModifiedPayload's nlriOverride argument, filter_ordered.go).
//
// PREVENTS: a predicate keyed on the SOURCE payload. The source here advertises
// 10.0.0.0/24, so a source-only reading calls this an advertisement and stamps
// the attribute onto a body whose NLRI section the same call is emptying.
func TestRelayCreatesNoAttributeWhenEveryPrefixIsFiltered(t *testing.T) {
	handlers := map[uint8]filterapi.AttrModHandler{3: creatingHandler(0x40, 3)}

	record := func() filterapi.ModAccumulator {
		var mods filterapi.ModAccumulator
		mods.Op(3, filterapi.AttrModSet, []byte{192, 0, 2, 1})
		return mods
	}

	t.Run("every-prefix-denied", func(t *testing.T) {
		mods := record()
		result, _, fail := buildModifiedPayload(advertiseBody(), &mods, handlers, nil, []byte{})
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result, "the NLRI rewrite itself is still applied")
		assert.NotContains(t, rebuiltAttrs(t, result), byte(3),
			"a body left with no NLRI must not gain a NEXT_HOP")
		attrLen := int(binary.BigEndian.Uint16(result[2:4]))
		assert.Len(t, result, 4+attrLen, "every legacy prefix was dropped")
	})

	t.Run("one-prefix-kept", func(t *testing.T) {
		mods := record()
		kept := []byte{24, 172, 16, 0}
		result, _, fail := buildModifiedPayload(advertiseBody(), &mods, handlers, nil, kept)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)
		assert.Contains(t, rebuiltAttrs(t, result), byte(3),
			"a body that still advertises a prefix is owed the rewrite")
	})
}

// relayShapeSourcePeer builds an established source peer at 10.0.0.1 whose
// PeerAS decides whether the relay reads it as internal (RFC 4456) or external.
func relayShapeSourcePeer(t testing.TB, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peer := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(forwardSourceAddr),
		LocalAS:    65000,
		PeerAS:     peerAS,
		RouterID:   0x0102030A,
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.remoteRouterID.Store(0x0A000002)
	peer.refreshForwardFacts()
	return peer
}

// relayShapeDestPeer builds an established destination peer, applying whatever
// per-destination rule the caller wants to test.
func relayShapeDestPeer(t testing.TB, settings *PeerSettings, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()
	return peer
}

// forwardOneBody drives the real entry point -- reactorAPIAdapter.ForwardUpdate,
// which reaches forwardUpdateCore -- and returns the single body dispatched to
// the destination peer.
//
// Driving the rail rather than buildModifiedPayload alone is the point: the
// producers under test live in forwardUpdateCore and applyFactsNextHop, and a
// helper-only test says nothing about whether the rail reaches the gate
// (ai/rules/evidence.md).
func forwardOneBody(t *testing.T, srcPeerAS uint32, destSettings *PeerSettings, body []byte) []byte {
	t.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	wu := wireu.NewWireUpdate(body, ctxID)
	const updateID = 9300
	wu.SetMessageID(updateID)

	cache := newRecentUpdateCache(100)
	cache.Add(&ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	})
	cache.Activate(updateID, 1)

	src := relayShapeSourcePeer(t, srcPeerAS, ctx, ctxID)
	dest := relayShapeDestPeer(t, destSettings, ctx, ctxID)

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	r := &Reactor{
		recentUpdates: cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey():  src,
			dest.Settings().PeerKey(): dest,
		},
		fwdPool:         pool,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	adapter := &reactorAPIAdapter{r: r}

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, adapter.ForwardUpdate(sel, updateID, "relay-shape-test", plugin.OperatorSender()))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the forward dispatch")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, dispatched, 1, "exactly one destination")
	require.Len(t, dispatched[0].rawBodies, 1, "one UPDATE body")
	return slices.Clone(dispatched[0].rawBodies[0])
}

// VALIDATES: the whole forward rail, entry point to wire. With next-hop-self
// configured, a relayed withdrawal and a relayed End-of-RIB leave ze byte for
// byte as they arrived, and an advertisement still gets its next-hop rewritten.
//
// rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
// correctness-only test edits. A mis-drafted RFC4271-4.3-1 tag, written minutes
// ago in this same session, is REMOVED: that id is the Transitive-bit rule
// (rfc/short/rfc4271.md:698) and has nothing to do with the shape this test
// drives. A wrong tag counts as evidence for an obligation nobody proved.
// No assertion changes.
//
// NO `RFC requirement:` TAG. RFC 4271 Section 4.3's "will not include path
// attributes" is indicative prose with no checklist row, and the MUST that bites
// -- Section 6.3's Missing Well-known Attribute -- has an extracted row
// (RFC4271-6.3-1) that is a RECEIVER obligation this test does not drive.
//
// Both polarities on one rail: the withdrawal and the End-of-RIB must not
// change, and the advertisement below them MUST.
//
// PREVENTS: applyFactsNextHop (peer_forward_facts.go) recording Op(3, Set) with
// no NLRI question asked. That is the producer this spec's follow-on table named
// first, and it is reached whenever an operator configures a next-hop rewrite.
func TestForwardNextHopSelfLeavesAWithdrawalUntouched(t *testing.T) {
	nextHopSelf := func(addr string) *PeerSettings {
		return &PeerSettings{
			Connection:   ConnectionBoth,
			Address:      netip.MustParseAddr(addr),
			LocalAS:      65000,
			PeerAS:       65002, // external
			RouterID:     0x01020301,
			NextHopMode:  NextHopSelf,
			LocalAddress: netip.MustParseAddr("192.0.2.1"),
		}
	}

	t.Run("withdrawal-is-byte-identical", func(t *testing.T) {
		body := withdrawOnlyBody()
		got := forwardOneBody(t, 65001, nextHopSelf("10.0.0.2"), body)
		assert.Equal(t, hex.EncodeToString(body), hex.EncodeToString(got),
			"RFC 4271 Section 4.3: a relayed withdrawal carries no path attributes")
	})

	t.Run("end-of-rib-is-byte-identical", func(t *testing.T) {
		body := legacyEORBody()
		got := forwardOneBody(t, 65001, nextHopSelf("10.0.0.3"), body)
		assert.Equal(t, hex.EncodeToString(body), hex.EncodeToString(got),
			"RFC 4724 Section 2: one attribute stops the marker being a marker")
	})

	t.Run("advertisement-still-gets-the-next-hop", func(t *testing.T) {
		got := forwardOneBody(t, 65001, nextHopSelf("10.0.0.4"), advertiseBody())
		attrs := rebuiltAttrs(t, got)
		require.Contains(t, attrs, byte(3), "next-hop-self must still rewrite an advertisement")
		assert.Equal(t, []byte{192, 0, 2, 1}, attrs[3][3:], "the configured local address")
	})
}

// VALIDATES: the RFC 4456 half of the same rail. A route reflector relaying a
// withdrawal or an End-of-RIB to an internal client injects neither
// ORIGINATOR_ID nor CLUSTER_LIST, and still injects both for an advertisement.
//
// RFC requirement: RFC4456-8-1 negative -- the injection is owed for a route
// being reflected, and a withdrawal reflects no route. RFC 4271 Section 6.3
// governs the rest: two optional attributes with no well-known mandatory set
// beside them is the same Missing-Well-known-Attribute shape.
//
// PREVENTS: originatorIDHandler and clusterListHandler (filter_delta_handlers.go)
// creating their attribute from an absent source, driven by forwardUpdateCore's
// unconditional Op(9)/Op(10) on the iBGP rail.
func TestForwardReflectionLeavesAWithdrawalUntouched(t *testing.T) {
	internalClient := func(addr string) *PeerSettings {
		return &PeerSettings{
			Connection:           ConnectionBoth,
			Address:              netip.MustParseAddr(addr),
			LocalAS:              65000,
			PeerAS:               65000, // internal
			RouterID:             0x01020302,
			RouteReflectorClient: true,
			// rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation
			// for correctness-only test edits. PeerSettings.ClusterID is a uint32
			// (peer_settings.go); the netip.Addr this line first carried did not
			// compile. Same value, 10.0.0.1, no assertion touched.
			ClusterID: 0x0A000001,
		}
	}

	t.Run("withdrawal-is-byte-identical", func(t *testing.T) {
		body := withdrawOnlyBody()
		got := forwardOneBody(t, 65000, internalClient("10.0.0.5"), body)
		assert.Equal(t, hex.EncodeToString(body), hex.EncodeToString(got),
			"a reflected withdrawal gains no reflection attributes")
	})

	t.Run("end-of-rib-is-byte-identical", func(t *testing.T) {
		body := legacyEORBody()
		got := forwardOneBody(t, 65000, internalClient("10.0.0.6"), body)
		assert.Equal(t, hex.EncodeToString(body), hex.EncodeToString(got),
			"RFC 4724 Section 2: the marker survives reflection as a marker")
	})

	t.Run("advertisement-still-gets-both", func(t *testing.T) {
		got := forwardOneBody(t, 65000, internalClient("10.0.0.7"), advertiseBody())
		attrs := rebuiltAttrs(t, got)
		assert.Contains(t, attrs, byte(attribute.AttrOriginatorID), "RFC 4456 Section 8 ORIGINATOR_ID")
		assert.Contains(t, attrs, byte(attribute.AttrClusterList), "RFC 4456 Section 8 CLUSTER_LIST")
	})
}

// VALIDATES: advertiseGate and wireu.PayloadAdvertisesNLRI answer the SAME
// question. The gate reads values buildModifiedPayload already computed
// (attrEnd, the span index) instead of re-parsing the body, so this test is what
// keeps that shortcut honest.
//
// PREVENTS: the two drifting apart. A second implementation of a predicate is
// only acceptable while something checks it against the first; a comment saying
// "these agree" is a belief (ai/rules/evidence.md). Every shape below is run
// through both, including the truncations, where the predicate deliberately
// answers false on positive evidence rather than guessing.
func TestAdvertiseGateAgreesWithPayloadAdvertisesNLRI(t *testing.T) {
	mpReach := makeAttr(0x80, 14, []byte{
		0x00, 0x02, 0x01, 0x10,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00,
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01,
	})

	shapes := []struct {
		name string
		body []byte
	}{
		{"withdraw-only", withdrawOnlyBody()},
		{"legacy-end-of-rib", legacyEORBody()},
		{"mp-unreach-only", mpUnreachOnlyBody()},
		{"advertise-ipv4", advertiseBody()},
		{"mp-reach-only", buildModTestPayload(mpReach, nil)},
		{"mp-reach-plus-origin", buildModTestPayload(slices.Concat(makeAttr(0x40, 1, []byte{0x00}), mpReach), nil)},
		{"origin-only-no-nlri", buildModTestPayload(makeAttr(0x40, 1, []byte{0x00}), nil)},
	}

	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			want := wireu.PayloadAdvertisesNLRI(sh.body)

			// Rebuild the same state buildModifiedPayload hands the gate.
			withdrawnLen := int(binary.BigEndian.Uint16(sh.body[0:2]))
			attrOff := 2 + withdrawnLen
			attrLen := int(binary.BigEndian.Uint16(sh.body[attrOff : attrOff+2]))
			attrStart := attrOff + 2
			attrEnd := attrStart + attrLen

			var spans attribute.SpanIndex
			require.NoError(t, spans.Rebuild(sh.body[attrStart:attrEnd]))

			gate := advertiseGate{payload: sh.body, attrEnd: attrEnd, spans: &spans}
			assert.Equal(t, want, gate.advertises(),
				"the gate and wireu.PayloadAdvertisesNLRI must agree on every shape")
		})
	}
}

// VALIDATES: the gate reads the NLRI section the rebuild WRITES, in BOTH
// directions. An override that ADDS prefixes to a withdraw-only source makes the
// body an advertisement, so the attribute the policy asked for must land.
//
// PREVENTS: the hole an independent review found on 2026-08-04. The first
// version corrected only the emptying direction, so a filter whose
// `nlri ipv4/unicast add` block is not a subset of the source (which
// extractLegacyNLRIOverride never proves, filter_delta.go) produced a body that
// advertised NLRI with the policy's attribute silently missing, and reported
// modifyFailureNone while doing it.
func TestRelayCreatesTheAttributeWhenAnOverrideAddsNLRI(t *testing.T) {
	handlers := map[uint8]filterapi.AttrModHandler{3: creatingHandler(0x40, 3)}
	added := []byte{24, 172, 16, 0} // 172.16.0.0/24, absent from the source

	var mods filterapi.ModAccumulator
	mods.Op(3, filterapi.AttrModSet, []byte{192, 0, 2, 1})

	result, _, fail := buildModifiedPayload(withdrawOnlyBody(), &mods, handlers, nil, added)
	require.Equal(t, modifyFailureNone, fail)
	require.NotNil(t, result, "the override alone forces a rebuild")

	assert.Contains(t, rebuiltAttrs(t, result), byte(3),
		"the rebuilt body advertises 172.16.0.0/24, so it is owed the NEXT_HOP the policy set")

	attrLen := int(binary.BigEndian.Uint16(result[2+4 : 2+4+2]))
	assert.Equal(t, added, result[2+4+2+attrLen:], "the override is the NLRI section")
}
