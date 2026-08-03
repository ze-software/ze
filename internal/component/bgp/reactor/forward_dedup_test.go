// Related: forward_dedup.go -- the materialization counters these tests read
// Related: forward_dedup_bench_test.go -- the fan-out benchmark sharing this harness

package reactor

import (
	"bytes"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// newReflectorHarness builds an IBGP route reflector fanning one route out to n
// clients spread over g clusters.
//
// It is a separate fixture rather than a flag on the next-hop one because the
// edits differ in KIND: RFC 4456 Section 8 injects ORIGINATOR_ID and prepends
// CLUSTER_LIST, and the cluster id is what must keep two clients apart. A flag
// would have made one fixture that proves neither case clearly.
func newReflectorHarness(t testing.TB, n, g int) *fanoutHarness {
	t.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	wu := wireu.NewWireUpdate(fanoutPayload(), ctxID)
	wu.SetMessageID(1)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(16)
	t.Cleanup(cache.Stop)
	cache.Add(update)
	cache.Activate(1, 1)

	// An IBGP source, so RFC 4456 reflection applies at all.
	srcSettings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(forwardSourceAddr),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x0102030A,
	}
	src := NewPeer(srcSettings)
	src.state.Store(int32(PeerStateEstablished))
	src.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	src.sendCtx.Store(ctx)
	src.sendCtxID = ctxID
	src.refreshForwardFacts()

	peers := map[netip.AddrPort]*Peer{srcSettings.PeerKey(): src}
	dests := make([]*Peer, 0, n)
	for i := range n {
		cluster := i % g
		s := &PeerSettings{
			Connection:           ConnectionBoth,
			Address:              netip.AddrFrom4([4]byte{10, 2, byte(i / 256), byte(i % 256)}), //nolint:gosec // fixture index
			LocalAS:              65000,
			PeerAS:               65000,
			RouterID:             uint32(0x0A020000 + i),       //nolint:gosec // fixture index
			ClusterID:            uint32(0x0C000000 + cluster), //nolint:gosec // fixture index
			RouteReflectorClient: true,
		}
		p := NewPeer(s)
		p.state.Store(int32(PeerStateEstablished))
		p.negotiated.Store(&NegotiatedCapabilities{
			families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			ExtendedMessage: false,
		})
		p.sendCtx.Store(ctx)
		p.sendCtxID = ctxID
		p.refreshForwardFacts()
		peers[s.PeerKey()] = p
		dests = append(dests, p)
	}

	h := &fanoutHarness{}
	pool := newFwdPool(func(key fwdKey, items []fwdItem) {
		h.mu.Lock()
		defer h.mu.Unlock()
		for i := range items {
			var body []byte
			for _, raw := range items[i].rawBodies {
				body = append(body, raw...)
			}
			h.sent = append(h.sent, fanoutSent{
				peer:    key.peerAddr.Addr(),
				body:    body,
				bufIdx:  items[i].peerBufIdx,
				bufPool: items[i].peerPoolRef,
			})
		}
	}, fwdPoolConfig{chanSize: 4096, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	// Session establishment is what registers a destination's outgoing pool in
	// production (forward_pool.go RegisterOutgoingPool). Without it every
	// destination falls back to a plain allocation, so the fixture would measure
	// and assert against a path the daemon almost never takes.
	for _, p := range dests {
		pool.RegisterOutgoingPool(fwdKey{peerAddr: p.forwardFacts().peerKey}, message.MaxMsgLen)
	}

	h.adapter = &reactorAPIAdapter{r: &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peers,
		fwdPool:         pool,
		updateGroups:    NewUpdateGroupIndex(true),
	}}
	h.update = update
	h.dests = dests
	h.srcInfo = forwardSourceInfo{
		isIBGP:         true,
		isRRClient:     false,
		remoteRouterID: srcSettings.RouterID,
		globalLocalAS:  srcSettings.GlobalLocalAS,
		resolved:       true,
	}
	return h
}

// fanoutHarness stands up one received UPDATE and N destination peers spread
// over G policy groups, so a single forwardUpdateCore call reproduces the shape
// this child exists to make cheaper.
//
// The destinations are IBGP and the source is EBGP, which keeps RFC 4456
// reflection out of the fixture: every edit in it comes from the per-peer
// next-hop policy, so "same group" means "same edit set" by construction and
// nothing else varies.
type fanoutHarness struct {
	adapter *reactorAPIAdapter
	update  *ReceivedUpdate
	srcInfo forwardSourceInfo
	dests   []*Peer

	mu   sync.Mutex
	sent []fanoutSent
}

// fanoutSent is one destination's delivered wire, plus the buffer that carried
// it. The buffer identity is what proves the ownership model: a design that
// shared one buffer between destinations would show the same (pool, index) twice.
type fanoutSent struct {
	peer    netip.Addr
	body    []byte
	bufIdx  int
	bufPool *peerPool
}

// fanoutPayload is the received UPDATE every fan-out fixture forwards: ORIGIN,
// AS_PATH, NEXT_HOP, MED, LOCAL_PREF and eight communities over two prefixes.
//
// Deliberately not the four zero bytes the older forward benchmarks use. An
// empty UPDATE makes the rebuild's copy free, which is precisely the cost dedup
// exists to remove, so measuring against it would answer a question nobody asked.
func fanoutPayload() []byte {
	asPath := []byte{0x02, 0x03, 0, 0, 0xFD, 0xE9, 0, 0, 0xFD, 0xEA, 0, 0, 0xFD, 0xEB}
	var attrs []byte
	attrs = append(attrs, makeAttr(0x40, 1, []byte{0x00})...)         // ORIGIN igp
	attrs = append(attrs, makeAttr(0x40, 2, asPath)...)               // AS_PATH 65001 65002 65003
	attrs = append(attrs, makeAttr(0x40, 3, []byte{192, 0, 2, 1})...) // NEXT_HOP
	attrs = append(attrs, makeAttr(0x80, 4, []byte{0, 0, 0, 10})...)  // MED
	attrs = append(attrs, makeAttr(0x40, 5, []byte{0, 0, 0, 100})...) // LOCAL_PREF
	comms := make([]byte, 0, 32)
	for i := range 8 {
		comms = append(comms, 0xFD, 0xE9, 0x00, byte(i+1)) //nolint:gosec // fixture index
	}
	attrs = append(attrs, makeAttr(0xC0, 8, comms)...) // COMMUNITY x8
	nlri := []byte{24, 10, 0, 0, 24, 10, 0, 1}         // 10.0.0.0/24, 10.0.1.0/24
	return buildModTestPayload(attrs, nlri)
}

// newFanoutHarness builds the reactor, the source peer, and n destinations
// assigned round-robin to g policy groups.
//
// Round-robin rather than in blocks on purpose: consecutive destinations land in
// DIFFERENT groups, so a dedup that only ever compares against the immediately
// preceding destination would score zero here. Blocks would hide that.
func newFanoutHarness(t testing.TB, n, g int) *fanoutHarness {
	return newFanoutHarnessWith(t, n, g, fanoutOpts{modify: true, groups: true})
}

// fanoutOpts selects the fixture's two independent axes.
//
// groups is the existing update-groups gate, which this child inherits as its
// off switch rather than adding one (AC-10). modify turns the per-peer next-hop
// policy on and off, which is what decides whether a destination materializes at
// all.
type fanoutOpts struct {
	modify bool
	groups bool
	// dedupOff reproduces the pre-change forward rail: everything else
	// identical, the edit-set dedup alone turned off. It is what makes an A/B
	// measurable in ONE process. Comparing two `go test -bench` runs on this
	// machine cannot answer the question -- a sibling session's QEMU boot moved
	// every number by 2x mid-comparison, including a case the change cannot
	// touch -- so the two arms have to be interleaved under the same load.
	dedupOff bool
}

// newFanoutHarnessWith builds the same fixture with the per-peer next-hop policy
// on or off.
//
// With modify=false no destination accumulates an operation, so no destination
// materializes and the existing pointer-keyed body cache already shares the one
// body across every peer. That configuration is the FLOOR: it is what a fan-out
// costs when the only remaining work is the destination loop, the facts read,
// the item construction and the dispatch. The distance between it and the
// modify=true measurement is the entire budget this child can ever recover, so
// it is worth measuring before deciding the child is worth landing.
func newFanoutHarnessWith(t testing.TB, n, g int, opts fanoutOpts) *fanoutHarness {
	t.Helper()
	modify := opts.modify
	require.Positive(t, n)
	require.Positive(t, g)
	require.LessOrEqual(t, g, n)

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	wu := wireu.NewWireUpdate(fanoutPayload(), ctxID)
	wu.SetMessageID(1)
	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}

	cache := NewRecentUpdateCache(16)
	t.Cleanup(cache.Stop)
	cache.Add(update)
	// One consumer that never acks, so the entry outlives every RetainN/Release
	// pair the fan-out performs and the fixture measures forwarding rather than
	// cache eviction.
	cache.Activate(1, 1)

	src := makeForwardSourcePeer(t, ctx, ctxID)
	peers := map[netip.AddrPort]*Peer{src.Settings().PeerKey(): src}

	dests := make([]*Peer, 0, n)
	for i := range n {
		group := i % g
		s := &PeerSettings{
			Connection: ConnectionBoth,
			Address:    netip.AddrFrom4([4]byte{10, 1, byte(i / 256), byte(i % 256)}), //nolint:gosec // fixture index
			LocalAS:    65000,
			PeerAS:     65000,
			RouterID:   uint32(0x0A010000 + i), //nolint:gosec // fixture index
		}
		if modify {
			// The group's own next-hop. Identical within a group and distinct
			// across groups, which is what makes the equality classes exactly g.
			s.NextHopMode = NextHopSelf
			s.LocalAddress = netip.AddrFrom4([4]byte{10, 99, byte(group / 256), byte(group % 256)}) //nolint:gosec // fixture index
		}
		p := NewPeer(s)
		p.state.Store(int32(PeerStateEstablished))
		p.negotiated.Store(&NegotiatedCapabilities{
			families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			ExtendedMessage: false,
		})
		p.sendCtx.Store(ctx)
		p.sendCtxID = ctxID
		p.refreshForwardFacts()
		peers[s.PeerKey()] = p
		dests = append(dests, p)
	}

	h := &fanoutHarness{}
	// The bytes are COPIED inside the handler, never retained as slices. A
	// forward item's rawBodies alias a per-peer pool buffer that releaseItem
	// returns as soon as the handler returns, so a test that kept the slice
	// would read whatever the next UPDATE wrote there and call it evidence.
	pool := newFwdPool(func(key fwdKey, items []fwdItem) {
		h.mu.Lock()
		defer h.mu.Unlock()
		for i := range items {
			var body []byte
			for _, raw := range items[i].rawBodies {
				body = append(body, raw...)
			}
			h.sent = append(h.sent, fanoutSent{
				peer:    key.peerAddr.Addr(),
				body:    body,
				bufIdx:  items[i].peerBufIdx,
				bufPool: items[i].peerPoolRef,
			})
		}
	}, fwdPoolConfig{chanSize: 4096, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	// Session establishment is what registers a destination's outgoing pool in
	// production (forward_pool.go RegisterOutgoingPool). Without it every
	// destination falls back to a plain allocation, so the fixture would measure
	// and assert against a path the daemon almost never takes.
	for _, p := range dests {
		pool.RegisterOutgoingPool(fwdKey{peerAddr: p.forwardFacts().peerKey}, message.MaxMsgLen)
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peers,
		fwdPool:         pool,
		// The dedup this child adds is gated on update groups, exactly as the
		// pointer-keyed body cache it replaces already was. A fixture that left
		// this nil would measure the ungated path and report a flat result as a
		// failed change.
		updateGroups:    NewUpdateGroupIndex(opts.groups),
		forwardDedupOff: opts.dedupOff,
	}

	h.adapter = &reactorAPIAdapter{r: r}
	h.update = update
	h.dests = dests
	h.srcInfo = forwardSourceInfo{
		isIBGP:         false,
		remoteRouterID: src.RemoteRouterID(),
		globalLocalAS:  src.Settings().GlobalLocalAS,
		resolved:       true,
	}
	return h
}

// forward runs one fan-out call over every destination.
func (h *fanoutHarness) forward() error {
	return h.adapter.forwardUpdateCore(h.update, 1, h.dests, h.srcInfo)
}

// delivered waits for every destination's worker to hand its bytes back and
// returns what each peer received.
//
// It waits on the COUNT rather than on a duration: the forward pool dispatches
// to per-peer workers, so the bytes arrive on other goroutines and a fixed sleep
// would be a load-sensitive assertion (ai/rules/completion.md).
func (h *fanoutHarness) delivered(t testing.TB, want int) []fanoutSent {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		h.mu.Lock()
		n := len(h.sent)
		h.mu.Unlock()
		if n >= want {
			break
		}
		if time.Now().After(deadline) {
			require.Failf(t, "forward did not reach every destination",
				"wanted %d delivered items, saw %d", want, n)
		}
		time.Sleep(time.Millisecond) // poll interval; the loop returns as soon as every worker has reported
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]fanoutSent(nil), h.sent...)
}

// fanoutCounters snapshots the three fan-out counters.
type fanoutCounters struct {
	materializations uint64
	dedupHits        uint64
	collisions       uint64
}

func readFanoutCounters() fanoutCounters {
	return fanoutCounters{
		materializations: fwdMaterializations.Load(),
		dedupHits:        fwdDedupHits.Load(),
		collisions:       fwdDedupCollisions.Load(),
	}
}

func (c fanoutCounters) since(prev fanoutCounters) fanoutCounters {
	return fanoutCounters{
		materializations: c.materializations - prev.materializations,
		dedupHits:        c.dedupHits - prev.dedupHits,
		collisions:       c.collisions - prev.collisions,
	}
}

// fanoutCounterMu serializes the counter-reading tests. The counters are
// process-global, so two tests sampling them concurrently would each attribute
// the other's materializations to itself.
var fanoutCounterMu sync.Mutex

// fanoutCases are the (destinations, policy groups) points the benchmark and the
// counting test share. (1,1) and (2,1) are where dedup must not cost anything;
// (100,100) is the worst case, where every destination is its own group and the
// fingerprint buys nothing at all.
var fanoutCases = []struct{ n, g int }{
	{1, 1}, {2, 1}, {2, 2}, {10, 2}, {100, 2}, {100, 100},
}

// VALIDATES: AC-1 -- a fan-out of N destinations over G distinct edit sets
// performs exactly G materializations, not N.
// PREVENTS: a dedup that silently degrades to per-destination work, which is
// indistinguishable from working correctly at the wire and is the whole point of
// this child.
func TestFanoutMaterializesOncePerGroup(t *testing.T) {
	fanoutCounterMu.Lock()
	defer fanoutCounterMu.Unlock()

	for _, tc := range fanoutCases {
		t.Run(fanoutCaseName(tc.n, tc.g), func(t *testing.T) {
			h := newFanoutHarness(t, tc.n, tc.g)
			before := readFanoutCounters()
			require.NoError(t, h.forward())
			got := readFanoutCounters().since(before)

			require.Equal(t, uint64(tc.g), got.materializations, //nolint:gosec // fixture count
				"a fan-out of %d destinations over %d policy groups must materialize %d times",
				tc.n, tc.g, tc.g)
			require.Equal(t, uint64(tc.n-tc.g), got.dedupHits, //nolint:gosec // fixture count
				"every destination beyond the first of its group reuses a materialization")
			require.Zero(t, got.collisions,
				"the fixture's edit sets are distinct by construction, so no candidate should be refused")
		})
	}
}

// VALIDATES: AC-2, AC-3 -- destinations in one policy group receive
// byte-identical UPDATEs, and destinations in different groups do not.
// PREVENTS: THE catastrophic failure. Sharing a rebuild is the only mechanism in
// this work that can move one peer's bytes to another, and a wrong share is
// silent: the peer cannot tell it was sent somebody else's next-hop. This test
// reads what every destination actually received rather than trusting the
// counters, because the counters would look perfect either way.
func TestSharedMaterializationBytesIdentical(t *testing.T) {
	const n, g = 12, 3
	h := newFanoutHarness(t, n, g)
	require.NoError(t, h.forward())
	sent := h.delivered(t, n)
	require.Len(t, sent, n)

	// The group's next-hop is its identity, and it is the fourth octet of the
	// LocalAddress the harness gave every member (10.99.0.<group>).
	byGroup := make(map[int][]fanoutSent, g)
	for i, s := range sent {
		_ = i
		byGroup[groupOfPeer(t, h, s.peer)] = append(byGroup[groupOfPeer(t, h, s.peer)], s)
	}
	require.Len(t, byGroup, g, "every group must have received something")

	seen := make(map[string]int, g)
	for group, members := range byGroup {
		require.Len(t, members, n/g)
		first := members[0].body
		require.NotEmpty(t, first)
		for _, m := range members[1:] {
			require.Equal(t, first, m.body,
				"group %d shares one rebuild, so its members must receive identical bytes", group)
		}
		seen[string(first)]++
	}
	require.Len(t, seen, g,
		"each group's policy produces its own bytes; %d distinct wires for %d groups means a group received another group's UPDATE", len(seen), g)
}

// VALIDATES: AC-8, A-4 -- no two forward items reference the same buffer, so
// every buffer is owned by exactly one item and returned exactly once.
// PREVENTS: R-3. The design shares the REBUILD, not the buffer, and this is what
// pins that. If a later change starts handing one buffer to several items, the
// release ordering becomes the caller's problem and a worker can write bytes a
// sibling has already returned to the pool.
func TestSharedBufferOwnedByExactlyOneItem(t *testing.T) {
	const n, g = 12, 3
	h := newFanoutHarness(t, n, g)
	require.NoError(t, h.forward())
	sent := h.delivered(t, n)
	require.Len(t, sent, n)

	type bufRef struct {
		pool *peerPool
		idx  int
	}
	owners := make(map[bufRef]netip.Addr, n)
	pooled := 0
	for _, s := range sent {
		if s.bufIdx == 0 {
			continue // independently allocated; nothing to double-return
		}
		pooled++
		ref := bufRef{pool: s.bufPool, idx: s.bufIdx}
		prev, dup := owners[ref]
		require.False(t, dup,
			"peers %s and %s hold the same pool buffer: whichever worker finishes first returns it under the other",
			prev, s.peer)
		owners[ref] = s.peer
	}
	require.Positive(t, pooled, "the fixture must exercise the per-peer pool path, not only the fallback")
}

// VALIDATES: AC-6, R-5 -- reflector clients in different clusters never share a
// rebuild.
// PREVENTS: an RFC 4456 Section 8 violation that a test on the counters alone
// would miss. CLUSTER_LIST is per-cluster loop prevention, so a client receiving
// another cluster's list can accept a route it must reject, and the resulting
// loop is persistent rather than transient.
func TestReflectorClustersNeverShare(t *testing.T) {
	fanoutCounterMu.Lock()
	defer fanoutCounterMu.Unlock()

	h := newReflectorHarness(t, 6, 3)
	before := readFanoutCounters()
	require.NoError(t, h.forward())
	got := readFanoutCounters().since(before)
	sent := h.delivered(t, 6)
	require.Len(t, sent, 6)

	require.Equal(t, uint64(3), got.materializations,
		"three clusters means three distinct CLUSTER_LISTs and so three rebuilds")
	bodies := make(map[string]int, 3)
	for _, s := range sent {
		bodies[string(s.body)]++
	}
	require.Len(t, bodies, 3,
		"each cluster must receive its own CLUSTER_LIST; fewer distinct wires means a client got another cluster's")
}

// VALIDATES: AC-10 -- the existing update-groups gate still turns everything off.
// PREVENTS: a feature that cannot be disabled. The failure mode here is one peer
// receiving another's UPDATE, so an operator must be able to switch it off
// without an upgrade.
func TestGroupsDisabledNoDedup(t *testing.T) {
	fanoutCounterMu.Lock()
	defer fanoutCounterMu.Unlock()

	for _, tc := range []struct {
		name string
		opts fanoutOpts
	}{
		{"update groups disabled", fanoutOpts{modify: true, groups: false}},
		{"dedup escape hatch", fanoutOpts{modify: true, groups: true, dedupOff: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newFanoutHarnessWith(t, 10, 2, tc.opts)
			before := readFanoutCounters()
			require.NoError(t, h.forward())
			got := readFanoutCounters().since(before)
			require.Equal(t, uint64(10), got.materializations,
				"with dedup off every destination rebuilds its own payload")
			require.Zero(t, got.dedupHits)
		})
	}
}

// overrideMED is the marker byte the export policy chain's raw override writes
// into MULTI_EXIT_DISC, so a destination's delivered wire says which BASE it was
// rebuilt from rather than only which edit set was applied to it.
const overrideMED = 0xAA

// fanoutOverridePayload is the raw full-UPDATE-body replacement a raw=true
// export filter returns. It is fanoutPayload with one attribute value changed,
// so the two bases differ by a needle a test can point at while every other
// property (length, attribute order, NLRI) stays equal.
func fanoutOverridePayload() []byte {
	asPath := []byte{0x02, 0x03, 0, 0, 0xFD, 0xE9, 0, 0, 0xFD, 0xEA, 0, 0, 0xFD, 0xEB}
	var attrs []byte
	attrs = append(attrs, makeAttr(0x40, 1, []byte{0x00})...)
	attrs = append(attrs, makeAttr(0x40, 2, asPath)...)
	attrs = append(attrs, makeAttr(0x40, 3, []byte{192, 0, 2, 1})...)
	attrs = append(attrs, makeAttr(0x80, 4, []byte{0, 0, 0, overrideMED})...) // the marker
	attrs = append(attrs, makeAttr(0x40, 5, []byte{0, 0, 0, 100})...)
	comms := make([]byte, 0, 32)
	for i := range 8 {
		comms = append(comms, 0xFD, 0xE9, 0x00, byte(i+1)) //nolint:gosec // fixture index
	}
	attrs = append(attrs, makeAttr(0xC0, 8, comms)...)
	nlri := []byte{24, 10, 0, 0, 24, 10, 0, 1}
	return buildModTestPayload(attrs, nlri)
}

// VALIDATES: AC-2, AC-3, R-1 -- the dedup identity's BASE half. Two destinations
// in ONE policy group (identical edit set, so identical digest and fingerprint)
// whose BASES differ must not share a materialization.
// PREVENTS: the cross-peer wrong-wire leak this child exists to prevent, entered
// through the one door the digest cannot see. `exportWireOverride` is declared
// inside forwardUpdateCore's destination loop (reactor_api_forward.go), assigned
// from an egress filter's res.wireOverride and fed to peerBaseWire, so a
// destination carrying a raw export override and a sibling without one produce
// IDENTICAL digests over DIFFERENT bases. Drop `|| e.id != id` from
// fwdDedupTable.begin and the sibling is sent bytes rebuilt from the other
// peer's payload -- silently, because the digest, the counters and every other
// fan-out test agree it was a legitimate hit.
func TestDifferentBaseSameEditSetNeverShares(t *testing.T) {
	fanoutCounterMu.Lock()
	defer fanoutCounterMu.Unlock()

	// ONE policy group: both destinations get the same NextHopSelf local address,
	// so their edit sets -- and therefore their digests and fingerprints -- are
	// equal by construction. The base is the only thing left that differs.
	h := newFanoutHarnessWith(t, 2, 1, fanoutOpts{modify: true, groups: true})

	override := fanoutOverridePayload()
	r := h.adapter.r
	r.api = &pluginserver.Server{} // non-nil: past the fail-closed r.api guard
	// A raw=true filter returning a full UPDATE-body replacement. Terminal for the
	// chain, and the production producer of a non-nil res.wireOverride.
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Raw: override}
	}
	r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}

	// Only the FIRST destination has an export policy, so only it gets a wire
	// override. The second runs the same chain step, finds no filters, and keeps
	// the original payload as its base.
	overridden := h.dests[0]
	overridden.settings.ExportFilters = []filterapi.FilterRef{{Name: "someplugin:rewrite"}}
	overridden.refreshForwardFacts()
	require.Empty(t, h.dests[1].forwardFacts().exportFilters,
		"the second destination must reach the chain with no filters, so its base stays the original")

	before := readFanoutCounters()
	require.NoError(t, h.forward())
	got := readFanoutCounters().since(before)
	sent := h.delivered(t, 2)
	require.Len(t, sent, 2)

	byPeer := make(map[netip.Addr][]byte, 2)
	for _, s := range sent {
		byPeer[s.peer] = s.body
	}
	overriddenBody := byPeer[overridden.Settings().Address]
	plainBody := byPeer[h.dests[1].Settings().Address]
	require.NotEmpty(t, overriddenBody)
	require.NotEmpty(t, plainBody)

	// The needle: the whole MULTI_EXIT_DISC attribute, header included, so a
	// coincidental byte run elsewhere cannot satisfy it.
	overrideNeedle := makeAttr(0x80, 4, []byte{0, 0, 0, overrideMED})
	originalNeedle := makeAttr(0x80, 4, []byte{0, 0, 0, 10})

	// The wire assertions come first: they are the leak itself, and a failure
	// here prints the borrowed bytes rather than a counter.
	require.True(t, bytes.Contains(overriddenBody, overrideNeedle),
		"the destination with the export override must be rebuilt from the OVERRIDE base\noverridden peer wire: %x", overriddenBody)
	require.True(t, bytes.Contains(plainBody, originalNeedle),
		"the destination without an override must be rebuilt from the ORIGINAL base\nplain peer wire: %x", plainBody)
	require.False(t, bytes.Contains(plainBody, overrideNeedle),
		"the destination WITHOUT an export override received bytes rebuilt from the other peer's override base -- this is the cross-peer wire leak\nplain peer wire: %x\noverridden peer wire: %x", plainBody, overriddenBody)
	require.NotEqual(t, overriddenBody, plainBody,
		"two destinations over two different bases must not receive one wire")

	// And the mechanism behind them: each destination rebuilt for itself.
	require.Equal(t, uint64(2), got.materializations,
		"same edit set but different bases is two equality classes, so two rebuilds")
	require.Zero(t, got.dedupHits,
		"a fingerprint match over a different BASE must not be answered from the table")
}

// VALIDATES: AC-4, R-1 -- inside the forward rail, a fingerprint that matches an
// entry whose edit set differs does NOT authorize sharing.
// PREVENTS: the collision becoming a leak. The filterapi test proves the
// comparison rejects unequal digests; this proves the TABLE consults it, which
// is the half a future refactor could drop while every other test stayed green.
//
// The collision is planted rather than searched for. A real 64-bit collision
// cannot be produced on demand, and waiting for one is not a test.
func TestCollisionInTableDoesNotShare(t *testing.T) {
	fanoutCounterMu.Lock()
	defer fanoutCounterMu.Unlock()

	table := getFwdDedupTable()
	defer putFwdDedupTable(table)

	base := &wireu.WireUpdate{}
	id := fwdDedupIdentity{base: base}

	var victim filterapi.ModAccumulator
	victim.Op(3, filterapi.AttrModSet, []byte{10, 0, 0, 1})
	victimPayload := []byte("victim-wire")
	shared, cand := table.begin(id, &victim)
	require.Nil(t, shared)
	require.True(t, cand.valid)
	table.commit(cand, victimPayload)

	// A different destination whose edit set differs. Forge the collision by
	// rewriting the recorded entry's fingerprint to whatever this one hashes to,
	// which is exactly the state a real collision leaves the table in.
	var other filterapi.ModAccumulator
	other.Op(3, filterapi.AttrModSet, []byte{192, 168, 0, 1})
	probe, ok := other.AppendEditDigest(nil)
	require.True(t, ok)
	require.Len(t, table.entries, 1)
	table.entries[0].fp = filterapi.EditFingerprint(probe)
	clear(table.index[:])
	for slot := int(table.entries[0].fp & fwdDedupIndexMask); ; slot = (slot + 1) & fwdDedupIndexMask {
		if table.index[slot] == fwdDedupIndexAbsent {
			table.index[slot] = 1
			break
		}
	}

	before := readFanoutCounters()
	got, cand2 := table.begin(id, &other)
	after := readFanoutCounters().since(before)

	require.Nil(t, got,
		"a fingerprint match over a DIFFERENT edit set must not hand over another destination's wire")
	require.True(t, cand2.valid, "the destination must go on to rebuild its own bytes")
	require.Equal(t, uint64(1), after.collisions, "a refused candidate must be counted, not silent")
	require.Zero(t, after.dedupHits)
}

// VALIDATES: AC-7, R-4 -- a destination with an empty edit set never reaches the
// dedup table, so the zero-copy passthrough is untouched.
// PREVENTS: turning a free forward into a hashed one.
func TestEmptyEditSetSkipsDedup(t *testing.T) {
	fanoutCounterMu.Lock()
	defer fanoutCounterMu.Unlock()

	h := newFanoutHarnessWith(t, 10, 2, fanoutOpts{modify: false, groups: true})
	before := readFanoutCounters()
	require.NoError(t, h.forward())
	got := readFanoutCounters().since(before)
	require.Zero(t, got.materializations, "nothing to modify means nothing to rebuild")
	require.Zero(t, got.dedupHits, "and so nothing to share")
}

// groupOfPeer recovers a destination's policy group from its address, so an
// assertion can talk about groups rather than about peer indices.
func groupOfPeer(t testing.TB, h *fanoutHarness, addr netip.Addr) int {
	t.Helper()
	for i, p := range h.dests {
		if p.Settings().Address == addr {
			return i % fanoutGroupsOf(h)
		}
	}
	require.Failf(t, "unknown destination", "no harness peer has address %s", addr)
	return -1
}

// fanoutGroupsOf recovers g from the harness by counting distinct next-hops.
func fanoutGroupsOf(h *fanoutHarness) int {
	seen := make(map[netip.Addr]struct{}, len(h.dests))
	for _, p := range h.dests {
		seen[p.Settings().LocalAddress] = struct{}{}
	}
	return len(seen)
}

func fanoutCaseName(n, g int) string {
	var b [24]byte
	out := append(b[:0], "n="...)
	out = appendFanoutInt(out, n)
	out = append(out, "/g="...)
	out = appendFanoutInt(out, g)
	return string(out)
}

func appendFanoutInt(dst []byte, v int) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var tmp [8]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, tmp[i:]...)
}
