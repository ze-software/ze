// Design: docs/contributing/feature-maturity.md -- the recorded green run Supported requires
// Related: runrecord.go -- the record this writes and the check reads
// Related: check.go -- the S1 criterion that consumes it
//
// The writer records only what it watched pass. It runs every real-path test
// item of one feature through the repository's own runners, and every Interop
// entry that counts toward a level through its suite's own runner (the
// catalog's RunScenario). It refuses the whole record when one item fails,
// when the output does not show that item passing, or when the item's file or
// scenario directory changed while it ran, and writes nothing in each of those
// cases. A run that selected no test exits 0 under `go test -run`, so the exit
// code alone is never the evidence: the item's own PASS line is, and for a
// scenario the suite report's own result for that one scenario.

package feature

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// itemRunDeadline bounds one item's run. An expired deadline is an error, never
// a red and never a pass: an unmeasured answer is not recorded.
const itemRunDeadline = 20 * time.Minute

// scenarioRunDeadline bounds one interop scenario's run, image builds
// included. An expired deadline is an error, never a red and never a pass.
const scenarioRunDeadline = 2 * time.Hour

// excerptOctetsMax bounds the run output quoted in a refusal.
const excerptOctetsMax = 4000

// observation is one run of one real-path test item.
type observation struct {
	exited bool // the command exited 0
	output string
}

// runItem runs one real-path test item and answers what it observed. The error
// is for a run that could not be made or did not finish.
type runItem func(item string) (observation, error)

// runScenario runs one Interop entry `<suite>/<scenario>` and answers its
// suite's report. The error is for a run that could not be made or did not
// finish.
type runScenario func(item string) (interoplab.SuiteReport, error)

// runners is what record-run runs items with: one runner per item kind.
type runners struct {
	item     runItem
	scenario runScenario
}

// RecordRun runs every real-path test item and every counted Interop entry of
// feature id and, only when every one was observed passing, writes
// features/runs/<id>.json.
func RecordRun(tree, id string) (RunRecord, error) {
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return RunRecord{}, err
	}
	commit, err := headCommit(tree)
	if err != nil {
		return RunRecord{}, err
	}
	runner := &repoRunner{tree: tree, toolchain: toolchain}
	defer runner.release()

	return recordRun(tree, id, runners{item: runner.run, scenario: catalogScenarioRunner(tree)}, commit,
		time.Now().UTC().Format(time.DateOnly))
}

// recordRun is RecordRun with its runner, commit and date passed in, so a test
// can drive every refusal without a toolchain.
func recordRun(tree, id string, run runners, commit, date string) (RunRecord, error) {
	declaration, err := declarationNamed(tree, id)
	if err != nil {
		return RunRecord{}, err
	}
	in := &evidence{tree: tree, scenarios: map[string]map[string]string{}, catalogErrors: map[string]string{}}
	scenarios := in.interopToRun(&declaration)
	if len(declaration.RealPathTests) == 0 && len(scenarios) == 0 {
		return RunRecord{}, errors.New("feature " + id + " lists no real-path test and no counted interop " +
			"scenario, so there is nothing to run")
	}
	record := RunRecord{Feature: id, Runs: make([]TestRun, 0, len(declaration.RealPathTests))}
	for _, item := range declaration.RealPathTests {
		file, _, _ := strings.Cut(item, goTestSeparator)
		before, err := blobID(tree, file)
		if err != nil {
			return RunRecord{}, err
		}
		seen, err := run.item(item)
		if err != nil {
			return RunRecord{}, errors.New(item + ": " + err.Error() + "; nothing recorded")
		}
		if problem := observedPass(item, seen); problem != "" {
			return RunRecord{}, errors.New(item + ": " + problem + "; nothing recorded:\n" + excerpt(seen.output))
		}
		after, err := blobID(tree, file)
		if err != nil {
			return RunRecord{}, err
		}
		if after != before {
			return RunRecord{}, errors.New(item + ": " + file + " changed while it ran, so the pass belongs " +
				"to neither content; nothing recorded")
		}
		record.Runs = append(record.Runs, TestRun{Test: item, TestBlob: before, Commit: commit, Date: date,
			Result: runResultPass})
	}
	for _, item := range scenarios {
		scenarioRun, err := recordScenario(in, item, run.scenario)
		if err != nil {
			return RunRecord{}, err
		}
		scenarioRun.Commit, scenarioRun.Date = commit, date
		record.Interop = append(record.Interop, scenarioRun)
	}
	if err := writeRunRecord(tree, &record); err != nil {
		return RunRecord{}, err
	}
	return record, nil
}

// interopToRun answers the Interop entries a record must hold a run of: every
// counted (non-stub) Interop entry, and every extra criterion pointer that
// names an interop entry, each once, in declaration order.
func (in *evidence) interopToRun(d *Declaration) []string {
	scenarios := countedInterop(d)
	for _, extra := range d.Extra {
		if extra.Pointer == "" {
			continue
		}
		if !in.isInteropPointer(extra.Pointer) {
			continue
		}
		if slices.Contains(scenarios, extra.Pointer) {
			continue
		}
		scenarios = append(scenarios, extra.Pointer)
	}
	return scenarios
}

// recordScenario runs the Interop entry item once and answers its run, or why
// it is not recorded: an entry that does not resolve, a run not observed
// passing, or a scenario directory that changed while it ran.
func recordScenario(in *evidence, item string, run runScenario) (ScenarioRun, error) {
	if problem := in.interopItem(item); problem != "" {
		return ScenarioRun{}, errors.New(problem + "; nothing recorded")
	}
	suite, scenario, _ := strings.Cut(item, "/")
	names, _ := in.catalog(suite)
	directory := names[scenario]
	before, err := scenarioTreeID(in.tree, directory)
	if err != nil {
		return ScenarioRun{}, err
	}
	report, err := run(item)
	if err != nil {
		return ScenarioRun{}, errors.New(item + ": " + err.Error() + "; nothing recorded")
	}
	if problem := observedScenarioPass(scenario, &report); problem != "" {
		return ScenarioRun{}, errors.New(item + ": " + problem + "; nothing recorded:\n" + excerpt(report.Text()))
	}
	after, err := scenarioTreeID(in.tree, directory)
	if err != nil {
		return ScenarioRun{}, err
	}
	if after != before {
		return ScenarioRun{}, errors.New(item + ": " + directory + " changed while it ran, so the pass " +
			"belongs to neither content; nothing recorded")
	}
	return ScenarioRun{Scenario: item, ScenarioTree: before, Result: runResultPass}, nil
}

// observedScenarioPass answers why report is not a pass of exactly scenario,
// or "". A suite run whose selector matched another scenario, or more than
// one, did not prove this one passed.
func observedScenarioPass(scenario string, report *interoplab.SuiteReport) string {
	if report.SetupError != "" {
		return "the suite did not set up: " + report.SetupError
	}
	if report.Code != 0 {
		return "the run failed"
	}
	if len(report.Scenarios) != 1 {
		return "the run reported " + strconv.Itoa(len(report.Scenarios)) + " scenarios, not exactly " + scenario
	}
	result := &report.Scenarios[0]
	if result.Name != scenario {
		return "the run reported scenario '" + result.Name + "', not " + scenario
	}
	if !result.Passed {
		return "the run reported " + scenario + " failed: " + result.Error
	}
	return ""
}

// catalogScenarioRunner runs an Interop entry through its suite's registered
// RunScenario, the same runner `./le test integration` and
// `./le test deployment` drive.
func catalogScenarioRunner(tree string) runScenario {
	return func(item string) (interoplab.SuiteReport, error) {
		suite, scenario, _ := strings.Cut(item, "/")
		catalog, registered := interoplab.CatalogNamed(suite)
		if !registered {
			return interoplab.SuiteReport{}, errors.New("no interop suite '" + suite + "' is registered")
		}
		ctx, cancel := context.WithTimeout(context.Background(), scenarioRunDeadline)
		defer cancel()

		report := catalog.RunScenario(ctx, tree, scenario)
		if ctx.Err() != nil {
			return interoplab.SuiteReport{}, errors.New("did not finish inside " + scenarioRunDeadline.String() +
				", so whether it passes is unknown")
		}
		return report, nil
	}
}

// declarationNamed answers the declaration whose id is id, refusing one that
// does not parse: a record for a declaration the check refuses proves nothing.
func declarationNamed(tree, id string) (Declaration, error) {
	declarations, problems, err := Load(tree)
	if err != nil {
		return Declaration{}, err
	}
	for _, problem := range problems {
		if strings.Contains(problem.Error(), declarationDir+"/"+id+".md") {
			return Declaration{}, problem
		}
	}
	for i := range declarations {
		if declarations[i].ID == id {
			return declarations[i], nil
		}
	}
	return Declaration{}, errors.New("no feature declaration " + declarationDir + "/" + id + ".md")
}

// observedPass answers why seen is not a pass of item, or "".
func observedPass(item string, seen observation) string {
	if !seen.exited {
		return "the run failed"
	}
	file, function, named := strings.Cut(item, goTestSeparator)
	if named {
		// `go test -v` prints this line for a test that ran and passed; a -run
		// pattern that matched nothing exits 0 and prints none.
		if strings.Contains(seen.output, "--- PASS: "+function+" (") {
			return ""
		}
		return "the output shows no '--- PASS: " + function + "' line, so the run did not prove it passed"
	}
	runner, problem := runnerOf(file)
	if problem != "" {
		return problem
	}
	for line := range strings.Lines(seen.output) {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[len(fields)-1] != runner.selector {
			continue
		}
		if fields[len(fields)-3] == "PASS" {
			return ""
		}
	}
	return "the output shows no PASS line for " + runner.selector + ", so the run did not prove it passed"
}

func writeRunRecord(tree string, record *RunRecord) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	full := filepath.Join(tree, filepath.FromSlash(runRecordRel(record.Feature)))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	return os.WriteFile(full, append(raw, '\n'), 0o600)
}

func excerpt(output string) string {
	if len(output) <= excerptOctetsMax {
		return output
	}
	return output[len(output)-excerptOctetsMax:]
}

// repoRunner runs an item through the runner that owns its kind: `go test` for
// a Go test, the runner runnerOf answers for a .ci or .et. The isolated binary
// set a functional test runs against is built once, on the first one, and
// released by release.
// Not safe for concurrent use.
type repoRunner struct {
	tree      string
	toolchain gotoolchain.Toolchain
	set       testfunctional.BinarySet
	prepared  bool
}

func (r *repoRunner) run(item string) (observation, error) {
	file, function, named := strings.Cut(item, goTestSeparator)
	if named {
		var tb textbuf.Buffer
		argv := r.toolchain.GoTest(gotoolchain.TestOptions{}, "-run", tb.Byte('^').Str(function).Byte('$').String(),
			"-count=1", "-v", "./"+path.Dir(file))
		return r.exec(argv, r.toolchain.Environment(gotoolchain.EnvOptions{Test: true, Procs: true}))
	}
	runner, problem := runnerOf(file)
	if problem != "" {
		return observation{}, errors.New(problem)
	}
	if !r.prepared {
		set, err := testfunctional.Prepare(r.toolchain, "feature-record-run")
		if err != nil {
			return observation{}, errors.New("cannot build the isolated binaries a functional test runs against: " + err.Error())
		}
		r.set, r.prepared = set, true
	}
	argv := runner.argv(filepath.Join(r.set.Dir, testfunctional.LE))
	return r.exec(argv, r.set.Environment(r.toolchain))
}

func (r *repoRunner) release() {
	if r.prepared {
		testfunctional.Release(r.set)
	}
}

func (r *repoRunner) exec(argv, environ []string) (observation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), itemRunDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv is built from a declaration item that passed the check's path resolution
	cmd.Dir = r.tree
	cmd.Env = environ
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return observation{}, errors.New("did not finish inside " + itemRunDeadline.String() +
			", so whether it passes is unknown")
	}
	return observation{exited: runErr == nil, output: out.String()}, nil
}

// headCommit answers HEAD, which a record carries for a reader.
func headCommit(tree string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
	cmd.Dir = tree
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("cannot read HEAD: " + err.Error())
	}
	return strings.TrimSpace(string(out)), nil
}
