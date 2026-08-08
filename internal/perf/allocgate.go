// Design: docs/functional-tests.md -- alloc-ceiling gate (ze-alloc-gate stage)
//
// The gate parses `go test -benchmem` output for the reactor hot-path
// ReportAllocs benchmarks (bufmux / forward-pool / EBGPWire) and asserts a
// per-benchmark allocs/op ceiling. mk/alloc-gate.mk drives it as a ze-verify
// stage. allocs/op is machine-independent, so an integer ceiling is a stable
// regression signal without a stored baseline host.
package perf

import (
	"bufio"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// AllocCeilings is the registration point for the allocation-regression gate.
// It maps a hot-path benchmark (bare name, without the "-N" GOMAXPROCS suffix
// go test appends) to the maximum allocs/op the gate tolerates.
//
// Registration over hardcoding: a new hot-path benchmark opts into the gate by
// adding ONE entry here, not by editing gate logic or mk/alloc-gate.mk (which
// runs every reactor benchmark and lets the checker enforce only registered
// names). Ceilings are seeded from a measured `-benchmem` run plus small
// headroom (spec-fixit-perf-alloc-ci-gate, R-1); each entry records its
// measured baseline so a future tightening is auditable.
var AllocCeilings = map[string]int{
	// bufmux buffer-pool Get/Return cycle -- zero-alloc pool access. Measured 0.
	"BenchmarkBufMuxGetReturn": 0,
	// forward-pool non-blocking TryDispatch -- zero-alloc steady state. Measured 0.
	"BenchmarkFwdPoolTryDispatch": 0,
	// EBGPWire cache hit -- single atomic pointer load. Measured 0. The method
	// has no production caller since the AS-path fold (e2037e598). The entry
	// stays until the cache is deleted. A change to it cannot regress
	// unnoticed in the meantime.
	"BenchmarkEBGPWireCacheHitParallel": 0,
	// rs-fastpath ForwardUpdatesDirect per-UPDATE path. Measured 5, +1 headroom.
	"BenchmarkForwardDirect": 6,
	// checkPrefixLimits per-UPDATE, default `count offered` family. Measured 0.
	"BenchmarkCheckPrefixLimitsOffered": 0,
	// checkPrefixLimits per-UPDATE, `count installed` family re-announcing an
	// unchanged table. Measured 0: the steady state looks up the prefix set and
	// inserts nothing. The set's four warm-up inserts amortize to zero only
	// because ALLOC_GATE_BENCHTIME is 300x (mk/alloc-gate.mk); a shorter
	// benchtime turns this red, which is the direction that fails closed.
	"BenchmarkCheckPrefixLimitsInstalled": 0,
	// checkPrefixLimits per-UPDATE, `count installed` family under churn. The
	// ceiling is arithmetic, not a measurement with headroom: the benchmark
	// alternates a four-prefix announce with its withdraw, so four map keys are
	// allocated every two operations.
	"BenchmarkCheckPrefixLimitsInstalledChurn": 2,
}

// AllocResult is one parsed allocs/op sample from `go test -benchmem` output.
type AllocResult struct {
	Name        string // bare benchmark name, "-N" GOMAXPROCS suffix stripped
	AllocsPerOp int
}

// AllocViolation records a benchmark that broke its ceiling or a registered
// benchmark absent from the benchmark output. Absence is a violation
// (fail-closed): a masked build/run failure that emits no benchmark lines must
// fail the gate, never pass silently.
type AllocViolation struct {
	Name        string
	AllocsPerOp int
	Ceiling     int
	Missing     bool
	Message     string
}

// ParseAllocsPerOp extracts the allocs/op column for every benchmark line in
// `go test -benchmem` output. Lines without an allocs/op column (PASS, ok,
// build errors, ns/op-only lines) are skipped.
func ParseAllocsPerOp(text string) []AllocResult {
	var out []AllocResult
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		allocs, ok := allocsFromFields(fields)
		if !ok {
			continue
		}
		out = append(out, AllocResult{Name: stripProcSuffix(fields[0]), AllocsPerOp: allocs})
	}
	return out
}

// allocsFromFields returns the integer immediately preceding an "allocs/op"
// token, e.g. the "5" in "... 5 allocs/op".
func allocsFromFields(fields []string) (int, bool) {
	for i, f := range fields {
		if f != "allocs/op" || i == 0 {
			continue
		}
		n, err := strconv.Atoi(fields[i-1])
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// stripProcSuffix removes the trailing "-N" GOMAXPROCS suffix go test appends
// to benchmark names (BenchmarkForwardDirect-4 -> BenchmarkForwardDirect).
func stripProcSuffix(name string) string {
	i := strings.LastIndexByte(name, '-')
	if i <= 0 {
		return name
	}
	if _, err := strconv.Atoi(name[i+1:]); err != nil {
		return name
	}
	return name[:i]
}

// CheckAllocCeilings parses `go test -benchmem` output and returns the
// violations against ceilings, in stable (sorted) order. Every registered
// benchmark MUST appear in the output; a missing one is reported as a
// fail-closed violation. When a benchmark appears more than once (e.g.
// `-count=N`), the worst (highest) allocs/op sample is used.
func CheckAllocCeilings(text string, ceilings map[string]int) []AllocViolation {
	worst := make(map[string]int, len(ceilings))
	seen := make(map[string]bool, len(ceilings))
	for _, r := range ParseAllocsPerOp(text) {
		if cur, ok := worst[r.Name]; !ok || r.AllocsPerOp > cur {
			worst[r.Name] = r.AllocsPerOp
		}
		seen[r.Name] = true
	}

	names := make([]string, 0, len(ceilings))
	for n := range ceilings {
		names = append(names, n)
	}
	sort.Strings(names)

	var viol []AllocViolation
	for _, name := range names {
		ceiling := ceilings[name]
		if !seen[name] {
			var tb textbuf.Buffer
			msg := tb.Str(name).Str(": absent from benchmark output (expected allocs/op <= ").Int(int64(ceiling)).Str("; did the benchmark build and run?)").String()
			viol = append(viol, AllocViolation{Name: name, Ceiling: ceiling, Missing: true, Message: msg})
			continue
		}
		if got := worst[name]; got > ceiling {
			var tb textbuf.Buffer
			msg := tb.Str(name).Str(": ").Int(int64(got)).Str(" allocs/op exceeds ceiling ").Int(int64(ceiling)).String()
			viol = append(viol, AllocViolation{Name: name, AllocsPerOp: got, Ceiling: ceiling, Message: msg})
		}
	}
	return viol
}
