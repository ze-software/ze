// Related: bench.go -- the three chains these tests drive from their entry points
//
// VALIDATES: the retired ze-perf-bench, ze-perf-history-record and
// ze-evidence-perf-record chains, as the argv of each step, the order the steps
// run in, and the history line each result becomes.
// PREVENTS: a benchmark action that compiles a target-architecture ze-perf, one
// that measures a DUT the runner does not know, and a history append that puts
// a file the regression check cannot read into the committed NDJSON.

package perfbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
)

const fixturePin = "go1.26.6"

// step records one call to a process seam, so a test reads the chain's order
// and each step's exact argv.
type step struct {
	Action  string
	Argv    []string
	Dir     string
	Environ []string
}

// recorder holds the chain a fake run produced.
type recorder struct {
	steps    []step
	measured []string
	perfBin  string
	buildRC  int
	checkRC  int
	measRC   int
}

// build is the compiler seam. The evidence chain sends its regression check
// through the same seam, so the recorder answers each with its own code.
func (r *recorder) build(action string, argv []string, dir string, environ []string) int {
	r.steps = append(r.steps, step{Action: action, Argv: argv, Dir: dir, Environ: environ})
	if action == checkAction {
		return r.checkRC
	}
	return r.buildRC
}

// measure is the Docker benchmark seam.
func (r *recorder) measure(root, perfBinary string, args []string) int {
	r.steps = append(r.steps, step{Action: "measure", Argv: args, Dir: root})
	r.measured = args
	r.perfBin = perfBinary
	return r.measRC
}

// fixtureBench answers a chain over a throwaway checkout with both process
// seams recorded.
func fixtureBench(t *testing.T) (*Bench, *recorder) {
	t.Helper()
	root := t.TempDir()
	rec := &recorder{}
	bench := &Bench{
		Root:      root,
		Toolchain: gotoolchain.Toolchain{Root: root, GoToolchain: fixturePin},
		Command:   rec.build,
		Measure:   rec.measure,
	}
	return bench, rec
}

// sampleResult is one benchmark result as the runner writes it, indented the
// way the ze-perf run subcommand emits a JSON document.
const sampleResult = `{
  "dut-name": "ze",
  "routes": 100000,
  "convergence-ms": 4200,
  "throughput-avg": 240000,
  "latency-p99-ms": 12
}`

// writeResult puts one result where the runner leaves it.
func writeResult(t *testing.T, bench *Bench, dut, body string) {
	t.Helper()
	path := bench.resultFile(dut)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("results directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("result: %v", err)
	}
}

// TestBuildArgvIsTheRetiredZePerfBuildRecipe pins the one command the retired
// $(ZEBIN_PERF) rule ran, and the host platform pin that keeps its output
// runnable on this machine.
func TestBuildArgvIsTheRetiredZePerfBuildRecipe(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "checkout")
	bench := &Bench{Root: root, Toolchain: gotoolchain.Toolchain{Root: root, GoToolchain: fixturePin}}

	want := []string{
		"go", "build",
		"-tags", "ze_perf ze_bgp",
		"-o", filepath.Join(root, "bin", "ze-perf"),
		"./cmd/ze",
	}
	if got := bench.buildArgv(); !reflect.DeepEqual(got, want) {
		t.Fatalf("build argv = %#v, want %#v", got, want)
	}

	wantEnvironment := []string{
		"GOCACHE=" + filepath.Join(root, "cache", "go-cache"),
		"GOLANGCI_LINT_CACHE=" + filepath.Join(root, "tmp", "golangci-lint-cache"),
		"CGO_ENABLED=0",
		"GOTOOLCHAIN=" + fixturePin,
		"GOOS=" + runtime.GOOS,
		"GOARCH=" + runtime.GOARCH,
	}
	if got := bench.Toolchain.Overrides(buildEnvOptions()); !reflect.DeepEqual(got, wantEnvironment) {
		t.Fatalf("build environment = %#v, want %#v", got, wantEnvironment)
	}
}

// TestMeasureArgsAskTheRunnerToBuildImagesAndTest pins the runner command line
// the retired ze-perf-bench recipe passed, with and without a DUT selection.
func TestMeasureArgsAskTheRunnerToBuildImagesAndTest(t *testing.T) {
	if got := measureArgs(nil); !reflect.DeepEqual(got, []string{"--build", "--test"}) {
		t.Fatalf("every-DUT args = %#v", got)
	}
	if got := measureArgs([]string{"ze", "bird"}); !reflect.DeepEqual(got, []string{"--build", "--test", "ze", "bird"}) {
		t.Fatalf("selected args = %#v", got)
	}
}

// TestCheckArgvReadsTheCommittedHistoryOfOneDUT pins the regression check the
// release evidence gate and the nightly workflow both run.
func TestCheckArgvReadsTheCommittedHistoryOfOneDUT(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "checkout")
	bench := &Bench{Root: root}
	want := []string{
		filepath.Join(root, "bin", "ze-perf"),
		"track", "--check",
		filepath.Join(root, "test", "perf", "history", "ze.ndjson"),
	}
	if got := bench.checkArgv(); !reflect.DeepEqual(got, want) {
		t.Fatalf("check argv = %#v, want %#v", got, want)
	}
}

// TestRunBuildsThenMeasuresThenRecords is the ze-perf-bench chain in order.
func TestRunBuildsThenMeasuresThenRecords(t *testing.T) {
	bench, rec := fixtureBench(t)

	report, code := bench.Run([]string{"ze"})
	if code != 0 {
		t.Fatalf("Run answered %d: %s", code, report.Error)
	}
	if len(rec.steps) != 2 {
		t.Fatalf("the chain ran %d steps: %#v", len(rec.steps), rec.steps)
	}
	if rec.steps[0].Action != buildAction || rec.steps[1].Action != "measure" {
		t.Fatalf("the chain ran %s then %s, want the build then the measurement",
			rec.steps[0].Action, rec.steps[1].Action)
	}
	if !reflect.DeepEqual(rec.measured, []string{"--build", "--test", "ze"}) {
		t.Fatalf("the runner was asked for %#v", rec.measured)
	}
	if rec.perfBin != bench.perfBinary() {
		t.Fatalf("the runner reports with %q, want the binary the chain built at %q", rec.perfBin, bench.perfBinary())
	}
	if report.Recorded == "" {
		t.Fatal("the run wrote no marker, so the nudge would still ask for a perf run")
	}
	if _, err := os.Stat(filepath.Join(bench.Root, filepath.FromSlash(MarkerPath))); err != nil {
		t.Fatalf("marker: %v", err)
	}
}

// TestRunStopsWhenTheBuildFails keeps a failed compile from reaching Docker.
func TestRunStopsWhenTheBuildFails(t *testing.T) {
	bench, rec := fixtureBench(t)
	rec.buildRC = 2

	report, code := bench.Run(nil)
	if code != 2 {
		t.Fatalf("Run answered %d, want the compiler's own 2", code)
	}
	if len(rec.steps) != 1 {
		t.Fatalf("the chain ran %d steps after a failed build", len(rec.steps))
	}
	if report.Error == "" || report.Recorded != "" {
		t.Fatalf("report = %+v, want the build failure and no marker", report)
	}
}

// TestRunRefusesADUTTheRunnerDoesNotKnow spends no compile on a typo.
func TestRunRefusesADUTTheRunnerDoesNotKnow(t *testing.T) {
	bench, rec := fixtureBench(t)

	report, code := bench.Run([]string{"zebra"})
	if code != 1 {
		t.Fatalf("Run answered %d, want 1", code)
	}
	if len(rec.steps) != 0 {
		t.Fatalf("the chain ran %d steps for an unknown DUT", len(rec.steps))
	}
	if !strings.Contains(report.Error, "zebra") || !strings.Contains(report.Error, "gobgp") {
		t.Fatalf("error = %q, want the name refused and the names accepted", report.Error)
	}
}

// TestHistoryRecordAppendsOneCompactLinePerResult is the ze-perf-history-record
// chain: every result of the last measurement joins its own DUT's history.
func TestHistoryRecordAppendsOneCompactLinePerResult(t *testing.T) {
	bench, _ := fixtureBench(t)
	writeResult(t, bench, "ze", sampleResult)
	writeResult(t, bench, "bird", sampleResult)

	report, code := bench.HistoryRecord()
	if code != 0 {
		t.Fatalf("HistoryRecord answered %d: %s", code, report.Error)
	}
	want := []string{bench.historyFile("bird"), bench.historyFile("ze")}
	if !reflect.DeepEqual(report.Appended, want) {
		t.Fatalf("appended %#v, want %#v", report.Appended, want)
	}
	raw, err := os.ReadFile(bench.historyFile("ze"))
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("the history holds %d lines, want one per result", len(lines))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("the appended line does not decode: %v", err)
	}
	if decoded["throughput-avg"] != float64(240000) {
		t.Fatalf("the line lost the measurement: %v", decoded)
	}

	// A second measurement extends the series rather than replacing it, which
	// is what the regression check compares against.
	if _, code := bench.HistoryRecord(); code != 0 {
		t.Fatalf("the second append answered %d", code)
	}
	raw, err = os.ReadFile(bench.historyFile("ze"))
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if got := strings.Count(string(raw), "\n"); got != 2 {
		t.Fatalf("the history holds %d lines after two runs, want 2", got)
	}
}

// TestHistoryRecordRefusesAnEmptyResultsDirectory keeps "nothing was measured"
// from reading as a recorded run.
func TestHistoryRecordRefusesAnEmptyResultsDirectory(t *testing.T) {
	bench, _ := fixtureBench(t)

	report, code := bench.HistoryRecord()
	if code != 1 {
		t.Fatalf("HistoryRecord answered %d, want 1", code)
	}
	if !strings.Contains(report.Error, resultsDir) {
		t.Fatalf("error = %q, want the directory it read named", report.Error)
	}
}

// TestHistoryRecordRefusesAFileThatIsNotAResult keeps the committed history
// readable by the regression check.
func TestHistoryRecordRefusesAFileThatIsNotAResult(t *testing.T) {
	bench, _ := fixtureBench(t)
	writeResult(t, bench, "ze", "{\"dut-name\": ")

	report, code := bench.HistoryRecord()
	if code != 1 {
		t.Fatalf("HistoryRecord answered %d, want 1", code)
	}
	if !strings.Contains(report.Error, "not a benchmark result") {
		t.Fatalf("error = %q, want the file refused as a result", report.Error)
	}
	if _, err := os.Stat(bench.historyFile("ze")); !os.IsNotExist(err) {
		t.Fatalf("the history was written from a truncated result: %v", err)
	}
}

// TestEvidenceRecordMeasuresZeAppendsAndChecks is the ze-evidence-perf-record
// chain, whose four steps are the release evidence this gate produces.
func TestEvidenceRecordMeasuresZeAppendsAndChecks(t *testing.T) {
	bench, rec := fixtureBench(t)
	// The runner writes the result; the fake measurement stands in for it.
	writeResult(t, bench, zeDUT, sampleResult)

	report, code := bench.EvidenceRecord()
	if code != 0 {
		t.Fatalf("EvidenceRecord answered %d: %s", code, report.Error)
	}
	if len(rec.steps) != 3 {
		t.Fatalf("the chain ran %d steps: %#v", len(rec.steps), rec.steps)
	}
	if !reflect.DeepEqual(rec.measured, []string{"--build", "--test", zeDUT}) {
		t.Fatalf("the gate measured %#v, want the ze DUT alone", rec.measured)
	}
	if rec.steps[2].Action != checkAction {
		t.Fatalf("the third step is %q, want the regression check", rec.steps[2].Action)
	}
	if !reflect.DeepEqual(rec.steps[2].Argv, bench.checkArgv()) {
		t.Fatalf("the check ran %#v, want %#v", rec.steps[2].Argv, bench.checkArgv())
	}
	if report.Checked != bench.historyFile(zeDUT) || report.Recorded == "" {
		t.Fatalf("report = %+v, want the history it checked and the marker it wrote", report)
	}
}

// TestEvidenceRecordFailsOnARegression is why the gate exists.
func TestEvidenceRecordFailsOnARegression(t *testing.T) {
	bench, rec := fixtureBench(t)
	rec.checkRC = 1
	writeResult(t, bench, zeDUT, sampleResult)

	report, code := bench.EvidenceRecord()
	if code != 1 {
		t.Fatalf("EvidenceRecord answered %d, want the check's own 1", code)
	}
	if !strings.Contains(report.Error, "regression") {
		t.Fatalf("error = %q, want the regression named", report.Error)
	}
}

// TestSplitDUTsCarriesTheRetiredPERFDUTList keeps `dut "ze bird"` measuring two
// DUTs, which is what PERF_DUT held.
func TestSplitDUTsCarriesTheRetiredPERFDUTList(t *testing.T) {
	if got := splitDUTs("  ze   bird "); !reflect.DeepEqual(got, []string{"ze", "bird"}) {
		t.Fatalf("splitDUTs = %#v", got)
	}
	if got := splitDUTs(""); len(got) != 0 {
		t.Fatalf("an absent keyword selects %#v, want every DUT", got)
	}
}

// TestActionTableDeclaresTheThreeBenchmarkVerbs pins the command surface the
// three retired targets are reached through.
func TestActionTableDeclaresTheThreeBenchmarkVerbs(t *testing.T) {
	rows := make(map[string]leaction.Row, len(Actions().Actions))
	for _, row := range Actions().Actions {
		rows[row.Verb] = row
	}
	for _, verb := range []string{runVerb, historyVerb, evidenceVerb} {
		row, ok := rows[verb]
		if !ok {
			t.Fatalf("the area declares no %q verb: %v", verb, rows)
		}
		if !row.Writes {
			t.Errorf("%q does not declare that it writes", verb)
		}
		if row.Why == "" {
			t.Errorf("%q renders a blank reason in the listing", verb)
		}
	}
	// The two verbs that read the checkout stay, because the nudge and its
	// marker are what every other tool in the repository calls this area for.
	for _, verb := range []string{"suggestion-report", recordVerb} {
		if _, ok := rows[verb]; !ok {
			t.Fatalf("the area lost its %q verb", verb)
		}
	}
}

// TestRunReportRendersEveryStepItPerformed keeps the answer readable as prose
// and as data.
func TestRunReportRendersEveryStepItPerformed(t *testing.T) {
	report := RunReport{
		Action:      evidenceVerb,
		Benchmarked: []string{"ze"},
		Appended:    []string{"test/perf/history/ze.ndjson"},
		Checked:     "test/perf/history/ze.ndjson",
		Recorded:    "abc123",
	}
	text := report.Text()
	for _, want := range []string{evidenceVerb, "ze", "no regression", "abc123"} {
		if !strings.Contains(text, want) {
			t.Errorf("the prose has no %q: %s", want, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Errorf("the prose does not end with a newline: %q", text)
	}
	// A failed run says nothing on stdout: the diagnosis is already on stderr,
	// and a piped document must not carry it.
	failed := RunReport{Action: runVerb, Error: "the Docker benchmark failed", Code: 1}
	if got := failed.Text(); got != "" {
		t.Errorf("a failed run renders %q on stdout, want nothing", got)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{`"action"`, `"benchmarked"`, `"appended"`, `"checked"`, `"recorded"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}
