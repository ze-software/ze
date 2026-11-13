// Design: docs/architecture/core-design.md -- FIB shutdown admission.
// Related: fibkernel.go -- processEvent, run and flushRoutes.

package fibkernel

import (
	"context"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/replay"
)

// heldFIBReplayBackend owns one paused replacement. Its constructor supplies
// every channel; its zero value is invalid. The caller MUST release the pause
// and join the callback before reading the replacement records.
type heldFIBReplayBackend struct {
	*mockBackend
	replacements []string
	pauseAt      int
	entered      chan struct{}
	resume       chan struct{}
	release      sync.Once
}

func newHeldFIBReplayBackend() *heldFIBReplayBackend {
	return &heldFIBReplayBackend{
		mockBackend: newMockBackend(),
		pauseAt:     4,
		entered:     make(chan struct{}),
		resume:      make(chan struct{}),
	}
}

func (backend *heldFIBReplayBackend) replaceRoute(prefix, nextHop string) error {
	backend.replacements = append(backend.replacements, prefix)
	if len(backend.replacements) == backend.pauseAt {
		close(backend.entered)
		<-backend.resume
	}
	return backend.mockBackend.replaceRoute(prefix, nextHop)
}

func (backend *heldFIBReplayBackend) releaseReplacement() {
	backend.release.Do(func() { close(backend.resume) })
}

func fibStopReplayBatch() *incomingBatch {
	return makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.81.0/24"), NextHop: netip.MustParseAddr("192.0.2.1"), Protocol: "static"},
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.82.0/24"), NextHop: netip.MustParseAddr("192.0.2.1"), Protocol: "static"},
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.83.0/24"), NextHop: netip.MustParseAddr("192.0.2.1"), Protocol: "static"},
	})
}

// TestFIBKernelReplayReassertsInstalledRoutes keeps the explicit repair replay:
// already installed entries must reach the backend again while admission is open.
func TestFIBKernelReplayReassertsInstalledRoutes(t *testing.T) {
	backend := newHeldFIBReplayBackend()
	fib := newFIBKernel(backend)
	batch := fibStopReplayBatch()
	fib.processEvent(batch)
	if len(backend.added) != 3 {
		t.Fatalf("initial installation reached %d routes, want three useful controls", len(backend.added))
	}
	batch.ReplayID = replay.Broadcast
	fib.processEvent(batch)
	if len(backend.replacements) != 3 {
		t.Fatalf("repair replay performed %d backend replacements, want all three installed routes", len(backend.replacements))
	}
	for _, change := range batch.Changes {
		if got := backend.replaced[change.Prefix.String()]; got != "192.0.2.1" {
			t.Errorf("repair replay for %s reached gateway %q, want the installed gateway", change.Prefix, got)
		}
	}
}

// TestFIBKernelStopEndsReplayAdmission holds a real batch inside its first
// backend replacement. Shutdown must close admission without waiting for that
// operation, permit it to finish, then reach the complete owned-route flush.
func TestFIBKernelStopEndsReplayAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	backend := newHeldFIBReplayBackend()
	fib := newFIBKernel(backend)
	batch := fibStopReplayBatch()
	fib.processEvent(batch)
	if len(fib.installed) != 3 {
		t.Fatalf("initial installation retained %d routes, want three useful controls", len(fib.installed))
	}
	batch.ReplayID = replay.Broadcast
	fib.processEvent(batch)
	if len(backend.replacements) != 3 {
		t.Fatalf("ordinary replay performed %d replacements, want all three before shutdown", len(backend.replacements))
	}

	replayDone := make(chan struct{})
	stopDone := make(chan struct{})
	stopStarted := false
	go runHeldFIBReplay(fib, batch, replayDone)
	defer func() {
		backend.releaseReplacement()
		<-replayDone
		if stopStarted {
			<-stopDone
		}
		fib.flushRoutes()
	}()
	select {
	case <-backend.entered:
	case <-ctx.Done():
		t.Fatal("repair replay did not enter its controlled backend replacement", ctx.Err())
	}
	stopStarted = true
	go requestHeldFIBStop(fib, stopDone)
	select {
	case <-stopDone:
	case <-ctx.Done():
		t.Fatal("closing FIB admission waited for the held backend operation", ctx.Err())
	}
	backend.releaseReplacement()
	select {
	case <-replayDone:
	case <-ctx.Done():
		t.Fatal("released repair replay did not finish", ctx.Err())
	}
	if len(backend.replacements) != 4 {
		t.Errorf("shutdown admitted %d replay replacements after three healthy ones, want only the one already in flight", len(backend.replacements)-3)
	}
	if len(fib.installed) != 3 {
		t.Errorf("shutdown lost ownership before flush: %d installed routes, want all three", len(fib.installed))
	}
	fib.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.84.0/24"), NextHop: netip.MustParseAddr("192.0.2.2"), Protocol: "static"},
	}))
	if len(backend.added) != 3 {
		t.Errorf("a later event reached the stopped writer: %d backend adds, want only the three original routes", len(backend.added))
	}
	fib.flushRoutes()
	if len(fib.installed) != 0 {
		t.Errorf("owned flush retained %d installed routes", len(fib.installed))
	}
	if len(fib.forwarding) != 0 {
		t.Errorf("owned flush retained %d forwarding records", len(fib.forwarding))
	}
	want := []string{"198.18.81.0/24", "198.18.82.0/24", "198.18.83.0/24"}
	got := slices.Clone(backend.deleted)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("owned flush deleted %v, want exactly the three preinstalled routes %v", got, want)
	}
}

// runHeldFIBReplay owns one callback. Its caller MUST release the backend pause
// and join done before returning, including every failure path.
func runHeldFIBReplay(fib *fibKernel, batch *incomingBatch, done chan<- struct{}) {
	defer close(done)
	fib.processEvent(batch)
}

// requestHeldFIBStop exercises the synchronous stop transition. Its caller MUST
// join done after releasing the backend, even when the transition blocks.
func requestHeldFIBStop(fib *fibKernel, done chan<- struct{}) {
	defer close(done)
	fib.beginStop()
}
