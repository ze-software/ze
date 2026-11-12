package server

import (
	"slices"
	"sync"
	"testing"
	"time"
)

// TestProcessCleanupHooksRegisterWhileRunning drives the process cleanup hook
// list the way the daemon does: plugins exiting on their own goroutines run
// it while a registration may still arrive.
//
// VALIDATES: every hook registered before a run is called by it with the
// process name; a hook that registers another hook does not deadlock, because
// hooks run on a copy taken under the lock and never with the lock held;
// registrations and runs on several goroutines do not race (under -race).
// PREVENTS: the unguarded package slice the first proxy-enricher registration
// appended to while another plugin's exit iterated it.
func TestProcessCleanupHooksRegisterWhileRunning(t *testing.T) {
	processCleanupMu.Lock()
	saved := slices.Clone(processCleanupHooks)
	processCleanupHooks = nil
	processCleanupMu.Unlock()
	t.Cleanup(func() {
		processCleanupMu.Lock()
		processCleanupHooks = saved
		processCleanupMu.Unlock()
	})

	var calledMu sync.Mutex
	called := map[string][]string{}
	recorder := func(id string) ProcessCleanupFunc {
		return func(name string) {
			calledMu.Lock()
			defer calledMu.Unlock()
			called[id] = append(called[id], name)
		}
	}
	calls := func(id string) []string {
		calledMu.Lock()
		defer calledMu.Unlock()
		return slices.Clone(called[id])
	}

	RegisterProcessCleanup(recorder("first"))
	RegisterProcessCleanup(recorder("second"))
	runProcessCleanupHooks("plugin-a")
	for _, id := range []string{"first", "second"} {
		if got := calls(id); !slices.Equal(got, []string{"plugin-a"}) {
			t.Fatalf("hook %s was called with %v, want [plugin-a]", id, got)
		}
	}

	// A hook that registers runs with the lock released, or this run never
	// returns. The bound is a deadline for the failure, not a wait for success.
	RegisterProcessCleanup(func(string) { RegisterProcessCleanup(recorder("late")) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		runProcessCleanupHooks("plugin-b")
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a hook that registers another hook deadlocked the cleanup run: hooks run under the lock")
	}
	runProcessCleanupHooks("plugin-c")
	if got := calls("late"); !slices.Contains(got, "plugin-c") {
		t.Fatalf("a hook registered by a hook was not called by the next run: %v", got)
	}

	// Concurrent registrations and runs: the race detector judges this part.
	processCleanupMu.Lock()
	processCleanupHooks = nil
	processCleanupMu.Unlock()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 100 {
				runProcessCleanupHooks("racer")
			}
		})
	}
	wg.Go(func() {
		for range 100 {
			RegisterProcessCleanup(func(string) {})
		}
	})
	wg.Wait()
	processCleanupMu.Lock()
	registered := len(processCleanupHooks)
	processCleanupMu.Unlock()
	if registered != 100 {
		t.Fatalf("%d hooks registered under concurrent runs, want 100", registered)
	}
}
