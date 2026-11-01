// Design: docs/guide/benchmarking.md -- Filter throughput
// Related: actions.go -- the filter action's row in the perf table
//
// filter.go is the throughput harness of the route filter paths (filter-wire
// FR-7, AC-17): the in-process typed slot, the external filter-update batch
// at several batch sizes, and the text filter-update the two replace (the
// before figure). Bare, it compiles each benchmark package and lists the
// benchmarks it would run, measuring nothing, and prints the command that
// measures them. `measure` runs them, and only on macOS: performance is
// measured on the owner's Mac alone, whose hardware is known.

package perf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The verb and the one optional word of `le perf filter`.
const (
	filterVerb    = "filter"
	filterMode    = "mode"
	filterMeasure = "measure"
)

// filterMeasureHost is the only GOOS `le perf filter measure` runs on.
const filterMeasureHost = "darwin"

// filterBench is one benchmark set of the harness: which path it measures,
// the package that holds it, the tags that package builds under, and the
// benchmark names.
type filterBench struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Tags    string   `json:"tags,omitempty"`
	Names   []string `json:"names"`
	// Command is the go command that measures the set.
	Command string `json:"command"`
}

// filterBenches is the harness. BenchmarkFilterForkedBatch runs its own
// sub-benchmarks at 1, 16 and 64 routes per batch.
var filterBenches = []filterBench{
	{
		Path:    "in-process: the typed DirectBridge filter slot",
		Package: "./pkg/plugin/sdk/",
		Names:   []string{"BenchmarkFilterFastPath"},
	},
	{
		Path:    "external: one filter-update batch of 1, 16 and 64 routes",
		Package: "./pkg/plugin/sdk/",
		Names:   []string{"BenchmarkFilterForkedBatch"},
	},
	{
		Path:    "text filter-update, the before figure",
		Package: "./internal/component/bgp/reactor/",
		Tags:    "ze_bgp",
		Names: []string{"BenchmarkFilterModifyEgress", "BenchmarkAppendUpdateForFilter_Reused",
			"BenchmarkAppendAttrsForFilter_Reused"},
	},
}

// FilterReport answers `le perf filter`.
type FilterReport struct {
	Action string `json:"action"`
	// Measured is true when the benchmarks ran; bare, they are only listed.
	Measured   bool          `json:"measured"`
	Benchmarks []filterBench `json:"benchmarks"`
	// Run is the command that measures every set, for the owner's Mac.
	Run   string `json:"run"`
	Error string `json:"error,omitempty"`
	Code  int    `json:"-"`
}

// filterMeasureCommand is the one command the owner runs.
const filterMeasureCommand = "./le perf filter measure"

// filterHere is the `le perf filter [measure]` action, over this checkout.
func filterHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return FilterReport{Action: filterVerb, Error: err.Error(), Code: 1}, 1
	}
	measure := args.One(filterMode) == filterMeasure
	report := FilterReport{Action: filterVerb, Measured: measure, Run: filterMeasureCommand}
	for _, bench := range filterBenches {
		bench.Command = "go " + strings.Join(filterArgs(bench, true), " ")
		report.Benchmarks = append(report.Benchmarks, bench)
	}
	if measure {
		err = measureFilters(root, runtime.GOOS)
	} else {
		err = listFilters(root)
	}
	if err != nil {
		leaction.ReportError(err)
		report.Error, report.Code = err.Error(), 1
		return report, 1
	}
	return report, 0
}

// filterArgs answers the go arguments for bench: the measuring run, or the
// listing that compiles the package and runs nothing.
func filterArgs(bench filterBench, measure bool) []string {
	args := []string{"test"}
	if bench.Tags != "" {
		args = append(args, "-tags", bench.Tags)
	}
	pattern := "^(" + strings.Join(bench.Names, "|") + ")$"
	if measure {
		args = append(args, "-run", "^$", "-bench", pattern, "-benchmem", "-count=10")
	} else {
		args = append(args, "-list", pattern)
	}
	return append(args, bench.Package)
}

// errFilterMeasureHost refuses a measurement off the owner's Mac.
var errFilterMeasureHost = errors.New("le perf filter measure runs only on macOS: performance is" +
	" measured on the owner's Mac alone, whose hardware is known; run `" + filterMeasureCommand +
	"` there, or `./le perf filter` here to check that the harness builds")

// measureFilters runs every set, its output streamed to the terminal.
func measureFilters(root, goos string) error {
	if goos != filterMeasureHost {
		return errFilterMeasureHost
	}
	for _, bench := range filterBenches {
		cmd := exec.CommandContext(context.Background(), "go", filterArgs(bench, true)...) //nolint:gosec // the arguments come from filterBenches
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("perf filter: %s: %w", bench.Path, err)
		}
	}
	return nil
}

// listFilters compiles each set's package and refuses a set whose benchmarks
// the package no longer holds, so a renamed benchmark breaks the harness
// here rather than on the owner's Mac.
func listFilters(root string) error {
	for _, bench := range filterBenches {
		cmd := exec.CommandContext(context.Background(), "go", filterArgs(bench, false)...) //nolint:gosec // the arguments come from filterBenches
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("perf filter: %s: the package %s does not build its benchmarks: %w\n%s",
				bench.Path, bench.Package, err, out)
		}
		listed := strings.Fields(string(out))
		for _, name := range bench.Names {
			if !slices.Contains(listed, name) {
				return fmt.Errorf("perf filter: %s: %s holds no %s; correct filterBenches in internal/le/perf/filter.go",
					bench.Path, bench.Package, name)
			}
		}
	}
	return nil
}
