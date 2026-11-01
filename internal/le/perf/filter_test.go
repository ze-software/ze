package perf

import (
	"errors"
	"slices"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestFilterMeasureRefusedOffMac proves `le perf filter measure` measures
// nothing off macOS: performance is measured on the owner's Mac alone.
func TestFilterMeasureRefusedOffMac(t *testing.T) {
	for _, goos := range []string{"linux", "freebsd"} {
		if err := measureFilters(t.TempDir(), goos); !errors.Is(err, errFilterMeasureHost) {
			t.Fatalf("measure on %s answered %v, want errFilterMeasureHost", goos, err)
		}
	}
}

// TestFilterArgsListRunsNothing proves the bare harness only lists (it
// compiles the package and runs no benchmark and no test), while the
// measuring command runs every named benchmark ten times with allocations,
// and both carry the package's tags.
func TestFilterArgsListRunsNothing(t *testing.T) {
	for _, bench := range filterBenches {
		list := filterArgs(bench, false)
		if !slices.Contains(list, "-list") || slices.Contains(list, "-bench") {
			t.Fatalf("%s: the listing %q is not a list-only run", bench.Path, list)
		}
		measure := filterArgs(bench, true)
		for _, want := range []string{"-bench", "-benchmem", "-count=10", "^$", bench.Package} {
			if !slices.Contains(measure, want) {
				t.Fatalf("%s: the measuring command %q lacks %s", bench.Path, measure, want)
			}
		}
		if bench.Tags != "" && !slices.Contains(list, bench.Tags) {
			t.Fatalf("%s: the listing %q lacks the package's tags %s", bench.Path, list, bench.Tags)
		}
	}
}

// TestFilterHarnessListsItsBenchmarks proves the harness builds: every
// package compiles its benchmarks and holds every name the harness runs.
// It compiles the reactor's test binary, so it is skipped under -short.
func TestFilterHarnessListsItsBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the sdk and reactor test binaries")
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	if err := listFilters(root); err != nil {
		t.Fatal(err)
	}
}

// TestFilterCommandIsShellSafe proves the printed command pastes into a
// shell as the words the harness runs: the patterns are quoted.
func TestFilterCommandIsShellSafe(t *testing.T) {
	got := shellLine([]string{"go", "test", "-run", "^$", "-bench", "^(A|B)$", "./pkg/"})
	want := `go test -run '^$' -bench '^(A|B)$' ./pkg/`
	if got != want {
		t.Fatalf("shellLine = %s, want %s", got, want)
	}
	if got := shellLine([]string{"it's"}); got != `'it'\''s'` {
		t.Fatalf("shellLine quoted a single quote as %s", got)
	}
}
