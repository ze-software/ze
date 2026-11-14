// Design: docs/contributing/running-commands.md -- testing one package while you develop it
// Overview: actions.go -- the table this action is a row of
// Related: groups.go -- the host group whose tags, race detector and environment the action reuses, and place
// Related: ../../runlog/runlog.go -- the log readers the answer is built from
//
// `le test unit pkg path <package> [path <package>...] [run <regex>] [vet]`
// tests or vets the packages it names under the checkout's feature tags, with
// the race detector, admitted under a job label, its output in this session's
// scratch log. `le test unit pkg changed [vet]` does the same for the packages
// the change reaches. It is the scoped test an agent runs while it develops a
// package, spelled in one short line: the long form it replaces is `le job run
// label unit-pkg quiet command go test -timeout ... -tags ... -race <package>`,
// whose tag list the caller had to copy from feature-gates.txt.
//
// The action builds no argv of its own: the toolchain's GoTest and GoVet do,
// with the options a host group uses. It starts no child of its own either: the
// quiet run of internal/le/job admits and runs it. What it adds is the answer:
// the lines under each failed test, then one summary line that counts only the
// packages that ran a test.
package testunit

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	"github.com/ze-software/ze/internal/le/runlog"
)

// The action's word and its keywords. Each keyword types the value after it,
// so a package spelled like a keyword is still a package.
const (
	pkgVerb        = "pkg"
	pathKeyword    = "path"
	runKeyword     = "run"
	vetKeyword     = "vet"
	changedKeyword = "changed"
)

// The job labels a run is admitted under. A vet has its own, because the
// previous-run banner and the duration record are per label, and a vet's
// seconds would distort a test's.
const (
	testLabel = "unit-pkg"
	vetLabel  = "vet-pkg"
)

// failureLinesMax is how many failure lines the answer carries, the quiet
// mode's cap. A reader who needs more opens the log, whose path the summary
// names.
const failureLinesMax = 20

// packageGroup is the host group the action's packages run as: the feature
// manifest's tag set, the race detector, cgo and the concurrency cap, which is
// what every host row of the group table carries. It is not a row of the table:
// its packages are the ones the line names.
var packageGroup = Group{Verb: pkgVerb, Race: true}

// QuietRun admits one job under label, runs argv in dir under environ with its
// output in this session's scratch log, and answers the job's report and exit
// code. It is the seam between this action and job admission.
type QuietRun func(label string, argv []string, dir string, environ []string) (job.Report, int)

// quietRun is the seam's production value, the quiet run of internal/le/job.
// Only SetQuietRunForTest replaces it.
var quietRun QuietRun = admittedQuietRun

// SetQuietRunForTest replaces the quiet run for one test and returns the
// function that restores the previous one. The caller MUST call it, and MUST
// NOT run tests that replace the seam in parallel: the seam is package state.
func SetQuietRunForTest(run QuietRun) (restore func()) {
	previous := quietRun
	quietRun = run
	return func() { quietRun = previous }
}

// ChangedSelector answers the packages the change in the checkout at root
// reaches, and the selector's exit code. It is the seam between `changed` and
// the reached-package selector.
type ChangedSelector func(root string) (repochanged.ScopeReport, int)

// selectChanged is the seam's production value, the selector of
// internal/le/repo/changed. Only SetChangedSelectorForTest replaces it.
var selectChanged ChangedSelector = repochanged.Packages

// SetChangedSelectorForTest replaces the reached-package selector for one test
// and returns the function that restores the previous one. The caller MUST
// call it, and MUST NOT run tests that replace the seam in parallel.
func SetChangedSelectorForTest(selector ChangedSelector) (restore func()) {
	previous := selectChanged
	selectChanged = selector
	return func() { selectChanged = previous }
}

// admittedQuietRun is the quiet run of internal/le/job over the checkout dir
// names, which is the root the action resolved its toolchain from.
func admittedQuietRun(label string, argv []string, dir string, environ []string) (job.Report, int) {
	adm, err := job.NewIn(dir)
	if err != nil {
		// NewIn fails only on an admission policy the environment states out of
		// range (job.Admission.Validate), so the fix is that variable.
		leaction.ReportError(opererr.Wrap(err, "test unit pkg could not set up job admission",
			"the admission policy the environment states is out of range, so no slot can be claimed",
			"correct or unset "+job.SlotsKey+" and "+job.StallKey+", then run ./le test unit pkg again"))
		return job.Report{Label: label, Command: argv, Code: 2}, 2
	}
	return adm.RunQuiet(label, argv, dir, environ)
}

// pkgParameters is the action's grammar. Every value follows its keyword, and
// the parser refuses a value that begins with a dash, so neither a package nor
// the regex can become a `go` flag.
func pkgParameters() []leaction.Parameter {
	return []leaction.Parameter{
		{Keyword: pathKeyword, Kind: leaction.KindPath, Value: "package", Requirement: leaction.Optional, Repeat: true,
			Help: "a package to test, as ./dir/; repeat for each package"},
		{Keyword: runKeyword, Kind: leaction.KindString, Value: "regex", Requirement: leaction.Optional,
			Help: "run only the tests matching the regex (go test -run)"},
		{Keyword: vetKeyword, Kind: leaction.KindSwitch,
			Help: "vet the packages and test files instead of testing"},
		{Keyword: changedKeyword, Kind: leaction.KindSwitch,
			Help: "test the packages the change reaches, in place of path"},
	}
}

// pkgAction is the table row. tc is the toolchain the area resolved for this
// line; a metadata-only table passes the zero toolchain and never runs it.
func pkgAction(tc gotoolchain.Toolchain) leaction.Action {
	return leaction.Action{
		Verb:    pkgVerb,
		Does:    "run",
		Summary: "test or vet named packages under the feature tags",
		Why: "test the named packages with the feature tags, the race detector and the toolchain timeout," +
			" admitted under the unit-pkg job label, the output in this session's scratch log." +
			" `vet` type-checks them and their test files instead, under vet-pkg. `changed` takes the packages" +
			" the change reaches. The scoped test of a package you are developing; the groups and `all` are" +
			" the wider runs",
		// admittedQuietRun takes the slot, with the run's own log and report.
		// The row is heavy by its area (leroot.RegisterHeavy).
		AdmitsItself: true,
		Parameters:   pkgParameters(),
		AnswerArgs:   func(args leaction.Arguments) (any, int) { return answerPkg(tc, args) },
	}
}

// pkgLine is one parsed `pkg` line.
type pkgLine struct {
	Packages []string
	Run      string
	Vet      bool
	Changed  bool
}

// answerPkg runs one parsed `pkg` line through the quiet-run seam and answers
// what the log says. Every refusal is answered before admission, so a line
// that cannot run claims no slot.
func answerPkg(tc gotoolchain.Toolchain, args leaction.Arguments) (any, int) {
	line, err := pkgLineOf(args)
	if err != nil {
		return nil, metadataOnly().Refuse(pkgVerb, err)
	}
	module, err := lepath.Module(tc.Root)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "test unit pkg could not read the checkout's module path",
			"a package named by import path is matched against the group table through it",
			"check that "+filepath.Join(tc.Root, "go.mod")+" exists and declares a module, then run the line again"))
		return nil, 1
	}

	report := PkgReport{Vet: line.Vet, Run: line.Run}
	if line.Changed {
		packages, leftOut, code := changedPackages(tc.Root, module)
		if code != 0 {
			return nil, code
		}
		report.LeftOut = leftOut
		line.Packages = packages
		if len(packages) == 0 {
			empty := report.nothingToRun()
			return empty, empty.Code
		}
	} else if err := namedPackagesRun(line.Packages, module); err != nil {
		return nil, metadataOnly().Refuse(pkgVerb, err)
	}

	label, argv := line.command(tc)
	ran, code := quietRun(label, argv, tc.Root, tc.Environment(packageGroup.EnvOptions()))
	report.Job = ran
	report.read(tc.Root, code)
	return report, report.Code
}

// pkgLineOf turns the parsed keywords into the line to run. A line that does
// not say which packages, names them twice over, or asks for a test filter on a
// vet has no single meaning, so it is refused rather than given one.
func pkgLineOf(args leaction.Arguments) (pkgLine, error) {
	if args.Has(changedKeyword) && args.Has(pathKeyword) {
		return pkgLine{}, errors.New("test unit pkg takes path or changed, not both: each names the packages to test;" +
			" run ./le test unit pkg changed, or ./le test unit pkg path <package>")
	}
	if !args.Has(changedKeyword) && !args.Has(pathKeyword) {
		return pkgLine{}, errors.New("test unit pkg needs path <package> or changed: nothing says which packages to test;" +
			" run ./le test unit pkg path <package>, repeating path for each one, or ./le test unit pkg changed")
	}
	if args.Has(runKeyword) && args.Has(vetKeyword) {
		return pkgLine{}, errors.New("test unit pkg takes run or vet, not both: vet runs no test for run to select;" +
			" drop run to vet, or drop vet to test")
	}
	return pkgLine{
		Packages: args.Values(pathKeyword),
		Run:      args.One(runKeyword),
		Vet:      args.Has(vetKeyword),
		Changed:  args.Has(changedKeyword),
	}, nil
}

// namedPackagesRun refuses a named package the group table says this action
// must not run: the whole tree, a group's whole pattern, a package behind a
// group's own tags, and a sub-tree spanning two or more groups.
func namedPackagesRun(packages []string, module string) error {
	for _, pkg := range packages {
		at := place(pkg, module, groups[:])
		if at.kind == placementRuns {
			continue
		}
		return placementRefusal("test unit pkg path "+pkg, at)
	}
	return nil
}

// PackageRuns reports whether `test unit pkg path <pkg>` runs pkg in the
// checkout at root, by the refusal the action itself gives (namedPackagesRun),
// so the raw-job refusal of the Bash hook advises the short form only where
// the action runs it. An error is a checkout whose module path cannot be read.
func PackageRuns(root, pkg string) (bool, error) {
	module, err := lepath.Module(root)
	if err != nil {
		return false, err
	}
	return namedPackagesRun([]string{pkg}, module) == nil, nil
}

// changedPackages answers the packages the change reaches that this action
// runs, the ones it leaves out because a group's own tags hide their tests, and
// 0; or a non-zero code once the refusal is reported. A widened answer, and any
// answer holding the whole tree, is refused: the selector fails open to every
// package with exit 0, and a whole-tree race run is not an agent's to start.
func changedPackages(root, module string) ([]string, []LeftOut, int) {
	scope, code := selectChanged(root)
	if code != 0 {
		var tb textbuf.Buffer
		leaction.ReportError(opererr.New(
			tb.Str("test unit pkg changed: the reached-package selector answered exit ").Int(int64(code)).String(),
			"so no package set is known to test",
			"run ./le repo changed scope print packages to see why"))
		return nil, nil, 2
	}
	if scope.Widened {
		leaction.ReportError(opererr.New(
			"test unit pkg changed: the selector widened to every package ("+scope.Reason+")",
			"a whole-tree run is the owner's (./le test unit all), not an agent's",
			"name the packages you changed with ./le test unit pkg path <package>"))
		return nil, nil, 2
	}

	packages := make([]string, 0, len(scope.Packages))
	var leftOut []LeftOut
	for _, pkg := range scope.Packages {
		at := place(pkg, module, groups[:])
		switch at.kind {
		case placementRuns:
			packages = append(packages, pkg)
		case placementBehindGroupTags:
			leftOut = append(leftOut, LeftOut{Package: pkg, Group: at.groups[0].Verb})
		case placementUnspecified, placementWholeTree, placementGroupPattern, placementSpansGroups:
			leaction.ReportError(opererr.New(
				"test unit pkg changed reached "+pkg+", which this action does not run",
				placementRefusal("the package", at).Error(),
				"name the packages you changed with ./le test unit pkg path <package>, or run the command named above"))
			return nil, nil, 2
		}
	}
	return packages, leftOut, 0
}

// placementRefusal says why the action does not run a package value, and the
// command that does.
func placementRefusal(subject string, at placement) error {
	var tb textbuf.Buffer
	tb.Str(subject)
	switch at.kind {
	case placementWholeTree:
		tb.Str(" names the whole tree, which is the owner's run (./le test unit all):" +
			" name the packages you changed with path <package>")
	case placementGroupPattern:
		tb.Str(" is the whole pattern of the ").Str(at.groups[0].Verb).
			Str(" group: run ./le test unit ").Str(at.groups[0].Verb)
	case placementBehindGroupTags:
		group := at.groups[0]
		tb.Str(" lies behind the ").Str(group.Verb).Str(" group's own tags (").Str(strings.Join(group.Tags, " ")).
			Str("), which the feature tags lack, so its tagged tests would compile out: run ./le test unit ").Str(group.Verb)
	case placementSpansGroups:
		tb.Str(" spans the groups")
		for _, group := range at.groups {
			tb.Byte(' ').Str(group.Verb)
		}
		tb.Str(", a near-whole-tree run: name the packages you meant with path <package>, or run ./le test unit <group>")
	case placementUnspecified, placementRuns:
		// A package that runs is never refused; reaching here is a defect of the
		// caller, which is said rather than turned into a run.
		tb.Str(" was refused without a reason, a defect of le: report it")
	}
	return errors.New(tb.String())
}

// command answers the job label and the argv for this line, built by the
// toolchain with the host group's options.
func (l pkgLine) command(tc gotoolchain.Toolchain) (string, []string) {
	if l.Vet {
		return vetLabel, tc.GoVet(packageGroup.options(), l.Packages...)
	}
	words := make([]string, 0, 2+len(l.Packages))
	if l.Run != "" {
		words = append(words, "-run", l.Run)
	}
	words = append(words, l.Packages...)
	return testLabel, tc.GoTest(packageGroup.options(), words...)
}

// pkgOutcome is what a `pkg` line came to. Zero is no outcome, never a pass.
type pkgOutcome uint8

const (
	outcomeUnspecified pkgOutcome = iota
	// outcomeRan: the job ran and its code is the child's.
	outcomeRan
	// outcomeNoTestRan: go test exited 0 and no package ran a test.
	outcomeNoTestRan
	// outcomeNotStarted: the quiet run answered no log, so the job never ran.
	outcomeNotStarted
	// outcomeLogUnread: the job ran and its log could not be read.
	outcomeLogUnread
	// outcomeNothingReached: `changed` reached no Go package.
	outcomeNothingReached
	// outcomeAllLeftOut: every package `changed` reached is behind a group's
	// own tags.
	outcomeAllLeftOut
)

// String names the outcome in the payload.
func (o pkgOutcome) String() string {
	switch o {
	case outcomeRan:
		return "ran"
	case outcomeNoTestRan:
		return "no-test-ran"
	case outcomeNotStarted:
		return "not-started"
	case outcomeLogUnread:
		return "log-unread"
	case outcomeNothingReached:
		return "nothing-reached"
	case outcomeAllLeftOut:
		return "all-left-out"
	case outcomeUnspecified:
	}
	return "unspecified"
}

// MarshalText renders the outcome as its name in the JSON payload.
func (o pkgOutcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// LeftOut is one package `changed` reached and the action did not run, and the
// group whose own action runs it.
type LeftOut struct {
	Package string `json:"package"`
	Group   string `json:"group"`
}

// PkgReport is the answer of one `pkg` line: the job admission ran, what its
// log says, and the packages left out.
type PkgReport struct {
	Outcome  pkgOutcome           `json:"outcome"`
	Job      job.Report           `json:"job"`
	Vet      bool                 `json:"vet,omitempty"`
	Run      string               `json:"run,omitempty"`
	Packages runlog.PackageCounts `json:"packages"`
	Failures []string             `json:"failures,omitempty"`
	LeftOut  []LeftOut            `json:"left-out,omitempty"`
	Code     int                  `json:"code"`
}

// nothingToRun answers a `changed` line that left nothing to run: 0 when the
// change reached no Go package, 1 when every reached package was left out,
// because the packages the change reached were then not tested here.
func (r PkgReport) nothingToRun() PkgReport {
	if len(r.LeftOut) == 0 {
		r.Outcome, r.Code = outcomeNothingReached, 0
		return r
	}
	r.Outcome, r.Code = outcomeAllLeftOut, 1
	return r
}

// read fills the report from the job's log. The code is the child's, with one
// exception: a test run that exited 0 with no package that ran a test answers
// 1, because a question that got no answer is not a pass.
func (r *PkgReport) read(root string, code int) {
	r.Code = code
	if r.Job.Log == "" {
		r.Outcome = outcomeNotStarted
		if code == 0 {
			r.Code = 1
		}
		return
	}

	counts, failures, err := readLog(filepath.Join(root, r.Job.Log), code != 0)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "test unit pkg could not read the job's log",
			"the verdict is counted from it, so no test is confirmed to have run",
			"read "+r.Job.Log+" by hand, or run the line again"))
		r.Outcome = outcomeLogUnread
		if code == 0 {
			r.Code = 1
		}
		return
	}
	r.Packages, r.Failures = counts, failures
	r.Outcome = outcomeRan
	if r.Vet {
		return
	}
	if code == 0 && counts.Passed == 0 {
		r.Outcome, r.Code = outcomeNoTestRan, 1
	}
}

// readLog counts the package result lines of the log at path and, when the run
// failed, reads its failure lines.
func readLog(path string, failed bool) (runlog.PackageCounts, []string, error) {
	file, err := os.Open(path) //nolint:gosec // the log the quiet run named, under this session's scratch
	if err != nil {
		return runlog.PackageCounts{}, nil, err
	}
	defer file.Close() //nolint:errcheck // the log is only read

	counts, err := runlog.PackageResults(file)
	if err != nil {
		return runlog.PackageCounts{}, nil, err
	}
	if !failed {
		return counts, nil, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return runlog.PackageCounts{}, nil, err
	}
	failures, err := runlog.Failures(file, failureLinesMax)
	return counts, failures, err
}

// Text renders the answer: a line for each package left out, the failure lines,
// then one summary line.
func (r PkgReport) Text() string {
	var text textbuf.Buffer
	for _, out := range r.LeftOut {
		text.Str("left out ").Str(out.Package).Str(": its tests need the ").Str(out.Group).
			Str(" group's own tags; run ./le test unit ").Str(out.Group).Byte('\n')
	}
	for _, line := range r.Failures {
		text.Str(line).Byte('\n')
	}
	text.Str("test unit pkg: exit ").Int(int64(r.Code)).Str(", ")
	r.summary(&text)
	if r.Job.WaitedSeconds > 0 {
		text.Str(", waited ").Int(int64(r.Job.WaitedSeconds)).Byte('s')
	}
	if r.Job.Log != "" {
		text.Str(", log ").Str(r.Job.Log)
	}
	return text.Byte('\n').String()
}

// summary writes what the outcome says, between the exit code and the wait.
func (r PkgReport) summary(text *textbuf.Buffer) {
	switch r.Outcome {
	case outcomeRan:
		if r.Vet {
			text.Str("vet")
			return
		}
		r.counts(text)
	case outcomeNoTestRan:
		text.Str("no test ran (go test exited 0")
		if r.Run != "" {
			text.Str("; run ").Str(r.Run).Str(" matched no test")
		}
		text.Byte(')')
	case outcomeNotStarted:
		text.Str("the job did not start, so no package ran")
	case outcomeLogUnread:
		text.Str("the log could not be read, so no test is confirmed to have run")
	case outcomeNothingReached:
		text.Str("the change reaches no Go package, so there is nothing to test")
	case outcomeAllLeftOut:
		text.Str("every package the change reaches was left out, so nothing ran here")
	case outcomeUnspecified:
		text.Str("no outcome was recorded, a defect of le: report it")
	}
}

// counts writes the package counts: the packages ok, the failed ones on a red
// run, and the ones that ran no test when there are any.
func (r PkgReport) counts(text *textbuf.Buffer) {
	text.Int(int64(r.Packages.Passed)).Str(plural(r.Packages.Passed, " package", " packages")).Str(" ok")
	if r.Packages.Failed > 0 || r.Code != 0 {
		text.Str(", ").Int(int64(r.Packages.Failed)).Str(" failed")
	}
	if r.Packages.NoTest > 0 {
		text.Str(", ").Int(int64(r.Packages.NoTest)).Str(" ran no test")
	}
}

// plural answers one when n is 1, and many otherwise.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
