package redistributeegress

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/pkg/ze"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedDispatch captures an UpdateRoute call.
type recordedDispatch struct {
	selector string
	command  string
}

type fakeDispatcher struct {
	mu    sync.Mutex
	calls []recordedDispatch
}

func (f *fakeDispatcher) UpdateRoute(_ context.Context, selector, command string) (uint32, uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedDispatch{selector: selector, command: command})
	return 1, 1, nil
}

func (f *fakeDispatcher) snapshot() []recordedDispatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedDispatch, len(f.calls))
	copy(out, f.calls)
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
	eventBusPtr.Store(nil)
	t.Cleanup(func() {
		redistevents.ResetForTest()
		configredist.SetGlobal(nil)
		eventBusPtr.Store(nil)
	})
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

// VALIDATES: AC-2 -- accepted add for ipv4/unicast produces the canonical
// announce text with `nhop self`.
// PREVENTS: Reactor parse errors caused by syntax drift in the announce text.
func TestHandleBatchAcceptedAddDispatches(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv4, "10.0.0.1/32", ""))

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "*", calls[0].selector)
	assert.Equal(t, "update text origin incomplete nhop self nlri ipv4/unicast add 10.0.0.1/32", calls[0].command)
}

// VALIDATES: AC-3 -- accepted add for ipv6/unicast renders /128 with the
// ipv6 family token.
// PREVENTS: IPv6 path mis-stringifying the family or prefix.
func TestHandleBatchAcceptedAddIPv6(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv6Unicast}},
	}))

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv6, "2001:db8::1/128", ""))

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin incomplete nhop self nlri ipv6/unicast add 2001:db8::1/128", calls[0].command)
}

// VALIDATES: AC-11 -- non-zero NextHop produces explicit `nhop <addr>` (NOT
// `nhop self`).
// PREVENTS: Reactor accidentally substituting LocalAddress when the producer
// supplied a real next-hop.
func TestHandleBatchExplicitNextHop(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv4, "10.0.0.1/32", "192.0.2.1"))

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin incomplete nhop 192.0.2.1 nlri ipv4/unicast add 10.0.0.1/32", calls[0].command)
}

// VALIDATES: AC-4 / AC-5 -- batch family or protocol not in any import rule
// triggers zero UpdateRoute calls.
// PREVENTS: Routes leaking past the evaluator.
func TestHandleBatchRejectedAddNoop(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv6Unicast}},
	}))

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, disp.snapshot())
}

// VALIDATES: AC-8 -- accepted remove dispatches the canonical withdraw text.
// PREVENTS: Withdraw building wrong nlri spec.
func TestHandleBatchRemoveDispatches(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), disp, bgpID, removeBatch(id, afiIPv4, "10.0.0.1/32"))

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text nlri ipv4/unicast del 10.0.0.1/32", calls[0].command)
}

// VALIDATES: AC-1 -- evaluator nil means no dispatch and no panic.
// PREVENTS: Plugin crashing when redistribute is unconfigured.
func TestHandleBatchNoEvaluatorNoop(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")

	handleBatch(context.Background(), disp, bgpID, addBatch(id, afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, disp.snapshot())
}

// VALIDATES: AC-9 / AC-10 -- atomic swap of the global evaluator changes
// subsequent accept decisions.
// PREVENTS: Plugin holding a stale evaluator pointer after reload.
func TestHandleBatchReloadApplies(t *testing.T) {
	resetState(t)

	id := redistevents.RegisterProtocol("fakeredist")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator(nil)) // empty rules: rejects all

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	batch := addBatch(id, afiIPv4, "10.0.0.1/32", "")

	handleBatch(context.Background(), disp, bgpID, batch)
	assert.Empty(t, disp.snapshot(), "first call should be rejected by empty rules")

	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	handleBatch(context.Background(), disp, bgpID, batch)
	assert.Len(t, disp.snapshot(), 1, "second call should be accepted after reload")
}

// VALIDATES: AC-6 -- batches whose protocol matches BGP are dropped at the
// handler entry, regardless of the evaluator.
// PREVENTS: BGP-sourced batches being re-dispatched, creating a loop.
func TestHandleBatchBGPSourceSkipped(t *testing.T) {
	resetState(t)

	bgpID := redistevents.RegisterProtocol("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	disp := &fakeDispatcher{}
	handleBatch(context.Background(), disp, bgpID, addBatch(bgpID, afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, disp.snapshot())
}

// VALIDATES: Defense-in-depth -- batch with an unregistered ProtocolID is
// dropped (handler logs a warn and returns).
// PREVENTS: Memory corruption / drive-by injection via a forged ProtocolID.
func TestHandleBatchUnknownProtocol(t *testing.T) {
	resetState(t)

	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(context.Background(), disp, bgpID, addBatch(redistevents.ProtocolID(99), afiIPv4, "10.0.0.1/32", ""))

	assert.Empty(t, disp.snapshot())
}

// VALIDATES: subscribe() enumerates non-BGP producers and registers per-
// protocol typed handlers (skips BGP ID).
// PREVENTS: Consumer wiring up its own protocol's events back into itself.
func TestSubscribeSkipsOwnProtocol(t *testing.T) {
	resetState(t)

	bgpID := redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProducer(bgpID)
	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)

	bus := newTestBus()
	setEventBus(bus)
	disp := &fakeDispatcher{}
	unsubs := subscribe(context.Background(), disp, bus, bgpID)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	require.Len(t, unsubs, 1, "only fakeredist should be subscribed (BGP skipped)")
}

// VALIDATES: subscribe() builds a typed handle per producer; emit on that
// handle is delivered to handleBatch.
// PREVENTS: Wrong namespace / event type being subscribed.
func TestSubscribeNonBGPProducers(t *testing.T) {
	resetState(t)

	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "fakeredist", Families: []family.Family{family.IPv4Unicast}},
	}))

	fakeID := redistevents.RegisterProtocol("fakeredist")
	redistevents.RegisterProducer(fakeID)

	bus := newTestBus()
	setEventBus(bus)
	disp := &fakeDispatcher{}
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	unsubs := subscribe(context.Background(), disp, bus, bgpID)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	_, err := bus.Emit("fakeredist", redistevents.EventType, addBatch(fakeID, afiIPv4, "10.0.0.1/32", ""))
	require.NoError(t, err)

	calls := disp.snapshot()
	require.Len(t, calls, 1, "subscriber should receive emitted batch")
	assert.Contains(t, calls[0].command, "10.0.0.1/32")
}

// VALIDATES: command-text builder shape across families.
// PREVENTS: Per-family text drift.
func TestCommandTextAllFamilies(t *testing.T) {
	cases := []struct {
		name  string
		fam   string
		entry redistevents.RouteChangeEntry
		want  string
	}{
		{
			name: "ipv4/unicast /32 self",
			fam:  "ipv4/unicast",
			entry: redistevents.RouteChangeEntry{
				Action: redistevents.ActionAdd,
				Prefix: netip.MustParsePrefix("10.0.0.1/32"),
			},
			want: "update text origin incomplete nhop self nlri ipv4/unicast add 10.0.0.1/32",
		},
		{
			name: "ipv4/unicast /24 explicit nhop",
			fam:  "ipv4/unicast",
			entry: redistevents.RouteChangeEntry{
				Action:  redistevents.ActionAdd,
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.0.2.1"),
			},
			want: "update text origin incomplete nhop 192.0.2.1 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "ipv6/unicast /128 self",
			fam:  "ipv6/unicast",
			entry: redistevents.RouteChangeEntry{
				Action: redistevents.ActionAdd,
				Prefix: netip.MustParsePrefix("2001:db8::1/128"),
			},
			want: "update text origin incomplete nhop self nlri ipv6/unicast add 2001:db8::1/128",
		},
		{
			name: "ipv6/unicast /64 explicit nhop",
			fam:  "ipv6/unicast",
			entry: redistevents.RouteChangeEntry{
				Action:  redistevents.ActionAdd,
				Prefix:  netip.MustParsePrefix("2001:db8::/64"),
				NextHop: netip.MustParseAddr("2001:db8::1"),
			},
			want: "update text origin incomplete nhop 2001:db8::1 nlri ipv6/unicast add 2001:db8::/64",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAnnounce(tt.fam, &tt.entry)
			assert.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: withdraw text shape across families.
// PREVENTS: Withdraw command picking up announce-only fragments.
func TestWithdrawTextAllFamilies(t *testing.T) {
	cases := []struct {
		fam    string
		prefix string
		want   string
	}{
		{"ipv4/unicast", "10.0.0.1/32", "update text nlri ipv4/unicast del 10.0.0.1/32"},
		{"ipv4/unicast", "10.0.0.0/24", "update text nlri ipv4/unicast del 10.0.0.0/24"},
		{"ipv6/unicast", "2001:db8::1/128", "update text nlri ipv6/unicast del 2001:db8::1/128"},
		{"ipv6/unicast", "2001:db8::/64", "update text nlri ipv6/unicast del 2001:db8::/64"},
	}

	for _, tt := range cases {
		t.Run(tt.fam+" "+tt.prefix, func(t *testing.T) {
			entry := redistevents.RouteChangeEntry{
				Action: redistevents.ActionRemove,
				Prefix: netip.MustParsePrefix(tt.prefix),
			}
			got := formatWithdraw(tt.fam, &entry)
			assert.Equal(t, tt.want, got)
		})
	}
}
