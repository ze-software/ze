// Design: docs/architecture/bgp/structural-forwarding.md — route-server control communities on both forward rails
// Related: forward_rs_community_test.go — the code 8 control-community gate on the route-server rail

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// rsControlRail names one of the two route-server forward rails. They carry
// separate copies of the control-community code, so every test below runs on
// both: a fix on one rail only would leak on whichever a deployment selects.
type rsControlRail int

const (
	rsControlRailFastPath rsControlRail = iota + 1 // reactorForwardRS
	rsControlRailGeneral                           // reactorAPIAdapter.forwardUpdateCore
)

func (r rsControlRail) String() string {
	switch r {
	case rsControlRailFastPath:
		return "fast-path"
	case rsControlRailGeneral:
		return "general"
	}
	return "unknown"
}

// rsControlRails is every rail the tests below must pass on.
var rsControlRails = []rsControlRail{rsControlRailFastPath, rsControlRailGeneral}

// rsControlForward runs one UPDATE through rail toward every client, all of them
// route-server clients of a route server in AS 65000, and returns the UPDATE
// body each client was asked to write. A client absent from the map was written
// nothing. The reactor's metrics are built on reg, so a caller reads what the
// forward counted.
func rsControlForward(t *testing.T, rail rsControlRail, reg *spyRegistry, payload []byte, clients ...*Peer) map[netip.Addr][]byte {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	type delivery struct {
		addr  netip.Addr
		items []fwdItem
	}
	delivered := make(chan delivery, 8)
	acc := newFwdAccumulator()
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), items: acc.add(k.peerAddr.Addr(), items)}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	srcAddr := netip.MustParseAddr("10.0.0.1")
	src := makeRSPeer(t, srcAddr.String(), 65001, ctx, ctxID)
	peerMap := map[netip.AddrPort]*Peer{src.Settings().PeerKey(): src}
	for _, c := range clients {
		key := fwdKey{peerAddr: c.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = c
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peerMap,
		fwdPool:         pool,
		rmetrics:        initReactorMetrics(reg, "test", "1.2.3.4", "65000"),
	}
	switch rail {
	case rsControlRailFastPath:
		reactorForwardRS(r, update, id, srcAddr, src)
	case rsControlRailGeneral:
		adapter := &reactorAPIAdapter{r: r}
		require.NoError(t, adapter.forwardUpdateCore(update, id, clients, forwardSourceInfo{
			resolved:      true,
			globalLocalAS: 65000,
		}))
	}

	got := make(map[netip.Addr][]byte, len(clients))
	record := func(d delivery) {
		for i := range d.items {
			for _, body := range d.items[i].rawBodies {
				got[d.addr] = body
			}
		}
	}
	for range clients {
		select {
		case d := <-delivered:
			record(d)
		case <-time.After(time.Second):
			return got
		}
	}
	fwdDrainGrace(delivered, record)
	return got
}

// rsControlPayload is an announcement of 10.0.0.0/24 from AS 65001 carrying the
// given LARGE_COMMUNITY value bytes:
//
//	40 01 01 00                  ORIGIN IGP
//	40 02 06 02 01 0000FDE9      AS_PATH AS_SEQUENCE[65001], four-octet
//	40 03 04 01010101            NEXT_HOP 1.1.1.1
//	C0 20 <len> <values>         LARGE_COMMUNITY
//	18 0A0000                    NLRI 10.0.0.0/24
func rsControlPayload(large []byte) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9,
		0x40, 0x03, 0x04, 0x01, 0x01, 0x01, 0x01,
		0xC0, 0x20, byte(len(large)),
	}
	attrs = append(attrs, large...)
	payload := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))}
	payload = append(payload, attrs...)
	return append(payload, 0x18, 0x0A, 0x00, 0x00)
}

// rsLarge packs one large community value.
func rsLarge(ga, ld1, ld2 uint32) []byte {
	return []byte{
		byte(ga >> 24), byte(ga >> 16), byte(ga >> 8), byte(ga),
		byte(ld1 >> 24), byte(ld1 >> 16), byte(ld1 >> 8), byte(ld1),
		byte(ld2 >> 24), byte(ld2 >> 16), byte(ld2 >> 8), byte(ld2),
	}
}

const (
	rsNamedClientAddr   = "10.0.0.2"
	rsUnnamedClientAddr = "10.0.0.3"
)

// rsReceivedASPath is the AS_PATH attribute rsControlPayload carries, header
// included: what a client the route does not ask a prepend for receives.
var rsReceivedASPath = []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}

// TestRSPrependReachesWire sends one route asking the route server to prepend
// its AS once toward AS 65002, and reads the AS_PATH each client is written.
//
// VALIDATES: spec-bcp194-1-communities AC-11 and AC-14 on both rails. The client
// the request names receives AS_SEQUENCE[65000, 65001]; the client it does not
// name receives the AS_PATH byte-identical to the one the route server received.
// The request itself, 65000:101:65002, reaches neither (AC-12).
// PREVENTS: CommunityPolicy.PrependCount staying a function only tests call, so
// the request a client sent never reached the wire.
func TestRSPrependReachesWire(t *testing.T) {
	for _, rail := range rsControlRails {
		t.Run(rail.String(), func(t *testing.T) {
			ctx := bgpctx.EncodingContextForASN4(true)
			ctxID, err := bgpctx.Registry.Register(ctx)
			require.NoError(t, err)

			named := rsCommunityClient(t, rsNamedClientAddr, 65002, ctx, ctxID)
			unnamed := rsCommunityClient(t, rsUnnamedClientAddr, 65003, ctx, ctxID)

			reg := newSpyRegistry()
			got := rsControlForward(t, rail, reg, rsControlPayload(rsLarge(65000, 101, 65002)), named, unnamed)

			namedBody, ok := got[netip.MustParseAddr(rsNamedClientAddr)]
			require.True(t, ok, "the named client must be sent the route")
			namedAttrs := rebuiltAttrs(t, namedBody)
			assert.Equal(t,
				[]byte{0x40, 0x02, 0x0A, 0x02, 0x02, 0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0xFD, 0xE9},
				namedAttrs[0x02], "the route server's AS 65000 prepended once, outermost")
			assert.NotContains(t, namedAttrs, byte(0x20), "the consumed request does not reach the client")

			unnamedBody, ok := got[netip.MustParseAddr(rsUnnamedClientAddr)]
			require.True(t, ok, "the client not named must be sent the route")
			unnamedAttrs := rebuiltAttrs(t, unnamedBody)
			assert.Equal(t, rsReceivedASPath, unnamedAttrs[0x02],
				"a client the request does not name receives the AS_PATH unchanged")
			assert.NotContains(t, unnamedAttrs, byte(0x20), "the consumed request does not reach the client")

			assert.Equal(t, 1.0, rsCounterValue(t, reg, "ze_bgp_rs_prepend_applied_total"),
				"one prepend reached one client: the named one")
			assert.Equal(t, 2.0, rsCounterValue(t, reg, "ze_bgp_rs_large_control_community_stripped_total"),
				"the request value was removed from what each of the two clients is written")
		})
	}
}

// TestRSStripsLargeControlCommunities sends one route carrying two values in
// the route server's namespace and two that are not, and reads the
// LARGE_COMMUNITY each client is written.
//
// VALIDATES: spec-bcp194-1-communities AC-12 on both rails. Both values whose
// Global Administrator is AS 65000 are removed in one operation, whatever their
// function, and the two others reach the client in their original order.
// PREVENTS: the route server's large control values leaking to its clients while
// the standard ones were already stripped.
func TestRSStripsLargeControlCommunities(t *testing.T) {
	for _, rail := range rsControlRails {
		t.Run(rail.String(), func(t *testing.T) {
			ctx := bgpctx.EncodingContextForASN4(true)
			ctxID, err := bgpctx.Registry.Register(ctx)
			require.NoError(t, err)

			client := rsCommunityClient(t, rsUnnamedClientAddr, 65003, ctx, ctxID)

			var large []byte
			large = append(large, rsLarge(65000, 101, 65099)...) // control, names nobody here
			large = append(large, rsLarge(65001, 1, 2)...)       // the sender's own value
			large = append(large, rsLarge(65000, 7, 7)...)       // control, any function
			large = append(large, rsLarge(65099, 101, 65003)...) // another AS's namespace

			reg := newSpyRegistry()
			got := rsControlForward(t, rail, reg, rsControlPayload(large), client)

			body, ok := got[netip.MustParseAddr(rsUnnamedClientAddr)]
			require.True(t, ok, "the client must be sent the route")
			attrs := rebuiltAttrs(t, body)

			want := []byte{0xD0, 0x20, 0x00, 0x18}
			want = append(want, rsLarge(65001, 1, 2)...)
			want = append(want, rsLarge(65099, 101, 65003)...)
			assert.Equal(t, want, attrs[0x20],
				"only the values outside the route server's namespace remain, in order")
			assert.Equal(t, rsReceivedASPath, attrs[0x02],
				"no request names this client, so its AS_PATH is unchanged")

			assert.Equal(t, 2.0, rsCounterValue(t, reg, "ze_bgp_rs_large_control_community_stripped_total"),
				"two values in the route server's namespace were removed")
			assert.Zero(t, rsCounterValue(t, reg, "ze_bgp_rs_prepend_applied_total"),
				"no request names this client, so no prepend is counted")
		})
	}
}

// TestRSControlCommunityCountersStayAtZero sends a route that carries no value in
// the route server's namespace, on both rails.
//
// VALIDATES: the negative polarity of ze_bgp_rs_prepend_applied_total and
// ze_bgp_rs_large_control_community_stripped_total. A client written a route the
// route server did not rewrite counts nothing, so a non-zero rate means the
// route server acted.
// PREVENTS: a counter stepped where the control communities are parsed, which
// would count every route-server client delivery.
func TestRSControlCommunityCountersStayAtZero(t *testing.T) {
	for _, rail := range rsControlRails {
		t.Run(rail.String(), func(t *testing.T) {
			ctx := bgpctx.EncodingContextForASN4(true)
			ctxID, err := bgpctx.Registry.Register(ctx)
			require.NoError(t, err)

			client := rsCommunityClient(t, rsUnnamedClientAddr, 65003, ctx, ctxID)
			reg := newSpyRegistry()
			got := rsControlForward(t, rail, reg, rsControlPayload(rsLarge(65001, 101, 65003)), client)

			_, ok := got[netip.MustParseAddr(rsUnnamedClientAddr)]
			require.True(t, ok, "the client must be sent the route")
			assert.Zero(t, rsCounterValue(t, reg, "ze_bgp_rs_prepend_applied_total"))
			assert.Zero(t, rsCounterValue(t, reg, "ze_bgp_rs_large_control_community_stripped_total"))
		})
	}
}

// TestRecordRSCommunityEditsCountsEachUpdateWritten drives the counting seam
// directly.
//
// VALIDATES: every UPDATE queued for a client counts its edits, so the sections
// of a split UPDATE each count, and a nil reactor or a reactor with no registry
// counts nothing without panicking.
// PREVENTS: a count that differs by split path: forwardUpdateSelected and
// reactorForwardRS split across calls, the general rail splits within one, and
// all three write each section with the edits applied.
func TestRecordRSCommunityEditsCountsEachUpdateWritten(t *testing.T) {
	edits := rsCommunityEdits{prepended: true, largeStrippedValues: 3}
	var none *Reactor
	assert.NotPanics(t, func() { none.recordRSCommunityEdits(edits) }, "a nil reactor counts nothing")
	assert.NotPanics(t, func() { (&Reactor{}).recordRSCommunityEdits(edits) }, "a reactor with no registry counts nothing")

	reg := newSpyRegistry()
	r := &Reactor{rmetrics: initReactorMetrics(reg, "test", "1.2.3.4", "65000")}
	r.recordRSCommunityEdits(edits)
	r.recordRSCommunityEdits(edits)
	assert.Equal(t, 2.0, rsCounterValue(t, reg, "ze_bgp_rs_prepend_applied_total"))
	assert.Equal(t, 6.0, rsCounterValue(t, reg, "ze_bgp_rs_large_control_community_stripped_total"))

	r.recordRSCommunityEdits(rsCommunityEdits{})
	assert.Equal(t, 2.0, rsCounterValue(t, reg, "ze_bgp_rs_prepend_applied_total"), "no edit, no count")
	assert.Equal(t, 6.0, rsCounterValue(t, reg, "ze_bgp_rs_large_control_community_stripped_total"), "no edit, no count")
}

// rsCounterValue reads a registered scalar counter from the spy registry.
func rsCounterValue(t *testing.T, reg *spyRegistry, name string) float64 {
	t.Helper()
	reg.mu.Lock()
	c, ok := reg.counters[name]
	reg.mu.Unlock()
	require.True(t, ok, "counter %q must be registered", name)
	return c.Value()
}
