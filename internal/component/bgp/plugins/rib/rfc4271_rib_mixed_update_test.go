package rib

// rfc-test-change-approved: 2026-08-01 Thomas approved adding three imports
// (routeaction, family, rpc) required by two NEW tests appended to this file, which
// cover the pool and injection receive paths. No existing test, assertion or RFC tag was
// changed, removed, reworded or re-tagged: the edit is three import lines plus new
// functions. The guard fired because an import block sits outside every function, so its
// scope is the whole file.

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"

	// rfc-test-change-approved: 2026-08-01 Thomas approved these three imports for two
	// NEW tests (pool and injection paths). No existing assertion or tag was touched.
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC 4271 Section 4.3, the same prefix in WITHDRAWN ROUTES and NLRI.
//
// The section states three things about this shape, at two different strengths:
//
//	"An UPDATE message SHOULD NOT include the same address prefix in the WITHDRAWN
//	 ROUTES and Network Layer Reachability Information fields. However, a BGP speaker
//	 MUST be able to process UPDATE messages in this form. A BGP speaker SHOULD treat
//	 an UPDATE message of this form as though the WITHDRAWN ROUTES do not contain the
//	 address prefix."
//
// handleReceivedStructured applies every withdrawal before every announce, which is
// what makes the announce win. It used to be the other way round, so the withdrawal
// deleted the route the same message had just announced.

// VALIDATES: an UPDATE carrying the same prefix in both the WITHDRAWN ROUTES and the
// NLRI fields is processed without error, and the prefix ends up INSTALLED, exactly as
// if the WITHDRAWN field had never named it.
// PREVENTS: the announce-then-withdraw ordering that left such a prefix removed, turning
// a reachable prefix into an unreachable one on a message the RFC says must leave it
// reachable.
//
// RFC requirement: RFC4271-4.3-5 positive -- an UPDATE with the same prefix in WITHDRAWN
// and NLRI is processed: it neither errors nor leaves the RIB in a state the message did
// not ask for.
// RFC requirement: RFC4271-4.3-7 positive -- that UPDATE is treated as though WITHDRAWN
// ROUTES does not contain the prefix, so the announce wins and the route is installed.
func TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peer := netip.MustParseAddr("192.0.2.3")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	// 10.0.0.0/8 appears in WITHDRAWN ROUTES *and* in the NLRI of one message.
	mixed := []byte{
		0x00, 0x02, // Withdrawn Routes length 2
		0x08, 0x0a, // withdrawn 10.0.0.0/8
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8 (the same prefix)
	}
	feedReceived(r, peer, ctxID, mixed)

	require.Equal(t, 1, r.bgpPeers[peer].Len(),
		"the prefix is installed: Section 4.3 says treat the UPDATE as though WITHDRAWN did not contain it")
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		bestChangePrefixes(bus, ribevents.BestChangeAdd),
		"the install reaches the Loc-RIB best path as an Add")
	assert.Empty(t, bestChangePrefixes(bus, ribevents.BestChangeWithdraw),
		"no withdrawal reaches the Loc-RIB: the announce in the same message wins")

	// The same shape arriving for an ALREADY-installed prefix must leave it installed
	// rather than flapping it out.
	feedReceived(r, peer, ctxID, mixed)
	assert.Equal(t, 1, r.bgpPeers[peer].Len(),
		"a repeat of the same mixed UPDATE leaves the route installed")
	assert.Empty(t, bestChangePrefixes(bus, ribevents.BestChangeWithdraw),
		"and still publishes no withdrawal")
}

// VALIDATES: the JSON/pool receive path treats the same-prefix UPDATE the same way as
// the structured path: the prefix ends up installed.
// PREVENTS: the two ingest paths disagreeing about one message. Before this, the pool
// path applied every insert before every remove, so an external plugin feeding the same
// prefix in both fields lost the route while a DirectBridge plugin kept it.
//
// RFC requirement: RFC4271-4.3-5 positive -- the pool path processes an UPDATE with the
// same prefix in WITHDRAWN and NLRI.
// RFC requirement: RFC4271-4.3-7 positive -- and treats it as though WITHDRAWN did not
// contain the prefix.
func TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("10.0.0.1")

	// 10.0.0.0/8 in BOTH the raw NLRI and the raw Withdrawn map, one family.
	const wire = "080a" // prefix length 8, one octet 0x0a
	event := &Event{
		Message:       &MessageInfo{Type: rpc.EventKindUpdate, ID: 300},
		Peer:          mustMarshal(t, map[string]any{"local": map[string]any{"address": "10.0.0.2", "as": uint32(65002)}, "remote": map[string]any{"address": "10.0.0.1", "as": uint32(65001)}}),
		RawAttributes: "40010100", // ORIGIN IGP
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: wire},
		RawWithdrawn:  map[family.Family]string{family.IPv4Unicast: wire},
		FamilyOps: map[family.Family][]FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "1.1.1.1", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/8"}},
			},
		},
	}

	r.handleReceived(event)

	require.NotNil(t, r.bgpPeers[peer], "the peer RIB is created")
	assert.Equal(t, 1, r.bgpPeers[peer].Len(),
		"the announce wins on the pool path too: the prefix stays installed")
}

// VALIDATES: the injection path treats the same-prefix UPDATE the same way.
// PREVENTS: a BMP feed carrying the shape losing the route, so the injected view
// disagrees with what the monitored router actually holds.
//
// RFC requirement: RFC4271-4.3-5 positive -- the injection path processes an UPDATE with
// the same prefix in WITHDRAWN and NLRI.
// RFC requirement: RFC4271-4.3-7 positive -- and treats it as though WITHDRAWN did not
// contain the prefix.
func TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute(t *testing.T) {
	r := newTestRIBManager(t)

	// Withdrawn 10.0.0.0/8, then attributes, then NLRI 10.0.0.0/8.
	mixed := []byte{
		0x00, 0x02, // Withdrawn Routes length 2
		0x08, 0x0a, // withdrawn 10.0.0.0/8
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8 (the same prefix)
	}
	require.NoError(t, r.handleInjectWireRoute("bmp", "router1:10.0.0.1", mixed))

	bmpPeers := r.ribInPool[bmpProtocolID]
	require.NotNil(t, bmpPeers["router1:10.0.0.1"], "the injected peer RIB is created")
	assert.Equal(t, 1, bmpPeers["router1:10.0.0.1"].Len(),
		"the announce wins on the injection path too: the prefix stays installed")
}

// VALIDATES: the reordering does not disturb the ordinary case. A withdrawal and an
// announce for DIFFERENT prefixes in one UPDATE both take effect.
// PREVENTS: a fix for the same-prefix case that silently drops one of the two sections,
// which the same-prefix test alone could not catch.
func TestRIBWithdrawAndAnnounceDifferentPrefixesBothApply(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peer := netip.MustParseAddr("192.0.2.4")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	announce := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
	feedReceived(r, peer, ctxID, announce)
	require.Equal(t, 1, r.bgpPeers[peer].Len(), "the first route installs")

	// Withdraw 10.0.0.0/8 and announce 11.0.0.0/8 in one message.
	both := []byte{
		0x00, 0x02, // Withdrawn Routes length 2
		0x08, 0x0a, // withdrawn 10.0.0.0/8
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0b, // NLRI 11.0.0.0/8
	}
	feedReceived(r, peer, ctxID, both)

	assert.Equal(t, 1, r.bgpPeers[peer].Len(),
		"one route leaves and one arrives, so the count is unchanged")
	assert.Contains(t, bestChangePrefixes(bus, ribevents.BestChangeWithdraw),
		netip.MustParsePrefix("10.0.0.0/8"),
		"the withdrawn prefix is withdrawn from the Loc-RIB")
	assert.Contains(t, bestChangePrefixes(bus, ribevents.BestChangeAdd),
		netip.MustParsePrefix("11.0.0.0/8"),
		"the announced prefix is added to the Loc-RIB")
}
