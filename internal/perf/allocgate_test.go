// VALIDATES: the alloc-ceiling gate parses `go test -benchmem` allocs/op and
//
//	compares each hot-path benchmark to its registered integer ceiling
//	(spec-fixit-perf-alloc-ci-gate AC-1, AC-2, boundary).
//
// PREVENTS: a per-op heap allocation reintroduced on a registered hot path
//
//	merging undetected, a masked benchmark build/run failure passing the
//	gate silently (fail-closed missing-benchmark), and a registered
//	benchmark sitting in a package the gate never benchmarks.
package perf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sampleBenchOutput mimics `go test -benchmem` output for every registered
// hot-path benchmark, all within their ceilings. A name registered in
// perf.AllocCeilings and absent here is a Missing violation by design
// (fail-closed), so this fixture gains a line whenever the map gains an entry.
//
// It states what the COMPARATOR is fed, never what the daemon measures. Every
// line here is a fixture, and a number in it says nothing about the product:
// `./le verify-deps alloc` is the one place the daemon's real numbers live.
const sampleBenchOutput = `goos: linux
goarch: amd64
pkg: github.com/ze-software/ze/internal/component/bgp/reactor
cpu: AMD EPYC 7351 16-Core Processor
BenchmarkForwardDirect-4              	    3000	      4466 ns/op	     477 B/op	       5 allocs/op
BenchmarkBufMuxGetReturn-4            	    3000	       105.3 ns/op	     175 B/op	       0 allocs/op
BenchmarkFwdPoolTryDispatch-4         	    3000	       270.3 ns/op	     192 B/op	       0 allocs/op
BenchmarkCheckPrefixLimitsOffered-4   	    3000	       198.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkCheckPrefixLimitsInstalled-4 	    3000	       237.2 ns/op	       2 B/op	       0 allocs/op
BenchmarkCheckPrefixLimitsInstalledChurn-4	    3000	       418.1 ns/op	      10 B/op	       2 allocs/op
PASS
ok  	github.com/ze-software/ze/internal/component/bgp/reactor	0.081s
goos: linux
goarch: amd64
pkg: github.com/ze-software/ze/internal/component/plugin
cpu: AMD EPYC 7351 16-Core Processor
BenchmarkRecordAnswerRows-4           	     300	       310.0 ns/op	      64 B/op	       0 allocs/op
PASS
ok  	github.com/ze-software/ze/internal/component/plugin	0.094s
`

// TestAllocGateCeiling verifies AC-1: benchmark output within every registered
// ceiling produces no violations.
func TestAllocGateCeiling(t *testing.T) {
	viol := checkAllocCeilings(sampleBenchOutput, AllocCeilings)
	if len(viol) != 0 {
		t.Fatalf("expected no violations for in-ceiling output, got %d: %+v", len(viol), viol)
	}
}

// TestAllocGateRegressionFails verifies AC-2: a benchmark whose allocs/op
// exceeds its registered ceiling yields a violation that names the benchmark.
func TestAllocGateRegressionFails(t *testing.T) {
	// BenchmarkFwdPoolTryDispatch ceiling is 0; report 3 allocs/op.
	regressed := "BenchmarkFwdPoolTryDispatch-4  1000  200 ns/op  64 B/op  3 allocs/op\n"
	full := sampleBenchOutput + regressed

	viol := checkAllocCeilings(full, AllocCeilings)
	if len(viol) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %+v", len(viol), viol)
	}
	v := viol[0]
	if v.Name != "BenchmarkFwdPoolTryDispatch" {
		t.Errorf("violation names %q, want BenchmarkFwdPoolTryDispatch", v.Name)
	}
	if v.AllocsPerOp != 3 || v.Ceiling != 0 {
		t.Errorf("got allocs=%d ceiling=%d, want allocs=3 ceiling=0", v.AllocsPerOp, v.Ceiling)
	}
	if v.Missing {
		t.Errorf("regression should not be flagged Missing")
	}
	if v.Message == "" {
		t.Errorf("violation message is empty")
	}
}

// TestAllocGateBoundary verifies the numeric boundary: allocs == ceiling passes,
// ceiling+1 fails. Uses a single-entry ceiling map for isolation.
func TestAllocGateBoundary(t *testing.T) {
	ceilings := map[string]int{"BenchmarkForwardDirect": 5}

	atCeiling := "BenchmarkForwardDirect-4  100  4000 ns/op  400 B/op  5 allocs/op\n"
	if viol := checkAllocCeilings(atCeiling, ceilings); len(viol) != 0 {
		t.Errorf("allocs == ceiling must pass, got violations: %+v", viol)
	}

	overCeiling := "BenchmarkForwardDirect-4  100  4000 ns/op  400 B/op  6 allocs/op\n"
	viol := checkAllocCeilings(overCeiling, ceilings)
	if len(viol) != 1 {
		t.Fatalf("allocs == ceiling+1 must fail, got %d violations", len(viol))
	}
	if viol[0].AllocsPerOp != 6 || viol[0].Ceiling != 5 {
		t.Errorf("got allocs=%d ceiling=%d, want 6/5", viol[0].AllocsPerOp, viol[0].Ceiling)
	}
}

// TestAllocGateMissingFailsClosed verifies the fail-closed guard: a registered
// benchmark absent from the output (e.g. a masked build failure emitting no
// benchmark lines) is a violation, never a silent pass.
func TestAllocGateMissingFailsClosed(t *testing.T) {
	// Build-failure-like output: no Benchmark lines at all.
	buildFailure := "# github.com/ze-software/ze/internal/component/bgp/reactor\n./x.go:1:1: undefined: foo\nFAIL\n"
	viol := checkAllocCeilings(buildFailure, AllocCeilings)
	if len(viol) != len(AllocCeilings) {
		t.Fatalf("expected all %d registered benchmarks flagged missing, got %d: %+v", len(AllocCeilings), len(viol), viol)
	}
	for _, v := range viol {
		if !v.Missing {
			t.Errorf("%s should be flagged Missing", v.Name)
		}
	}
}

// TestParseAllocsPerOp verifies the parser strips the -N suffix and reads the
// allocs/op column, ignoring non-benchmark lines.
func TestParseAllocsPerOp(t *testing.T) {
	got := parseAllocsPerOp(sampleBenchOutput)
	want := map[string]int{
		"BenchmarkForwardDirect":                   5,
		"BenchmarkBufMuxGetReturn":                 0,
		"BenchmarkFwdPoolTryDispatch":              0,
		"BenchmarkCheckPrefixLimitsOffered":        0,
		"BenchmarkCheckPrefixLimitsInstalled":      0,
		"BenchmarkCheckPrefixLimitsInstalledChurn": 2,
		"BenchmarkRecordAnswerRows":                0,
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d results, want %d: %+v", len(got), len(want), got)
	}
	for _, r := range got {
		w, ok := want[r.Name]
		if !ok {
			t.Errorf("unexpected benchmark %q", r.Name)
			continue
		}
		if r.AllocsPerOp != w {
			t.Errorf("%s: parsed %d allocs/op, want %d", r.Name, r.AllocsPerOp, w)
		}
	}
}

// TestAllocGateWorstSample verifies that with repeated samples (-count) the
// worst (highest) allocs/op is used for the ceiling comparison.
func TestAllocGateWorstSample(t *testing.T) {
	ceilings := map[string]int{"BenchmarkBufMuxGetReturn": 0}
	repeated := "BenchmarkBufMuxGetReturn-4  1  1 ns/op  0 B/op  0 allocs/op\n" +
		"BenchmarkBufMuxGetReturn-4  1  1 ns/op  8 B/op  1 allocs/op\n"
	viol := checkAllocCeilings(repeated, ceilings)
	if len(viol) != 1 {
		t.Fatalf("worst-of-repeats should trip ceiling 0, got %d violations", len(viol))
	}
	if viol[0].AllocsPerOp != 1 {
		t.Errorf("worst sample = %d, want 1", viol[0].AllocsPerOp)
	}
}

// TestAllocGateEnforce is the gate-side reader: `./le verify-deps alloc` runs
// the selected benchmarks with -benchmem, writes the output to a file, and
// points ZE_ALLOC_GATE_BENCH at it. When the env var is unset (a normal
// `go test` run) the test skips.
func TestAllocGateEnforce(t *testing.T) {
	path := os.Getenv("ZE_ALLOC_GATE_BENCH")
	if path == "" {
		t.Skip("ZE_ALLOC_GATE_BENCH unset; run via `./le verify-deps alloc`")
	}
	data, err := os.ReadFile(path) //nolint:gosec // path supplied by the native verifier
	if err != nil {
		t.Fatalf("read benchmark output %s: %v", path, err)
	}
	for _, v := range checkAllocCeilings(string(data), AllocCeilings) {
		t.Errorf("alloc gate: %s", v.Message)
	}
}

// recordPathBenchmark is the command-answer record path's benchmark, and
// recordPathPackage is the package directory that defines it. They are spelled
// here rather than derived so the ceiling registry and package's own test source
// cannot drift.
const (
	recordPathBenchmark = "BenchmarkRecordAnswerRows"
	recordPathPackage   = "internal/component/plugin"
)

// recordPathCeiling is the allocs/op the record path is held to, and it is a
// GOAL rather than a measurement with headroom: AC-1 of
// spec-record-answers-3-zero-alloc is zero allocations for each row.
//
// It is spelled here as well as in AllocCeilings because the gate compares
// against whatever number the registry carries, so raising that number is the
// cheapest route from a red gate to a green one -- cheaper than fixing the
// allocation, and invisible in the gate's own output. Nothing else in the
// pipeline reads it: the enforce check, the parse tests and the relaxation
// audit all stay green at any ceiling.
const recordPathCeiling = 0

// VALIDATES: AC-3 of spec-record-answers-3-zero-alloc: the record path has a
// registered zero-allocation ceiling and the named benchmark exists in its
// package. The native verifier's own plan tests pin that package in the
// benchmark population.
//
// PREVENTS: a ceiling registered for a benchmark that no longer exists, or a
// loosened ceiling that turns the gate green without removing the allocation.
func TestAllocGateCoversRecordPath(t *testing.T) {
	ceiling, registered := AllocCeilings[recordPathBenchmark]
	if !registered {
		t.Errorf("%s has no ceiling in AllocCeilings, so the gate enforces nothing for the record path", recordPathBenchmark)
	}
	if registered && ceiling != recordPathCeiling {
		t.Errorf("%s is registered at %d allocs/op, want %d: a loosened ceiling turns the gate green without the allocation going away, and no other check reads this number",
			recordPathBenchmark, ceiling, recordPathCeiling)
	}

	root := repoRootForTest(t)
	if !benchmarkDefinedIn(t, filepath.Join(root, recordPathPackage), recordPathBenchmark) {
		t.Errorf("no `func %s(` in %s, so the registered ceiling names a benchmark that does not exist", recordPathBenchmark, recordPathPackage)
	}

}

// repoRootForTest returns the repository root from this test file's location
// (<root>/internal/perf/allocgate_test.go -> <root>).
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed, so the repository root cannot be found")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// benchmarkDefinedIn reports whether dir holds a Go test file defining name. It
// reads the directory rather than the whole tree. A benchmark defined anywhere
// else is in another package, and the gate would not run it from here.
func benchmarkDefinedIn(t *testing.T, dir, name string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	definition := "func " + name + "("
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // a tracked test file under the repository root
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		if strings.Contains(string(data), definition) {
			return true
		}
	}
	return false
}
