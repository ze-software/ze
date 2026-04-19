package fakeredist

import (
	"net/netip"
	"strings"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/ze"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureBus records every Emit call as raw payload + (ns, et).
type captureBus struct {
	mu    sync.Mutex
	emits []emitRecord
	subs  map[string][]func(any)
}

type emitRecord struct {
	ns      string
	et      string
	payload *redistevents.RouteChangeBatch // copy of the batch (value-deep)
	entries []redistevents.RouteChangeEntry
}

var _ ze.EventBus = (*captureBus)(nil)

func newCaptureBus() *captureBus {
	return &captureBus{subs: map[string][]func(any){}}
}

func (b *captureBus) Emit(ns, et string, payload any) (int, error) {
	b.mu.Lock()
	src, _ := payload.(*redistevents.RouteChangeBatch)
	rec := emitRecord{ns: ns, et: et}
	if src != nil {
		// Snapshot the batch and entries because the producer recycles the
		// pointer immediately after Emit returns.
		dup := *src
		rec.payload = &dup
		rec.entries = append([]redistevents.RouteChangeEntry(nil), src.Entries...)
	}
	b.emits = append(b.emits, rec)
	hs := append([]func(any){}, b.subs[ns+"/"+et]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *captureBus) Subscribe(ns, et string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := ns + "/" + et
	b.subs[key] = append(b.subs[key], handler)
	return func() {}
}

func (b *captureBus) records() []emitRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]emitRecord, len(b.emits))
	copy(out, b.emits)
	return out
}

// resetBus swaps in a fresh capture bus and clears it after the test.
func resetBus(t *testing.T) *captureBus {
	t.Helper()
	bus := newCaptureBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })
	return bus
}

// VALIDATES: init() registers ProtocolID and producer flag in redistevents.
// PREVENTS: bgp-redistribute failing to enumerate fakeredist as a producer.
func TestInitRegistersProtocol(t *testing.T) {
	got, ok := redistevents.ProtocolIDOf(ProtocolName)
	require.True(t, ok, "fakeredist must be registered as a redistevents protocol")
	assert.Equal(t, ProtocolID, got)

	prods := redistevents.Producers()
	assert.Contains(t, prods, ProtocolID, "fakeredist must be in the producer set")
}

// VALIDATES: `fakeredist emit add ipv4/unicast 10.0.0.1/32` builds a single-
// entry add batch and emits it.
// PREVENTS: Args parsing dropping fields silently.
func TestCommandEmitAdd(t *testing.T) {
	bus := resetBus(t)

	status, data, err := dispatchCommand("", "fakeredist emit", []string{"add", "ipv4/unicast", "10.0.0.1/32"}, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.Contains(t, data, `"delivered":`)

	recs := bus.records()
	require.Len(t, recs, 1)
	rec := recs[0]
	assert.Equal(t, ProtocolName, rec.ns)
	assert.Equal(t, redistevents.EventType, rec.et)
	require.NotNil(t, rec.payload)
	assert.Equal(t, ProtocolID, rec.payload.Protocol)
	assert.Equal(t, uint16(1), rec.payload.AFI)
	assert.Equal(t, uint8(1), rec.payload.SAFI)
	require.Len(t, rec.entries, 1)
	assert.Equal(t, redistevents.ActionAdd, rec.entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.1/32"), rec.entries[0].Prefix)
	assert.False(t, rec.entries[0].NextHop.IsValid(), "NextHop should be zero (consumer emits nhop self)")
}

// VALIDATES: `fakeredist emit remove ipv4/unicast 10.0.0.1/32` emits a remove batch.
// PREVENTS: Action token mis-parsing.
func TestCommandEmitRemove(t *testing.T) {
	bus := resetBus(t)

	status, _, err := dispatchCommand("", "fakeredist emit", []string{"remove", "ipv4/unicast", "10.0.0.1/32"}, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)

	recs := bus.records()
	require.Len(t, recs, 1)
	require.Len(t, recs[0].entries, 1)
	assert.Equal(t, redistevents.ActionRemove, recs[0].entries[0].Action)
}

// VALIDATES: `fakeredist emit add ipv4/unicast 10.0.0.1/32 192.0.2.1` carries
// the explicit next-hop through to the entry.
// PREVENTS: Optional 4th arg being silently dropped.
func TestCommandEmitWithNextHop(t *testing.T) {
	bus := resetBus(t)

	_, _, err := dispatchCommand("", "fakeredist emit", []string{"add", "ipv4/unicast", "10.0.0.1/32", "192.0.2.1"}, "")
	require.NoError(t, err)

	recs := bus.records()
	require.Len(t, recs, 1)
	require.Len(t, recs[0].entries, 1)
	got := recs[0].entries[0].NextHop
	assert.True(t, got.IsValid())
	assert.Equal(t, "192.0.2.1", got.String())
}

// VALIDATES: `fakeredist emit-burst N add ipv4/unicast 10.0.0.0/32` emits N
// distinct prefixes (host bits incremented).
// PREVENTS: Burst test asserting on N prefixes when only 1 was emitted.
func TestCommandEmitBurst(t *testing.T) {
	bus := resetBus(t)

	const n = 10
	status, data, err := dispatchCommand("", "fakeredist emit-burst",
		[]string{"10", "add", "ipv4/unicast", "10.0.0.0/32"}, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.Contains(t, data, `"emitted":10`)

	recs := bus.records()
	require.Len(t, recs, n)
	seen := map[string]bool{}
	for _, r := range recs {
		require.Len(t, r.entries, 1)
		seen[r.entries[0].Prefix.String()] = true
	}
	assert.Len(t, seen, n, "each emit should carry a distinct prefix")
}

// VALIDATES: invalid family / prefix / count / action surfaces an error
// status without emitting.
// PREVENTS: Bad CLI args silently emitting garbage.
func TestCommandBadArgs(t *testing.T) {
	bus := resetBus(t)

	cases := []struct {
		name string
		cmd  string
		args []string
	}{
		{"missing args", "fakeredist emit", []string{"add"}},
		{"bad action", "fakeredist emit", []string{"keep", "ipv4/unicast", "10.0.0.1/32"}},
		{"bad family", "fakeredist emit", []string{"add", "garbage", "10.0.0.1/32"}},
		{"bad prefix", "fakeredist emit", []string{"add", "ipv4/unicast", "not-a-prefix"}},
		{"bad nexthop", "fakeredist emit", []string{"add", "ipv4/unicast", "10.0.0.1/32", "::garbage"}},
		{"bad burst count", "fakeredist emit-burst", []string{"-1", "add", "ipv4/unicast", "10.0.0.0/32"}},
		{"non-numeric burst", "fakeredist emit-burst", []string{"abc", "add", "ipv4/unicast", "10.0.0.0/32"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			status, _, err := dispatchCommand("", tt.cmd, tt.args, "")
			require.Error(t, err)
			assert.Equal(t, rpc.StatusError, status)
		})
	}

	assert.Empty(t, bus.records(), "no batch should leak from a bad command")
}

// VALIDATES: unknown command returns error status without crashing.
// PREVENTS: Misrouted dispatcher producing a panic.
func TestDispatchUnknownCommand(t *testing.T) {
	resetBus(t)
	status, _, err := dispatchCommand("", "fakeredist nope", nil, "")
	require.Error(t, err)
	assert.Equal(t, rpc.StatusError, status)
}

// VALIDATES: help command returns the usage stub.
// PREVENTS: Help disappearing as the surface evolves.
func TestDispatchHelp(t *testing.T) {
	resetBus(t)
	status, data, err := dispatchCommand("", "fakeredist help", nil, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.True(t, strings.Contains(data, "emit add"))
	assert.True(t, strings.Contains(data, "emit-burst"))
}
