package supportreg

import (
	"slices"
	"sync"
	"testing"
)

// TestSupportRegRegisterAndUnregister verifies registration, duplicate/invalid
// panics, idempotent token-checked unregistration, re-registration after
// unregister, sorted Names(), Snapshot() isolation, and concurrent race safety.
//
// VALIDATES: AC-7 -- supportreg.Register, Names, and Snapshot contracts for
// static and daemon-registered live support modules.
// PREVENTS: duplicate module silent overwrite, stale unregister closures
// removing a re-registered module, or map mutation races on Snapshot.
func TestSupportRegRegisterAndUnregister(t *testing.T) {
	t.Run("panics on empty name nil fn and duplicate live name", func(t *testing.T) {
		assertPanics(t, "empty name", func() {
			Register("", func(*CollectOptions) (any, error) { return "empty", nil })
		})
		assertPanics(t, "nil collector", func() {
			Register("mod-nil", nil)
		})

		unreg := Register("mod-dup", func(*CollectOptions) (any, error) { return "v1", nil })
		t.Cleanup(unreg)

		assertPanics(t, "duplicate live name", func() {
			Register("mod-dup", func(*CollectOptions) (any, error) { return "v2", nil })
		})
	})

	t.Run("unregister idempotency token safety and re-register", func(t *testing.T) {
		unreg1 := Register("mod-life", func(*CollectOptions) (any, error) { return "first", nil })
		t.Cleanup(unreg1)

		unreg1()
		unreg1() // idempotent

		if slices.Contains(Names(), "mod-life") {
			t.Fatal("mod-life still present after unregister")
		}

		// Re-register after unregister must succeed; stale unreg1 must not remove it.
		unreg2 := Register("mod-life", func(*CollectOptions) (any, error) { return "second", nil })
		t.Cleanup(unreg2)

		unreg1()

		snap := Snapshot()
		fn, ok := snap["mod-life"]
		if !ok {
			t.Fatal("stale unreg1 removed re-registered mod-life")
		}
		val, err := fn(&CollectOptions{})
		if err != nil || val != "second" {
			t.Fatalf("collector returned (%v, %v), want (\"second\", nil)", val, err)
		}

		unreg2()
		unreg2() // idempotent
		if slices.Contains(Names(), "mod-life") {
			t.Fatal("mod-life still present after unreg2")
		}
	})

	t.Run("Names sorted and Snapshot isolated", func(t *testing.T) {
		unregC := Register("z-mod", func(*CollectOptions) (any, error) { return "z", nil })
		t.Cleanup(unregC)
		unregA := Register("a-mod", func(*CollectOptions) (any, error) { return "a", nil })
		t.Cleanup(unregA)
		unregB := Register("m-mod", func(*CollectOptions) (any, error) { return "m", nil })
		t.Cleanup(unregB)

		names := Names()
		if !slices.IsSorted(names) {
			t.Fatalf("Names() not sorted: %v", names)
		}
		for _, want := range []string{"a-mod", "m-mod", "z-mod"} {
			if !slices.Contains(names, want) {
				t.Errorf("Names() = %v missing %q", names, want)
			}
		}

		snap := Snapshot()
		delete(snap, "a-mod")
		snap["injected"] = func(*CollectOptions) (any, error) { return "injected", nil }

		fresh := Snapshot()
		if _, ok := fresh["a-mod"]; !ok {
			t.Error("mutating Snapshot deleted a-mod from live registry")
		}
		if _, ok := fresh["injected"]; ok {
			t.Error("mutating Snapshot inserted injected into live registry")
		}
	})

	t.Run("concurrent register unregister and read are race-free", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)
		go runConcurrentSupportRegWriter(&wg)
		go runConcurrentSupportRegReader(&wg)
		wg.Wait()
	})
}

func runConcurrentSupportRegWriter(wg *sync.WaitGroup) {
	defer wg.Done()
	for range 50 {
		unreg := Register("concurrent-mod", func(*CollectOptions) (any, error) { return 1, nil })
		unreg()
	}
}

func runConcurrentSupportRegReader(wg *sync.WaitGroup) {
	defer wg.Done()
	for range 50 {
		_ = Names()
		_ = Snapshot()
	}
}

func assertPanics(t *testing.T, label string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", label)
		}
	}()
	fn()
}
