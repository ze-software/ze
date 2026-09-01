// Design: docs/architecture/core-design.md -- observing the red a record rests on
// Overview: discriminate_action.go -- the record mode this observation serves
// Detail: carriers.go -- the carrier table that says what runs a tagged unit
//
// discriminate_observe.go runs ONE tagged unit, with the break and without it.
// One runner per carrier kind, and each one is the carrier's OWN runner: `go
// test` over the tagged function, ze-test over the one tagged `.ci`, and the
// native interop action over the one scenario the checker declares. Nothing
// here re-implements a runner the repository already has.
package rfc

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/functional"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/lepath"
)

// observationRunner runs one tagged unit, with and without a break.
//
// One runner per carrier kind, and each one is the carrier's OWN runner: the
// unit kind is `go test` over the tagged function, the functional kind is the
// suite `./le functional` runs, and the interop kind is the scenario
// `./le integration` runs. Nothing here re-implements a runner the repository
// already has.
type observationRunner struct {
	tree      string
	toolchain gotoolchain.Toolchain
	carrier   Carrier
	tag       Tag
	record    DiscriminationRecord
	// unitName is the Go function to select, empty for a file-scoped unit.
	unitName string
	// names is what the output must contain for a failure to be THIS unit's.
	names string
	// scenario is the interop scenario the checker drives, empty otherwise.
	scenario string
	// self is the le binary this process is, re-executed for a carrier whose
	// runner is a native action.
	self string
	// command is the last command this runner executed, recorded so the report
	// carries what was actually run rather than what one branch would have run.
	command string
}

// newObservationRunner resolves everything one observation needs before any of
// it runs, so a missing scenario or an unresolvable unit costs no `go test`.
func newObservationRunner(tree string, reader *sourceReader, index *scopeIndex, carrier Carrier,
	tag Tag, record DiscriminationRecord) (*observationRunner, error) {
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str("cannot resolve this le binary, which is what runs a ").
			Str("functional or interop carrier: ").Err(err))
	}
	runner := &observationRunner{tree: tree, toolchain: toolchain, carrier: carrier,
		tag: tag, record: record, self: self}
	_, runner.unitName, err = fingerprintKey(record.Unit, record.Source)
	if err != nil {
		return nil, err
	}
	runner.names = runner.unitName
	if runner.names == "" {
		// A file-scoped unit is named by its stem, which is both what ze-test
		// selects one .ci by and what its output prints.
		runner.names = strings.TrimSuffix(filepath.Base(tag.File), filepath.Ext(tag.File))
	}
	if carrier.Kind != kindInterop {
		return runner, nil
	}
	runner.scenario, err = interopScenarioOf(tree, reader, index, record)
	if err != nil {
		return nil, err
	}
	runner.names = runner.scenario
	return runner, nil
}

// interopScenarioOf answers the scenario one interop checker drives.
//
// The checker declares it as `const name = "<scenario>"`, and the answer is
// confirmed against test/interop/scenarios/, so what reaches the runner's
// environment is a directory this tree holds rather than authored text.
func interopScenarioOf(tree string, reader *sourceReader, index *scopeIndex,
	record DiscriminationRecord) (string, error) {
	var tb textbuf.Buffer
	rel, symbol, err := fingerprintKey(record.Unit, record.Source)
	if err != nil {
		return "", err
	}
	content := reader.read(rel)
	if content == nil || symbol == "" {
		return "", parseErr(tb.Str(record.Unit).
			Str(": an interop record names one checker FUNCTION, which is what declares the scenario"))
	}
	texts := index.funcTexts(*content, symbol)
	if len(texts) != 1 {
		return "", parseErr(tb.Str(rel).Str(" declares ").Int(int64(len(texts))).
			Str(" function(s) named ").Str(symbol))
	}
	match := interopScenarioRE.FindStringSubmatch(texts[0])
	if match == nil {
		return "", parseErr(tb.Str(record.Unit).Str(" declares no `const name = \"<scenario>\"`, ").
			Str("so which scenario to run cannot be read off the checker"))
	}
	if _, statErr := os.Stat(treePath(tree, interopScenarioRel, match[1])); statErr != nil {
		return "", parseErr(tb.Str(interopScenarioRel).Byte('/').Str(match[1]).
			Str(" is not a directory in this tree, so that scenario cannot be run"))
	}
	return match[1], nil
}

// requireCleanGreen refuses to break a unit that is already failing.
//
// A red that was there before the break proves nothing about the break, and it
// is the cheapest way to spend an hour on a scenario for no evidence. For a
// unit carrier the same run carries the coverage profile that answers AC-6.
func (o *observationRunner) requireCleanGreen() error {
	profile := ""
	if o.carrier.Kind == kindUnit && o.record.Route == RouteRevert {
		scratch, err := observationScratch(o.tree)
		if err != nil {
			return err
		}
		profile = filepath.Join(scratch, "cover.out")
	}
	passed, output, err := o.run("", profile)
	if err != nil {
		return err
	}
	if !passed {
		var tb textbuf.Buffer
		return parseErr(tb.Str(o.record.Unit).Str(" is already failing before any break is ").
			Str("applied, so no red this run produces could be attributed to one:\n").
			Str(excerpt(output)))
	}
	if profile == "" {
		return nil
	}
	return o.requireProducerReached(profile)
}

// requireProducerReached is AC-6: a producer the tagged unit never executes is
// the defect, not a proof of one.
//
// It is checked where a coverage profile exists, which is the unit carrier. For
// a functional or interop carrier the observed red IS the reachability
// evidence: a break the run never reaches cannot redden it, so a producer that
// is not executed leaves the run green and the record is refused by AC-11.
func (o *observationRunner) requireProducerReached(profile string) error {
	module, err := modulePath(o.tree)
	if err != nil {
		return err
	}
	return o.requireProducerReachedIn(module, profile)
}

// requireProducerReachedIn is the reachability judgement over one profile.
//
// The module path is a parameter rather than a read, so the judgement can be
// exercised over a fixture tree that is not a Go module.
func (o *observationRunner) requireProducerReachedIn(module, profile string) error {
	raw, err := os.ReadFile(profile) // #nosec G304 -- a path this run just wrote under the session scratch
	if err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str("cannot read the coverage profile the clean run wrote: ").Err(err))
	}
	covered, err := parseCoverProfile(module, string(raw))
	if err != nil {
		return err
	}
	rel, symbol, err := fingerprintKey(o.record.Producer, o.record.Source)
	if err != nil {
		return err
	}
	lines, held := producerLineSpan(o.tree, rel, symbol)
	if !held {
		var tb textbuf.Buffer
		return parseErr(tb.Str(o.record.Producer).Str(" does not resolve in this tree. An ").
			Str("unresolvable producer is the defect, never a proof of one"))
	}
	for line := lines.first; line <= lines.last; line++ {
		if covered.covers(rel, line) {
			return nil
		}
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(o.record.Unit).Str(" never executes ").Str(o.record.Producer).
		Str(": no statement of that function appears as covered in the profile the clean run ").
		Str("wrote. A claim that rests on code its own test never reaches is the over-claim ").
		Str("this gate exists to catch, not a proof of one (R-10)"))
}

// requireRed applies the break and demands a failure that NAMES the tagged unit.
//
// Naming is what makes the red this unit's. A build error, a sibling test and a
// flake each turn a run red, and none of them says the break was discriminated
// by the claim's own test.
func (o *observationRunner) requireRed(broken overlayFile) (ObservedRed, error) {
	if o.carrier.Kind == kindInterop {
		return o.requireRedInTree(broken)
	}
	overlay, err := writeOverlay(o.tree, broken)
	if err != nil {
		return ObservedRed{}, err
	}
	started := time.Now()
	passed, output, err := o.run(overlay, "")
	if err != nil {
		return ObservedRed{}, err
	}
	if err := o.judgeRed(passed, output); err != nil {
		return ObservedRed{}, err
	}
	return ObservedRed{Command: o.command,
		Seconds: int(time.Since(started).Seconds()), Red: excerpt(output)}, nil
}

// judgeRed answers whether one run's result is a red this unit produced.
//
// Two refusals, and the pair is the whole of AC-11. A run that stayed GREEN
// under the break records nothing, because the proof would then be a claim. A
// run that went red without naming the unit records nothing either: a build
// error, a sibling test and a flake each turn a run red, and none of them says
// the claim's own test discriminated the break.
func (o *observationRunner) judgeRed(passed bool, output string) error {
	var tb textbuf.Buffer
	if passed {
		return parseErr(tb.Str(o.record.Unit).Str(" stayed GREEN under the break ").
			Quoted(o.record.Break).Str(" applied to ").Str(o.record.Producer).
			Str(". Nothing is written: a proof this run did not observe is never recorded. ").
			Str("Either the test does not discriminate the claim, or the break does not engage it"))
	}
	if !o.attributes(output) {
		return parseErr(tb.Str("the run went red under the break, and its output ").
			Str("does not name ").Quoted(o.names).Str(", so the failure is not this unit's. ").
			Str("Nothing is written:\n").Str(excerpt(output)))
	}
	return nil
}

// attributes answers whether a failing run named the tagged unit.
//
// For a Go unit that is the `--- FAIL: <Name>` line `go test` prints, which is
// also where a killed mutant's attribution has to be read from: gomu declares
// Result.TestOutput and fills it nowhere (A-2, measured over 3,683 results).
func (o *observationRunner) attributes(output string) bool {
	if o.scenario != "" {
		// An interop run is SELECTED down to one scenario, by a name read off
		// the checker and confirmed against test/interop/scenarios/, so nothing
		// else ran and there is no second failure to confuse this one with. The
		// `./le integration` summary names the action rather than the scenario,
		// so there is nothing in the text to match on either. What makes the red
		// the break's is the clean run that passed minutes earlier over the same
		// lab, which requireCleanGreen has already demanded.
		return true
	}
	if o.unitName == "" {
		return strings.Contains(output, o.names)
	}
	var tb textbuf.Buffer
	return strings.Contains(output, tb.Str("--- FAIL: ").Str(o.unitName).String())
}

// argv answers the command that runs this tagged unit.
func (o *observationRunner) argv(overlay, profile string) []string {
	if o.carrier.Kind != kindUnit {
		return o.carrierArgv()
	}
	var tb textbuf.Buffer
	options := make([]string, 0, 10)
	options = append(options, "-run", tb.Byte('^').Str(o.unitName).Byte('$').String(), "-count=1")
	if overlay != "" {
		options = append(options, "-overlay", overlay)
	}
	if profile != "" {
		options = append(options, "-coverprofile", profile, "-coverpkg", o.coverPackages())
	}
	options = append(options, tb.Reset().Str("./").
		Str(filepath.ToSlash(filepath.Dir(o.tag.File))).String())
	return o.toolchain.GoTest(gotoolchain.TestOptions{}, options...)
}

// carrierArgv re-executes this le binary on the interop action the carrier names.
//
// The already-built binary rather than the `./le` script, because the script
// rebuilds itself from the tree and the tree is about to be compiled under an
// overlay. Re-running the action the carrier table NAMES is what keeps this
// from being a second interop runner.
func (o *observationRunner) carrierArgv() []string {
	protocol := strings.TrimPrefix(o.carrier.Name, "interop-")
	verb := "interop"
	if protocol != "bgp" {
		var tb textbuf.Buffer
		verb = tb.Str("interop-").Str(protocol).String()
	}
	return []string{o.self, "integration", verb}
}

// coverPackages answers the -coverpkg patterns one clean run instruments: the
// tagged unit's package and the producer's.
func (o *observationRunner) coverPackages() string {
	var tb textbuf.Buffer
	unit := tb.Str("./").Str(filepath.ToSlash(filepath.Dir(o.tag.File))).String()
	producer := tb.Reset().Str("./").
		Str(filepath.ToSlash(filepath.Dir(keyFile(o.record.Producer)))).String()
	if producer == unit {
		return unit
	}
	return tb.Reset().Str(unit).Byte(',').Str(producer).String()
}

// run executes one observation under its deadline.
func (o *observationRunner) run(overlay, profile string) (bool, string, error) {
	if o.carrier.Kind == kindFunctional {
		return o.runFunctional(overlay)
	}
	deadline := unitRunDeadline
	if o.carrier.Kind != kindUnit {
		deadline = carrierRunDeadline
	}
	return o.exec(deadline, o.argv(overlay, profile), o.environment(overlay), o.tree)
}

// exec runs one command under a deadline and answers whether it passed.
//
// A deadline that expires is an ERROR rather than a red, because "the break
// reddens this unit" and "I never found out" must never be the same answer
// (ai/rules/principles.md).
func (o *observationRunner) exec(deadline time.Duration, argv, environ []string,
	dir string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	o.command = strings.Join(argv, " ")
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // every argv value is derived from ScanTree, UnitAt and the carrier table
	cmd.Dir = dir
	cmd.Env = environ
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if ctx.Err() != nil {
		var tb textbuf.Buffer
		return false, out.String(), parseErr(tb.Str("the observation of ").Str(o.record.Unit).
			Str(" did not finish inside ").Str(deadline.String()).
			Str(", so whether the break reddens it is unknown, and an unmeasured answer is ").
			Str("never recorded as a proof"))
	}
	return runErr == nil, out.String(), nil
}

// runFunctional runs ONE `.ci`, not the suite that holds it.
//
// The suite is the wrong unit twice over. It is slow, and it is hostage to
// every other test in it: `./le functional parse` was already red in this
// checkout from another session's work, so a suite-wide run could never
// attribute a red to this one carrier. functional.Prepare builds the isolated
// set the suite runner builds, and ze-test takes one test's name in place of
// the suite's --all.
//
// The overlay reaches the compiler through GOFLAGS, which every Go build
// command reads. It is set on THIS process because Prepare compiles in-process,
// and restored straight after: the action runs one observation at a time and
// starts no goroutine of its own, so the window belongs to nobody else.
func (o *observationRunner) runFunctional(overlay string) (bool, string, error) {
	var tb textbuf.Buffer
	suite, held := functionalSuite(strings.TrimPrefix(o.carrier.Name, "functional-"))
	if !held {
		return false, "", parseErr(tb.Str(o.carrier.Name).
			Str(" names no suite `./le functional` runs, so this .ci has no runner"))
	}
	label := "discrimination-clean"
	if overlay != "" {
		label = "discrimination-broken"
		restore, err := setGoFlagsOverlay(overlay)
		if err != nil {
			return false, "", err
		}
		defer restore()
	}
	set, err := functional.Prepare(o.toolchain, label, false)
	if err != nil {
		return false, "", parseErr(tb.Str("cannot build the isolated binaries one .ci runs ").
			Str("against: ").Err(err))
	}
	defer functional.Release(set)

	argv := append([]string{filepath.Join(set.Dir, functional.ZeTest)}, suite...)
	argv = append(argv, o.names)
	return o.exec(carrierRunDeadline, argv, set.Environment(o.toolchain), o.tree)
}

// functionalSuite answers one suite's ze-test arguments with the all-tests
// selector removed, so a single named .ci takes its place.
func functionalSuite(name string) ([]string, bool) {
	for _, suite := range functional.Suites {
		if suite.Name != name {
			continue
		}
		argv := make([]string, 0, len(suite.Args))
		for _, arg := range suite.Args {
			if arg == functional.AllTests {
				continue
			}
			argv = append(argv, arg)
		}
		return argv, true
	}
	return nil, false
}

// setGoFlagsOverlay puts the overlay where every Go compile this process starts
// will read it, and answers the restore.
func setGoFlagsOverlay(overlay string) (func(), error) {
	const key = "GOFLAGS"
	previous, held := os.LookupEnv(key)
	var tb textbuf.Buffer
	if err := os.Setenv(key, tb.Str("-overlay=").Str(overlay).String()); err != nil {
		return nil, parseErr(tb.Reset().Str("cannot place the overlay in GOFLAGS: ").Err(err))
	}
	return func() {
		if held {
			os.Setenv(key, previous) //nolint:errcheck // restoring what was read a moment ago
			return
		}
		os.Unsetenv(key) //nolint:errcheck // restoring what was read a moment ago
	}, nil
}

// environment answers the environment one observation runs under.
//
// A unit run takes the overlay as a flag. A functional or interop run is a
// second process that compiles for itself, so the overlay travels in GOFLAGS,
// which every `go build` and `go test` inside it reads. Either way no file on
// disk is modified, which is what lets this run in a checkout several sessions
// share.
func (o *observationRunner) environment(overlay string) []string {
	if o.carrier.Kind == kindUnit {
		return o.toolchain.Environment(gotoolchain.EnvOptions{Procs: true})
	}
	environ := os.Environ()
	if overlay != "" {
		var tb textbuf.Buffer
		environ = append(environ, tb.Str("GOFLAGS=-overlay=").Str(overlay).String())
	}
	if o.scenario != "" {
		var tb textbuf.Buffer
		environ = append(environ, tb.Str("INTEROP_SCENARIO=").Str(o.scenario).String())
	}
	return environ
}

// writeOverlay writes the replacement file and the overlay that selects it.
//
// Go's own overlay, which is what gomu applies its mutants through: the
// compiler reads the replacement and the file on disk is never touched.
func writeOverlay(tree string, broken overlayFile) (string, error) {
	scratch, err := observationScratch(tree)
	if err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	replacement := filepath.Join(scratch, tb.Str("broken-").Str(filepath.Base(broken.rel)).String())
	if err := os.WriteFile(replacement, []byte(broken.content), 0o600); err != nil {
		return "", parseErr(tb.Reset().Str("cannot write the broken source: ").Err(err))
	}
	raw, err := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{treePath(tree, broken.rel): replacement}})
	if err != nil {
		return "", parseErr(tb.Reset().Str("cannot render the overlay: ").Err(err))
	}
	path := filepath.Join(scratch, "overlay.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", parseErr(tb.Reset().Str("cannot write the overlay: ").Err(err))
	}
	return path, nil
}

// observationScratch answers this session's directory for one observation.
func observationScratch(tree string) (string, error) {
	paths, err := lepath.ResolveSession(tree, true)
	if err != nil {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str("cannot resolve this session's scratch directory, which is ").
			Str("where a break is staged: ").Err(err))
	}
	directory := treePath(tree, paths.Scratch, "discrimination")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(relTo(tree, directory)).Str(": cannot create directory: ").Err(err))
	}
	return directory, nil
}

// producerLineSpan answers the first and last line of one function.
func producerLineSpan(tree, rel, symbol string) (lineRange, bool) {
	raw, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- a repo-relative path fingerprintKey validated
	if err != nil {
		return lineRange{}, false
	}
	content := string(raw)
	index := newScopeIndex()
	for _, found := range index.of(content) {
		if funcNameIn(content[found.begin:found.end]) != symbol {
			continue
		}
		first := strings.Count(content[:found.begin], "\n") + 1
		last := strings.Count(content[:found.end], "\n") + 1
		return lineRange{first: first, last: last}, true
	}
	return lineRange{}, false
}

// requireRedInTree is requireRed for the one carrier an overlay cannot reach.
//
// The BGP interop lab compiles ze INSIDE Docker, from the repository as the
// build context: internal/le/interoplab/docker.go builds the image with
// `-f Dockerfile.ze <context>`. A host-side overlay is a file that container
// never sees and a GOFLAGS value it never reads, so the only place a break can
// go is the working tree, which is what the house method in
// docs/contributing/testing.md has always said to do by hand.
//
// The producer is put back on every path and the restore is CONFIRMED byte for
// byte. Several sessions share this checkout, so a producer left broken is not
// this record's problem alone. A restore that did not take is reported as the
// failure it is, never folded into the record's own verdict.
func (o *observationRunner) requireRedInTree(broken overlayFile) (ObservedRed, error) {
	path := treePath(o.tree, broken.rel)
	original, err := os.ReadFile(path) // #nosec G304 -- a repo-relative path fingerprintKey validated
	if err != nil {
		var tb textbuf.Buffer
		return ObservedRed{}, parseErr(tb.Str(broken.rel).Str(": cannot read the producer before ").
			Str("breaking it, and a break with no way back is never applied: ").Err(err))
	}
	if err := os.WriteFile(path, []byte(broken.content), sourceMode); err != nil {
		var tb textbuf.Buffer
		return ObservedRed{}, parseErr(tb.Str(broken.rel).Str(": cannot apply the break: ").Err(err))
	}

	started := time.Now()
	passed, output, runErr := o.run("", "")
	if restoreErr := restoreSource(path, broken.rel, original); restoreErr != nil {
		return ObservedRed{}, restoreErr
	}
	if runErr != nil {
		return ObservedRed{}, runErr
	}
	if err := o.judgeRed(passed, output); err != nil {
		return ObservedRed{}, err
	}
	return ObservedRed{Command: o.command,
		Seconds: int(time.Since(started).Seconds()), Red: excerpt(output)}, nil
}

// sourceMode is the permission a Go source file carries in this checkout.
const sourceMode = 0o644

// restoreSource puts a broken producer back and CONFIRMS it went back.
//
// Reading the bytes again is the whole point: a write that reported success and
// left a short file would leave this checkout broken for every other session,
// and the next thing this run does is report a proof.
func restoreSource(path, rel string, original []byte) error {
	var tb textbuf.Buffer
	if err := os.WriteFile(path, original, sourceMode); err != nil {
		return parseErr(tb.Str(rel).Str(": THE PRODUCER IS STILL BROKEN. Putting it back failed: ").
			Err(err).Str(". Restore it from git before anything else"))
	}
	back, err := os.ReadFile(path) // #nosec G304 -- the path just written
	if err != nil || !bytes.Equal(back, original) {
		return parseErr(tb.Str(rel).Str(": THE PRODUCER MAY STILL BE BROKEN. It was put back and ").
			Str("does not read back byte for byte. Restore it from git before anything else"))
	}
	return nil
}

// excerptLines is how much of a run's output a refusal and a record carry.
//
// The tail, because that is where `go test` prints the failure and where a
// suite prints its summary. Enough to read, bounded so a record stays a record.
const excerptLines = 40

// excerpt answers the tail of one run's output.
func excerpt(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > excerptLines {
		lines = lines[len(lines)-excerptLines:]
	}
	return strings.Join(lines, "\n")
}
