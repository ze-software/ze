// Design: docs/architecture/core-design.md -- the perf benchmark, as three actions
// Detail: ../../test/perfrunner/run.go -- the multi-DUT Docker run these verbs drive
// Overview: actions.go -- the table these three verbs are declared in
//
// bench.go holds the three verbs that EXECUTE a benchmark, apart from the nudge
// that only reads the checkout. Each one is a chain, and the chain is what the
// retired Make targets carried: build the host ze-perf, measure the DUTs under
// Docker, append each result to the committed NDJSON history, and check the
// history for a regression.
//
// The host program is built at bin/ze-perf, which is where the multi-DUT runner
// looks for it, where .github/workflows/perf-nightly.yml runs the regression
// check from, and what docs/guide/benchmarking.md tells a developer to build.
// The runner cross-builds its own bin/ze-perf-linux for the container.
package perfbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/perf"
	"github.com/ze-software/ze/internal/test/perfrunner"
)

const (
	// runVerb measures every DUT the runner knows, or the ones named.
	runVerb = "run"
	// historyVerb appends the results of the last measurement to the history.
	historyVerb = "history-record"
	// evidenceVerb is the release gate: measure ze, record it, fail on a regression.
	evidenceVerb = "evidence-record"

	// dutKeyword selects the DUTs a run measures.
	dutKeyword = "dut"

	// perfTags selects the ze-perf program out of the one cmd/ze codebase.
	perfTags = "ze_perf ze_bgp"

	// resultsDir is where the runner writes one <dut>.json per measured DUT.
	// It is a build output and .gitignore excludes it.
	resultsDir = "test/perf/results"

	// historyDir holds the committed NDJSON history the regression check reads.
	historyDir = "test/perf/history"

	// zeDUT is the device under test the release evidence gate measures.
	zeDUT = "ze"

	// buildAction, measureAction and checkAction are the headings a person
	// watching a run reads. gaterun.Run announces the two it runs; the
	// measurement runs in this process, so its heading is announced here.
	buildAction   = "perf-bench build"
	measureAction = "perf-bench measure"
	checkAction   = "perf-bench regression check"

	// jsonSuffix and ndjsonSuffix name the result and history file types.
	jsonSuffix   = ".json"
	ndjsonSuffix = ".ndjson"
)

// RunReport is the answer of the three verbs that execute a benchmark. It
// states what each step of the chain did, because a reader cannot recover any
// of it from the runner's streamed output.
type RunReport struct {
	// Action is the verb that produced this report.
	Action string `json:"action"`
	// Benchmarked names the DUTs the run measured. It is empty when the run
	// asked for every DUT the runner knows.
	Benchmarked []string `json:"benchmarked,omitempty"`
	// Appended names the history files the run added a result line to.
	Appended []string `json:"appended,omitempty"`
	// Checked is the history file the regression check read.
	Checked string `json:"checked,omitempty"`
	// Recorded is the SHA written as "perf ran here", which clears the nudge.
	Recorded string `json:"recorded,omitempty"`
	// Error explains the step that failed.
	Error string `json:"error,omitempty"`
	// Writes says this action changed the tree.
	Writes bool `json:"writes"`
	// Code is the exit code of the first step that failed.
	Code int `json:"code"`
}

// Text renders the report for a person and ends with a newline.
//
// A failed run renders nothing: leaction.ReportError has already written the
// diagnosis to stderr, and restating it here would put that diagnosis into a
// piped document. The Error field carries it for `| json` and `| yaml`.
func (r RunReport) Text() string {
	if r.Error != "" {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("perf-bench ").Str(r.Action).Str(": ")
	if len(r.Benchmarked) > 0 {
		tb.Str("measured ").Join(r.Benchmarked, ", ").Str("; ")
	}
	if len(r.Appended) > 0 {
		tb.Str("appended ").Join(r.Appended, ", ").Str("; ")
	}
	if r.Checked != "" {
		tb.Str("no regression in ").Str(r.Checked).Str("; ")
	}
	return tb.Str("recorded ").Str(r.Recorded).Byte('\n').String()
}

// The four failures a chain step reports. Each names the step rather than the
// program under it, because the program has already written its own diagnosis
// to this terminal.
var (
	errBuild      = errors.New("the ze-perf build failed")
	errMeasure    = errors.New("the Docker benchmark failed")
	errRegression = errors.New("the committed history shows a regression")
)

// markerError says the marker that clears the nudge was not written.
func markerError(marker Report) error {
	if marker.Error != "" {
		return errors.New(marker.Error)
	}
	return errors.New("the perf-run marker was not written")
}

// commandStep runs one command with the child on this terminal and answers its
// exit code. The compiler and the regression check both go through it.
type commandStep func(action string, argv []string, dir string, environ []string) int

// measureStep runs the multi-DUT Docker benchmark and answers its exit code.
// perfBinary is the host program the runner reports with.
type measureStep func(root, perfBinary string, args []string) int

// Bench is one benchmark chain over a checkout. The two process seams are
// fields so a package test pins what each verb runs without Docker, a compiler,
// or minutes of machine time.
type Bench struct {
	Root      string
	Toolchain gotoolchain.Toolchain
	Command   commandStep
	Measure   measureStep
}

// newBench answers a chain over the checkout this command was run in.
func newBench() (*Bench, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return nil, err
	}
	return &Bench{Root: root, Toolchain: toolchain, Command: streamCommand, Measure: measureDUTs}, nil
}

// streamCommand runs one command with the child on this terminal.
func streamCommand(action string, argv []string, dir string, environ []string) int {
	_, code := gaterun.Run(action, argv, dir, environ)
	return code
}

// measureDUTs runs the multi-DUT Docker benchmark in this process.
//
// PerfBinary is set rather than left to the runner's own ZE_PERF_BIN lookup:
// the chain has just built that file, and the program it reports with must be
// the program it built.
func measureDUTs(root, perfBinary string, args []string) int {
	runner := perfrunner.New(root, os.Stdout, os.Stderr)
	runner.PerfBinary = perfBinary
	return runner.RunCLI(args)
}

// perfBinary answers the host benchmark program's path in this checkout.
func (b *Bench) perfBinary() string { return filepath.Join(b.Root, "bin", "ze-perf") }

// buildArgv answers the compiler invocation that produces the host ze-perf.
// It is the retired `$(ZEBIN_PERF)` recipe, whose one command was
// `CGO_ENABLED=0 go build -tags 'ze_perf ze_bgp' -o bin/ze-perf ./cmd/ze`.
func (b *Bench) buildArgv() []string {
	return []string{"go", "build", "-tags", perfTags, "-o", b.perfBinary(), "./cmd/ze"}
}

// buildEnvOptions pins the host platform. An inherited GOOS or GOARCH names an
// appliance target, and a target-architecture ze-perf cannot run the report
// this chain asks it for.
func buildEnvOptions() gotoolchain.EnvOptions {
	return gotoolchain.EnvOptions{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

// measureArgs answers the runner's own command line for these DUTs. `--build`
// makes the Docker images, `--test` measures, and the retired ze-perf-bench
// recipe passed both.
func measureArgs(duts []string) []string {
	return append([]string{"--build", "--test"}, duts...)
}

// checkArgv answers the regression check over the ze DUT's committed history.
// That is the one series the release gate and .github/workflows/perf-nightly.yml
// both judge: the other DUTs are the comparison, not the subject.
func (b *Bench) checkArgv() []string {
	return []string{b.perfBinary(), "track", "--check", b.historyFile(zeDUT)}
}

// historyFile answers one DUT's committed NDJSON history.
func (b *Bench) historyFile(dut string) string {
	var tb textbuf.Buffer
	return filepath.Join(b.Root, filepath.FromSlash(historyDir), tb.Str(dut).Str(ndjsonSuffix).String())
}

// runner answers the nudge marker writer over the same checkout.
func (b *Bench) runner() *Runner { return New(b.Root) }

// fail answers a report for a step that did not succeed.
func fail(action string, code int, err error) (RunReport, int) {
	leaction.ReportError(err)
	return RunReport{Action: action, Error: err.Error(), Writes: true, Code: code}, code
}

// Run builds the benchmark program, measures the named DUTs, and records the
// marker that clears the nudge. An empty list measures every DUT.
func (b *Bench) Run(duts []string) (RunReport, int) {
	if err := validateDUTs(duts); err != nil {
		return fail(runVerb, 1, err)
	}
	if code := b.buildPerf(); code != 0 {
		return fail(runVerb, code, errBuild)
	}
	gaterun.Announce(measureAction)
	if code := b.Measure(b.Root, b.perfBinary(), measureArgs(duts)); code != 0 {
		return fail(runVerb, code, errMeasure)
	}
	marker, code := b.runner().Record()
	if code != 0 {
		return fail(runVerb, code, markerError(marker))
	}
	return RunReport{Action: runVerb, Benchmarked: duts, Recorded: marker.Recorded, Writes: true}, 0
}

// HistoryRecord appends every result of the last measurement to its DUT's
// committed history, then records the marker.
func (b *Bench) HistoryRecord() (RunReport, int) {
	results, err := b.results()
	if err != nil {
		return fail(historyVerb, 1, err)
	}
	appended, err := b.appendAll(results)
	if err != nil {
		return fail(historyVerb, 1, err)
	}
	marker, code := b.runner().Record()
	if code != 0 {
		return fail(historyVerb, code, markerError(marker))
	}
	return RunReport{Action: historyVerb, Appended: appended, Recorded: marker.Recorded, Writes: true}, 0
}

// EvidenceRecord measures the ze DUT, appends its result to the committed
// history, and fails when that history shows a regression. It is the release
// evidence gate the retired ze-evidence-perf-record target carried.
func (b *Bench) EvidenceRecord() (RunReport, int) {
	if report, code := b.Run([]string{zeDUT}); code != 0 {
		report.Action = evidenceVerb
		return report, code
	}
	appended, err := b.appendAll([]string{b.resultFile(zeDUT)})
	if err != nil {
		return fail(evidenceVerb, 1, err)
	}
	code := b.Command(checkAction, b.checkArgv(), b.Root, b.Toolchain.Environment(gotoolchain.EnvOptions{}))
	if code != 0 {
		return fail(evidenceVerb, code, errRegression)
	}
	marker, markerCode := b.runner().Record()
	if markerCode != 0 {
		return fail(evidenceVerb, markerCode, markerError(marker))
	}
	return RunReport{
		Action:      evidenceVerb,
		Benchmarked: []string{zeDUT},
		Appended:    appended,
		Checked:     b.historyFile(zeDUT),
		Recorded:    marker.Recorded,
		Writes:      true,
	}, 0
}

// buildPerf compiles the host benchmark program.
func (b *Bench) buildPerf() int {
	options := buildEnvOptions()
	return b.Command(buildAction, b.buildArgv(), b.Root, b.Toolchain.Environment(options))
}

// resultFile answers one DUT's result from the last measurement.
func (b *Bench) resultFile(dut string) string {
	var tb textbuf.Buffer
	return filepath.Join(b.Root, filepath.FromSlash(resultsDir), tb.Str(dut).Str(jsonSuffix).String())
}

// results answers every result file the last measurement wrote, sorted.
//
// An empty directory is an error rather than an empty success: a caller asked
// for results to be recorded, and recording none of them is the failure this
// verb exists to report.
func (b *Bench) results() ([]string, error) {
	dir := filepath.Join(b.Root, filepath.FromSlash(resultsDir))
	found, err := filepath.Glob(filepath.Join(dir, "*"+jsonSuffix))
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no benchmark result in %s: run `le perf-bench run` first", dir)
	}
	sort.Strings(found)
	return found, nil
}

// appendAll adds each result to its DUT's history and answers the history
// files it wrote.
func (b *Bench) appendAll(results []string) ([]string, error) {
	written := make([]string, 0, len(results))
	for _, result := range results {
		history, err := b.appendResult(result)
		if err != nil {
			return nil, err
		}
		written = append(written, history)
	}
	return written, nil
}

// appendResult adds one result to its DUT's history as a single NDJSON line,
// and answers the history file it wrote.
//
// The result is decoded before it is appended, so a truncated or foreign file
// fails here instead of making the whole history unreadable to the regression
// check. The bytes appended are the file's own, compacted, so nothing the
// result carries is dropped on the way in.
func (b *Bench) appendResult(result string) (string, error) {
	raw, err := os.ReadFile(result) // #nosec G304 -- a benchmark output this chain just wrote
	if err != nil {
		return "", err
	}
	var decoded perf.Result
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("%s is not a benchmark result: %w", result, err)
	}
	var line bytes.Buffer
	if err := json.Compact(&line, raw); err != nil {
		return "", fmt.Errorf("%s: %w", result, err)
	}
	line.WriteByte('\n')

	history := b.historyFile(strings.TrimSuffix(filepath.Base(result), jsonSuffix))
	if err := os.MkdirAll(filepath.Dir(history), 0o750); err != nil {
		return "", err
	}
	file, err := os.OpenFile(history, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- a history file this area owns
	if err != nil {
		return "", err
	}
	if _, err := file.Write(line.Bytes()); err != nil {
		_ = file.Close()
		return "", err
	}
	return history, file.Close()
}

// validateDUTs refuses a name the runner does not know, before the chain spends
// a compile and a Docker image build on it.
func validateDUTs(duts []string) error {
	known := perfrunner.DUTs()
	names := make([]string, 0, len(known))
	for _, dut := range known {
		names = append(names, dut.Name)
	}
	for _, name := range duts {
		if !slices.Contains(names, name) {
			var tb textbuf.Buffer
			return fmt.Errorf("unknown DUT %q; use one of: %s", name, tb.Join(names, ", ").String())
		}
	}
	return nil
}

// splitDUTs reads the dut keyword's value, which names one DUT or several
// separated by spaces. That is the list the retired PERF_DUT variable carried.
func splitDUTs(value string) []string {
	return strings.Fields(value)
}
