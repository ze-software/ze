package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestQuiescerRegistryRegistersAndLists verifies runtime registration and
// retrieval of subsystem drains.
//
// VALIDATES: AC-1 -- a registered Quiescer is discoverable by the barrier.
// PREVENTS: a subsystem registering a drain that `request quiesce` never sees.
func TestQuiescerRegistryRegistersAndLists(t *testing.T) {
	var reg QuiescerRegistry
	reg.Register("a", func(context.Context) error { return nil })
	reg.Register("b", func(context.Context) error { return nil })

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 quiescers, got %d", len(all))
	}
	names := map[string]bool{all[0].Name: true, all[1].Name: true}
	if !names["a"] || !names["b"] {
		t.Errorf("missing registrant: %v", names)
	}
}

// TestQuiesceDispatchDrainsRegisteredQuiescers verifies the barrier invokes
// every registrant and replies StatusDone only after all drain.
//
// VALIDATES: AC-1, AC-2 -- all registrants run; success reply after all drain.
// PREVENTS: the barrier replying before a subsystem has settled.
func TestQuiesceDispatchDrainsRegisteredQuiescers(t *testing.T) {
	var ranA, ranB bool
	quiescers := []Quiescer{
		{Name: "a", Quiesce: func(context.Context) error { ranA = true; return nil }},
		{Name: "b", Quiesce: func(context.Context) error { ranB = true; return nil }},
	}
	resp := quiesceAll(context.Background(), quiescers, time.Second)
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %v, want Done (err=%q)", resp.Status, resp.Error)
	}
	if !ranA || !ranB {
		t.Errorf("not all quiescers ran: a=%v b=%v", ranA, ranB)
	}
}

// TestQuiesceNoRegistrantsIsNoop verifies an empty registry drains instantly.
//
// VALIDATES: AC-3 -- bare engine quiesces immediately, never hangs.
// PREVENTS: the barrier blocking when nothing is registered.
func TestQuiesceNoRegistrantsIsNoop(t *testing.T) {
	resp := quiesceAll(context.Background(), nil, time.Second)
	if resp.Status != plugin.StatusDone {
		t.Fatalf("empty quiesce status = %v, want Done", resp.Status)
	}
}

// TestQuiesceTimeoutNamesStuckSubsystem verifies a blocking quiescer is bounded
// and named, never hanging the daemon.
//
// VALIDATES: AC-5 -- a stuck subsystem yields a bounded error naming it.
// PREVENTS: a wedged subsystem hanging `request quiesce` forever.
func TestQuiesceTimeoutNamesStuckSubsystem(t *testing.T) {
	quiescers := []Quiescer{
		{Name: "fast", Quiesce: func(context.Context) error { return nil }},
		{Name: "stuck", Quiesce: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
	}
	start := time.Now()
	resp := quiesceAll(context.Background(), quiescers, 100*time.Millisecond)
	elapsed := time.Since(start)

	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %v, want Error", resp.Status)
	}
	if !contains(resp.Error, "stuck") {
		t.Errorf("error %q does not name the stuck subsystem", resp.Error)
	}
	if elapsed > time.Second {
		t.Errorf("quiesce took %v, should be bounded near the 100ms timeout", elapsed)
	}
}

// TestQuiesceReportsQuiescerError verifies a failing drain surfaces as an error
// naming the subsystem.
//
// VALIDATES: AC-2 -- a drain error is reported, not swallowed.
// PREVENTS: a silent drain failure masquerading as success.
func TestQuiesceReportsQuiescerError(t *testing.T) {
	quiescers := []Quiescer{
		{Name: "boom", Quiesce: func(context.Context) error { return errors.New("drain failed") }},
	}
	resp := quiesceAll(context.Background(), quiescers, time.Second)
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %v, want Error", resp.Status)
	}
	if !contains(resp.Error, "boom") {
		t.Errorf("error %q does not name the failing subsystem", resp.Error)
	}
}

// flushCountReactor wraps the package mockReactor to count FlushForwardPool
// calls, proving the quiescer body is the reactor's forward-pool drain.
type flushCountReactor struct {
	*mockReactor
	flushed int
}

func (r *flushCountReactor) FlushForwardPool(context.Context) error {
	r.flushed++
	return nil
}

// TestBGPForwardPoolRegistersQuiescer verifies the reactor's forward-pool drain
// is auto-registered as the "bgp-forward-pool" Quiescer at server construction,
// so `request quiesce` drains BGP.
//
// VALIDATES: the reactor is registered and its FlushForwardPool is the drain.
// PREVENTS: `request quiesce` silently draining nothing for BGP (the dominant
// sleep bucket).
func TestBGPForwardPoolRegistersQuiescer(t *testing.T) {
	fr := &flushCountReactor{mockReactor: &mockReactor{}}
	s := &Server{}
	registerReactorQuiescer(s, fr)

	// Two quiescers: the forward pool AND the per-peer initial-sync drain.
	byName := map[string]Quiescer{}
	for _, q := range s.Quiescers() {
		byName[q.Name] = q
	}
	fp, okFP := byName["bgp-forward-pool"]
	if _, okPS := byName["bgp-peer-sync"]; !okFP || !okPS || len(byName) != 2 {
		t.Fatalf("expected 'bgp-forward-pool' and 'bgp-peer-sync' quiescers, got %v", byName)
	}
	if err := fp.Quiesce(context.Background()); err != nil {
		t.Fatalf("forward-pool quiesce returned error: %v", err)
	}
	if fr.flushed != 1 {
		t.Errorf("FlushForwardPool called %d times, want 1", fr.flushed)
	}

	// A nil reactor (bare/web-only server) registers nothing.
	s2 := &Server{}
	registerReactorQuiescer(s2, nil)
	if len(s2.Quiescers()) != 0 {
		t.Error("nil reactor must register no quiescer")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
