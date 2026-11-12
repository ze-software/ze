package show

import (
	"slices"
	"strconv"
	"sync"
	"testing"
)

// TestRegisterNeverMovesAListAReaderHolds registers keys that sort before the
// existing ones while readers enrich the same command.
//
// VALIDATES: every Enrich and EnrichBrief call sees one consistent list: each
// pre-registered key exactly once, no key twice, in key order. Under -race,
// Register writes no entry of a list a reader read under the read lock and
// iterates after releasing it.
// PREVENTS: Register's append into spare capacity and in-place sort moving
// the entries of a list a `show` is iterating, which a plugin restart reaches
// at run time when it registers its enrichers again.
//
// Three keys are registered first, so the list has spare capacity; each later
// key sorts before all of them, so a sort in place would move every entry.
func TestRegisterNeverMovesAListAReaderHolds(t *testing.T) {
	const (
		command = "show register-race"
		readers = 4
		added   = 64
	)
	record := func(key string) func(map[string]any) {
		return func(base map[string]any) {
			// Absent on the first enricher of a call, so the list starts empty.
			seen, _ := base["seen"].([]string)
			base["seen"] = append(seen, key)
		}
	}
	register := func(key string) {
		t.Helper()
		if err := Register(command, key, Enricher{Detail: record(key), Brief: record(key)}); err != nil {
			t.Fatalf("register %s: %v", key, err)
		}
		t.Cleanup(func() { Unregister(command, key) })
	}
	existing := []string{"z00", "z01", "z02"}
	for _, key := range existing {
		register(key)
	}

	stop := make(chan struct{})
	var started, wg sync.WaitGroup
	started.Add(readers)
	for i := range readers {
		enrich := Enrich
		if i%2 == 1 {
			enrich = EnrichBrief
		}
		wg.Go(func() {
			first := true
			for {
				base := map[string]any{}
				enrich(command, base)
				// An empty list fails below: every existing key must be in it.
				seen, _ := base["seen"].([]string)
				if !consistentList(seen, existing) {
					t.Errorf("a show saw an inconsistent enricher list: %v", seen)
					if first {
						started.Done()
					}
					return
				}
				if first {
					started.Done()
					first = false
				}
				select {
				case <-stop:
					return
				default:
				}
			}
		})
	}
	started.Wait()

	// Each new key sorts before every key already registered.
	for i := added - 1; i >= 0; i-- {
		register("a" + strconv.Itoa(100+i))
	}
	close(stop)
	wg.Wait()
}

// consistentList reports whether seen holds no key twice, holds every key of
// existing, and is in key order.
func consistentList(seen, existing []string) bool {
	if !slices.IsSorted(seen) {
		return false
	}
	if len(slices.Compact(slices.Clone(seen))) != len(seen) {
		return false
	}
	for _, key := range existing {
		if !slices.Contains(seen, key) {
			return false
		}
	}
	return true
}
