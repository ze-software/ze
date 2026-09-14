// Design: docs/architecture/core-design.md -- the enumeration gate, as one command
//
// Overview: enumeration.go -- the walk each action runs
//
// actions.go is the TABLE, and the two exit conventions the area answers with.
// Dispatch, the listing, the help line and the refusals live in
// internal/le/leaction.
//
// The three codes stay apart: 0 for a tree that copies nothing, 1 for one that
// does, and 2 for a run the gate could not make -- a registry that did not
// answer, or a walk that read nothing. A caller that reads them apart keeps
// reading them apart.

package enumeration

import (
	"errors"
	"fmt"
	pathpkg "path"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/le/changed"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each action name to derive its verb.
const area = "enumeration"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   "check",
		Why:    "no literal this change set introduces enumerates what a live registry already holds, so the class stops growing while the copies already in the tree stay visible in report",
		Answer: runCheck,
	},
	leaction.Action{
		Verb:   "report",
		Why:    "every literal in the tree that restates a registry, counted by the registry each one copies, for a reader deciding which copy to end first",
		Answer: runReport,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le enumeration` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck is the `le enumeration check` action. It blocks on what the change
// set introduces, and nothing else.
//
// The tree holds 230 copies this gate believes in, which is more than anybody
// can mark before one is fixed, and marking them would make the gate's first
// state mostly suppression (owner decision, 2026-09-14). Judging the change set
// stops the class GROWING, which is what the gate is for, and `report` keeps
// the backlog in the open rather than behind markers.
//
// A gated row is answered and not counted: it is in the report the caller
// reads, and it is zero findings for the exit code, because the test it names
// is what proves the row (report.go, KindGated).
func runCheck() (any, int) {
	found, code := walkCheckout()
	if code != 0 {
		return nil, code
	}
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	scope, scopeCode := changed.Packages(tree)
	if scopeCode != 0 {
		leaction.ReportError(fmt.Errorf("%w: the change-set selector exited %d", ErrNoChangeSet, scopeCode))
		return nil, 2
	}
	report, err := inChangeSet(found, scope, func() ([]string, error) { return changed.WorkingTreePaths(tree) })
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.exitCode()
}

// runReport is the `le enumeration report` action. It answers every row in the
// tree and exits 0 on them, because a reader deciding what to fix is not a
// gate.
//
// A run the gate could not make still exits 2 here. That code says the walk did
// not happen, which is not a finding count of zero.
func runReport() (any, int) {
	found, code := walkCheckout()
	if code != 0 {
		return nil, code
	}
	return CheckReport{Scope: "every Go file in the tree", Findings: found}, 0
}

// ErrNoChangeSet names a change set the gate could not read, or one that
// selected nothing.
//
// An empty selection is an error rather than a clean run for the reason this
// whole gate exists: a scoped check that scopes to zero passes everything, and
// it prints what a tree with no new copies prints. A change set is empty only
// when the selector failed, because a checkout with nothing changed still
// answers the packages it was asked about (internal/le/changed, Scope.Resolve).
var ErrNoChangeSet = errors.New("the change set selected nothing")

// inChangeSet answers the findings the change set covers, and says which route
// answered.
//
// Two routes, and which one runs is the selector's call rather than this
// gate's. When the selector says what MOVED, the findings are filtered by
// package, which is the unit it answers in. When it has WIDENED, it is saying
// it cannot tell, and this gate does not read that as "everything is new": the
// tree holds 230 copies that predate every session, and judging them on every
// run is whole-tree blocking wearing a change-set label. So the second route
// asks the same question a different way, from the working tree against HEAD,
// which needs no green baseline and excludes the backlog by construction.
//
// Failing both is exit 2, never a pass. A gate that cannot tell what moved and
// answers "nothing" is the silent zero this gate is named after.
func inChangeSet(found Findings, scope changed.ScopeReport, workingTree func() ([]string, error)) (CheckReport, error) {
	widened, reason := widening(scope)
	if !widened {
		if len(scope.Packages) == 0 {
			return CheckReport{}, fmt.Errorf("%w: %s", ErrNoChangeSet, scope.Reason)
		}
		prefixes := make([]string, 0, len(scope.Packages))
		for _, selected := range scope.Packages {
			prefixes = append(prefixes, packagePrefix(selected))
		}
		scoped := make(Findings, 0, len(found))
		for _, finding := range found {
			if !anyPrefix(prefixes, pathpkg.Dir(finding.File)) {
				continue
			}
			scoped = append(scoped, finding)
		}
		return CheckReport{Scope: scopePackages(len(scope.Packages)), Findings: scoped}, nil
	}

	paths, err := workingTree()
	if err != nil {
		return CheckReport{}, fmt.Errorf("%w: the selector widened, so the working tree against HEAD is the only remaining answer, and it could not be read: %w",
			ErrNoChangeSet, err)
	}
	changedFile := make(map[string]bool, len(paths))
	for _, path := range paths {
		changedFile[path] = true
	}
	scoped := make(Findings, 0, len(found))
	for _, finding := range found {
		if !changedFile[finding.File] {
			continue
		}
		scoped = append(scoped, finding)
	}
	return CheckReport{Scope: scopeWorkingTree(len(changedFile), reason), Findings: scoped}, nil
}

// widening answers whether the selector could tell what moved, and why not.
//
// The flag alone is not the fact, because it does not survive the route this
// gate takes inside a verify run. A run selects the change set ONCE and
// publishes the PACKAGES to a file (changed.WriteScopePackages), which carries
// no Widened column, so changed.Scope.fromFile hands a widened answer back with
// the flag clear. Every stage of the run then reads `./...` as a narrow answer.
//
// The packages survive that round trip and the flag does not, and the two state
// one fact: `./...` is written by widen (internal/le/changed/scope.go) and by
// failOpen (internal/le/changed/selector.go), and by nothing else. So the
// selection is read for the answer rather than the flag believed.
//
// Reading it the other way is the whole defect this gate's fallback exists to
// prevent. A checkout with no green baseline widens on every run, the gate then
// judges the 230 copies that predate every session, and a gate that is red for
// every session gets disarmed.
func widening(scope changed.ScopeReport) (bool, string) {
	if scope.Widened {
		return true, scope.Reason
	}
	for _, selected := range scope.Packages {
		if packagePrefix(selected) != "" {
			continue
		}
		return true, "the published package answer covers the whole checkout, which is what the selector writes when it cannot tell what moved"
	}
	return false, scope.Reason
}

// scopePackages says the check judged the selector's package answer.
func scopePackages(packages int) string {
	var tb textbuf.Buffer
	return tb.Str("the change set the selector answered: ").Int(int64(packages)).Str(" package(s)").String()
}

// scopeWorkingTree says the check fell back to the working tree, and why. The
// reason is the selector's own words, so a reader is told what widened it.
func scopeWorkingTree(files int, reason string) string {
	var tb textbuf.Buffer
	tb.Str("the working tree against HEAD: ").Int(int64(files)).
		Str(" changed file(s). The package selector widened to every package, so it could not say what moved")
	if reason != "" {
		tb.Str(" (").Str(reason).Byte(')')
	}
	return tb.String()
}

// packagePrefix turns one selector entry into the directory prefix it covers.
// `./...` covers the checkout, `./internal/foo/...` covers a subtree, and
// `./internal/foo` covers one package.
func packagePrefix(selected string) string {
	trimmed := strings.TrimPrefix(selected, "./")
	trimmed = strings.TrimSuffix(trimmed, "...")
	return strings.TrimSuffix(trimmed, "/")
}

// anyPrefix reports whether pkgDir is at or under one of the prefixes.
//
// A prefix of "" covers the whole checkout and never reaches here: widening
// reads that selection as the selector saying it cannot tell what moved, and
// the working tree answers instead. Matching on it here would be the match-all
// arm the fallback exists to prevent, so it is not written.
func anyPrefix(prefixes []string, pkgDir string) bool {
	for _, prefix := range prefixes {
		if prefix == "" {
			continue
		}
		if pkgDir == prefix || strings.HasPrefix(pkgDir, prefix+"/") {
			return true
		}
	}
	return false
}

// walkCheckout runs the gate over the real checkout.
func walkCheckout() (Findings, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	corpora, err := readCorpora()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return walkTree(tree, corpora, scanFloor)
}

// walkTree runs both walks and answers their rows, or 2 with the reason already
// reported.
//
// The three arguments are arguments rather than constants because each one is a
// way the run can come back empty, and a test proves each refusal on this path:
// a corpus that did not answer, and a walk that read nothing.
func walkTree(tree string, corpora []Corpus, floor int) (Findings, int) {
	if err := checkFloors(corpora); err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	found, err := Check(tree, productRoots(), corpora, floor)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	handCalled, err := handCalledDoctorChecks(tree, doctorCheckFloor)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	found = append(found, handCalled...)
	found.sort()
	return found, 0
}
