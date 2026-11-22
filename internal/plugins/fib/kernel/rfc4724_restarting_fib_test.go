// Design: docs/architecture/core-design.md -- RFC 4724 Section 4.1 Restarting Speaker forwarding preservation
// Related: fibkernel.go -- startupSweep, processEvent, sweepStale, run, waitAndSweepStale
// Related: register.go -- parseFIBConfig (FlushOnStop default false)
// RFC 4724 Section 4.1 -- see rfc/short/rfc4724.md

package fibkernel

import (
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type rfc4724FIBTestBus struct {
	mu       sync.Mutex
	nextID   uint64
	handlers map[string]map[uint64]func(any)
}

func newRFC4724FIBTestBus() *rfc4724FIBTestBus {
	return &rfc4724FIBTestBus{handlers: make(map[string]map[uint64]func(any))}
}

func (b *rfc4724FIBTestBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	subs := make([]func(any), 0, len(b.handlers[namespace+"/"+eventType]))
	for _, fn := range b.handlers[namespace+"/"+eventType] {
		subs = append(subs, fn)
	}
	b.mu.Unlock()
	for _, fn := range subs {
		fn(payload)
	}
	return len(subs), nil
}

func (b *rfc4724FIBTestBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.handlers[key] == nil {
		b.handlers[key] = make(map[uint64]func(any))
	}
	b.handlers[key][id] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.handlers[key], id)
		b.mu.Unlock()
	}
}

// TestRFC4724RestartingSpeakerForwardingState proves the Restarting Speaker
// forwarding-state preservation across shutdown/restart, stale marking,
// non-differentiation during forwarding, and BGP-coordinated post-selection
// forwarding update and stale-route removal in fib-kernel.
//
// VALIDATES:
//   - RFC4724-4.1-1: parseFIBConfig defaults FlushOnStop to false, fib.run
//     with flushOnStop=false preserves installed routes across shutdown
//     (zero backend deletes on ctx cancel), and startupSweep retains surviving
//     routes. Explicit flush-on-stop=true flushes installed routes on shutdown.
//   - RFC4724-4.1-2: startupSweep marks every surviving Ze route stale in
//     staleTables and retained, while newly installed post-restart routes are
//     not marked stale and refreshing a retained route clears its stale mark.
//   - RFC4724-4.1-3: startupSweep issues zero backend modifications/deletes,
//     leaving stale routes indistinguishable from fresh routes during forwarding.
//   - RFC4724-4.1-6: waitAndSweepStale holds off sweeping while BGP emits
//     SelectionDeferred (even past fallback SweepDelay), updates forwarding
//     state via processEvent, and immediately sweeps unrefreshed stale routes
//     when BGP emits SelectionComplete while preserving refreshed routes.
//
// PREVENTS: deleting forwarding state on graceful restart, sweeping preserved
// routes before BGP finishes deferred route selection, degrading stale routes
// during forwarding, or leaving orphaned stale routes after reconvergence.
//
// RFC requirement: RFC4724-4.1-1 positive -- default FlushOnStop is false, fib.run(flushOnStop=false) preserves installed routes across shutdown with zero backend deletes, and startupSweep retains surviving routes without deleting them.
// RFC requirement: RFC4724-4.1-1 negative -- explicit flush-on-stop true causes fib.run(flushOnStop=true) to flush installed routes on shutdown.
// RFC requirement: RFC4724-4.1-2 positive -- startupSweep marks all retained routes from the previous run as stale in staleTables and retained.
// RFC requirement: RFC4724-4.1-2 negative -- newly installed post-restart routes are not marked stale in staleTables, and refreshing a retained prefix clears its stale mark.
// RFC requirement: RFC4724-4.1-3 positive -- startupSweep performs zero backend deletes, replacements, or additions, so retained stale routes remain intact and indistinguishable from fresh routes during forwarding.
// RFC requirement: RFC4724-4.1-6 positive -- waitAndSweepStale extends past SweepDelay while BGP SelectionDeferred is active, updates forwarding state via processEvent, and deletes unrefreshed stale routes immediately upon BGP SelectionComplete.
// RFC requirement: RFC4724-4.1-6 negative -- routes refreshed by route selection before SelectionComplete are not deleted when waitAndSweepStale sweeps stale entries.
func TestRFC4724RestartingSpeakerForwardingState(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	ribevents.ClearSelectionDeferred()
	t.Cleanup(ribevents.ClearSelectionDeferred)

	bus := newRFC4724FIBTestBus()
	prevBus := getEventBus()
	setEventBus(bus)
	t.Cleanup(func() {
		if prevBus != nil {
			setEventBus(prevBus)
		}
	})

	// RFC4724-4.1-1 positive & negative: verify config default AND actual shutdown behavior in fib.run.
	defaultCfg, err := parseFIBConfig(nil)
	if err != nil {
		t.Fatalf("parseFIBConfig(nil): %v", err)
	}
	if defaultCfg.FlushOnStop {
		t.Fatal("default FlushOnStop = true, want false so forwarding state is retained across restart")
	}
	flushRaw, _ := json.Marshal(map[string]any{
		"fib": map[string]any{
			"kernel": map[string]any{
				"flush-on-stop": "true",
			},
		},
	})
	flushCfg, err := parseFIBConfig([]sdk.ConfigSection{{Root: configRoot, Data: string(flushRaw)}})
	if err != nil {
		t.Fatalf("parseFIBConfig(flush-on-stop true): %v", err)
	}
	if !flushCfg.FlushOnStop {
		t.Fatal("explicit flush-on-stop true produced FlushOnStop = false")
	}

	// Positive shutdown retention: fib.run(flushOnStop=false) leaves installed routes intact when stopped.
	keepBackend := newMockBackend()
	keepFIB := newFIBKernel(keepBackend)
	keepFIB.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
			NextHop:  netip.MustParseAddr("192.0.2.1"),
			Protocol: "bgp",
		},
	}))
	keepCtx, keepCancel := context.WithCancel(context.Background())
	keepReady := make(chan error, 1)
	keepDone := make(chan struct{})
	go func() {
		defer close(keepDone)
		keepFIB.run(keepCtx, defaultCfg.FlushOnStop, keepReady)
	}()
	if runErr := <-keepReady; runErr != nil {
		t.Fatalf("keepFIB.run ready: %v", runErr)
	}
	keepCancel()
	<-keepDone
	if len(keepBackend.deleted) != 0 {
		t.Fatalf("fib.run(flushOnStop=false) deleted routes on shutdown: %v", keepBackend.deleted)
	}

	// Negative shutdown flush: fib.run(flushOnStop=true) deletes installed routes on shutdown.
	flushBackend := newMockBackend()
	flushFIB := newFIBKernel(flushBackend)
	flushFIB.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
			NextHop:  netip.MustParseAddr("192.0.2.1"),
			Protocol: "bgp",
		},
	}))
	flushCtx, flushCancel := context.WithCancel(context.Background())
	flushReady := make(chan error, 1)
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		flushFIB.run(flushCtx, flushCfg.FlushOnStop, flushReady)
	}()
	if runErr := <-flushReady; runErr != nil {
		t.Fatalf("flushFIB.run ready: %v", runErr)
	}
	flushCancel()
	<-flushDone
	if !slices.Contains(flushBackend.deleted, "198.51.100.0/24") {
		t.Fatalf("fib.run(flushOnStop=true) did not flush 198.51.100.0/24 on shutdown; deleted=%v", flushBackend.deleted)
	}

	// Simulate surviving kernel routes from before restart.
	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{
		{prefix: "198.51.100.0/24", nextHop: "192.0.2.1", table: 0},
		{prefix: "203.0.113.0/24", nextHop: "192.0.2.2", table: 0},
	}
	fib := newFIBKernel(backend)

	// RFC4724-4.1-1, 4.1-2, 4.1-3: startupSweep retains and marks stale without altering kernel forwarding entries.
	stale := fib.startupSweep()
	if len(stale) != 2 {
		t.Fatalf("startupSweep stale count = %d, want 2", len(stale))
	}
	if len(backend.deleted) != 0 {
		t.Fatalf("startupSweep deleted %v, want 0 deletes (must retain forwarding state)", backend.deleted)
	}
	if len(backend.replaced) != 0 {
		t.Fatalf("startupSweep modified %v, want 0 modifications (must not differentiate stale routes)", backend.replaced)
	}
	if len(backend.added) != 0 {
		t.Fatalf("startupSweep added %v, want 0 adds during stale marking", backend.added)
	}
	fib.mu.RLock()
	for _, pfx := range []string{"198.51.100.0/24", "203.0.113.0/24"} {
		if !fib.retained[pfx] {
			t.Errorf("prefix %s not marked retained after startupSweep", pfx)
		}
		if len(fib.staleTables[pfx]) == 0 {
			t.Errorf("prefix %s not marked stale in staleTables after startupSweep", pfx)
		}
	}
	fib.mu.RUnlock()

	// RFC4724-4.1-6: replicate real Stage 2 -> Stage 5 startup order where
	// bgp-rib arms selection deferral (NoteSelectionDeferred + emit) during
	// OnConfigure BEFORE fib-kernel starts waitAndSweepStale during OnStarted.
	// waitAndSweepStale must still observe the active deferral, extend past the
	// short 25ms fallback SweepDelay, and wait for BGP SelectionComplete.
	ribevents.NoteSelectionDeferred(5 * time.Second)
	if _, err := ribevents.SelectionDeferred.Emit(bus, &ribevents.SelectionDeferredEvent{MaxDelay: 5 * time.Second}); err != nil {
		t.Fatalf("emit SelectionDeferred: %v", err)
	}

	sweepCtx, sweepCancel := context.WithCancel(context.Background())
	defer sweepCancel()
	sweepReady := make(chan struct{})
	sweepDone := make(chan struct{})
	go func() {
		defer close(sweepDone)
		fib.waitAndSweepStale(sweepCtx, stale, 25*time.Millisecond, sweepReady)
	}()
	<-sweepReady

	// Wait past the 25ms fallback SweepDelay: stale routes must NOT be deleted while
	// BGP selection deferral is still active.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-sweepDone:
		t.Fatal("waitAndSweepStale swept prematurely before BGP SelectionComplete")
	default:
	}

	// RFC4724-4.1-2 negative & RFC4724-4.1-6: route selection refreshes 198.51.100.0/24
	// with updated next hop (clearing its stale mark) and installs a brand-new prefix
	// 198.18.0.0/24 (never marked stale); 203.0.113.0/24 stays unrefreshed.
	fib.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
			NextHop:  netip.MustParseAddr("192.0.2.10"),
			Protocol: "bgp",
		},
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("198.18.0.0/24"),
			NextHop:  netip.MustParseAddr("192.0.2.11"),
			Protocol: "bgp",
		},
	}))
	if got := backend.added["198.51.100.0/24"]; got != "192.0.2.10" {
		if rep := backend.replaced["198.51.100.0/24"]; rep != "192.0.2.10" {
			t.Fatalf("updated route 198.51.100.0/24 not programmed with new next hop 192.0.2.10 (added=%q replaced=%q)", got, rep)
		}
	}
	fib.mu.RLock()
	if len(fib.staleTables["198.51.100.0/24"]) != 0 {
		t.Errorf("refreshed prefix 198.51.100.0/24 still marked stale: %v", fib.staleTables["198.51.100.0/24"])
	}
	if len(fib.staleTables["198.18.0.0/24"]) != 0 {
		t.Errorf("newly installed prefix 198.18.0.0/24 unexpectedly marked stale: %v", fib.staleTables["198.18.0.0/24"])
	}
	fib.mu.RUnlock()

	// Emit BGP SelectionComplete: waitAndSweepStale must immediately sweep unrefreshed
	// 203.0.113.0/24 while preserving refreshed 198.51.100.0/24 and fresh 198.18.0.0/24.
	ribevents.ClearSelectionDeferred()
	if _, err := ribevents.SelectionComplete.Emit(bus, &ribevents.SelectionCompleteEvent{}); err != nil {
		t.Fatalf("emit SelectionComplete: %v", err)
	}
	select {
	case <-sweepDone:
	case <-time.After(2 * time.Second):
		t.Fatal("waitAndSweepStale did not finish promptly after BGP SelectionComplete")
	}

	if !slices.Contains(backend.deleted, "203.0.113.0/24") {
		t.Fatalf("unrefreshed stale route 203.0.113.0/24 was not removed after SelectionComplete; deleted=%v", backend.deleted)
	}
	if slices.Contains(backend.deleted, "198.51.100.0/24") {
		t.Fatalf("refreshed route 198.51.100.0/24 was erroneously deleted by sweepStale; deleted=%v", backend.deleted)
	}
	if slices.Contains(backend.deleted, "198.18.0.0/24") {
		t.Fatalf("fresh post-restart route 198.18.0.0/24 was erroneously deleted by sweepStale; deleted=%v", backend.deleted)
	}
	fib.mu.RLock()
	if len(fib.staleTables) != 0 {
		t.Errorf("staleTables not cleared after sweepStale: %v", fib.staleTables)
	}
	fib.mu.RUnlock()
}
