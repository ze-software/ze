package fakeredist

import (
	"encoding/json"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawString returns the JSON body a command handler produced. Command handlers
// return `any`; the emit handlers return json.RawMessage, which the SDK marshals
// on the wire. assert.Contains on the raw []byte would compare bytes element-wise
// (never a substring match), so assertions must use the string form.
func rawString(t *testing.T, data any) string {
	t.Helper()
	raw, ok := data.(json.RawMessage)
	require.True(t, ok, "command data: got %T, want json.RawMessage", data)
	return string(raw)
}

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

// resetBus swaps in a fresh capture bus and clears it after the test. It also
// resets the current-set store so route tracking does not leak across tests.
func resetBus(t *testing.T) *captureBus {
	t.Helper()
	bus := newCaptureBus()
	setEventBus(bus)
	resetStore()
	t.Cleanup(func() {
		eventBusPtr.Store(nil)
		resetStore()
	})
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

	status, data, err := dispatchCommand("", "request fakeredist emit", []string{"add", "ipv4/unicast", "10.0.0.1/32"}, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.Contains(t, rawString(t, data), `"delivered":`)

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

	status, _, err := dispatchCommand("", "request fakeredist emit", []string{"remove", "ipv4/unicast", "10.0.0.1/32"}, "")
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

	_, _, err := dispatchCommand("", "request fakeredist emit", []string{"add", "ipv4/unicast", "10.0.0.1/32", "192.0.2.1"}, "")
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
	status, data, err := dispatchCommand("", "request fakeredist emit-burst",
		[]string{"10", "add", "ipv4/unicast", "10.0.0.0/32"}, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.Contains(t, rawString(t, data), `"emitted":10`)

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
		{"missing args", "request fakeredist emit", []string{"add"}},
		{"bad action", "request fakeredist emit", []string{"keep", "ipv4/unicast", "10.0.0.1/32"}},
		{"bad family", "request fakeredist emit", []string{"add", "garbage", "10.0.0.1/32"}},
		{"bad prefix", "request fakeredist emit", []string{"add", "ipv4/unicast", "not-a-prefix"}},
		{"bad nexthop", "request fakeredist emit", []string{"add", "ipv4/unicast", "10.0.0.1/32", "::garbage"}},
		{"bad burst count", "request fakeredist emit-burst", []string{"-1", "add", "ipv4/unicast", "10.0.0.0/32"}},
		{"non-numeric burst", "request fakeredist emit-burst", []string{"abc", "add", "ipv4/unicast", "10.0.0.0/32"}},
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
	status, data, err := dispatchCommand("", "show fakeredist help", nil, "")
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	helpMap, ok := data.(map[string]any)
	require.True(t, ok)
	helpStr, ok := helpMap["help"].(string)
	require.True(t, ok)
	assert.Contains(t, helpStr, "emit add")
	assert.Contains(t, helpStr, "emit-burst")
}

// VALIDATES: TestProducerEchoesReplayIDOnReplayRequest (fakeredist) -- the .ci
// test producer tracks its current set (add tracked, remove untracked) and
// reemitAll re-emits the live set as adds tagged with the echoed ReplayID; a
// route removed before the replay is absent (AC-4); the incremental emit stays
// ReplayID=0 (AC-8).
// PREVENTS: the late-join .ci producer not answering a ReplayRequest, or a
// withdrawn route reappearing on replay.
func TestFakeredistReemitsReplayID(t *testing.T) {
	bus := resetBus(t)

	require.NotEqual(t, redistevents.ProtocolUnspecified, ProtocolID, "init must register fakeredist")

	// Two adds then a withdraw of the first, all incremental (ReplayID 0).
	for _, args := range [][]string{
		{"add", "ipv4/unicast", "10.0.0.1/32"},
		{"add", "ipv4/unicast", "10.0.0.2/32"},
		{"remove", "ipv4/unicast", "10.0.0.1/32"},
	} {
		status, _, err := dispatchCommand("", "request fakeredist emit", args, "")
		require.NoError(t, err)
		require.Equal(t, rpc.StatusDone, status)
	}
	for _, rec := range bus.records() {
		require.NotNil(t, rec.payload)
		assert.Equal(t, uint64(0), rec.payload.ReplayID, "incremental emit must not set ReplayID")
	}

	// Drain, then replay for a fresh peer.
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	reemitAll(99)

	recs := bus.records()
	require.Len(t, recs, 1, "only the live route (10.0.0.2/32) replays; the withdrawn one is absent (AC-4)")
	assert.Equal(t, uint64(99), recs[0].payload.ReplayID, "re-emit echoes the replayID")
	require.Len(t, recs[0].entries, 1)
	assert.Equal(t, redistevents.ActionAdd, recs[0].entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.2/32"), recs[0].entries[0].Prefix)

	// reemitAll(0) is a no-op.
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()
	reemitAll(0)
	assert.Empty(t, bus.records(), "reemitAll(0) must be a no-op")
}
