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
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/bgp"
	leaction "github.com/ze-software/ze/internal/le/le/action"
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
	// kernelErr is what the Docker kernel check answers; kernelZe records the
	// ze each check probed with.
	kernelErr error
	kernelZe  []string
	// stageErr is what staging the ze DUT's binary answers; staged records
	// the checkout each staging built into, and stagedAtKernel how many
	// stagings had run when each kernel check started.
	stageErr       error
	staged         []string
	stagedAtKernel []int
}

// stage is the seam that builds the ze DUT's linux binary.
func (r *recorder) stage(root string) error {
	r.staged = append(r.staged, root)
	return r.stageErr
}

// stepActions names each recorded step, for a failure message that does not
// print a child environment.
func stepActions(steps []step) []string {
	actions := make([]string, 0, len(steps))
	for i := range steps {
		actions = append(actions, steps[i].Action)
	}
	return actions
}

// kernel is the Docker kernel check seam.
func (r *recorder) kernel(zePath string) error {
	r.steps = append(r.steps, step{Action: "kernel", Argv: []string{zePath}})
	r.kernelZe = append(r.kernelZe, zePath)
	r.stagedAtKernel = append(r.stagedAtKernel, len(r.staged))
	return r.kernelErr
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
		Kernel:    rec.kernel,
		Stage:     rec.stage,
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

// TestRunMeasuresThenRecords is the ze-perf-bench chain in order: the Docker
// kernel check, one runner call with both steps, the running le as the
// reporter, then the marker.
func TestRunMeasuresThenRecords(t *testing.T) {
	bench, rec := fixtureBench(t)

	report, code := bench.Run(bothSteps(), []string{"ze"})
	if code != 0 {
		t.Fatalf("Run answered %d: %s", code, report.Error)
	}
	if len(rec.steps) != 2 || rec.steps[0].Action != "kernel" || rec.steps[1].Action != "measure" {
		t.Fatalf("the chain ran %d steps, want the kernel check then the measurement: %v", len(rec.steps), stepActions(rec.steps))
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
	if len(rec.steps) != 3 || rec.steps[0].Action != "kernel" {
		t.Fatalf("the chain ran %d steps, want the kernel check first: %v", len(rec.steps), stepActions(rec.steps))
	}
	if !reflect.DeepEqual(rec.measured.DUTs, []string{zeDUT}) || rec.measured.Steps != bothSteps() {
		t.Fatalf("the gate measured %+v, want both steps over the ze DUT alone", rec.measured)
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

// VALIDATES: a run that starts the ze DUT checks the Docker daemon's kernel
// with the ze the DUT image carries (test/interop/ze-linux) before the DUT
// starts, and a refusal stops the run before any measurement. A run that
// starts no ze, a build alone or other DUTs only, does not check.
// PREVENTS: a benchmark of Ze on a Docker kernel lacking a feature Ze enrolls
// (owner D-4: every Docker run that runs Ze; a wrong kernel must not be
// possible).
func TestRunChecksTheDockerKernelBeforeTheZeDUT(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps perfrunner.Steps
		duts  []string
		check bool
	}{
		{"every DUT", bothSteps(), nil, true},
		{"ze named", perfrunner.Steps{Test: true}, []string{"bird", "ze"}, true},
		{"build alone", perfrunner.Steps{Build: true}, []string{"ze"}, false},
		{"no ze", bothSteps(), []string{"bird"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bench, rec := fixtureBench(t)
			bench.Run(tc.steps, tc.duts)
			if !tc.check {
				if len(rec.kernelZe) != 0 {
					t.Errorf("checked the kernel for a run that starts no ze: %v", rec.kernelZe)
				}
				return
			}
			want := filepath.Join(bench.Root, "test", "interop", "ze-linux")
			if len(rec.kernelZe) != 1 || rec.kernelZe[0] != want {
				t.Fatalf("kernel checks = %v, want one with %s", rec.kernelZe, want)
			}
			if rec.steps[0].Action != "kernel" {
				t.Errorf("first step = %q, want the kernel check before the measurement", rec.steps[0].Action)
			}
		})
	}

	t.Run("refusal", func(t *testing.T) {
		bench, rec := fixtureBench(t)
		rec.kernelErr = errors.New("the Docker daemon's kernel lacks CONFIG_XFRM_MIGRATE")
		report, code := bench.Run(bothSteps(), []string{"ze"})
		if code == 0 {
			t.Fatal("a refused kernel answered exit 0")
		}
		if !strings.Contains(report.Error, "CONFIG_XFRM_MIGRATE") {
			t.Errorf("report error %q does not carry the refusal", report.Error)
		}
		for _, done := range rec.steps {
			if done.Action == "measure" {
				t.Error("the DUTs were measured after the kernel refused")
			}
		}
	})
}

// VALIDATES: review round 2, finding 3. A run that starts the ze DUT builds the
// linux ze the DUT image carries before the kernel check probes with it, at the
// path the BGP lab declares for that binary, and a failed build stops the run
// before the check and the measurement. A run that starts no ze builds nothing.
// PREVENTS: the kernel check probing test/interop/ze-linux on a checkout where
// nothing produced it, so Docker bind-mounts a missing path and creates a
// root-owned directory in its place.
func TestRunStagesTheZeTheKernelCheckProbes(t *testing.T) {
	bench, rec := fixtureBench(t)
	if _, code := bench.Run(bothSteps(), []string{"ze"}); code != 0 {
		t.Fatalf("run exit %d", code)
	}
	if len(rec.staged) != 1 || rec.staged[0] != bench.Root {
		t.Fatalf("staged = %v, want one build into %s", rec.staged, bench.Root)
	}
	if len(rec.stagedAtKernel) != 1 || rec.stagedAtKernel[0] != 1 {
		t.Errorf("the kernel check ran after %v stagings, want after the one", rec.stagedAtKernel)
	}
	want := interoplab.StagedZePath(bench.Root, bgp.LabBinaries())
	if want == "" || len(rec.kernelZe) != 1 || rec.kernelZe[0] != want {
		t.Errorf("kernel checks = %v, want one with the staged %q", rec.kernelZe, want)
	}

	for _, tc := range []struct {
		name  string
		steps perfrunner.Steps
		duts  []string
	}{
		{"build alone", perfrunner.Steps{Build: true}, []string{"ze"}},
		{"no ze", bothSteps(), []string{"bird"}},
	} {
		bench, rec := fixtureBench(t)
		bench.Run(tc.steps, tc.duts)
		if len(rec.staged) != 0 {
			t.Errorf("%s: staged %v for a run that starts no ze", tc.name, rec.staged)
		}
	}

	bench, rec = fixtureBench(t)
	rec.stageErr = errors.New("cross-compiling ze for linux/arm64 failed")
	report, code := bench.Run(bothSteps(), []string{"ze"})
	if code == 0 {
		t.Fatal("a failed staging answered exit 0")
	}
	if !strings.Contains(report.Error, "cross-compiling ze") {
		t.Errorf("report error %q does not carry the staging failure", report.Error)
	}
	if len(rec.kernelZe) != 0 {
		t.Errorf("the kernel check ran after the staging failed: %v", rec.kernelZe)
	}
	for _, done := range rec.steps {
		if done.Action == "measure" {
			t.Error("the DUTs were measured after the staging failed")
		}
	}
}
