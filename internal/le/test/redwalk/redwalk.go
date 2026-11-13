// Design: docs/contributing/running-commands.md -- le test redwalk check
// Detail: patch.go -- the break, applied in memory to the working tree
// Detail: verdict.go -- the two runs judged, and the answer
// Related: register.go -- the command registration
//
// Package testredwalk runs a red walk: one Go break against one unit test, one
// .ci or one interop scenario, clean then broken, and says whether the test
// discriminates the break. The break reaches the compiler through a Go overlay
// staged in session scratch, so the shared tree is never written. It drives
// the overlay package the way `le rfc record` does, and needs no RFC tag.
package testredwalk

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	"github.com/ze-software/ze/internal/le/test/overlay"
)

const area = "test redwalk"

// The keywords the check takes. A value never stands alone (ai/rules/cli.md).
const (
	keyBreak    = "break"
	keyUnit     = "unit"
	keyPackage  = "package"
	keyCI       = "ci"
	keyScenario = "scenario"
)

// The deadlines of one run. A run past its deadline is an error, never a red:
// "the break reddens it" and "the walk never found out" must not be one
// answer. A unit is one `go test` of one function; a .ci and a scenario build
// their binaries first, and a scenario builds its lab.
const (
	unitDeadline    = 15 * time.Minute
	carrierDeadline = 60 * time.Minute
)

// kind is the carrier a walk runs.
type kind uint8

const (
	kindUnspecified kind = iota
	kindUnit
	kindCI
	kindScenario
)

// name answers the carrier as the report prints it.
func (k kind) name() string {
	switch k {
	case kindUnit:
		return "unit"
	case kindCI:
		return "ci"
	case kindScenario:
		return "scenario"
	case kindUnspecified:
		panic("BUG: redwalk carrier unspecified: every target is parsed with its carrier")
	}
	panic("BUG: redwalk carrier out of range")
}

// target is the unit a walk judges.
type target struct {
	kind kind
	// subject names the unit in the report: `TestX in pkg`, `suite/name`,
	// `lab/name`.
	subject string
	// name is what the carrier's output names the unit by: the Go test, the
	// .ci stem, the scenario.
	name string
	// pkg is the unit's package directory, slash-separated and relative to
	// the tree. Empty for any other carrier.
	pkg string
	// suite is the functional suite of a .ci.
	suite testfunctional.Suite
	// catalog is the interop suite of a scenario.
	catalog interoplab.Catalog
}

var actions = leaction.New(area, leaction.Action{
	Verb:    "check",
	Heavy:   true,
	Default: true,
	Summary: "check that a test goes red under a Go break",
	Why: "apply a break, a git diff of tracked Go files, through a Go overlay in session scratch, run one " +
		"unit test, one .ci or one interop scenario clean and then broken, and judge both runs. Exit 0 " +
		"when the clean run passes and the broken run fails as the unit's own red; exit 1 when the test " +
		"stays green, has no clean baseline, or fails some other way. The shared tree is never written",
	Parameters: []leaction.Parameter{
		{Keyword: keyBreak, Kind: leaction.KindPath, Value: "patch", Requirement: leaction.Required,
			Help: "the break: a git diff of tracked Go files"},
		{Keyword: keyUnit, Kind: leaction.KindLabel, Value: "Test", Requirement: leaction.Optional,
			Help: "walk one Go test function (with package)"},
		{Keyword: keyPackage, Kind: leaction.KindPath, Value: "pkg", Requirement: leaction.Optional,
			Help: "the package directory of the unit test"},
		{Keyword: keyCI, Kind: leaction.KindString, Value: "suite/name", Requirement: leaction.Optional,
			Help: "walk one .ci of a functional suite"},
		{Keyword: keyScenario, Kind: leaction.KindString, Value: "lab/name", Requirement: leaction.Optional,
			Help: "walk one interop scenario of a lab"},
	},
	AnswerArgs: answerCheck,
})

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test redwalk` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// answerCheck runs one walk. A line or a break the walk refuses answers 2,
// before anything runs; a walk that ran answers its report's code.
func answerCheck(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	chosen, err := parseTarget(tree, args)
	if err != nil {
		return nil, refuse(err)
	}
	patch, err := cliio.ReadFile(args.One(keyBreak))
	if err != nil {
		var tb textbuf.Buffer
		return nil, refuse(errors.New(tb.Str(args.One(keyBreak)).Str(": cannot read the break: ").Err(err).
			Str("\n  fix: save it with `git diff -- <file> > <patch>` and name that file").String()))
	}
	files, err := parseBreak(tree, patch, gitUntracked)
	if err != nil {
		return nil, refuse(err)
	}
	run, err := carrierRunner(tree, chosen)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := (&walk{tree: tree, target: chosen, files: files, run: run}).check()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, report.Code()
}

// refuse reports a line or a break the walk refused, and answers 2.
func refuse(err error) int {
	leaction.ReportError(err)
	return 2
}

// parseTarget answers the one carrier unit the line names.
func parseTarget(tree string, args leaction.Arguments) (target, error) {
	named := 0
	for _, key := range []string{keyUnit, keyCI, keyScenario} {
		if args.Has(key) {
			named++
		}
	}
	if named != 1 {
		return target{}, errors.New("test redwalk check walks one unit: name exactly one of " +
			"unit <Test> package <pkg>, ci <suite>/<name> or scenario <lab>/<name>")
	}
	if args.Has(keyPackage) {
		if !args.Has(keyUnit) {
			return target{}, errors.New("test redwalk check: package <pkg> names the package of a unit " +
				"test, so it goes with unit <Test>")
		}
	}
	if args.Has(keyUnit) {
		return unitTarget(tree, args.One(keyUnit), args.One(keyPackage))
	}
	if args.Has(keyCI) {
		return ciTarget(args.One(keyCI))
	}
	return scenarioTarget(tree, args.One(keyScenario))
}

// unitTarget answers a Go test function in one package directory.
func unitTarget(tree, name, pkg string) (target, error) {
	var tb textbuf.Buffer
	if !goIdentifier(name) {
		return target{}, errors.New(tb.Str("unit ").Str(name).Str(": a unit is one Go test function name, ").
			Str("letters, digits and underscores only\n  fix: name the function, such as unit TestParse").String())
	}
	if pkg == "" {
		return target{}, errors.New("test redwalk check: unit <Test> needs package <pkg>, the directory " +
			"that holds the test, such as package internal/le/test/redwalk")
	}
	rel := strings.TrimPrefix(filepath.ToSlash(pkg), "./")
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return target{}, errors.New(tb.Str("package ").Str(pkg).Str(": the package must be a directory ").
			Str("inside the checkout\n  fix: name it relative to the checkout, such as internal/le/test/redwalk").String())
	}
	info, err := os.Stat(filepath.Join(tree, filepath.FromSlash(rel)))
	if err != nil {
		return target{}, errors.New(tb.Str("package ").Str(pkg).Str(": no such directory in the checkout\n").
			Str("  fix: name the directory that holds the test").String())
	}
	if !info.IsDir() {
		return target{}, errors.New(tb.Str("package ").Str(pkg).Str(": not a directory\n").
			Str("  fix: name the directory that holds the test").String())
	}
	return target{kind: kindUnit, subject: tb.Str(name).Str(" in ").Str(rel).String(), name: name, pkg: rel}, nil
}

// ciTarget answers one .ci of a functional suite.
func ciTarget(text string) (target, error) {
	suiteName, stem, split := strings.Cut(text, "/")
	if !split {
		return target{}, errors.New("ci " + text + ": name the .ci as <suite>/<name>, such as ci parse/" +
			"bgp-basic\n  fix: run ./le test functional to see the suites")
	}
	if stem == "" {
		return target{}, errors.New("ci " + text + ": the name after the suite is empty\n" +
			"  fix: name the .ci by its file name without .ci")
	}
	if strings.Contains(stem, "/") {
		return target{}, errors.New("ci " + text + ": the name after the suite is one .ci name, with no slash\n" +
			"  fix: name the .ci by its file name without .ci")
	}
	suite, held := testfunctional.SuiteNamed(suiteName)
	if !held {
		names := make([]string, 0, len(testfunctional.Suites))
		for _, known := range testfunctional.Suites {
			names = append(names, known.Name)
		}
		return target{}, errors.New("ci " + text + ": " + suiteName + " is no functional suite\n" +
			"  fix: use one of " + strings.Join(names, ", "))
	}
	return target{kind: kindCI, subject: text, name: stem, suite: suite}, nil
}

// scenarioTarget answers one scenario of a registered interop lab.
func scenarioTarget(tree, text string) (target, error) {
	lab, name, split := strings.Cut(text, "/")
	if !split {
		return target{}, errors.New("scenario " + text + ": name the scenario as <lab>/<name>\n" +
			"  fix: use one of the labs " + strings.Join(interoplab.CatalogSuites(), ", "))
	}
	catalog, held := interoplab.CatalogNamed(lab)
	if !held {
		return target{}, errors.New("scenario " + text + ": " + lab + " is no interop lab\n" +
			"  fix: use one of " + strings.Join(interoplab.CatalogSuites(), ", "))
	}
	if !filepath.IsLocal(name) {
		return target{}, errors.New("scenario " + text + ": the scenario is one directory name under " +
			catalog.ScenarioRoot + "\n  fix: name a directory there")
	}
	info, err := os.Stat(filepath.Join(tree, filepath.FromSlash(catalog.ScenarioRoot), name))
	if err != nil {
		return target{}, errors.New("scenario " + text + ": no scenario " + name + " under " +
			catalog.ScenarioRoot + "\n  fix: name a directory there")
	}
	if !info.IsDir() {
		return target{}, errors.New("scenario " + text + ": " + name + " under " + catalog.ScenarioRoot +
			" is not a directory\n  fix: name a scenario directory there")
	}
	return target{kind: kindScenario, subject: text, name: name, catalog: catalog}, nil
}

// goIdentifier answers whether name is a Go identifier, so `-run ^name$`
// selects exactly that function.
func goIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if character == '_' {
			continue
		}
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= 'A' && character <= 'Z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

// carrierRunner answers the runner of the target's own carrier, the one `le
// rfc record` drives for the same kind: `go test` over one function, the
// functional runner over one .ci, the lab's declared action over one scenario.
func carrierRunner(tree string, chosen target) (runner, error) {
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return nil, err
	}
	switch chosen.kind {
	case kindUnit:
		return func(overlayPath string) (bool, string, error) {
			return overlay.Exec(unitDeadline, overlay.UnitArgv(toolchain, chosen.pkg, chosen.name, overlayPath),
				overlay.UnitEnvironment(toolchain), tree, chosen.subject)
		}, nil
	case kindCI:
		return func(overlayPath string) (bool, string, error) {
			label := "redwalk-clean"
			if overlayPath != "" {
				label = "redwalk-broken"
			}
			selector := append(chosen.suite.OneTestWords(), chosen.name)
			run, err := overlay.PrepareFunctional(toolchain, label, overlayPath, selector)
			if err != nil {
				return false, "", &buildError{err: err}
			}
			defer run.Release()

			return overlay.Exec(carrierDeadline, run.Argv, run.Environ, tree, chosen.subject)
		}, nil
	case kindScenario:
		self, err := os.Executable()
		if err != nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("cannot resolve this le binary, which runs the lab's action: ").
				Err(err).String())
		}
		return func(overlayPath string) (bool, string, error) {
			return overlay.Exec(carrierDeadline, overlay.ActionArgv(self, chosen.catalog.Action),
				overlay.ProcessEnvironment(overlayPath, chosen.catalog.SelectorVariable, chosen.name),
				tree, chosen.subject)
		}, nil
	case kindUnspecified:
		panic("BUG: redwalk carrier unspecified: every target is parsed with its carrier")
	}
	panic("BUG: redwalk carrier out of range")
}
