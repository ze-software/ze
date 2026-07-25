package redistributeegress

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/ze"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type injectedEntry struct {
	fam   family.Family
	entry configredist.RouteEntry
}

type withdrawnEntry struct {
	fam    family.Family
	prefix string
}

type stubConsumer struct {
	name      string
	mu        sync.Mutex
	injected  []injectedEntry
	withdrawn []withdrawnEntry
}

func (s *stubConsumer) Name() string { return s.name }

func (s *stubConsumer) InjectRoute(_ context.Context, fam family.Family, entry configredist.RouteEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.injected = append(s.injected, injectedEntry{fam: fam, entry: entry})
}

func (s *stubConsumer) WithdrawRoute(_ context.Context, fam family.Family, prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.withdrawn = append(s.withdrawn, withdrawnEntry{fam: fam, prefix: prefix})
}

func (s *stubConsumer) snapshotInjected() []injectedEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]injectedEntry, len(s.injected))
	copy(out, s.injected)
	return out
}

func (s *stubConsumer) snapshotWithdrawn() []withdrawnEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]withdrawnEntry, len(s.withdrawn))
	copy(out, s.withdrawn)
	return out
}

type testBusSubscription struct {
	ns      string
	et      string
	handler func(any)
}

type testBus struct {
	mu   sync.Mutex
	subs []*testBusSubscription
}

var _ ze.EventBus = (*testBus)(nil)

func newTestBus() *testBus { return &testBus{} }

func (b *testBus) Emit(ns, et string, payload any) (int, error) {
	b.mu.Lock()
	hs := make([]func(any), 0, len(b.subs))
	for _, s := range b.subs {
		if s.ns == ns && s.et == et {
			hs = append(hs, s.handler)
		}
	}
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *testBus) Subscribe(ns, et string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &testBusSubscription{ns: ns, et: et, handler: handler}
	b.subs = append(b.subs, s)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, ss := range b.subs {
			if ss == s {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				return
			}
		}
	}
}

func resetState(t *testing.T) {
	t.Helper()
	redistevents.ResetForTest()
	configredist.SetGlobal(nil)
	configredist.ResetConsumersForTest()
	eventBusPtr.Store(nil)
	t.Cleanup(func() {
		redistevents.ResetForTest()
		configredist.SetGlobal(nil)
		configredist.ResetConsumersForTest()
		eventBusPtr.Store(nil)
	})
}

func registerBGPConsumer(t *testing.T) *stubConsumer {
	t.Helper()
	c := &stubConsumer{name: "bgp"}
	require.NoError(t, configredist.RegisterConsumer(c))
	return c
}

func skipIDs(ids ...redistevents.ProtocolID) map[redistevents.ProtocolID]bool {
	m := make(map[redistevents.ProtocolID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func addBatch(p redistevents.ProtocolID, afi uint16, prefix, nh string) *redistevents.RouteChangeBatch {
	pp := netip.MustParsePrefix(prefix)
	var addr netip.Addr
	if nh != "" {
		addr = netip.MustParseAddr(nh)
	}
	return &redistevents.RouteChangeBatch{
		Protocol: p,
		AFI:      afi,
		SAFI:     safiUnicst,
		Entries:  []redistevents.RouteChangeEntry{{Action: redistevents.ActionAdd, Prefix: pp, NextHop: addr}},
	}
}

func removeBatch(p redistevents.ProtocolID, afi uint16, prefix string) *redistevents.RouteChangeBatch {
	pp := netip.MustParsePrefix(prefix)
	return &redistevents.RouteChangeBatch{
		Protocol: p,
		AFI:      afi,
		SAFI:     safiUnicst,
		Entries:  []redistevents.RouteChangeEntry{{Action: redistevents.ActionRemove, Prefix: pp}},
	}
}

const (
	afiIPv4    = 1
	afiIPv6    = 2
	safiUnicst = 1
)

// VALIDATES: AC-10 -- accepted add dispatches to consumer's InjectRoute.
// PREVENTS: Orchestrator not dispatching to registered consumers.
func TestHandleBatchAcceptedAddDispatches(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv4, "10.0.0.1/32", ""))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, family.IPv4Unicast, inj[0].fam)
	assert.Equal(t, "10.0.0.1/32", inj[0].entry.Prefix)
	assert.Equal(t, "", inj[0].entry.NextHop)
}

// VALIDATES: AC-10 -- IPv6 add dispatches correctly.
// PREVENTS: IPv6 family lost through orchestrator dispatch.
func TestHandleBatchAcceptedAddIPv6(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv6Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv6, "2001:db8::1/128", ""))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, family.IPv6Unicast, inj[0].fam)
	assert.Equal(t, "2001:db8::1/128", inj[0].entry.Prefix)
}

// VALIDATES: AC-10 -- explicit next-hop preserved through dispatch.
// PREVENTS: NextHop silently replaced with empty.
func TestHandleBatchExplicitNextHop(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv4, "10.0.0.1/32", "192.0.2.1"))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, "192.0.2.1", inj[0].entry.NextHop)
}

// VALIDATES: Evaluator rejects batch family not in import rule.
// PREVENTS: Routes leaking past the evaluator.
func TestHandleBatchRejectedAddNoop(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv6Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, consumer.snapshotInjected())
}

// VALIDATES: AC-11 -- accepted remove dispatches to consumer's WithdrawRoute.
// PREVENTS: Withdraw not reaching consumer.
func TestHandleBatchRemoveDispatches(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), removeBatch(id, afiIPv4, "10.0.0.1/32"))

	wd := consumer.snapshotWithdrawn()
	require.Len(t, wd, 1)
	assert.Equal(t, family.IPv4Unicast, wd[0].fam)
	assert.Equal(t, "10.0.0.1/32", wd[0].prefix)
}

// VALIDATES: Evaluator nil means no dispatch and no panic.
// PREVENTS: Plugin crashing when redistribute is unconfigured.
func TestHandleBatchNoEvaluatorNoop(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(id, afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, consumer.snapshotInjected())
}

// VALIDATES: Atomic swap of the global evaluator changes subsequent accept decisions.
// PREVENTS: Orchestrator holding a stale evaluator pointer after reload.
func TestHandleBatchReloadApplies(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator(nil))

	consumer := registerBGPConsumer(t)
	batch := addBatch(id, afiIPv4, "10.0.0.1/32", "")

	handleBatch(context.Background(), skipIDs(bgpID), batch)
	assert.Empty(t, consumer.snapshotInjected(), "first call should be rejected by empty rules")

	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	handleBatch(context.Background(), skipIDs(bgpID), batch)
	assert.Len(t, consumer.snapshotInjected(), 1, "second call should be accepted after reload")
}

// VALIDATES: Batches whose protocol matches a consumer are skipped for that consumer.
// PREVENTS: Consumer-sourced batches being re-dispatched to the same protocol, creating a loop.
func TestHandleBatchConsumerSourceSkipped(t *testing.T) {
	resetState(t)

	bgpID := redistevents.RegisterProtocol("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(bgpID, afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, consumer.snapshotInjected())
}

func TestHandleBatchConsumerSourceDispatchesToOtherConsumers(t *testing.T) {
	resetState(t)

	bgpID := redistevents.RegisterProtocol("bgp")
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "bgp", Families: []family.Family{family.IPv6Unicast}},
	}))

	bgpConsumer := registerBGPConsumer(t)
	ospfConsumer := &stubConsumer{name: "ospf"}
	require.NoError(t, configredist.RegisterConsumer(ospfConsumer))

	handleBatch(context.Background(), skipIDs(bgpID), addBatch(bgpID, afiIPv6, "2001:db8:5e5::/48", "fd00:1e::4"))

	assert.Empty(t, bgpConsumer.snapshotInjected(), "BGP consumer must not receive its own source")
	inj := ospfConsumer.snapshotInjected()
	require.Len(t, inj, 1, "non-BGP consumers should receive BGP source batches")
	assert.Equal(t, "2001:db8:5e5::/48", inj[0].entry.Prefix)
	assert.Equal(t, "bgp", inj[0].entry.Source)
}

// VALIDATES: Defense-in-depth -- batch with an unregistered ProtocolID is dropped.
// PREVENTS: Memory corruption / drive-by injection via a forged ProtocolID.
func TestHandleBatchUnknownProtocol(t *testing.T) {
	resetState(t)

	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), addBatch(redistevents.ProtocolID(99), afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, consumer.snapshotInjected())
}

// VALIDATES: AC-5 -- a nonzero per-entry OriginAS is preferred over the batch
// OriginASN; a zero per-entry OriginAS falls back to the batch OriginASN,
// preserving the as112 single-ASN virtual-router behavior.
// PREVENTS: BGP best-paths redistributed with their own per-prefix origin AS
// being flattened to one batch-level ASN (or losing their origin AS entirely).
func TestHandleBatchPrefersEntryOriginAS(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))
	consumer := registerBGPConsumer(t)

	// Nonzero per-entry OriginAS wins over the batch OriginASN.
	handleBatch(context.Background(), skipIDs(bgpID), &redistevents.RouteChangeBatch{
		Protocol:  id,
		AFI:       afiIPv4,
		SAFI:      safiUnicst,
		OriginASN: 65001,
		Entries: []redistevents.RouteChangeEntry{{
			Action:   redistevents.ActionAdd,
			Prefix:   netip.MustParsePrefix("10.0.0.1/32"),
			OriginAS: 64512,
		}},
	})

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1)
	assert.Equal(t, uint32(64512), inj[0].entry.OriginASN, "per-entry OriginAS must win over batch OriginASN")

	// Zero per-entry OriginAS falls back to the batch OriginASN.
	handleBatch(context.Background(), skipIDs(bgpID), &redistevents.RouteChangeBatch{
		Protocol:  id,
		AFI:       afiIPv4,
		SAFI:      safiUnicst,
		OriginASN: 65001,
		Entries: []redistevents.RouteChangeEntry{{
			Action: redistevents.ActionAdd,
			Prefix: netip.MustParsePrefix("10.0.0.2/32"),
		}},
	})

	inj = consumer.snapshotInjected()
	require.Len(t, inj, 2)
	assert.Equal(t, uint32(65001), inj[1].entry.OriginASN, "zero per-entry OriginAS falls back to batch OriginASN")
}

// VALIDATES: subscribe() listens to every producer; same-protocol loop prevention is per-consumer.
// PREVENTS: a source such as BGP being skipped globally, which would block BGP -> OSPF redistribution.
func TestSubscribeIncludesConsumerProtocolProducer(t *testing.T) {
	resetState(t)

	bgpID := redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProducer(bgpID)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)

	bus := newTestBus()
	setEventBus(bus)
	unsubs := subscribe(context.Background(), bus, skipIDs(bgpID))
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	require.Len(t, unsubs, 2, "both BGP and fakeredist producers should be subscribed")
}

// VALIDATES: subscribe() builds a typed handle per producer; emit is delivered.
// PREVENTS: Wrong namespace / event type being subscribed.
func TestSubscribeNonConsumerProducers(t *testing.T) {
	resetState(t)

	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)

	bus := newTestBus()
	setEventBus(bus)
	consumer := registerBGPConsumer(t)
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	unsubs := subscribe(context.Background(), bus, skipIDs(bgpID))
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	_, err := bus.Emit("fakeredist", redistevents.EventType, addBatch(fakeID, afiIPv4, "10.0.0.1/32", ""))
	require.NoError(t, err)

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 1, "subscriber should receive emitted batch")
	assert.Equal(t, "10.0.0.1/32", inj[0].entry.Prefix)
}
