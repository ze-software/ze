package registry

import (
	"errors"
	"slices"
	"testing"
)

// StartAfter changes the order of a selected graph, never its population.
// A selected cycle is refused even though one of its edges is order-only.
func TestStartAfterOrdersOnlySelectedPlugins(t *testing.T) {
	saved := Snapshot()
	t.Cleanup(func() { Restore(saved) })
	Reset()
	a, b := validReg("a"), validReg("b")
	a.StartAfter = []string{"b"}
	b.Dependencies = []string{"a"}
	for _, reg := range []Registration{a, b} {
		if err := Register(reg); err != nil {
			t.Fatal(err)
		}
	}
	selected, err := ResolveDependencies([]string{"a"}, nil)
	if err != nil || !slices.Equal(selected, []string{"a"}) {
		t.Fatalf("order-only target was activated: selected=%v err=%v", selected, err)
	}
	tiers, err := TopologicalTiers(selected, nil)
	if err != nil || len(tiers) != 1 || !slices.Equal(tiers[0], []string{"a"}) {
		t.Fatalf("absent target constrained startup: tiers=%v err=%v", tiers, err)
	}
	if _, err := ResolveDependencies([]string{"a", "b"}, nil); !errors.Is(err, ErrCircularDependency) {
		t.Fatalf("selected cycle accepted by dependency expansion: %v", err)
	}
	if _, err := TopologicalTiers([]string{"a", "b"}, nil); !errors.Is(err, ErrCircularDependency) {
		t.Fatalf("selected cycle accepted by startup ordering: %v", err)
	}
	// A same-named external program does not inherit this binary's edge.
	external := map[string]bool{"a": true}
	selected, err = ResolveDependencies([]string{"a", "b"}, external)
	if err != nil {
		t.Fatal(err)
	}
	tiers, err = TopologicalTiers(selected, external)
	if err != nil || len(tiers) != 2 || !slices.Equal(tiers[0], []string{"a"}) || !slices.Equal(tiers[1], []string{"b"}) {
		t.Fatalf("external program inherited StartAfter: tiers=%v err=%v", tiers, err)
	}
}

func TestStartAfterRejectsEmptyAndSelfEdges(t *testing.T) {
	saved := Snapshot()
	t.Cleanup(func() { Restore(saved) })
	Reset()
	for _, tc := range []struct {
		target string
		want   error
	}{{"", ErrEmptyDependency}, {"a", ErrSelfDependency}} {
		reg := validReg("a")
		reg.StartAfter = []string{tc.target}
		if err := Register(reg); !errors.Is(err, tc.want) {
			t.Errorf("StartAfter %q: got %v, want %v", tc.target, err, tc.want)
		}
	}
}
