package firewall

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// overlapBackend detects whether two Backend.Apply calls are ever in flight at
// the same time. inFlight is bumped at entry and dropped at exit WITHOUT holding
// a lock across the body, so a genuine concurrent Apply is observable. The tiny
// yield+sleep widens the overlap window so the assertion is not merely lucky.
type overlapBackend struct {
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	applies  atomic.Int32
}

func (o *overlapBackend) Apply(_ []Table) error {
	n := o.inFlight.Add(1)
	for {
		m := o.maxSeen.Load()
		if n <= m || o.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	runtime.Gosched()
	time.Sleep(time.Millisecond)
	o.applies.Add(1)
	o.inFlight.Add(-1)
	return nil
}

func (o *overlapBackend) ListTables() ([]Table, error)                { return nil, nil }
func (o *overlapBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (o *overlapBackend) Close() error                                { return nil }

// installBackend registers a factory returning the given backend and loads it,
// with cleanup that restores registry/table isolation.
func installBackend(t *testing.T, name string, b Backend) {
	t.Helper()
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})
	if err := RegisterBackend(name, func() (Backend, error) { return b, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend(name); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
}

// VALIDATES: D-1 / AC-4 -- the registry never lets two owners be inside
// Backend.Apply at the same time (reconcileMu serializes the whole reconcile).
// Without reconcileMu the overlap counter reaches 2 (RED); with it, it stays 1.
// PREVENTS: concurrent Backend.Apply corrupting the nft reconcile (Finding 1:
// shared command batch drained by a foreign Flush, raced applied-tables map).
func TestApplyAllSerialisesBackendApply(t *testing.T) {
	be := &overlapBackend{}
	installBackend(t, "overlap", be)

	// A handful of owners so there is always a non-empty desired set to apply.
	for i := range 4 {
		_ = RegisterTables(fmt.Sprintf("owner-%d", i),
			[]Table{{Name: fmt.Sprintf("ze_o%d", i), Family: FamilyInet}})
	}

	const goroutines = 8
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			if err := ApplyAll(); err != nil {
				t.Errorf("ApplyAll: %v", err)
			}
		})
	}
	wg.Wait()

	if got := be.maxSeen.Load(); got != 1 {
		t.Fatalf("max concurrent Backend.Apply = %d, want 1 (ApplyAll must serialize reconciles)", got)
	}
	if got := be.applies.Load(); got != goroutines {
		t.Fatalf("Backend.Apply calls = %d, want %d", got, goroutines)
	}
}

// recordingBackend keeps the desired set of every Apply, in completion order.
type recordingBackend struct {
	mu        sync.Mutex
	completed [][]string // table names of each completed Apply, in completion order
}

func (r *recordingBackend) Apply(desired []Table) error {
	names := make([]string, 0, len(desired))
	for _, tbl := range desired {
		names = append(names, tbl.Name)
	}
	sort.Strings(names)
	r.mu.Lock()
	r.completed = append(r.completed, names)
	r.mu.Unlock()
	return nil
}

func (r *recordingBackend) last() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.completed) == 0 {
		return nil
	}
	return r.completed[len(r.completed)-1]
}

func (r *recordingBackend) ListTables() ([]Table, error)                { return nil, nil }
func (r *recordingBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (r *recordingBackend) Close() error                                { return nil }

// VALIDATES: D-1 / AC-4 -- N goroutines each register a distinct owner then
// ApplyAll; the LAST Apply to complete observes every registration that
// completed before it (all N owners), because reconcileMu serializes
// snapshot+apply.
// PREVENTS: a stale early snapshot landing last and silently dropping owners
// (lost update across concurrent reconciles).
func TestApplyAllConcurrentOwnersConverge(t *testing.T) {
	be := &recordingBackend{}
	installBackend(t, "record", be)

	const owners = 12
	want := make([]string, 0, owners)
	for i := range owners {
		want = append(want, fmt.Sprintf("ze_c%d", i))
	}
	sort.Strings(want)

	var wg sync.WaitGroup
	for i := range owners {
		wg.Go(func() {
			_ = RegisterTables(fmt.Sprintf("owner-%d", i),
				[]Table{{Name: fmt.Sprintf("ze_c%d", i), Family: FamilyInet}})
			if err := ApplyAll(); err != nil {
				t.Errorf("ApplyAll: %v", err)
			}
		})
	}
	wg.Wait()

	got := be.last()
	if len(got) != len(want) {
		t.Fatalf("last Apply saw %d tables %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("last Apply tables = %v, want %v", got, want)
		}
	}
}

// gateBackend blocks the FIRST Apply on a channel so a test can open a window
// during which another owner registers, then verify the reconcile that lands
// LAST carries the newer desired set. lastCompleted is recorded at Apply exit,
// i.e. in kernel-completion order, which is what "the kernel ends on" means.
type gateBackend struct {
	gated   atomic.Bool // only the FIRST Apply blocks; later ones must run freely
	started chan struct{}
	release chan struct{}

	mu            sync.Mutex
	lastCompleted []string
}

func (g *gateBackend) Apply(desired []Table) error {
	names := make([]string, 0, len(desired))
	for _, tbl := range desired {
		names = append(names, tbl.Name)
	}
	sort.Strings(names)

	// CompareAndSwap (not sync.Once) so a SECOND concurrent Apply does NOT wait
	// on the first: if reconcileMu were missing, the second Apply must be free
	// to race ahead and complete first. Once.Do would block it and hide the bug.
	if g.gated.CompareAndSwap(false, true) {
		close(g.started)
		<-g.release
	}

	g.mu.Lock()
	g.lastCompleted = names
	g.mu.Unlock()
	return nil
}

func (g *gateBackend) last() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastCompleted
}

func (g *gateBackend) ListTables() ([]Table, error)                { return nil, nil }
func (g *gateBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (g *gateBackend) Close() error                                { return nil }

// TestApplyAllStaleSnapshotNotApplied proves the D-1 rationale: locking only
// around b.Apply is insufficient. Owner A applies (blocked mid-flight), owner B
// registers during that window, then a second ApplyAll runs. With reconcileMu
// the second reconcile cannot start until the first finishes, so it snapshots
// {A,B} and the kernel ends on {A,B}. Without it, the first (stale {A}) apply
// completes LAST and the kernel ends on the A-only set.
func TestApplyAllStaleSnapshotNotApplied(t *testing.T) {
	be := &gateBackend{started: make(chan struct{}), release: make(chan struct{})}
	installBackend(t, "gate", be)

	_ = RegisterTables("A", []Table{{Name: "ze_a", Family: FamilyInet}})

	var wg sync.WaitGroup
	wg.Go(func() {
		if err := ApplyAll(); err != nil { // snapshots {ze_a}, blocks in Apply
			t.Errorf("first ApplyAll: %v", err)
		}
	})

	<-be.started // first Apply is now in flight holding reconcileMu

	// B registers while the first reconcile is blocked. RegisterTables takes
	// only tableRegistry.mu, so it is not blocked by reconcileMu -- intended.
	_ = RegisterTables("B", []Table{{Name: "ze_b", Family: FamilyInet}})

	secondStarted := make(chan struct{})
	wg.Go(func() {
		close(secondStarted)
		if err := ApplyAll(); err != nil { // must block on reconcileMu until first returns
			t.Errorf("second ApplyAll: %v", err)
		}
	})

	<-secondStarted
	// Give the second goroutine time to reach (and block on) reconcileMu before
	// we release the first. Without serialization it would instead race ahead.
	time.Sleep(20 * time.Millisecond)

	close(be.release) // let the first reconcile finish
	wg.Wait()

	got := be.last()
	want := []string{"ze_a", "ze_b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("kernel ended on %v, want %v (stale A-only snapshot must not land last)", got, want)
	}
}
