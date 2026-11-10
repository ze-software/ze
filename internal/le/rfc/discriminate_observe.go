// Design: docs/architecture/core-design.md -- observing the red a record rests on
// Overview: discriminate_action.go -- the record mode this observation serves
// Detail: carriers.go -- the carrier table that says what runs a tagged unit
//
// discriminate_observe.go runs ONE tagged unit, with the break and without it.
// One runner per carrier kind, and each one is the carrier's OWN runner: `go
// test` over the tagged function, ze-test over the one tagged `.ci`, and the
// native interop action over the one scenario the checker declares. Nothing
// here re-implements a runner the repository already has: the overlay, the
// carriers' argv and environment, and the verdict readers are
// internal/le/test/overlay's, and this file adds what a record needs.
package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	testoverlay "github.com/ze-software/ze/internal/le/test/overlay"
	testrunner "github.com/ze-software/ze/internal/test/runner"
)

// discriminationArea is the session scratch subdirectory an observation
// stages its breaks and profiles in.
const discriminationArea = "discrimination"

// observationRunner runs one tagged unit, with and without a break.
//
// One runner per carrier kind, and each one is the carrier's OWN runner: the
// unit kind is `go test` over the tagged function, the functional kind is the
// suite `./le test functional` runs, and the interop kind is the scenario
// `./le test integration` runs. Nothing here re-implements a runner the repository
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
	// suite is the catalog of the interop suite whose package holds the
	// checker: its scenario directory, selector variable and action. Zero for
	// any other carrier kind.
	suite interoplab.Catalog
	// self is the le binary this process is, re-executed for a carrier whose
	// runner is a native action.
	self string
	// command is the last command this runner executed, recorded so the report
	// carries what was actually run rather than what one branch would have run.
	command string
	// guest is true when the selected kernel requests a compatible Linux
	// guest, the tagged file compiles only there, or the host skips the .ci.
	guest bool
	// kernel is the runtime kernel a guest run boots, empty for a host run.
	kernel string
	// root is true when the unit's test declares guestRootDirective, so its
	// guest run is elevated (guestRunArgv).
	root bool
}

// newObservationRunner resolves everything one observation needs before any of
// it runs, so a missing scenario or an unresolvable unit costs no `go test`.
func newObservationRunner(tree string, reader *sourceReader, index *scopeIndex, carrier Carrier,
	tag Tag, record *DiscriminationRecord, kernel string) (*observationRunner, error) {
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
		tag: tag, record: *record, self: self, kernel: kernel}
	_, runner.unitName, err = fingerprintKey(record.Unit, record.Source)
	if err != nil {
		return nil, err
	}
	if carrier.Kind == kindUnit {
		host, guest := placementContexts(strings.Fields(toolchain.TestTags()))
		runner.guest, err = unitNeedsGuest(&host, &guest, treePath(tree, tag.File), kernel != "")
		if err != nil {
			return nil, err
		}
		if runner.unitName != "" {
			runner.root, err = unitDeclaresGuestRoot(treePath(tree, tag.File), runner.unitName)
			if err != nil {
				return nil, err
			}
		}
	}
	if carrier.Kind == kindFunctional {
		runner.guest, err = functionalNeedsGuest(carrier, treePath(tree, tag.File), testrunner.NeedsGuest)
		if err != nil {
			return nil, err
		}
	}
	if err := requireGuestKernel(record.Unit, runner.guest, kernel); err != nil {
		return nil, err
	}
	if err := requireRootInGuest(record.Unit, runner.guest, runner.root); err != nil {
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
	runner.suite, err = interopSuiteOf(tag.File)
	if err != nil {
		return nil, err
	}
	runner.scenario, err = interopScenarioOf(tree, reader, index, record, runner.suite)
	if err != nil {
		return nil, err
	}
	runner.names = runner.scenario
	return runner, nil
}

// interopScenarioOf answers the scenario one interop checker drives.
//
// The checker declares it as `const name = "<scenario>"`, and the answer is
// confirmed against the ScenarioRoot of the suite whose package holds the
// checker, so what reaches the runner's environment is a directory this tree
// holds rather than authored text.
func interopScenarioOf(tree string, reader *sourceReader, index *scopeIndex,
	record *DiscriminationRecord, suite interoplab.Catalog) (string, error) {
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
	text, refusal := index.keyText(rel, *content, symbol)
	if refusal != "" {
		return "", parseErr(tb.Str(refusal))
	}
	match := interopScenarioRE.FindStringSubmatch(text)
	if match == nil {
		return "", parseErr(tb.Str(record.Unit).Str(" declares no `const name = \"<scenario>\"`, ").
			Str("so which scenario to run cannot be read off the checker"))
	}
	if _, statErr := os.Stat(treePath(tree, suite.ScenarioRoot, match[1])); statErr != nil {
		return "", parseErr(tb.Str(suite.ScenarioRoot).Byte('/').Str(match[1]).
			Str(" is not a directory in this tree, so the ").Str(suite.Suite).
			Str(" suite cannot run that scenario"))
	}
	return match[1], nil
}

// observeBreaks is the whole observation of one record's breaks: the clean
// run, the red of every break together, and for a pair each break alone.
//
// solos holds one overlay set per break, Producer's first, and is empty for a
// single-break record. A pair is recorded only when every break applied alone
// leaves the unit green: a break that reddens it alone is a single-break proof
// that the pair dresses up, and a break the unit never executes leaves the pair
// behaving as the others alone, which the same runs show green. The solo runs
// are what makes that hold on a carrier no coverage profile reaches.
func (o *observationRunner) observeBreaks(broken []testoverlay.File,
	solos [][]testoverlay.File) (ObservedRed, []SoloGreen, error) {
	if err := o.requireCleanGreen(); err != nil {
		return ObservedRed{}, nil, err
	}
	red, err := o.requireRed(broken...)
	if err != nil {
		return ObservedRed{}, nil, err
	}
	solo, err := o.requireSoloGreen(solos)
	if err != nil {
		return ObservedRed{}, nil, err
	}
	return red, solo, nil
}

// requireSoloGreen runs each break of a pair by itself and demands the unit
// stays green under every one, answering the green runs in break order.
func (o *observationRunner) requireSoloGreen(solos [][]testoverlay.File) ([]SoloGreen, error) {
	if len(solos) == 0 {
		return nil, nil
	}
	breaks := o.breaks()
	if len(solos) != len(breaks) {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str("BUG: ").Int(int64(len(solos))).Str(" solo break sets for ").
			Int(int64(len(breaks))).Str(" breaks"))
	}
	runs := make([]SoloGreen, 0, len(solos))
	for position, files := range solos {
		run, err := o.runAlone(breaks[position], files)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// runAlone applies one break of a pair by itself and answers its green run.
func (o *observationRunner) runAlone(alone PairedBreak, files []testoverlay.File) (SoloGreen, error) {
	overlay, err := testoverlay.Write(o.tree, discriminationArea, files...)
	if err != nil {
		return SoloGreen{}, err
	}
	started := time.Now()
	passed, output, err := o.run(overlay, "")
	if err != nil {
		return SoloGreen{}, err
	}
	var tb textbuf.Buffer
	if !passed {
		return SoloGreen{}, parseErr(tb.Str(o.record.Unit).Str(" went red under the break ").
			Quoted(alone.Break).Str(" in ").Str(alone.Producer).Str(" applied ALONE, so the pair ").
			Str("is not needed for the red. Nothing is written: record that break by itself, ").
			Str("or name the breaks the red really needs:\n").Str(testoverlay.Excerpt(output)))
	}
	if why := o.cleanRunUnobserved(output); why != "" {
		return SoloGreen{}, parseErr(tb.Str("the run of the break in ").Str(alone.Producer).
			Str(" alone observed nothing: ").Str(why))
	}
	return SoloGreen{Producer: alone.Producer, Break: alone.Break,
		Seconds: int(time.Since(started).Seconds()), Green: o.greenLine(output)}, nil
}

// breaks answers every break one broken run applies, Producer's first.
func (o *observationRunner) breaks() []PairedBreak {
	all := make([]PairedBreak, 0, 1+len(o.record.Paired))
	all = append(all, PairedBreak{Producer: o.record.Producer, Break: o.record.Break})
	return append(all, o.record.Paired...)
}

// greenLine answers the line of a green run's output a record keeps as the
// sight of it: the scenario's pass line for an interop run, the last line the
// runner printed otherwise.
func (o *observationRunner) greenLine(output string) string {
	if o.scenario != "" {
		if verdict, ran := testoverlay.ScenarioVerdict(output, o.scenario, testoverlay.VerdictPass); ran {
			return "interop: " + o.scenario + verdict
		}
	}
	last := ""
	for line := range strings.Lines(output) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			last = trimmed
		}
	}
	return last
}

// wantsProfile answers whether the clean run builds a coverage profile.
//
// A unit carrier on the revert route builds one, since a panic in a producer
// the unit never calls cannot redden it and could only be absent from the run.
// A record with paired breaks builds one on the mutant route too, so every
// paired producer is held to reachability. A single-break mutant record keeps
// its red as that evidence. A functional or interop carrier builds none: the
// unit is a second process, and its reds and greens are the evidence.
func (o *observationRunner) wantsProfile() bool {
	if o.carrier.Kind != kindUnit {
		return false
	}
	if o.record.Route == RouteRevert {
		return true
	}
	return len(o.record.Paired) > 0
}

// requireCleanGreen refuses to break a unit that is already failing.
//
// A red that was there before the break proves nothing about the break, and it
// is the cheapest way to spend an hour on a scenario for no evidence. Where
// wantsProfile holds, the same run carries the coverage profile that answers
// AC-6, in the Linux guest as on the host.
func (o *observationRunner) requireCleanGreen() error {
	profile := ""
	if o.wantsProfile() {
		scratch, err := testoverlay.Scratch(o.tree, discriminationArea)
		if err != nil {
			return err
		}
		// One profile per process: several agents of one session share this scratch
		// directory and run this command at once, and a fixed name let one run's
		// `mode: set` header land where another was about to read its blocks, which
		// refused a producer the unit does execute (plan/journal/concurrent-session-corruption.md).
		var name textbuf.Buffer
		profile = filepath.Join(scratch, name.Str("cover-").Int(int64(os.Getpid())).Str(".out").String())
	}
	passed, output, err := o.run("", profile)
	if err != nil {
		return err
	}
	if !passed {
		var tb textbuf.Buffer
		return parseErr(tb.Str(o.record.Unit).Str(" is already failing before any break is ").
			Str("applied, so no red this run produces could be attributed to one:\n").
			Str(testoverlay.Excerpt(output)))
	}
	if why := o.cleanRunUnobserved(output); why != "" {
		var tb textbuf.Buffer
		return parseErr(tb.Str(why))
	}
	if profile == "" {
		return nil
	}
	return o.requireProducerReached(profile)
}

// requireProducerReached is AC-6: a producer the tagged unit never executes is
// the defect, not a proof of one.
//
// It is checked where a coverage profile exists, which is a unit carrier on the
// revert route or with paired breaks (wantsProfile). For a single break on any
// other carrier the observed red IS the reachability evidence: a break the run
// never reaches cannot redden it, so a producer that is not executed leaves the
// run green and the record is refused by AC-11. For a pair on a functional or
// interop carrier the solo runs are (requireSoloGreen).
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
	// Every producer of a paired break is held to it on a unit carrier: a pair
	// whose second break sits in code the unit never runs is one break wearing
	// the name of two.
	for _, key := range o.producerKeys() {
		if err := o.requireKeyReached(covered, key); err != nil {
			return err
		}
	}
	return nil
}

// producerKeys answers every function one broken run breaks: the record's
// producer, then each paired producer.
func (o *observationRunner) producerKeys() []string {
	keys := make([]string, 0, 1+len(o.record.Paired))
	keys = append(keys, o.record.Producer)
	for _, paired := range o.record.Paired {
		keys = append(keys, paired.Producer)
	}
	return keys
}

// requireKeyReached refuses one producer no statement of which the profile
// shows as executed.
func (o *observationRunner) requireKeyReached(covered coverageSet, key string) error {
	rel, symbol, err := fingerprintKey(key, o.record.Source)
	if err != nil {
		return err
	}
	lines, refusal := producerLineSpan(o.tree, rel, symbol)
	if refusal != "" {
		var tb textbuf.Buffer
		return parseErr(tb.Str(key).Str(" does not resolve in this tree: ").Str(refusal).
			Str(". An unresolvable producer is the defect, never a proof of one"))
	}
	for line := lines.first; line <= lines.last; line++ {
		if covered.covers(rel, line) {
			return nil
		}
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(o.record.Unit).Str(" never executes ").Str(key).
		Str(": no statement of that function appears as covered in the profile the clean run ").
		Str("wrote. A claim that rests on code its own test never reaches is the over-claim ").
		Str("this gate exists to catch, not a proof of one (R-10)"))
}

// requireRed applies the break and demands a failure that NAMES the tagged unit.
//
// Naming is what makes the red this unit's. A build error, a sibling test and a
// flake each turn a run red, and none of them says the break was discriminated
// by the claim's own test.
func (o *observationRunner) requireRed(broken ...testoverlay.File) (ObservedRed, error) {
	overlay, err := testoverlay.Write(o.tree, discriminationArea, broken...)
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
		Seconds: int(time.Since(started).Seconds()), Red: testoverlay.Excerpt(output)}, nil
}

// judgeRed answers whether one run's result is a red this unit produced.
//
// Two refusals, and the pair is the whole of AC-11. A run that stayed GREEN
// under the break records nothing, because the proof would then be a claim. A
// run that went red without naming the unit records nothing either: a build
// error, a sibling test, a crash in package initialization and a flake each
// turn a run red, and none of them says the claim's own test discriminated the
// break. The refusal carries the reason attribution gave, so the operator
// learns which of those it was.
func (o *observationRunner) judgeRed(passed bool, output string) error {
	var tb textbuf.Buffer
	if why := o.brokenRunUnobserved(output); why != "" {
		return parseErr(tb.Str(why))
	}
	if passed {
		return parseErr(tb.Str(o.record.Unit).Str(" stayed GREEN under the break ").
			Quoted(o.record.Break).Str(" applied to ").Str(o.record.Producer).
			Str(". Nothing is written: a proof this run did not observe is never recorded. ").
			Str("Either the test does not discriminate the claim, or the break does not engage it"))
	}
	if why := o.attribution(output); why != "" {
		return parseErr(tb.Str("the run went red under the break, and ").Str(why).
			Str(", so the failure is not this unit's. Nothing is written:\n").Str(testoverlay.Excerpt(output)))
	}
	return nil
}

// attribution answers why a failing run is NOT the tagged unit's red, or the
// empty string when it is.
//
// For a Go unit the red is the unit's own `--- FAIL: <Name>` line, matched on
// the exact name or one of its subtests, never on a prefix: `TestWidget` is not
// named by `--- FAIL: TestWidgetSibling`. That line is also where a killed
// mutant's attribution has to be read from: gomu declares Result.TestOutput and
// fills it nowhere (A-2, measured over 3,683 results).
func (o *observationRunner) attribution(output string) string {
	var tb textbuf.Buffer
	if o.scenario != "" {
		// An interop run is SELECTED down to one scenario, and
		// brokenRunUnobserved has already demanded that scenario's FAIL line.
		// The red is the cited assertion's only when that line names it: a
		// setup error, a different assertion of the same checker and a build
		// failure each fail the run without the claim's assertion firing.
		return o.interopAttribution(output)
	}
	if o.readsResultLine() {
		// brokenRunUnobserved has already refused a skip and a missing line,
		// so the line here is a PASS on a failing exit or the red itself.
		result := testoverlay.CIResult(output, o.names)
		if result == testoverlay.CIFail {
			return ""
		}
		if result == testoverlay.CITimeout {
			return ""
		}
		return tb.Str("the runner reported ").Str(o.names).Str(" as ").Str(result).
			Str(", so the failing exit came from somewhere else").String()
	}
	if o.unitName == "" {
		if strings.Contains(output, o.names) {
			return ""
		}
		return tb.Str("its output does not name ").Quoted(o.names).String()
	}
	if testoverlay.GoTestLineNames(output, "--- FAIL: ", o.unitName) >= 0 {
		return ""
	}
	return o.killedByTheBreak(output)
}

// killedByTheBreak answers why a run that printed no `--- FAIL:` line for the
// unit is NOT a red the unit produced, or the empty string when it is.
//
// A producer reached only from a goroutine the test does not own cannot print
// `--- FAIL:` at all. The halt panics on that goroutine, which takes the test
// binary down before the testing package attributes anything, so the output
// carries a stack and no test name. `coaListener.serve` is the shape: it reads
// the socket in its own goroutine, so every tag on the RFC 5176 walk test named
// a producer this attribution could not reach.
//
// Such a crash counts only when three facts hold. The panic text is the
// break's OWN, so a crash the producer did not cause is not counted. The unit
// had STARTED before the crash: argv runs `-v`, so `go test` prints
// `=== RUN   <Name>` when the unit begins, and that line must come before the
// halt. And the run is selected to that one unit (`-run ^<unit>$`), so no other
// test could have been running. The second fact is what refuses a producer run
// by package initialization: `init` runs before any test is selected or
// started, so a halt there reddens every test in the package and names none of
// them (plan/journal/green-that-could-not-have-been-red.md, 2026-09-24).
//
// The revert route only. A mutant substitutes an expression and never halts, so
// a panic under one is a defect in the mutant rather than a proof.
func (o *observationRunner) killedByTheBreak(output string) string {
	var tb textbuf.Buffer
	tb.Str("its output carries no `--- FAIL: ").Str(o.unitName).Str("` line")
	if o.record.Route != RouteRevert {
		return tb.String()
	}
	halt := strings.Index(output, revertMarker)
	if halt < 0 {
		return tb.String()
	}
	started := testoverlay.GoTestLineNames(output, "=== RUN   ", o.unitName)
	if started < 0 || started > halt {
		return tb.Str(", and the break's halt fired before ").Str(o.unitName).
			Str(" started (no `=== RUN   ").Str(o.unitName).Str("` line precedes it). ").
			Str("The producer ran during package initialization or test setup, where ").
			Str("every test in the package would show the same red. Break a producer ").
			Str("the unit itself reaches, or take the mutant route inside the value ").
			Str("the initializer returns").String()
	}
	return ""
}

// argv answers the command that runs this tagged unit.
func (o *observationRunner) argv(overlay, profile string) []string {
	if o.carrier.Kind != kindUnit {
		// The action the suite's catalog declares runs the scenario.
		return testoverlay.ActionArgv(o.self, o.suite.Action)
	}
	var cover []string
	if profile != "" {
		cover = []string{"-coverprofile", profile, "-coverpkg", o.coverPackages()}
	}
	// UnitArgv runs `-v`, whose `=== RUN   <Name>` line killedByTheBreak reads.
	return testoverlay.UnitArgv(o.toolchain, filepath.ToSlash(filepath.Dir(o.tag.File)),
		o.unitName, overlay, cover...)
}

// coverPackages answers the -coverpkg patterns one clean run instruments: the
// tagged unit's package and the package of every producer the run breaks.
func (o *observationRunner) coverPackages() string {
	var tb textbuf.Buffer
	patterns := []string{tb.Str("./").Str(filepath.ToSlash(filepath.Dir(o.tag.File))).String()}
	for _, key := range o.producerKeys() {
		pattern := tb.Reset().Str("./").Str(filepath.ToSlash(filepath.Dir(keyFile(key)))).String()
		if !slices.Contains(patterns, pattern) {
			patterns = append(patterns, pattern)
		}
	}
	return strings.Join(patterns, ",")
}

// run executes one observation under its deadline.
func (o *observationRunner) run(overlay, profile string) (bool, string, error) {
	if o.carrier.Kind == kindFunctional {
		if o.guest {
			return o.runFunctionalInGuest(overlay)
		}
		return o.runFunctional(overlay)
	}
	if o.guest {
		return o.runInGuest(overlay, profile)
	}
	deadline := unitRunDeadline
	if o.carrier.Kind != kindUnit {
		deadline = carrierRunDeadline
	}
	return o.exec(deadline, o.argv(overlay, profile), o.environment(overlay), o.tree)
}

// exec runs one command under a deadline through testoverlay.Exec, and keeps
// the command as the one this runner last executed, so the record carries what
// was actually run.
func (o *observationRunner) exec(deadline time.Duration, argv, environ []string,
	dir string) (bool, string, error) {
	o.command = strings.Join(argv, " ")
	return testoverlay.Exec(deadline, argv, environ, dir, o.record.Unit)
}

// runFunctional runs ONE `.ci`, not the suite that holds it.
//
// The suite is the wrong unit twice over. It is slow, and it is hostage to
// every other test in it: `./le test functional parse` was already red in this
// checkout from another session's work, so a suite-wide run could never
// attribute a red to this one carrier. testoverlay.PrepareFunctional builds the
// isolated set the suite runner builds, under the overlay. Ordinary suites take
// one test's name in place of --all; the compatibility runner takes its suite
// and an exact .ci path.
func (o *observationRunner) runFunctional(overlay string) (bool, string, error) {
	selector, held := o.functionalSelection()
	if !held {
		var tb textbuf.Buffer
		return false, "", parseErr(tb.Str(o.carrier.Name).
			Str(" names no suite `./le test functional` runs, so this .ci has no runner"))
	}
	label := "discrimination-clean"
	if overlay != "" {
		label = "discrimination-broken"
	}
	run, err := testoverlay.PrepareFunctional(o.toolchain, label, overlay, selector)
	if err != nil {
		return false, "", err
	}
	defer run.Release()

	return o.exec(carrierRunDeadline, run.Argv, run.Environ, o.tree)
}

// functionalSelection answers the native runner's arguments for exactly one .ci.
// ExaBGP has its own action outside Suites. Its positional selector matches an
// exact path, unlike --pattern, which also selects siblings.
func (o *observationRunner) functionalSelection() ([]string, bool) {
	if o.carrier.Name == exabgpCarrierName {
		suite := strings.TrimPrefix(filepath.ToSlash(filepath.Dir(o.tag.File)), o.carrier.Prefix)
		return []string{"exabgp", suite, treePath(o.tree, o.tag.File)}, true
	}
	name := strings.TrimPrefix(o.carrier.Name, "functional-")
	for _, suite := range testfunctional.Suites {
		if suite.Name != name {
			continue
		}
		argv := make([]string, 0, len(suite.Args))
		for _, arg := range suite.Args {
			if arg == testfunctional.AllTests {
				continue
			}
			argv = append(argv, arg)
		}
		return append(argv, o.names), true
	}
	return nil, false
}

// environment answers the environment one observation runs under.
//
// A unit run takes the overlay as a flag (testoverlay.UnitArgv). A functional
// or interop run is a second process that compiles for itself, so the overlay
// travels in GOFLAGS (testoverlay.ProcessEnvironment), with the scenario in the
// suite's selector variable for an interop run.
func (o *observationRunner) environment(overlay string) []string {
	if o.carrier.Kind == kindUnit {
		return testoverlay.UnitEnvironment(o.toolchain)
	}
	return testoverlay.ProcessEnvironment(overlay, o.suite.SelectorVariable, o.scenario)
}

// producerLineSpan answers the first and last line of one function, or the
// refusal that says why the key names none.
//
// The symbol resolves through scopeIndex.keyText, the resolver the fingerprint
// and the revert break use, so the function whose coverage is judged is the one
// the break disabled: `Type.Method` names that receiver's method, and a bare
// name several methods share is refused rather than read as the first of them.
func producerLineSpan(tree, rel, symbol string) (lineRange, string) {
	if symbol == "" {
		return lineRange{}, "the key names the whole file, and coverage is judged over one function"
	}
	raw, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- a repo-relative path fingerprintKey validated
	if err != nil {
		var tb textbuf.Buffer
		return lineRange{}, tb.Str("cannot read ").Str(rel).Str(": ").Str(err.Error()).String()
	}
	content := string(raw)
	index := newScopeIndex()
	text, refusal := index.keyText(rel, content, symbol)
	if refusal != "" {
		return lineRange{}, refusal
	}
	for _, found := range index.of(content) {
		if content[found.begin:found.end] != text {
			continue
		}
		first := strings.Count(content[:found.begin], "\n") + 1
		// The span runs past the closing brace's newline, which is not a line of
		// the function.
		last := strings.Count(strings.TrimSuffix(content[:found.end], "\n"), "\n") + 1
		return lineRange{first: first, last: last}, ""
	}
	panic("BUG: keyText answered a text no span of the same content holds")
}

// readsResultLine answers whether this carrier's run prints the functional
// runner's per-test result line, which is then the evidence of what ran. The
// ExaBGP compatibility runner prints its own report, so it keeps the name match.
func (o *observationRunner) readsResultLine() bool {
	if o.carrier.Kind != kindFunctional {
		return false
	}
	return o.carrier.Name != exabgpCarrierName
}

// cleanRunUnobserved answers why a clean run that exited 0 did not observe
// the .ci passing, or the empty string when it did.
//
// A .ci the runner SKIPS exits 0 like one that passed: a host without
// CAP_NET_ADMIN skips every `caps=net-admin` test. A skip is not a green, and
// a break applied to a test that is skipped again can only "stay green", so the
// clean run must show the PASS line itself (journal row of 2026-09-30 in
// plan/journal/green-that-could-not-have-been-red.md).
func (o *observationRunner) cleanRunUnobserved(output string) string {
	if o.scenario != "" {
		return o.interopCleanUnobserved(output)
	}
	if why := o.unitSkipped(output, "the clean run"); why != "" {
		return why
	}
	if !o.readsResultLine() {
		return ""
	}
	result := testoverlay.CIResult(output, o.names)
	if result == testoverlay.CIPass {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(o.record.Unit).Str(" did not run in the clean run: ").
		Str(testoverlay.DescribeCIResult(o.names, result)).
		Str(". A skip is not a green, so nothing a break does to it can be observed:\n").
		Str(testoverlay.Excerpt(output)).String()
}

// brokenRunUnobserved answers why a run under the break observed nothing
// about the .ci: it was skipped, or no line names it.
func (o *observationRunner) brokenRunUnobserved(output string) string {
	if o.scenario != "" {
		return o.interopBrokenUnobserved(output)
	}
	if why := o.unitSkipped(output, "the run under the break"); why != "" {
		return why
	}
	if !o.readsResultLine() {
		return ""
	}
	result := testoverlay.CIResult(output, o.names)
	if result == testoverlay.CISkip {
		return testoverlay.DescribeUnobservedBreak(o.record.Unit,
			testoverlay.DescribeCIResult(o.names, result), output)
	}
	if result == "" {
		return testoverlay.DescribeUnobservedBreak(o.record.Unit,
			testoverlay.DescribeCIResult(o.names, result), output)
	}
	return ""
}

// unitSkipped answers why a Go unit's run observed nothing because the unit, or
// one of its subtests, skipped, or the empty string when it did not.
//
// A skipped test exits 0 like one that passed. A skip proves nothing either way:
// in the clean run it is no green, and under the break it is no "stayed green",
// which is how a root-gated unit run unprivileged in the guest was refused as a
// test that does not discriminate its claim. A skipped subtest counts too: the
// claim is the unit's, and part of it ran nowhere. argv and guestScript both run
// `-v`, which streams the skip reason on the indented lines above `--- SKIP:`.
func (o *observationRunner) unitSkipped(output, run string) string {
	if o.carrier.Kind != kindUnit {
		return ""
	}
	if o.unitName == "" {
		return ""
	}
	at := testoverlay.GoTestLineNames(output, "--- SKIP: ", o.unitName)
	if at < 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str(o.record.Unit).Str(" observed nothing in ").Str(run).Str(": the unit skipped: ").
		Str(testoverlay.SkipReason(output[:at])).
		Str(". A skip proves nothing either way, so nothing is written. Fix: run it where it does not skip")
	if o.guest {
		tb.Str("; a unit that skips unless it is root declares `").Str(guestRootDirective).
			Str("` on its own line in the test function's doc comment")
	}
	return tb.Byte(':').Byte('\n').Str(testoverlay.Excerpt(output)).String()
}
