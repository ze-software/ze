// Design: docs/architecture/core-design.md -- the perf benchmark, as three actions
// Detail: ../../test/perfrunner/run.go -- the multi-DUT Docker run these verbs drive
// Overview: actions.go -- the table these three verbs are declared in
//
// bench.go holds the three verbs that EXECUTE a benchmark, apart from the nudge
// that only reads the checkout. Each one is a chain, and the chain is what the
// retired Make targets carried: measure the DUTs under Docker, append each
// result to the committed NDJSON history, and check the history for a
// regression.
//
// No standalone benchmark program is built. The runner cross-builds a linux le
// for the sender container, and the le running this command renders the report
// and runs the regression check as `le perf report` and `le perf track`.
package perf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
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
	// stepKeyword selects one step of a run: stepBuild or stepTest.
	stepKeyword = "step"
	// stepBuild builds the DUT images and measures nothing.
	stepBuild = "build"
	// stepTest measures with the images that exist.
	stepTest = "test"

	// leTagBase is the personality tag of the linux le the sender container
	// runs. Every feature gate follows it, as in the launcher's own build.
	leTagBase = "ze_le"

	// resultsDir is where the runner writes one <dut>.json per measured DUT.
	// It is a build output and .gitignore excludes it.
	resultsDir = "test/perf/results"

	// historyDir holds the committed NDJSON history the regression check reads.
	historyDir = "test/perf/history"

	// zeDUT is the device under test the release evidence gate measures.
	zeDUT = "ze"

	// measureAction and checkAction are the headings a person watching a run
	// reads. gaterun.Run announces the check; the measurement runs in this
	// process, so its heading is announced here.
	measureAction = "perf measure"
	checkAction   = "perf regression check"

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
	// Built says the run built the DUT images and stopped there.
	Built bool `json:"built,omitempty"`
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
	tb.Str("perf ").Str(r.Action).Str(": ")
	if r.Built {
		return tb.Str("built the DUT images; measured and recorded nothing\n").String()
	}
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
// exit code. The regression check goes through it.
type commandStep func(action string, argv []string, dir string, environ []string) int

// suite is one call of the multi-DUT runner: the steps, the DUTs, and the two
// programs the runner needs, the le that renders the report and the tags of
// the linux le it cross-builds for the sender container.
type suite struct {
	Root      string
	Self      string
	LinuxTags string
	Steps     perfrunner.Steps
	DUTs      []string
}

// measureStep runs the multi-DUT Docker benchmark and answers its exit code.
type measureStep func(run suite) int

// Bench is one benchmark chain over a checkout. The two process seams are
// fields so a package test pins what each verb runs without Docker, a compiler,
// or minutes of machine time. Self is the le running this command, which
// answers `perf report` and `perf track` for the chain.
type Bench struct {
	Root      string
	Self      string
	LinuxTags string
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
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	tags, err := repofeaturetags.DaemonBuildTags(root, leTagBase)
	if err != nil {
		return nil, err
	}
	return &Bench{
		Root:      root,
		Self:      self,
		LinuxTags: tags,
		Toolchain: toolchain,
		Command:   streamCommand,
		Measure:   measureDUTs,
	}, nil
}

// streamCommand runs one command with the child on this terminal.
func streamCommand(action string, argv []string, dir string, environ []string) int {
	_, code := gaterun.Run(action, argv, dir, environ)
	return code
}

// measureDUTs runs the multi-DUT Docker benchmark in this process. The report
// is rendered by the le running this command, so no host benchmark program is
// built or looked up.
func measureDUTs(run suite) int {
	runner := perfrunner.New(run.Root, os.Stdout, os.Stderr)
	runner.Reporter = []string{run.Self, area, reportVerb}
	runner.LinuxTags = run.LinuxTags
	return runner.Execute(run.Steps, run.DUTs)
}

// suite answers the runner call for these steps and DUTs over this checkout.
func (b *Bench) suite(steps perfrunner.Steps, duts []string) suite {
	return suite{Root: b.Root, Self: b.Self, LinuxTags: b.LinuxTags, Steps: steps, DUTs: duts}
}

// checkArgv answers the regression check over the ze DUT's committed history.
// That is the one series the release gate and .github/workflows/perf-nightly.yml
// both judge: the other DUTs are the comparison, not the subject.
func (b *Bench) checkArgv() []string {
	return []string{b.Self, area, trackVerb, "--check", b.historyFile(zeDUT)}
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

// Run runs the selected steps over the named DUTs. An empty list selects every
// DUT. A run that measured records the marker that clears the nudge; a run
// that only built the images records nothing, because nothing was measured.
func (b *Bench) Run(steps perfrunner.Steps, duts []string) (RunReport, int) {
	if err := validateDUTs(duts); err != nil {
		return fail(runVerb, 1, err)
	}
	gaterun.Announce(measureAction)
	if code := b.Measure(b.suite(steps, duts)); code != 0 {
		return fail(runVerb, code, errMeasure)
	}
	if !steps.Test {
		return RunReport{Action: runVerb, Built: true, Writes: true}, 0
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
	if report, code := b.Run(bothSteps(), []string{zeDUT}); code != 0 {
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

// bothSteps is a run with no step keyword: build the images, then measure.
func bothSteps() perfrunner.Steps { return perfrunner.Steps{Build: true, Test: true} }

// stepsOf reads the step keyword. No keyword runs both steps. Any value other
// than stepBuild or stepTest is refused, naming the value, because running
// both steps on a typo would spend minutes of Docker on work nobody asked for.
func stepsOf(args leaction.Arguments) (perfrunner.Steps, error) {
	if !args.Has(stepKeyword) {
		return bothSteps(), nil
	}
	value := args.One(stepKeyword)
	switch value {
	case stepBuild:
		return perfrunner.Steps{Build: true}, nil
	case stepTest:
		return perfrunner.Steps{Test: true}, nil
	}
	var tb textbuf.Buffer
	return perfrunner.Steps{}, errors.New(tb.Str("step ").Quoted(value).
		Str(" is not a step of perf run; use ").Str(stepBuild).Str(" or ").Str(stepTest).String())
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
		return nil, fmt.Errorf("no benchmark result in %s: run `le perf run` first", dir)
	}
	slices.Sort(found)
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
