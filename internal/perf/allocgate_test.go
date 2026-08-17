// VALIDATES: the alloc-ceiling gate parses `go test -benchmem` allocs/op and
//
//	compares each hot-path benchmark to its registered integer ceiling
//	(spec-fixit-perf-alloc-ci-gate AC-1, AC-2, boundary).
//
// PREVENTS: a per-op heap allocation reintroduced on a forward/bufmux/EBGPWire
//
//	hot path merging undetected, and a masked benchmark build/run
//	failure passing the gate silently (fail-closed missing-benchmark).
package perf

import (
	"os"
	"testing"
)

// sampleBenchOutput mimics `go test -benchmem` output for every registered
// hot-path benchmark, all within their ceilings. A name registered in
// perf.AllocCeilings and absent here is a Missing violation by design
// (fail-closed), so this fixture gains a line whenever the map gains an entry.
const sampleBenchOutput = `goos: linux
goarch: amd64
pkg: github.com/ze-software/ze/internal/component/bgp/reactor
cpu: AMD EPYC 7351 16-Core Processor
BenchmarkForwardDirect-4              	    3000	      4466 ns/op	     477 B/op	       5 allocs/op
BenchmarkBufMuxGetReturn-4            	    3000	       105.3 ns/op	     175 B/op	       0 allocs/op
BenchmarkFwdPoolTryDispatch-4         	    3000	       270.3 ns/op	     192 B/op	       0 allocs/op
BenchmarkEBGPWireCacheHitParallel-4   	    3000	        36.41 ns/op	       0 B/op	       0 allocs/op
BenchmarkCheckPrefixLimitsOffered-4   	    3000	       198.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkCheckPrefixLimitsInstalled-4 	    3000	       237.2 ns/op	       2 B/op	       0 allocs/op
BenchmarkCheckPrefixLimitsInstalledChurn-4	    3000	       418.1 ns/op	      10 B/op	       2 allocs/op
PASS
ok  	github.com/ze-software/ze/internal/component/bgp/reactor	0.081s
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
		"BenchmarkEBGPWireCacheHitParallel":        0,
		"BenchmarkCheckPrefixLimitsOffered":        0,
		"BenchmarkCheckPrefixLimitsInstalled":      0,
		"BenchmarkCheckPrefixLimitsInstalledChurn": 2,
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

// TestAllocGateEnforce is the real gate driver: mk/alloc-gate.mk runs the
// reactor benchmarks with -benchmem, writes the output to a file, and points
// ZE_ALLOC_GATE_BENCH at it. When the env var is unset (a normal `go test`
// run) the test skips, so enforcement happens only via `make ze-alloc-check`.
func TestAllocGateEnforce(t *testing.T) {
	path := os.Getenv("ZE_ALLOC_GATE_BENCH")
	if path == "" {
		t.Skip("ZE_ALLOC_GATE_BENCH unset; run via `make ze-alloc-check`")
	}
	data, err := os.ReadFile(path) //nolint:gosec // path supplied by the make gate within the repo
	if err != nil {
		t.Fatalf("read benchmark output %s: %v", path, err)
	}
	for _, v := range checkAllocCeilings(string(data), AllocCeilings) {
		t.Errorf("alloc gate: %s", v.Message)
	}
}
