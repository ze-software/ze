// Related: bench.go -- the three chains these tests drive from their entry points
//
// VALIDATES: the retired ze-perf-bench, ze-perf-history-record and
// ze-evidence-perf-record chains, as the runner call and argv of each step, the
// order the steps run in, and the history line each result becomes.
// PREVENTS: a benchmark action that looks for a host ze-perf, one that measures
// a DUT the runner does not know, and a history append that puts a file the
// regression check cannot read into the committed NDJSON.

package perf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/test/perfrunner"
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
	measured suite
	checkRC  int
	measRC   int
}

// command is the process seam the evidence chain sends its regression check
// through.
func (r *recorder) command(action string, argv []string, dir string, environ []string) int {
	r.steps = append(r.steps, step{Action: action, Argv: argv, Dir: dir, Environ: environ})
	return r.checkRC
}

// measure is the Docker benchmark seam.
func (r *recorder) measure(run suite) int {
	r.steps = append(r.steps, step{Action: "measure", Argv: run.DUTs, Dir: run.Root})
	r.measured = run
	return r.measRC
}

// fixtureSelf and fixtureTags stand in for the running le and the tags of the
// linux le the runner cross-builds.
const (
	fixtureSelf = "/repo/bin/le"
	fixtureTags = "ze_le ze_bgp"
)

// fixtureBench answers a chain over a throwaway checkout with both process
// seams recorded.
func fixtureBench(t *testing.T) (*Bench, *recorder) {
	t.Helper()
	root := t.TempDir()
	rec := &recorder{}
	bench := &Bench{
		Root:      root,
		Self:      fixtureSelf,
		LinuxTags: fixtureTags,
		Toolchain: gotoolchain.Toolchain{Root: root, GoToolchain: fixturePin},
		Command:   rec.command,
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

// TestCheckArgvReadsTheCommittedHistoryOfOneDUT pins the regression check the
// evidence gate runs: the le running the chain, as `perf track --check`.
func TestCheckArgvReadsTheCommittedHistoryOfOneDUT(t *testing.T) {
	bench, _ := fixtureBench(t)
	want := []string{fixtureSelf, "perf", "track", "--check",
		filepath.Join(bench.Root, "test", "perf", "history", "ze.ndjson")}
	if got := bench.checkArgv(); !reflect.DeepEqual(got, want) {
		t.Fatalf("checkArgv = %#v, want %#v", got, want)
	}
}

// TestRunMeasuresThenRecords is the ze-perf-bench chain in order: one runner
// call with both steps, the running le as the reporter, then the marker.
func TestRunMeasuresThenRecords(t *testing.T) {
	bench, rec := fixtureBench(t)

	report, code := bench.Run(bothSteps(), []string{"ze"})
	if code != 0 {
		t.Fatalf("Run answered %d: %s", code, report.Error)
	}
	if len(rec.steps) != 1 || rec.steps[0].Action != "measure" {
		t.Fatalf("the chain ran %#v, want the measurement alone", rec.steps)
	}
	want := suite{Root: bench.Root, Self: fixtureSelf, LinuxTags: fixtureTags, Steps: bothSteps(), DUTs: []string{"ze"}}
	if !reflect.DeepEqual(rec.measured, want) {
		t.Fatalf("the runner was asked for %#v, want %#v", rec.measured, want)
	}
	if report.Recorded == "" {
		t.Fatal("the run wrote no marker, so the nudge would still ask for a perf run")
	}
	if _, err := os.Stat(filepath.Join(bench.Root, filepath.FromSlash(MarkerPath))); err != nil {
		t.Fatalf("marker: %v", err)
	}
}

// TestRunStopsWhenTheMeasurementFails keeps a failed run, the linux le build
// included, from writing the marker that clears the nudge.
func TestRunStopsWhenTheMeasurementFails(t *testing.T) {
	bench, rec := fixtureBench(t)
	rec.measRC = 2

	report, code := bench.Run(bothSteps(), nil)
	if code != 2 {
		t.Fatalf("Run answered %d, want the runner's own 2", code)
	}
	if report.Error == "" || report.Recorded != "" {
		t.Fatalf("report = %+v, want the failure and no marker", report)
	}
	if _, err := os.Stat(filepath.Join(bench.Root, filepath.FromSlash(MarkerPath))); err == nil {
		t.Fatal("a failed run wrote the marker")
	}
}

// TestRunBuildOnlyRecordsNothing is `perf run step build`: the images are
// built, nothing is measured, so no marker is written.
func TestRunBuildOnlyRecordsNothing(t *testing.T) {
	bench, rec := fixtureBench(t)

	report, code := bench.Run(perfrunner.Steps{Build: true}, []string{"ze"})
	if code != 0 {
		t.Fatalf("Run answered %d: %s", code, report.Error)
	}
	if !report.Built || report.Recorded != "" {
		t.Fatalf("report = %+v, want built and nothing recorded", report)
	}
	if rec.measured.Steps != (perfrunner.Steps{Build: true}) {
		t.Fatalf("the runner was asked for %+v, want the build step alone", rec.measured.Steps)
	}
	if _, err := os.Stat(filepath.Join(bench.Root, filepath.FromSlash(MarkerPath))); err == nil {
		t.Fatal("a build-only run wrote the marker")
	}
	if text := report.Text(); !strings.Contains(text, "built") {
		t.Fatalf("the prose does not say the images were built: %q", text)
	}
}

// TestRunRefusesADUTTheRunnerDoesNotKnow spends no compile on a typo.
func TestRunRefusesADUTTheRunnerDoesNotKnow(t *testing.T) {
	bench, rec := fixtureBench(t)

	report, code := bench.Run(bothSteps(), []string{"zebra"})
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
// chain, whose steps are the release evidence this gate produces.
func TestEvidenceRecordMeasuresZeAppendsAndChecks(t *testing.T) {
	bench, rec := fixtureBench(t)
	// The runner writes the result; the fake measurement stands in for it.
	writeResult(t, bench, zeDUT, sampleResult)

	report, code := bench.EvidenceRecord()
	if code != 0 {
		t.Fatalf("EvidenceRecord answered %d: %s", code, report.Error)
	}
	if len(rec.steps) != 2 {
		t.Fatalf("the chain ran %d steps: %#v", len(rec.steps), rec.steps)
	}
	if !reflect.DeepEqual(rec.measured.DUTs, []string{zeDUT}) || rec.measured.Steps != bothSteps() {
		t.Fatalf("the gate measured %+v, want both steps over the ze DUT alone", rec.measured)
	}
	if rec.steps[1].Action != checkAction {
		t.Fatalf("the second step is %q, want the regression check", rec.steps[1].Action)
	}
	if !reflect.DeepEqual(rec.steps[1].Argv, bench.checkArgv()) {
		t.Fatalf("the check ran %#v, want %#v", rec.steps[1].Argv, bench.checkArgv())
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
	for _, verb := range []string{suggestVerb, recordVerb} {
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
