package migration

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/version"
)

func TestCompareReleases(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"26.05.26", "26.05.26", 0},
		{"26.05.25", "26.05.26", -1},
		{"26.05.27", "26.05.26", 1},
		{"26.04.30", "26.05.01", -1},
		{"25.12.31", "26.01.01", -1},
		{"27.01.01", "26.12.31", 1},
	}
	for _, tc := range cases {
		got := version.CompareReleases(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("version.CompareReleases(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareReleasesUnparseable(t *testing.T) {
	if got := version.CompareReleases("dev", "26.05.26"); got != -1 {
		t.Errorf("unparseable a is infinitely old, want -1, got %d", got)
	}
	if got := version.CompareReleases("26.05.26", "dev"); got != 1 {
		t.Errorf("unparseable b is infinitely old, want 1, got %d", got)
	}
	if got := version.CompareReleases("dev", "dev"); got != 0 {
		t.Errorf("both unparseable should be equal, got %d", got)
	}
}

func TestRegisterEvolutionInvalidRelease(t *testing.T) {
	err := RegisterEvolution(Evolution{
		Release: "bad",
		Name:    "test",
		Detect:  func(*config.Tree) bool { return false },
		Apply:   func(t *config.Tree) (*config.Tree, error) { return t, nil },
	})
	if err == nil {
		t.Fatal("expected error for invalid release")
	}
}

func TestEvolveNoEvolutions(t *testing.T) {
	tree := config.NewTree()
	tree.Set("bgp", "test")

	result, err := Evolve(tree, "26.05.26")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result when no evolutions registered")
	}
}

func TestEvolveNilTree(t *testing.T) {
	_, err := Evolve(nil, "26.05.26")
	if err == nil {
		t.Fatal("expected error for nil tree")
	}
}

func TestEvolveAppliesNewerEvolutions(t *testing.T) {
	// Save and restore global state.
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	if err := RegisterEvolution(Evolution{
		Release:     "26.06.01",
		Name:        "add-marker",
		Description: "adds a marker leaf",
		Detect: func(t *config.Tree) bool {
			_, ok := t.Get("marker")
			return !ok
		},
		Apply: func(t *config.Tree) (*config.Tree, error) {
			result := t.Clone()
			result.Set("marker", "evolved")
			return result, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	tree := config.NewTree()
	tree.Set("bgp", "test")

	result, err := Evolve(tree, "26.05.26")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected evolution to apply")
	}
	if len(result.Applied) != 1 || result.Applied[0] != "add-marker" {
		t.Errorf("Applied = %v, want [add-marker]", result.Applied)
	}

	v, ok := result.Tree.Get("marker")
	if !ok || v != "evolved" {
		t.Errorf("marker = %q, ok = %v, want 'evolved'", v, ok)
	}
}

func TestEvolveSkipsOlderEvolutions(t *testing.T) {
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	if err := RegisterEvolution(Evolution{
		Release:     "26.04.01",
		Name:        "old-change",
		Description: "older than stamp",
		Detect:      func(*config.Tree) bool { return true },
		Apply: func(t *config.Tree) (*config.Tree, error) {
			return t, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	tree := config.NewTree()
	result, err := Evolve(tree, "26.05.26")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result when all evolutions are older than stamp")
	}
}

func TestEvolveDevStampRunsAll(t *testing.T) {
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	if err := RegisterEvolution(Evolution{
		Release:     "26.06.01",
		Name:        "future-change",
		Description: "should run for dev stamp",
		Detect:      func(*config.Tree) bool { return true },
		Apply: func(t *config.Tree) (*config.Tree, error) {
			result := t.Clone()
			result.Set("evolved", "yes")
			return result, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	tree := config.NewTree()
	result, err := Evolve(tree, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected evolution to apply for dev stamp")
	}
	if len(result.Applied) != 1 {
		t.Errorf("Applied = %v, want 1 entry", result.Applied)
	}
}

func TestEvolveEmptyStampRunsAll(t *testing.T) {
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	if err := RegisterEvolution(Evolution{
		Release:     "26.06.01",
		Name:        "test-change",
		Description: "should run for empty stamp",
		Detect:      func(*config.Tree) bool { return true },
		Apply: func(t *config.Tree) (*config.Tree, error) {
			result := t.Clone()
			result.Set("evolved", "yes")
			return result, nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	tree := config.NewTree()
	result, err := Evolve(tree, "")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected evolution to apply for empty stamp")
	}
}

func TestRegisterEvolutionDuplicate(t *testing.T) {
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	e := Evolution{
		Release: "26.07.01", Name: "dup",
		Detect: func(*config.Tree) bool { return false },
		Apply:  func(t *config.Tree) (*config.Tree, error) { return t, nil },
	}
	if err := RegisterEvolution(e); err != nil {
		t.Fatal(err)
	}
	if err := RegisterEvolution(e); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestEvolveNewerStampSkipsAll(t *testing.T) {
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	_ = RegisterEvolution(Evolution{
		Release: "26.07.01", Name: "check",
		Detect: func(*config.Tree) bool { return true },
		Apply: func(t *config.Tree) (*config.Tree, error) {
			result := t.Clone()
			result.Set("touched", "yes")
			return result, nil
		},
	})

	tree := config.NewTree()
	result, err := Evolve(tree, "26.08.01")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result when stamp is newer than all evolutions")
	}
}

func TestEvolveChainOrder(t *testing.T) {
	evolutionMu.Lock()
	saved := evolutions
	evolutions = nil
	evolutionMu.Unlock()
	defer func() {
		evolutionMu.Lock()
		evolutions = saved
		evolutionMu.Unlock()
	}()

	var order []string

	_ = RegisterEvolution(Evolution{
		Release: "26.07.01", Name: "second",
		Detect: func(*config.Tree) bool { return true },
		Apply: func(t *config.Tree) (*config.Tree, error) {
			order = append(order, "second")
			return t.Clone(), nil
		},
	})
	_ = RegisterEvolution(Evolution{
		Release: "26.06.01", Name: "first",
		Detect: func(*config.Tree) bool { return true },
		Apply: func(t *config.Tree) (*config.Tree, error) {
			order = append(order, "first")
			return t.Clone(), nil
		},
	})

	tree := config.NewTree()
	result, err := Evolve(tree, "26.05.01")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected evolutions to apply")
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("execution order = %v, want [first second]", order)
	}
}
