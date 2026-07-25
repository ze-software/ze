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
// re-emits the current announced (forward, table 0) route set as adds tagged
// with the echoed ReplayID; non-forward routes (blackhole) are not re-emitted;
// the incremental emit stays ReplayID=0 (AC-6, AC-8).
// PREVENTS: a late-joining peer missing static routes; a wire change to the
// incremental path; blackhole routes leaking into the replay.
func TestStaticReemitsReplayID(t *testing.T) {
	bus := &staticRecordingBus{}
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	mb := &mockStaticBackend{}
	rm := newRouteManager(mb)
	require.NoError(t, rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			Action: actionBlackhole, // not a redistribute source route
		},
	}))

	// The incremental add carried ReplayID 0 (no behavior change).
	for _, b := range bus.events() {
		require.Equal(t, uint64(0), b.ReplayID, "incremental emit must not set ReplayID")
	}
	bus.reset()

	rm.reemitAll(55)

	evts := bus.events()
	require.Len(t, evts, 1, "only the forward route re-emits (blackhole is not a redistribute route)")
	require.Equal(t, uint64(55), evts[0].ReplayID, "re-emit echoes the replayID")
	require.Len(t, evts[0].Entries, 1)
	require.Equal(t, redistevents.ActionAdd, evts[0].Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), evts[0].Entries[0].Prefix)

	// reemitAll(0) is a no-op.
	bus.reset()
	rm.reemitAll(0)
	require.Empty(t, bus.events(), "reemitAll(0) must be a no-op")
}
