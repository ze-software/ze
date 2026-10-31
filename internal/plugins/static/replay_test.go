package static

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/ze"

	"github.com/stretchr/testify/require"
)

// staticRecordingBus captures emitted RouteChangeBatch payloads (deep-copying
// the pooled batch so a post-Emit ReleaseBatch does not clobber the record).
type staticRecordingBus struct {
	mu    sync.Mutex
	emits []*redistevents.RouteChangeBatch
}

var _ ze.EventBus = (*staticRecordingBus)(nil)

func (b *staticRecordingBus) Emit(_, _ string, payload any) (int, error) {
	if batch, ok := payload.(*redistevents.RouteChangeBatch); ok && batch != nil {
		cp := *batch
		cp.Entries = make([]redistevents.RouteChangeEntry, len(batch.Entries))
		copy(cp.Entries, batch.Entries)
		b.mu.Lock()
		b.emits = append(b.emits, &cp)
		b.mu.Unlock()
	}
	return 0, nil
}

func (b *staticRecordingBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (b *staticRecordingBus) events() []*redistevents.RouteChangeBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*redistevents.RouteChangeBatch, len(b.emits))
	copy(out, b.emits)
	return out
}

func (b *staticRecordingBus) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emits = nil
}

// VALIDATES: TestProducerEchoesReplayIDOnReplayRequest (static) -- reemitAll
// re-emits the current announced (table 0) route set as adds tagged with the
// echoed ReplayID, a blackhole route included (it is a redistribute source, as
// in FRR and BIRD); the incremental emit stays ReplayID=0 (AC-6, AC-8).
// PREVENTS: a late-joining peer missing static routes; a wire change to the
// incremental path.
func TestStaticReemitsReplayID(t *testing.T) {
	bus := &staticRecordingBus{}
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)
	require.NoError(t, rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			Action: actionBlackhole, // a redistribute source like the forward route
		},
	}))

	// The incremental add carried ReplayID 0 (no behavior change).
	for _, b := range bus.events() {
		require.Equal(t, uint64(0), b.ReplayID, "incremental emit must not set ReplayID")
	}
	bus.reset()

	rm.reemitAll(55)

	evts := bus.events()
	require.Len(t, evts, 2, "both announced routes re-emit, the blackhole one included")
	got := map[netip.Prefix]bool{}
	for _, evt := range evts {
		require.Equal(t, uint64(55), evt.ReplayID, "re-emit echoes the replayID")
		require.Len(t, evt.Entries, 1)
		require.Equal(t, redistevents.ActionAdd, evt.Entries[0].Action)
		got[evt.Entries[0].Prefix] = true
	}
	require.Equal(t, map[netip.Prefix]bool{
		netip.MustParsePrefix("10.0.0.0/8"):   true,
		netip.MustParsePrefix("192.0.2.0/24"): true,
	}, got)

	// reemitAll(0) is a no-op.
	bus.reset()
	rm.reemitAll(0)
	require.Empty(t, bus.events(), "reemitAll(0) must be a no-op")
}
