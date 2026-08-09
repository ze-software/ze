// Design: docs/architecture/isis/isis-9-spf-rib.md / plan/spec-isis-11-redistribution.md -- SPF OnChange seam.
//
// VALIDATES: the redistribution read seam (spec-isis-11) -- SetOnChange installs a
//            callback the Computer fires after a Run that changed the route set,
//            handing it the applied delta; an unchanged re-run does NOT fire it.
// PREVENTS:  a regression where the producer never learns of SPF route changes
//            (so no IS-IS route reaches BGP) or fires on an empty delta (churn).

package spf

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// gatedResolver delegates to stubResolver but can pause the next (and all
// subsequent) ResolveNextHop calls until block is closed, signaling on entered the
// first time it blocks. It parks a Run inside its lock-free compute phase
// (BuildRoutes -> resolveHops) so a test can interleave Stop(). block == nil is
// pass-through (the baseline Run).
type gatedResolver struct {
	stubResolver
	block   chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *gatedResolver) ResolveNextHop(l Level, n types.SystemID) (NextHop, bool) {
	if g.block != nil {
		g.once.Do(func() { close(g.entered) })
		<-g.block
	}
	return g.stubResolver.ResolveNextHop(l, n)
}

// TestComputerOnChangeFires verifies the OnChange callback receives the delta on a
// route-set change and is not called on an unchanged re-run.
func TestComputerOnChangeFires(t *testing.T) {
	src := newStubSource()
	a, b := srcID(1), srcID(2)
	src.bidir(a, b, 10)
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == b {
			src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
				tlv135(netip.MustParsePrefix("10.30.0.0/24"), 5, false))
		}
	}

	c := NewComputer(Config{
		Source:   src,
		Resolver: stubResolver{},
		Root:     sysID(1),
		Levels:   []Level{Level1},
	})

	var deltas []RouteDelta
	c.SetOnChange(func(d RouteDelta) { deltas = append(deltas, d) })

	// First run: one route added -> callback fires once with that delta.
	c.Run()
	if len(deltas) != 1 {
		t.Fatalf("OnChange fired %d times after first run, want 1", len(deltas))
	}
	if len(deltas[0].Added) != 1 {
		t.Fatalf("delta.Added = %d, want 1", len(deltas[0].Added))
	}
	if deltas[0].Added[0].Prefix != netip.MustParsePrefix("10.30.0.0/24") {
		t.Fatalf("added prefix = %v, want 10.30.0.0/24", deltas[0].Added[0].Prefix)
	}

	// Second run with no topology change: delta is empty -> callback does NOT fire.
	c.Run()
	if len(deltas) != 1 {
		t.Fatalf("OnChange fired %d times after unchanged re-run, want still 1 (no churn)", len(deltas))
	}
}

// TestComputerOnChangeNil verifies a nil OnChange is a safe no-op (default).
func TestComputerOnChangeNil(t *testing.T) {
	src := newStubSource()
	src.bidir(srcID(1), srcID(2), 10)
	c := NewComputer(Config{Source: src, Resolver: stubResolver{}, Root: sysID(1), Levels: []Level{Level1}})
	// No SetOnChange call: Run must not panic.
	c.Run()
}

// TestComputerOnLeakFires verifies the RFC 2966 inter-level leak seam: a Run on an
// L1L2 topology computes the per-level leak set, fires SetOnLeak with it, and
// exposes it via LeakResult(). An L1-native prefix leaks UP into L2 (bit clear); an
// L2-native prefix leaks DOWN into L1 (bit set). This proves the Computer.Run ->
// onLeak wiring; the leak math itself is covered by leak_test.go.
func TestComputerOnLeakFires(t *testing.T) {
	root, l1n, l2n := sysID(1), srcID(2), srcID(3)
	l1pfx := netip.MustParsePrefix("10.1.0.0/24")
	l2pfx := netip.MustParsePrefix("10.2.0.0/24")

	src := newStubSource()
	// L1: root <-> node 2 (metric 10); node 2 originates the L1-native prefix.
	src.add(Level1, LSPRecord{Source: srcID(1), LSP: packet.LSP{TLVs: []packet.TLV{tlv22(isEdge{l1n, 10})}}})
	src.add(Level1, LSPRecord{Source: l1n, LSP: packet.LSP{TLVs: []packet.TLV{
		tlv22(isEdge{srcID(1), 10}), tlv135(l1pfx, 5, false),
	}}})
	// L2: root <-> node 3 (metric 20); node 3 originates the L2-native prefix.
	src.add(Level2, LSPRecord{Source: srcID(1), LSP: packet.LSP{TLVs: []packet.TLV{tlv22(isEdge{l2n, 20})}}})
	src.add(Level2, LSPRecord{Source: l2n, LSP: packet.LSP{TLVs: []packet.TLV{
		tlv22(isEdge{srcID(1), 20}), tlv135(l2pfx, 7, false),
	}}})

	c := NewComputer(Config{Source: src, Resolver: stubResolver{}, Root: root, Levels: []Level{Level1, Level2}})
	var got LeakResult
	var fired int
	c.SetOnLeak(func(r LeakResult) { got = r; fired++ })

	c.Run()
	if fired != 1 {
		t.Fatalf("onLeak fired %d times, want 1", fired)
	}
	if !hasLeak(got.IntoL2, l1pfx, false) {
		t.Errorf("IntoL2 missing %s up=false; got %+v", l1pfx, got.IntoL2)
	}
	if !hasLeak(got.IntoL1, l2pfx, true) {
		t.Errorf("IntoL1 missing %s up=true; got %+v", l2pfx, got.IntoL1)
	}
	// LeakResult() reflects the last Run.
	if snap := c.LeakResult(); !hasLeak(snap.IntoL1, l2pfx, true) {
		t.Errorf("LeakResult() did not reflect the run: %+v", snap)
	}
}

// computerWithPrefix builds a Computer over a 2-node topology where B originates
// pfx, so a Run installs exactly one route. Returns the Computer for stop-race
// tests.
func computerWithPrefix(t *testing.T, pfx netip.Prefix) *Computer {
	t.Helper()
	src := newStubSource()
	a, b := srcID(1), srcID(2)
	src.bidir(a, b, 10)
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == b {
			src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
				tlv135(pfx, 5, false))
		}
	}
	return NewComputer(Config{Source: src, Resolver: stubResolver{}, Root: sysID(1), Levels: []Level{Level1}})
}

// TestComputerRunAfterStopIsNoop is the regression for the untracked-debounce-Run
// bug: a Run firing after Stop()/cancel must NOT re-install IS-IS routes the engine
// just removed (stale FIB on shutdown / NET removal). Stop sets a stopped guard, so
// a direct Run (modeling a debounce callback that fired post-Stop) is a no-op.
func TestComputerRunAfterStopIsNoop(t *testing.T) {
	pfx := netip.MustParsePrefix("10.40.0.0/24")
	c := computerWithPrefix(t, pfx)

	// Prove the topology installs a route under a normal Run.
	c.Run()
	if got := len(c.Routes()); got != 1 {
		t.Fatalf("baseline Run installed %d routes, want 1", got)
	}

	// Engine shutdown: Stop removes the routes and marks the Computer stopped.
	c.Stop()
	if got := len(c.Routes()); got != 0 {
		t.Fatalf("after Stop, Routes() = %d, want 0", got)
	}

	// A Run racing shutdown (the debounce callback firing just after Stop) must be
	// a NO-OP: it must not re-populate the installed set.
	delta := c.Run()
	if !delta.Empty() {
		t.Errorf("post-Stop Run returned non-empty delta %+v, want empty (no-op)", delta)
	}
	if got := len(c.Routes()); got != 0 {
		t.Fatalf("post-Stop Run re-installed %d routes, want 0 (stale FIB on shutdown)", got)
	}
}

// TestComputerTriggerAfterStopDoesNotArm verifies a Trigger after Stop does not arm
// a new debounce timer (so no callback can ever fire to re-install routes).
func TestComputerTriggerAfterStopDoesNotArm(t *testing.T) {
	c := computerWithPrefix(t, netip.MustParsePrefix("10.41.0.0/24"))

	var mu sync.Mutex
	var armed int
	c.afterFunc = func(_ time.Duration, _ func()) *time.Timer {
		mu.Lock()
		armed++
		mu.Unlock()
		return time.NewTimer(time.Hour)
	}

	c.Stop()
	c.Trigger()
	mu.Lock()
	got := armed
	mu.Unlock()
	if got != 0 {
		t.Errorf("Trigger after Stop armed %d timers, want 0 (stopped Computer must not schedule SPF)", got)
	}
}

// TestComputerStopDrainsInFlightRun is the regression for the goroutine-tracking
// half of the bug: a debounce timer that ALREADY FIRED (callback on its own
// goroutine) is drained by Stop, and because Stop set the stopped guard first the
// callback's Run is a no-op -- so once Stop returns no SPF run is still touching the
// Loc-RIB and no routes were re-installed.
func TestComputerStopDrainsInFlightRun(t *testing.T) {
	c := computerWithPrefix(t, netip.MustParsePrefix("10.42.0.0/24"))
	c.Run() // install the baseline route
	if len(c.Routes()) != 1 {
		t.Fatalf("baseline Run did not install the route")
	}

	// Capture the debounce callback instead of timing it. The returned timer models
	// one that has ALREADY FIRED: its Stop() reports false, so the Computer relies on
	// runWG (not the timer.Stop() balance) to drain the callback.
	var captured func()
	c.afterFunc = func(_ time.Duration, fn func()) *time.Timer {
		captured = fn
		tm := time.NewTimer(time.Hour)
		tm.Stop() // a subsequent Stop() returns false: "already fired" model
		return tm
	}
	c.Trigger() // arms: runWG.Add(1), captured = the callback
	if captured == nil {
		t.Fatal("Trigger did not arm a callback")
	}

	// Run the callback on its OWN goroutine, gated so it executes only AFTER Stop has
	// set the stopped guard -- the exact shutdown race. Stop blocks on runWG.Wait()
	// until the callback's deferred Done runs, so both must run concurrently.
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-release
		captured() // defer runWG.Done(); sees stopped -> no-op Run
	}()
	go func() {
		defer wg.Done()
		close(release)
		c.Stop() // sets stopped, then runWG.Wait() drains the callback
	}()
	wg.Wait()

	if got := len(c.Routes()); got != 0 {
		t.Fatalf("after Stop drained the in-flight Run, Routes() = %d, want 0 (no re-install)", got)
	}
}

// TestComputerStopDuringComputeDoesNotReinstall is the regression for the apply-window
// half of the shutdown race (found in re-review): a Run that passed its entry guard
// and is in its LOCK-FREE compute phase when Stop() runs (RemoveAll) must NOT
// re-install the routes when it later reaches the apply lock. The entry guard alone
// does not cover this window because compute holds no lock; Run must re-check stopped
// under the apply lock. A gated resolver parks the Run mid-compute so Stop can
// interleave deterministically.
func TestComputerStopDuringComputeDoesNotReinstall(t *testing.T) {
	pfx := netip.MustParsePrefix("10.44.0.0/24")
	gr := &gatedResolver{}
	src := newStubSource()
	a, b := srcID(1), srcID(2)
	src.bidir(a, b, 10)
	for i := range src.byLevel[Level1] {
		if src.byLevel[Level1][i].Source == b {
			src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
				tlv135(pfx, 5, false))
		}
	}
	c := NewComputer(Config{Source: src, Resolver: gr, Root: sysID(1), Levels: []Level{Level1}})

	// Baseline with the resolver in pass-through mode: install the route.
	c.Run()
	if got := len(c.Routes()); got != 1 {
		t.Fatalf("baseline Run installed %d routes, want 1", got)
	}

	// Arm the resolver to block the next Run inside its compute phase.
	gr.entered = make(chan struct{})
	gr.block = make(chan struct{})

	done := make(chan RouteDelta, 1)
	go func() { done <- c.Run() }()

	<-gr.entered // the racing Run is now past the entry guard, parked in BuildRoutes, holding no lock
	c.Stop()     // sets stopped + RemoveAll while the Run is parked in compute
	if got := len(c.Routes()); got != 0 {
		t.Fatalf("after Stop, Routes() = %d, want 0", got)
	}
	close(gr.block) // let the racing Run proceed to its apply section

	delta := <-done
	if !delta.Empty() {
		t.Errorf("Run that raced Stop in its compute window returned non-empty delta %+v, want empty", delta)
	}
	if got := len(c.Routes()); got != 0 {
		t.Fatalf("Run re-installed %d routes after Stop's RemoveAll (stale FIB on shutdown)", got)
	}
}
