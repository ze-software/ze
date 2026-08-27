// Design: ai/rules/git-safety.md -- how wide the uncommitted tree is
//
// Package workingtree reports the uncommitted paths of a checkout, grouped by
// the area a reader thinks in.
//
// Ze commits one logical change at a time. The failure this exists to surface
// is not a big diff: it is SEVERAL FINISHED chunks held in one tree, where a
// checkout destroys them and every later chunk has to be diffed around them.
//
// It is advisory by default and says so by exiting 0, because only a person can
// say whether two areas are one logical change. `le working-tree max-areas N`
// makes it a gate for a caller that wants one.
//
// Detail: report.go holds the answer, register.go the registration.
package workingtree

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// name is the word this command is typed as, and ceilingKeyword is the one
// keyword it takes. A value never stands alone (the CLI rule: keyword before
// value).
const (
	name           = "working-tree"
	ceilingKeyword = "max-areas"

	// buildArea is named once because three prefixes carry it: the make
	// fragments, the Makefile itself and the linter's configuration are one
	// chunk to a reader.
	buildArea = "build"
)

// areas maps a path prefix to the area a reader thinks in. FIRST MATCH WINS, so
// the specific prefixes come before the general ones and the order of this
// table is behavior rather than style.
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
	{"scripts/evidence/", "evidence-tools"},
	{"scripts/", "tooling"},
	{"mk/", buildArea},
	{"Makefile", buildArea},
	{".golangci.yml", buildArea},
	{"pkg/plugin/", "plugin-sdk"},
	{"internal/component/bgp/", "bgp"},
	{"internal/component/plugin/", "plugin-engine"},
	{"internal/component/command/", "cli-command"},
	{"internal/", "internal"},
	{"cmd/", "cmd"},
}

// AreaOf answers the area a path belongs to, or "other" when no prefix claims
// it.
func AreaOf(path string) string {
	for _, area := range areas {
		if strings.HasPrefix(path, area.Prefix) {
			return area.Name
		}
	}
	return "other"
}

// ParsePorcelain answers the changed paths of `git status --porcelain`.
//
// The format is two status characters, a space, then the path. A rename carries
// "old -> new", and the NEW name is what a commit would name, so that is the
// one kept.
func ParsePorcelain(out string) []string {
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
	return ParsePorcelain(string(out)), nil
}

// Group builds the report from the changed paths and the ceiling asked for.
//
// The areas are ordered by size and then by name, which is the order a reader
// needs: the widest chunk is the one to land first.
func Group(paths []string, maxAreas int) Report {
	report := Report{Paths: len(paths), MaxAreas: maxAreas}

	byArea := make(map[string][]string, len(areas))
	for _, path := range paths {
		area := AreaOf(path)
		byArea[area] = append(byArea[area], path)
	}

	report.Areas = make([]Area, 0, len(byArea))
	for area, files := range byArea {
		sort.Strings(files)
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

// Answer is the `le working-tree` command.
//
// The one thing it takes is a ceiling, as a keyword and a value. A bare number
// is refused: `ai/rules/cli.md` puts a keyword before every value, and a tool
// that guesses at a lone positional is where that rule stops holding.
func Answer(args []string) (any, int) {
	maxAreas, code := parseCeiling(args)
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

// parseCeiling reads the one keyword this command takes. It answers the ceiling
// and 0, or 0 and the exit code of a refusal already printed.
//
// A negative ceiling is refused rather than treated as a ceiling of zero. The
// script this ports read it with argparse and let it through, where every run
// then failed: a ceiling nothing can satisfy is a caller's mistake, and saying
// so is what tells them which way to fix it.
func parseCeiling(args []string) (int, int) {
	if len(args) == 0 {
		return 0, 0
	}

	if args[0] != ceilingKeyword {
		return 0, refuse(usageLine(), "no such keyword: ", args[0])
	}
	if len(args) == 1 {
		return 0, refuse(usageLine(), ceilingKeyword, " takes a number")
	}
	if len(args) > 2 {
		return 0, refuse(usageLine(), ceilingKeyword, " takes one number")
	}

	ceiling, err := strconv.Atoi(args[1])
	if err != nil || ceiling < 0 {
		return 0, refuse(usageLine(), "not a number of areas: ", args[1])
	}
	return ceiling, 0
}

// usageLine is the one line a refusal prints under itself.
func usageLine() string {
	var tb textbuf.Buffer
	return tb.Str("usage: le ").Str(name).Byte(' ').Str(ceilingKeyword).
		Str(" <n> [| json | yaml | table]").String()
}

// refuse prints one refusal and answers the exit code for it.
//
// The message arrives in two pieces, what was wrong and the word it was wrong
// about, so the caller states the fact rather than handing over a line it
// formatted itself.
func refuse(usage, what, word string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(name).Str(": ").Str(what).Str(word).String()) //nolint:errcheck // CLI output
	fmt.Fprintln(os.Stderr, usage)                                                              //nolint:errcheck // CLI output
	return 1
}
