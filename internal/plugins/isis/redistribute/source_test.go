// Design: docs/architecture/isis/isis-11-redistribution.md -- IS-IS redistribution source tests.
//
// VALIDATES: spec-isis-11 producer side (AC-1, AC-2, AC-7, AC-8, AC-9) -- the
//            single config source "isis" is registered (idempotent, no per-level
//            names); SPF route deltas (both levels) are emitted as redistevents
//            RouteChangeBatch with Protocol = the single isis ProtocolID and the
//            correct ActionAdd/ActionRemove; withdrawals propagate; connected /
//            passive-interface prefixes become TLV 135 PrefixInfo; registration
//            order does not matter.
// PREVENTS:  per-level source names that would defeat self-import rejection; a
//            missing withdraw path; a producer batch with the wrong protocol.

package isisredistribute

import (
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
)

// captureBus is a minimal EventBus stand-in that records emitted batches as
// detached value copies (the producer releases the pooled batch after Emit, so the
// capture must copy the entries before returning).
type captureBatch struct {
	protocol redistevents.ProtocolID
	afi      uint16
	safi     uint8
	entries  []redistevents.RouteChangeEntry
}

// TestISISRegisterSource verifies the single config source "isis" is registered
// with Protocol "isis", is idempotent, is found by LookupSource, and that NO
// per-level isis-l1 / isis-l2 names exist (AC-1, AC-10 foundation).
func TestISISRegisterSource(t *testing.T) {
	RegisterISISSources()
	RegisterISISSources() // idempotent: a second call must not error or duplicate

	src, ok := configredist.LookupSource("isis")
	if !ok {
		t.Fatal("LookupSource(\"isis\") not found after RegisterISISSources")
	}
	if src.Protocol != "isis" {
		t.Fatalf("source protocol = %q, want isis", src.Protocol)
	}

	for _, bad := range []string{"isis-l1", "isis-l2"} {
		if _, ok := configredist.LookupSource(bad); ok {
			t.Fatalf("per-level source %q must NOT be registered (single source only)", bad)
		}
	}
}

// TestISISRedistDeltaToBatch verifies an SPF RouteDelta (both an L1 and an L2
// route) converts to one ActionAdd batch carrying both prefixes under the single
// isis ProtocolID (AC-1, AC-2). Level is NOT a redistribution selector.
func TestISISRedistDeltaToBatch(t *testing.T) {
	delta := spf.RouteDelta{
		Added: []spf.RouteEntry{
			{
				Prefix:   netip.MustParsePrefix("10.1.0.0/24"),
				Metric:   10,
				Level:    spf.Level1,
				NextHops: []spf.NextHop{{Addr: netip.MustParseAddr("192.0.2.1")}},
			},
			{
				Prefix:   netip.MustParsePrefix("10.2.0.0/24"),
				Metric:   20,
				Level:    spf.Level2,
				NextHops: []spf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2")}},
			},
		},
	}

	var captured []captureBatch
	emit := func(b *redistevents.RouteChangeBatch) {
		cb := captureBatch{protocol: b.Protocol, afi: b.AFI, safi: b.SAFI}
		cb.entries = append(cb.entries, b.Entries...)
		captured = append(captured, cb)
	}

	emitDelta(delta, testProtocolID(), emit)

	if len(captured) != 1 {
		t.Fatalf("got %d batches, want 1", len(captured))
	}
	b := captured[0]
	if b.protocol != testProtocolID() {
		t.Fatalf("batch protocol = %d, want %d", b.protocol, testProtocolID())
	}
	if b.afi != uint16(family.AFIIPv4) || b.safi != uint8(family.SAFIUnicast) {
		t.Fatalf("batch family = afi %d safi %d, want ipv4/unicast", b.afi, b.safi)
	}
	if len(b.entries) != 2 {
		t.Fatalf("got %d entries, want 2 (one L1, one L2 -- single source, both levels)", len(b.entries))
	}
	seen := map[netip.Prefix]redistevents.RouteChangeEntry{}
	for _, e := range b.entries {
		if e.Action != redistevents.ActionAdd {
			t.Fatalf("entry %v action = %v, want add", e.Prefix, e.Action)
		}
		seen[e.Prefix] = e
	}
	if _, ok := seen[netip.MustParsePrefix("10.1.0.0/24")]; !ok {
		t.Fatal("L1 route 10.1.0.0/24 not exported")
	}
	if _, ok := seen[netip.MustParsePrefix("10.2.0.0/24")]; !ok {
		t.Fatal("L2 route 10.2.0.0/24 not exported")
	}
}

// TestISISRedistDeltaWithdraw verifies a removed SPF route becomes an ActionRemove
// entry (AC-7: producer-side withdraw propagation).
func TestISISRedistDeltaWithdraw(t *testing.T) {
	delta := spf.RouteDelta{
		Removed: []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")},
	}

	var captured []captureBatch
	emit := func(b *redistevents.RouteChangeBatch) {
		cb := captureBatch{protocol: b.Protocol}
		cb.entries = append(cb.entries, b.Entries...)
		captured = append(captured, cb)
	}

	emitDelta(delta, testProtocolID(), emit)

	if len(captured) != 1 || len(captured[0].entries) != 1 {
		t.Fatalf("withdraw delta did not produce one entry: %+v", captured)
	}
	e := captured[0].entries[0]
	if e.Action != redistevents.ActionRemove {
		t.Fatalf("entry action = %v, want remove", e.Action)
	}
	if e.Prefix != netip.MustParsePrefix("10.9.0.0/24") {
		t.Fatalf("withdraw prefix = %v, want 10.9.0.0/24", e.Prefix)
	}
}

// TestISISRedistDeltaEmpty verifies an empty delta emits nothing (no liveness
// churn).
func TestISISRedistDeltaEmpty(t *testing.T) {
	called := false
	emitDelta(spf.RouteDelta{}, testProtocolID(), func(*redistevents.RouteChangeBatch) { called = true })
	if called {
		t.Fatal("empty delta must not emit a batch")
	}
}

// TestISISConnectedAdvertise verifies enabled/passive interface prefixes become
// internal-reachability PrefixInfo (TLV 135) with metric and up/down 0, with no
// adjacency required (AC-8). connectedPrefixInfos is the pure helper the engine
// calls at circuit-up.
func TestISISConnectedAdvertise(t *testing.T) {
	addrs := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.1/24"), // host address; masked network advertised
		netip.MustParsePrefix("172.16.5.9/30"),
	}
	infos := ConnectedPrefixInfos(addrs, 7)

	if len(infos) != 2 {
		t.Fatalf("got %d connected prefixes, want 2", len(infos))
	}
	byPfx := map[netip.Prefix]lsdb.PrefixInfo{}
	for _, in := range infos {
		byPfx[in.Prefix] = in
	}
	if _, ok := byPfx[netip.MustParsePrefix("10.0.0.0/24")]; !ok {
		t.Fatalf("connected prefix not masked to network 10.0.0.0/24: %+v", infos)
	}
	if in, ok := byPfx[netip.MustParsePrefix("172.16.5.8/30")]; !ok {
		t.Fatalf("connected prefix not masked to network 172.16.5.8/30: %+v", infos)
	} else {
		if in.Metric.Value() != 7 {
			t.Fatalf("connected prefix metric = %d, want circuit metric 7", in.Metric.Value())
		}
		if in.UpDown {
			t.Fatal("connected prefix up/down bit must be 0 (internal reachability)")
		}
	}
}

// testProtocolID returns the registered IS-IS ProtocolID for the source tests
// (the single identity, same as the events package and SPF install).
func testProtocolID() redistevents.ProtocolID {
	id, _ := redistevents.ProtocolIDOf("isis")
	return id
}

// fakeBus is a minimal ze.EventBus that records emitted (namespace, eventType,
// payload) tuples, copying the batch entries before the producer releases the
// pooled batch.
type fakeBus struct {
	got []captureBatch
	ns  []string
	et  []string
}

func (b *fakeBus) Emit(ns, et string, payload any) (int, error) {
	b.ns = append(b.ns, ns)
	b.et = append(b.et, et)
	if rb, ok := payload.(*redistevents.RouteChangeBatch); ok {
		cb := captureBatch{protocol: rb.Protocol, afi: rb.AFI, safi: rb.SAFI}
		cb.entries = append(cb.entries, rb.Entries...)
		b.got = append(b.got, cb)
	}
	return 1, nil
}

func (b *fakeBus) Subscribe(string, string, func(any)) func() { return func() {} }

// TestISISRedistSourceToBGP verifies the Source emits a RouteChangeBatch on the
// bus for an SPF add delta with both an L1 and an L2 route under the single isis
// ProtocolID (AC-1, AC-2): the orchestrator (which subscribes to that handle)
// resolves the source as ProtocolName(Protocol) == "isis" and dispatches to the
// BGP consumer. The dispatch-to-BGP half is covered by the redistribute_egress
// handleBatch tests (generic over producer) and the isis-redist-bgp.ci functional
// test; here we prove the producer emit half end to end on a bus.
func TestISISRedistSourceToBGP(t *testing.T) {
	RegisterISISSources() // the "isis" source name the orchestrator resolves
	bus := &fakeBus{}
	src := NewSource(bus)

	src.OnSPFChange(spf.RouteDelta{
		Added: []spf.RouteEntry{
			{Prefix: netip.MustParsePrefix("10.1.0.0/24"), Metric: 10, Level: spf.Level1,
				NextHops: []spf.NextHop{{Addr: netip.MustParseAddr("192.0.2.1")}}},
			{Prefix: netip.MustParsePrefix("10.2.0.0/24"), Metric: 20, Level: spf.Level2,
				NextHops: []spf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2")}}},
		},
	})

	if len(bus.got) != 1 {
		t.Fatalf("emitted %d batches, want 1", len(bus.got))
	}
	if bus.ns[0] != "isis" || bus.et[0] != redistevents.EventType {
		t.Fatalf("emitted on (%q,%q), want (isis,%q)", bus.ns[0], bus.et[0], redistevents.EventType)
	}
	if bus.got[0].protocol != testProtocolID() {
		t.Fatalf("batch protocol = %d, want isis %d", bus.got[0].protocol, testProtocolID())
	}
	if name := redistevents.ProtocolName(bus.got[0].protocol); name != "isis" {
		t.Fatalf("ProtocolName(batch.Protocol) = %q, want isis (the orchestrator's source resolution)", name)
	}
	if len(bus.got[0].entries) != 2 {
		t.Fatalf("batch carried %d entries, want 2 (L1 + L2, single source)", len(bus.got[0].entries))
	}
}

// TestISISRedistSourceWithdrawToBGP verifies a removed SPF route is emitted as an
// ActionRemove entry so the BGP consumer withdraws it (AC-7).
func TestISISRedistSourceWithdrawToBGP(t *testing.T) {
	bus := &fakeBus{}
	src := NewSource(bus)

	src.OnSPFChange(spf.RouteDelta{
		Removed: []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")},
	})

	if len(bus.got) != 1 || len(bus.got[0].entries) != 1 {
		t.Fatalf("withdraw did not emit one entry: %+v", bus.got)
	}
	if bus.got[0].entries[0].Action != redistevents.ActionRemove {
		t.Fatalf("entry action = %v, want remove", bus.got[0].entries[0].Action)
	}
}

// TestISISRedistSourceNilBus verifies a nil bus makes emit a no-op (engine without
// an event bus) without panicking.
func TestISISRedistSourceNilBus(t *testing.T) {
	src := NewSource(nil)
	src.OnSPFChange(spf.RouteDelta{
		Added: []spf.RouteEntry{{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Metric: 1, Level: spf.Level1}},
	})
}

// TestISISRedistRegistrationOrder verifies registration-order tolerance (AC-9):
// the source name is registered once via sync.Once regardless of how many times
// RegisterISISSources is called, and LookupSource finds it whether registration
// happened before or after this assertion runs.
func TestISISRedistRegistrationOrder(t *testing.T) {
	// Register-before: already registered by earlier tests / this call.
	RegisterISISSources()
	if _, ok := configredist.LookupSource("isis"); !ok {
		t.Fatal("isis source not found after registration (register-before order)")
	}
	// Register-after: a redundant call must not error or duplicate (sync.Once).
	RegisterISISSources()
	if _, ok := configredist.LookupSource("isis"); !ok {
		t.Fatal("isis source lost after a second RegisterISISSources (idempotency)")
	}
}

// TestISISRedistSelfImportRejected verifies IS-IS self-import is auto-rejected by
// the generic loop-prevention evaluator (AC-10): an isis-origin route imported
// into the isis consumer is rejected because origin == importing protocol. The
// single source name "isis" (== ConsumerName) is what makes this work.
func TestISISRedistSelfImportRejected(t *testing.T) {
	// The consumer name MUST equal the source name so origin == importingProtocol.
	if ConsumerName != "isis" {
		t.Fatalf("ConsumerName = %q, want isis (must equal the source name for self-import rejection)", ConsumerName)
	}
	// An `import isis` rule on the isis consumer must reject an isis-origin route.
	rule := configredist.ImportRule{Source: "isis"}
	route := configredist.RedistRoute{Origin: "isis", Family: family.IPv4Unicast, Source: "isis"}
	if rule.Accept(route, ConsumerName) {
		t.Fatal("isis self-import accepted; loop-prevention must reject origin == importing protocol")
	}
	// Sanity: a connected-origin route IS accepted by `import connected` on isis.
	connRule := configredist.ImportRule{Source: "connected"}
	connRoute := configredist.RedistRoute{Origin: "connected", Family: family.IPv4Unicast, Source: "connected"}
	if !connRule.Accept(connRoute, ConsumerName) {
		t.Fatal("connected import into isis rejected; a non-self source must be accepted")
	}
}

// TestISISRedistMetricBoundary verifies the redistributed prefix metric handling
// across the TLV 135 32-bit range (Boundary Tests): the fixed default is in range,
// and the SPF->redistevents metric narrowing saturates at the uint32 max rather
// than wrapping.
func TestISISRedistMetricBoundary(t *testing.T) {
	if DefaultRedistMetric == 0 || uint64(DefaultRedistMetric) > uint64(^uint32(0)) {
		t.Fatalf("DefaultRedistMetric %d out of the 32-bit TLV 135 range", DefaultRedistMetric)
	}
	cases := []struct {
		in   uint64
		want uint32
	}{
		{0, 0},
		{uint64(^uint32(0)), ^uint32(0)},     // last valid 32-bit value
		{uint64(^uint32(0)) + 1, ^uint32(0)}, // above 32-bit: saturates, never wraps
		{1 << 40, ^uint32(0)},                // far above: still saturates
	}
	for _, c := range cases {
		if got := metricToUint32(c.in); got != c.want {
			t.Fatalf("metricToUint32(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
