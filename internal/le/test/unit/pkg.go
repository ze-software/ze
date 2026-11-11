// Design: docs/contributing/running-commands.md -- testing one package while you develop it
// Overview: actions.go -- the table this action is a row of
// Related: groups.go -- the host group whose tags, race detector and environment the action reuses
//
// `le test unit pkg path <package> [path <package>...] [run <regex>] [vet]`
// tests or vets the packages it names under the checkout's feature tags, with
// the race detector, admitted under a job label, its output in this session's
// scratch log. It is the scoped test an agent runs while it develops a package,
// spelled in one short line: the long form it replaces is `le job run label
// unit-pkg quiet command go test -timeout ... -tags ... -race <package>`, whose
// tag list the caller had to copy from feature-gates.txt.
//
// The action builds no argv of its own: the toolchain's GoTest and GoVet do,
// with the options a host group uses. It starts no child of its own either: the
// quiet run of internal/le/job admits and runs it.
package testunit

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
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
			" `vet` type-checks them and their test files instead, under vet-pkg. The scoped test of a package" +
			" you are developing; the groups and `all` are the wider runs",
		Parameters: pkgParameters(),
		AnswerArgs: func(args leaction.Arguments) (any, int) { return answerPkg(tc, args) },
	}
}

// pkgLine is one parsed `pkg` line.
type pkgLine struct {
	Packages []string
	Run      string
	Vet      bool
}

// answerPkg runs one parsed `pkg` line through the quiet-run seam.
func answerPkg(tc gotoolchain.Toolchain, args leaction.Arguments) (any, int) {
	line, err := pkgLineOf(args)
	if err != nil {
		return nil, metadataOnly().Refuse(pkgVerb, err)
	}
	label, argv := line.command(tc)
	report, code := quietRun(label, argv, tc.Root, tc.Environment(packageGroup.EnvOptions()))
	return report, code
}

// pkgLineOf turns the parsed keywords into the packages to run. A line that
// does not say which packages, or asks for a test filter on a vet, has no
// single meaning, so it is refused rather than given one.
func pkgLineOf(args leaction.Arguments) (pkgLine, error) {
	if args.Has(changedKeyword) {
		// The reached-package selector, and its refusal of a widened answer, are
		// wired in the next phase of spec-agent-economy-2-scoped-test. Until
		// then the word is declared and refused, never read as an empty list.
		return pkgLine{}, errors.New("test unit pkg changed is not wired yet: name each package with path <package>")
	}
	if !args.Has(pathKeyword) {
		return pkgLine{}, errors.New("test unit pkg needs path <package>: it names the package to test; repeat path for each one")
	}
	if args.Has(runKeyword) && args.Has(vetKeyword) {
		return pkgLine{}, errors.New("test unit pkg takes run or vet, not both: vet runs no test for run to select")
	}
	packages := args.Values(pathKeyword)
	for _, pkg := range packages {
		if wholeTree(pkg) {
			var tb textbuf.Buffer
			return pkgLine{}, errors.New(tb.Str("test unit pkg path ").Str(pkg).
				Str(" names the whole tree, which is the owner's run (./le test unit all): name the packages you changed with path <package>").String())
		}
	}
	return pkgLine{Packages: packages, Run: args.One(runKeyword), Vet: args.Has(vetKeyword)}, nil
}

// wholeTree reports whether a package value names every package of the
// checkout. The matcher against the group patterns, which also refuses a group's
// whole pattern and a value spanning several groups, is the next phase's.
func wholeTree(pkg string) bool {
	if pkg == "./..." {
		return true
	}
	return pkg == "..."
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
