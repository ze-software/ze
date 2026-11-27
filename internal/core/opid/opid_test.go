package opid

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// VALIDATES: AC-1 -- New() mints strictly monotonic "op-<N>" correlation IDs.
// PREVENTS: duplicate or non-monotonic operation IDs breaking cross-surface correlation.
func TestNewMonotonic(t *testing.T) {
	var prev uint64
	for i := range 20 {
		id := New()
		raw, ok := strings.CutPrefix(id, "op-")
		if !ok {
			t.Fatalf("New()[%d] = %q, want op- prefix", i, id)
		}
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			t.Fatalf("New()[%d] = %q: parse uint: %v", i, id, err)
		}
		if n <= prev {
			t.Fatalf("New()[%d] = %d, want > %d", i, n, prev)
		}
		prev = n
	}
}

// VALIDATES: AC-1 -- With and From attach, inherit, override, and handle nil/empty inputs safely.
// PREVENTS: panics on nil context or empty IDs clobbering an existing correlation ID.
func TestWithAndFrom(t *testing.T) {
	//nolint:staticcheck // intentionally testing nil context contract
	if got := From(nil); got != "" {
		t.Errorf("From(nil) = %q, want empty", got)
	}
	if got := From(context.Background()); got != "" {
		t.Errorf("From(Background) = %q, want empty", got)
	}

	//nolint:staticcheck // intentionally testing nil context contract
	ctx := With(nil, "op-nil-parent")
	if got := From(ctx); got != "op-nil-parent" {
		t.Errorf("From(With(nil, ...)) = %q, want op-nil-parent", got)
	}

	// Empty ID returns ctx unchanged without erasing existing ID.
	same := With(ctx, "")
	if got := From(same); got != "op-nil-parent" {
		t.Errorf("With(ctx, empty) overwrote ID: got %q, want op-nil-parent", got)
	}

	// Child context inherits parent ID; overriding updates child only.
	type otherKey struct{}
	child := context.WithValue(ctx, otherKey{}, "keep")
	if got := From(child); got != "op-nil-parent" {
		t.Errorf("child From = %q, want op-nil-parent", got)
	}
	overridden := With(child, "op-override")
	if got := From(overridden); got != "op-override" {
		t.Errorf("overridden From = %q, want op-override", got)
	}
	if got := From(child); got != "op-nil-parent" {
		t.Errorf("parent From mutated to %q, want op-nil-parent", got)
	}
}

// VALIDATES: AC-1 -- Ensure reuses an existing ID and mints a fresh one only when absent.
// PREVENTS: nested command/reload calls overwriting an upstream correlation ID.
func TestEnsure(t *testing.T) {
	//nolint:staticcheck // intentionally testing nil context contract
	ctx1, id1 := Ensure(nil)
	if id1 == "" {
		t.Fatal("Ensure(nil) returned empty ID")
	}
	if got := From(ctx1); got != id1 {
		t.Errorf("From(ctx1) = %q, want %q", got, id1)
	}

	// Second Ensure on ctx1 must reuse id1 without minting a new ID.
	ctx2, id2 := Ensure(ctx1)
	if id2 != id1 {
		t.Errorf("Ensure(ctx1) = %q, want existing %q", id2, id1)
	}
	if got := From(ctx2); got != id1 {
		t.Errorf("From(ctx2) = %q, want %q", got, id1)
	}

	// Fresh context mints a distinct ID.
	ctx3, id3 := Ensure(context.Background())
	if id3 == "" {
		t.Fatal("Ensure(Background) returned empty ID")
	}
	if id3 == id1 {
		t.Errorf("Ensure(Background) reused %q, want distinct ID", id3)
	}
	if got := From(ctx3); got != id3 {
		t.Errorf("From(ctx3) = %q, want %q", got, id3)
	}
}

// VALIDATES: AC-1 -- concurrent New and Ensure calls are race-free and produce unique IDs.
// PREVENTS: atomic counter corruption or ID collisions under concurrent dispatches.
func TestConcurrentNewAndEnsure(t *testing.T) {
	const (
		workers   = 16
		perWorker = 64
	)
	results := make([][]string, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			local := make([]string, 0, perWorker*2)
			for range perWorker {
				local = append(local, New())
				ctx, id := Ensure(context.Background())
				if From(ctx) != id {
					t.Errorf("Ensure/From mismatch: %q != %q", From(ctx), id)
				}
				local = append(local, id)
			}
			results[w] = local
		})
	}
	wg.Wait()

	seen := make(map[string]struct{}, workers*perWorker*2)
	for _, list := range results {
		for _, id := range list {
			if _, dup := seen[id]; dup {
				t.Fatalf("duplicate op-id minted concurrently: %q", id)
			}
			seen[id] = struct{}{}
		}
	}
}
