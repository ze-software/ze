package firewall

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// useCommitObservers replaces the registered observers for one test and restores
// the previous list after it, so a test never sees an observer another package's
// init() registered and never leaves its own behind.
func useCommitObservers(t *testing.T) {
	t.Helper()
	commitObservers.mu.Lock()
	saved := commitObservers.observers
	commitObservers.observers = nil
	commitObservers.mu.Unlock()
	t.Cleanup(func() {
		commitObservers.mu.Lock()
		commitObservers.observers = saved
		commitObservers.mu.Unlock()
	})
}

// observerBackend loads a fake backend whose Apply appends "apply" to events and
// answers applyErr, and registers one table so ApplyAll reaches it.
func observerBackend(t *testing.T, events *[]string, applyErr error) {
	t.Helper()
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})
	if err := RegisterBackend("observed", func() (Backend, error) {
		return &countingBackend{onApply: func([]Table) { *events = append(*events, "apply") }, applyErr: applyErr}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("observed"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	if err := RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}}); err != nil {
		t.Fatalf("RegisterTables: %v", err)
	}
}

// recordingObserver registers an observer named name that appends "<name>-before"
// when called and "<name>-after:<err>" when its after runs.
func recordingObserver(events *[]string, name string) {
	RegisterCommitObserver(func() func(error) {
		*events = append(*events, name+"-before")
		return func(err error) {
			outcome := "ok"
			if err != nil {
				outcome = err.Error()
			}
			*events = append(*events, name+"-after:"+outcome)
		}
	})
}

// TestApplyAllRunsCommitObserversAroundApply drives ApplyAll with a fake backend
// and two recording observers, once with an Apply that succeeds and once with one
// that fails.
//
// VALIDATES: every observer runs before Backend.Apply, in registration order; each
// after runs once Apply has returned, in the same order, with Apply's error (nil
// on success, the backend's error on failure); ApplyAll still answers that error.
// PREVENTS: the IKE fast path hold being taken after the commit landed, or never
// released when the commit fails, which would leave bare ESP off the fast path
// for the life of the process.
func TestApplyAllRunsCommitObserversAroundApply(t *testing.T) {
	refused := errors.New("kernel refused the batch")
	for _, tc := range []struct {
		name     string
		applyErr error
		want     []string
	}{
		{
			name: "apply succeeds",
			want: []string{"first-before", "second-before", "apply", "first-after:ok", "second-after:ok"},
		},
		{
			name:     "apply fails",
			applyErr: refused,
			want: []string{"first-before", "second-before", "apply",
				"first-after:" + refused.Error(), "second-after:" + refused.Error()},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCommitObservers(t)
			var events []string
			observerBackend(t, &events, tc.applyErr)
			recordingObserver(&events, "first")
			recordingObserver(&events, "second")

			err := ApplyAll()
			if !errors.Is(err, tc.applyErr) {
				t.Fatalf("ApplyAll = %v, want %v", err, tc.applyErr)
			}
			if !slices.Equal(events, tc.want) {
				t.Fatalf("events = %v, want %v", events, tc.want)
			}
		})
	}
}

// TestCommitObserverNilAfterIsAllowed registers an observer that answers a nil
// after beside one that answers a real one.
//
// VALIDATES: a nil after is skipped, not called, and the other observer's after
// still runs after Apply.
// PREVENTS: a nil-function panic inside reconcileMu, which would leave every
// firewall owner's reconcile failing.
func TestCommitObserverNilAfterIsAllowed(t *testing.T) {
	useCommitObservers(t)
	var events []string
	observerBackend(t, &events, nil)
	RegisterCommitObserver(func() func(error) {
		events = append(events, "nil-before")
		return nil
	})
	recordingObserver(&events, "real")

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	want := []string{"nil-before", "real-before", "apply", "real-after:ok"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

// TestFlushAllTablesRunsCommitObservers flushes the registry with an observer
// registered.
//
// VALIDATES: the shutdown flush, which reconciles an empty desired set, reaches
// the observers like any other commit.
// PREVENTS: the flush that removes every ze table, the operator's ESP filters
// included, landing while the fast path still rewrites bare ESP.
func TestFlushAllTablesRunsCommitObservers(t *testing.T) {
	useCommitObservers(t)
	var events []string
	observerBackend(t, &events, nil)
	recordingObserver(&events, "flush")

	if err := FlushAllTables(); err != nil {
		t.Fatalf("FlushAllTables: %v", err)
	}
	want := []string{"flush-before", "apply", "flush-after:ok"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

// TestCommitObserversSkippedWithoutApply reconciles with no backend and nothing
// to apply, the one early return that reaches no Backend.Apply on every host.
//
// VALIDATES: an observer runs only for a commit that reaches the backend, so its
// before is never called without a matching after.
// PREVENTS: a hold taken on a reconcile that returns early and never releases it.
func TestCommitObserversSkippedWithoutApply(t *testing.T) {
	useCommitObservers(t)
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})
	var events []string
	recordingObserver(&events, "idle")

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll with no backend and no tables = %v, want nil", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %v, want none", events)
	}
}

// TestCommitObserverReleasedWhenApplyPanics drives a backend whose Apply panics.
//
// VALIDATES: the after still runs, with an error naming the panic, and the panic
// reaches the caller unchanged.
// PREVENTS: a backend panic leaving the IKE fast path held for good.
func TestCommitObserverReleasedWhenApplyPanics(t *testing.T) {
	useCommitObservers(t)
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})
	if err := RegisterBackend("panicking", func() (Backend, error) {
		return &countingBackend{onApply: func([]Table) { panic("test: backend apply unwound") }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("panicking"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	if err := RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}}); err != nil {
		t.Fatalf("RegisterTables: %v", err)
	}
	var events []string
	recordingObserver(&events, "held")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("the backend's panic did not reach ApplyAll's caller")
			}
		}()
		_ = ApplyAll()
	}()
	if len(events) != 2 || events[0] != "held-before" || !strings.Contains(events[1], "panicked") {
		t.Fatalf("events = %v, want held-before then an after naming the panic", events)
	}
}
