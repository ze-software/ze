// Design: ai/rules/git-safety.md -- how wide the uncommitted tree is
//
// Package repoworkingtree reports the uncommitted paths of a checkout, grouped by
// the area a reader thinks in.
//
// Ze commits one logical change at a time. The failure this exists to surface
// is not a big diff: it is SEVERAL FINISHED chunks held in one tree, where a
// checkout destroys them and every later chunk has to be diffed around them.
//
// It is advisory by default and says so by exiting 0, because only a person can
// say whether two areas are one logical change. `le repo working-tree max-areas N`
// makes it a gate for a caller that wants one.
//
// Detail: report.go holds the answer, register.go the registration.
package repoworkingtree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// name is the word this command is typed as, and ceilingKeyword is the one
// keyword it takes. A value never stands alone (the CLI rule: keyword before
// value).
const (
	name           = "repo working-tree"
	ceilingKeyword = "max-areas"

	// buildArea is named once because three prefixes carry it: the make
	// fragments, the Makefile itself and the linter's configuration are one
	// chunk to a reader.
	buildArea = "build"
)

// areas maps a path prefix to the area a reader thinks in. FIRST MATCH WINS, so
// the specific prefixes come before the general ones and the order of this
// table is behavior rather than style.
// areaEvidenceTools names the group whose changes produce release evidence.
const areaEvidenceTools = "evidence-tools"

var areas = []struct {
	Prefix string
	Name   string
}{
	{"ai/rules/", "rules"},
	{"ai/", "ai-docs"},
	{"plan/journal/", "journal"},
	{"plan/audits/", "audits"},
	{"plan/", "specs"},
	{"docs/", "docs"},
	{"test/", "tests"},
	{"internal/le/test/deployment/", areaEvidenceTools},
	{"internal/le/verify/evidence/", areaEvidenceTools},
	{"internal/le/test/qemu/", areaEvidenceTools},
	{"internal/le/", "tooling"},
	{".golangci.yml", buildArea},
	{"pkg/plugin/", "plugin-sdk"},
	{"internal/component/bgp/", "bgp"},
	{"internal/component/plugin/", "plugin-engine"},
	{"internal/component/command/", "cli-command"},
	{"internal/", "internal"},
	{"cmd/", "cmd"},
}

// areaOf answers the area a path belongs to, or "other" when no prefix claims
// it.
func areaOf(path string) string {
	for _, area := range areas {
		if strings.HasPrefix(path, area.Prefix) {
			return area.Name
		}
	}
	return "other"
}

// parsePorcelain answers the changed paths of `git status --porcelain`.
//
// The format is two status characters, a space, then the path. A rename carries
// "old -> new", and the NEW name is what a commit would name, so that is the
// one kept.
func parsePorcelain(out string) []string {
	var paths []string
	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) <= 3 {
			continue
		}
		path := line[3:]
		if _, after, found := strings.Cut(path, " -> "); found {
			path = after
		}
		paths = append(paths, strings.Trim(strings.TrimSpace(path), `"`))
	}
	return paths
}

// Changed answers the tracked modifications plus the untracked files of tree,
// ignoring what git ignores.
func Changed(tree string) ([]string, error) {
	// The query is bounded by git itself rather than by a timeout. `git status`
	// over a checkout is local work with no network and no lock this process
	// waits behind, and a timeout would turn a slow filesystem into a report
	// that the tree is clean.
	cmd := exec.Command("git", "-C", tree, //nolint:gosec,noctx // a build tool queries the checkout it was pointed at
		"status", "--porcelain", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parsePorcelain(string(out)), nil
}

// Group builds the report from the changed paths and the ceiling asked for.
//
// The areas are ordered by size and then by name, which is the order a reader
// needs: the widest chunk is the one to land first.
func Group(paths []string, maxAreas int) Report {
	report := Report{Paths: len(paths), MaxAreas: maxAreas}

	byArea := make(map[string][]string, len(areas))
	for _, path := range paths {
		area := areaOf(path)
		byArea[area] = append(byArea[area], path)
	}

	report.Areas = make([]Area, 0, len(byArea))
	for area, files := range byArea {
		slices.Sort(files)
		report.Areas = append(report.Areas, Area{Area: area, Files: files})
	}
	sort.Slice(report.Areas, func(i, j int) bool {
		if len(report.Areas[i].Files) != len(report.Areas[j].Files) {
			return len(report.Areas[i].Files) > len(report.Areas[j].Files)
		}
		return report.Areas[i].Area < report.Areas[j].Area
	})
	return report
}

// reportVerb is the one action: the width of the uncommitted tree.
const reportVerb = "report"

// actions is the whole command surface, declared once, so the listing, the
// usage, the help pages and the parse read one grammar (internal/le/le/action).
// The one thing the report takes is a ceiling, as a keyword and a value. A bare
// number is refused: `ai/rules/cli.md` puts a keyword before every value, and a
// tool that guesses at a lone positional is where that rule stops holding.
var actions = leaction.New(name, leaction.Action{
	Verb:    reportVerb,
	Summary: "report how wide the uncommitted tree is",
	Why:     "how wide the uncommitted tree is, grouped by area. Advisory unless max-areas names a ceiling",
	Parameters: []leaction.Parameter{
		{Keyword: ceilingKeyword, Kind: leaction.KindUint, Value: "n", Requirement: leaction.Optional,
			Help: "the most areas the tree may span; more answers 1"},
	},
	AnswerArgs: answerReport,
})

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le repo working-tree` command. A line that does not open with
// the verb runs the report, as callers have always typed it (D-P3-bare in
// spec-le-cli-usability), so the verb is supplied here and the table reads the
// rest of the line: `le repo working-tree max-areas 3` is `report max-areas 3`.
// A refusal the table's parser makes answers 2 (D-P3-stray).
func Answer(args []string) (any, int) {
	if len(args) > 0 && actions.Holds(args[0]) {
		return actions.Answer(args)
	}
	return actions.Answer(append([]string{reportVerb}, args...))
}

// answerReport groups the uncommitted paths of the checkout and judges them
// against the ceiling, when the line names one.
func answerReport(args leaction.Arguments) (any, int) {
	maxAreas, code := ceilingOf(args)
	if code != 0 {
		return nil, code
	}

	tree, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: working-tree: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	paths, err := Changed(tree)
	if err != nil {
		// 2 rather than 1: the advisory could not read the tree, which a
		// caller reads apart from a tree that is too wide.
		fmt.Fprintf(os.Stderr, "error: working-tree: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	report := Group(paths, maxAreas)
	if report.Exceeded() {
		return report, 1
	}
	return report, 0
}

// ceilingOf reads the ceiling the line named. It answers the ceiling and 0, or
// 0 and the exit code of a refusal already printed. No ceiling is 0, which
// Group reads as advisory.
//
// A negative ceiling never reaches here: the table refuses a dash-leading
// value by name. A word that is not a number is refused here with 1, the code
// the hand-rolled parser answered, because the body's refusals keep their
// codes until Phase 5 (AC-17).
func ceilingOf(args leaction.Arguments) (int, int) {
	if !args.Has(ceilingKeyword) {
		return 0, 0
	}
	value := args.One(ceilingKeyword)
	ceiling, err := strconv.Atoi(value)
	if err != nil || ceiling < 0 {
		leaction.ReportError(errors.New(name + ": not a number of areas: " + value))
		return 0, 1
	}
	return ceiling, 0
}
