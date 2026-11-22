// Design: docs/contributing/ze-go-style.md -- fast Go style check for main thread and subagents
// Overview: style.go -- shared diff-scoped formatting and AST style checks
// Related: register.go -- le area registration
package gostyle

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	archcompoundguard "github.com/ze-software/ze/internal/le/arch/compoundguard"
	archerrorwrap "github.com/ze-software/ze/internal/le/arch/errorwrap"
	archrefusebydefault "github.com/ze-software/ze/internal/le/arch/refusebydefault"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// area is the name this command is typed as.
const area = "go style"

// gitShowTimeout bounds a single git show HEAD:<path> read.
const gitShowTimeout = 10 * time.Second

// actions is the whole command surface. It stays light (Heavy: false) so both
// the main thread and ze-work subagents can run it before handback alongside
// gopls check without competing for heavy job slots.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "run fast AST Go style checks on changed or named files",
		Why:     "check gofmt, local import grouping, forbidden Go patterns, compound guards, refuse-by-default shapes and duplicate error wraps in milliseconds before handback or commit",
		Parameters: []leaction.Parameter{
			{
				Keyword:     "path",
				Kind:        leaction.KindString,
				Value:       "file",
				Help:        "Go file path to check against HEAD",
				Requirement: leaction.Optional,
				Repeat:      true,
			},
		},
		AnswerArgs: runCheck,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le go style` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// Issue is one style finding reported by `le go style check`.
type Issue struct {
	Check   string `json:"check"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// Report is the answer of one `le go style check` run.
type Report struct {
	Scope  string  `json:"scope"`
	Files  int     `json:"files"`
	Issues []Issue `json:"issues"`
}

// Text renders the scope summary and every issue, or OK when clean.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("go style scope: ").Str(r.Scope).Str(", ").Int(int64(r.Files)).Str(" Go file(s)\n")
	if len(r.Issues) == 0 {
		return tb.Str("go style: OK\n").String()
	}
	tb.Str("go style: ").Int(int64(len(r.Issues))).Str(" issue(s):\n")
	for _, issue := range r.Issues {
		tb.Str("  [").Str(issue.Check).Str("] ").Str(issue.File).Byte(':').Int(int64(issue.Line)).
			Str(": ").Str(issue.Message).Byte('\n')
	}
	tb.Str("See docs/contributing/ze-go-style.md.\n")
	return tb.String()
}

func (r Report) exitCode() int {
	if len(r.Issues) > 0 {
		return 1
	}
	return 0
}

func runCheck(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(
			err,
			"cannot locate the Ze checkout root",
			"go style check resolves repository paths and git history from the checkout root",
			"cd into the Ze checkout and rerun `./le go style check`",
		))
		return nil, 2
	}
	paths := arguments.Values("path")
	if len(paths) > 0 {
		report, checkErr := CheckTreePaths(root, paths)
		if checkErr != nil {
			leaction.ReportError(opererr.Wrap(
				checkErr,
				"go style check could not inspect the named path(s)",
				"every named path must exist inside the checkout and parse as Go",
				"verify the path(s) exist inside this checkout and rerun `./le go style check path <file>`",
			))
			return nil, 2
		}
		return report, report.exitCode()
	}
	changed, base, err := repochanged.LinesSinceUpstream(root)
	if err != nil {
		leaction.ReportError(opererr.Wrap(
			err,
			"go style check could not resolve changed lines since upstream",
			"without an upstream merge base the check cannot tell which lines changed",
			"run `git fetch` or pass explicit files with `./le go style check path <file>`",
		))
		return nil, 2
	}
	report, err := CheckChangedTree(root, changed, base)
	if err != nil {
		leaction.ReportError(opererr.Wrap(
			err,
			"go style check could not analyze the changed Go files",
			"one or more changed files could not be read or parsed",
			"fix syntax or readability errors in the changed files and rerun `./le go style check`",
		))
		return nil, 2
	}
	return report, report.exitCode()
}

// CheckTreePaths runs the fast AST style checks on the named paths under root,
// diffing each against HEAD (or treating an untracked file as all-added).
func CheckTreePaths(root string, rawPaths []string) (Report, error) {
	var rels []string
	changed := repochanged.ChangedLines{}
	beforeByPath := make(map[string]string, len(rawPaths))
	afterByPath := make(map[string]string, len(rawPaths))

	for _, raw := range rawPaths {
		rel, err := checkoutRel(root, raw)
		if err != nil {
			return Report{}, err
		}
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		if slices.Contains(rels, rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // user-named checkout file validated by checkoutRel
		if err != nil {
			return Report{}, err
		}
		after := string(data)
		before, err := gitBlobAt(root, "HEAD", rel)
		if err != nil {
			return Report{}, err
		}
		rels = append(rels, rel)
		beforeByPath[rel] = before
		afterByPath[rel] = after
		changed[rel] = lineSpansFromAdded(AddedLineNumbers(before, after))
	}
	slices.Sort(rels)
	return runAllChecks(root, "named paths against HEAD", rels, beforeByPath, afterByPath, changed)
}

// CheckChangedTree runs the fast AST style checks over changed lines since base.
func CheckChangedTree(root string, changed repochanged.ChangedLines, base repochanged.LineBase) (Report, error) {
	var rels []string
	beforeByPath := make(map[string]string)
	afterByPath := make(map[string]string)
	for rel := range changed {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		if strings.Contains(rel, "/testdata/") {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(full) //nolint:gosec // checkout-relative path from git diff
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Report{}, err
		}
		before, err := gitBlobAt(root, base.Commit, rel)
		if err != nil {
			return Report{}, err
		}
		rels = append(rels, rel)
		beforeByPath[rel] = before
		afterByPath[rel] = string(data)
	}
	slices.Sort(rels)
	scope := "working tree against " + base.Commit
	return runAllChecks(root, scope, rels, beforeByPath, afterByPath, changed)
}

func runAllChecks(
	root string,
	scope string,
	rels []string,
	beforeByPath map[string]string,
	afterByPath map[string]string,
	changed repochanged.ChangedLines,
) (Report, error) {
	report := Report{Scope: scope, Files: len(rels), Issues: []Issue{}}
	if len(rels) == 0 {
		return report, nil
	}

	for _, rel := range rels {
		after := afterByPath[rel]
		if IsGenerated(after) {
			continue
		}
		if fmtHit := CheckFormat(rel, after); fmtHit != nil {
			report.Issues = append(report.Issues, Issue{
				Check:   "format",
				File:    fmtHit.Path,
				Line:    fmtHit.Line,
				Message: fmtHit.Err.Error(),
			})
		}
		for _, hit := range CheckAdded(rel, beforeByPath[rel], after, Options{CheckDesignHeader: true}) {
			if hit.Severity != SeverityBlock {
				continue
			}
			report.Issues = append(report.Issues, Issue{
				Check:   "go-style",
				File:    hit.Path,
				Line:    hit.Line,
				Message: hit.Err.Error(),
			})
		}
	}

	cgReport, err := archcompoundguard.Check(root, changed)
	if err != nil {
		return Report{}, err
	}
	for _, f := range cgReport.Findings {
		report.Issues = append(report.Issues, Issue{
			Check:   "compound-guard",
			File:    f.File,
			Line:    f.Line,
			Message: "(" + f.Fn + ") splittable `if a || b { leave }` -- split into one guard per fact",
		})
	}

	rbdReport, err := archrefusebydefault.Check(root, changed)
	if err != nil {
		return Report{}, err
	}
	for _, f := range rbdReport.Findings {
		report.Issues = append(report.Issues, Issue{
			Check:   "refuse-by-default",
			File:    f.File,
			Line:    f.Line,
			Message: "(" + f.Fn + ") " + string(f.Rule) + ": " + f.Detail,
		})
	}

	ewReport, err := archerrorwrap.Check(root, changed)
	if err != nil {
		return Report{}, err
	}
	for _, f := range ewReport.Findings {
		var tb textbuf.Buffer
		msg := tb.Byte('(').Str(f.Fn).Str(") repeats \"").Str(f.Segment).Str("\" already written by ").
			Str(f.OriginFn).Str(" at ").Str(f.OriginFile).Byte(':').Int(int64(f.OriginLine)).String()
		report.Issues = append(report.Issues, Issue{
			Check:   "error-wrap",
			File:    f.File,
			Line:    f.Line,
			Message: msg,
		})
	}

	return report, nil
}

func checkoutRel(root, raw string) (string, error) {
	cleaned := filepath.Clean(raw)
	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Join(root, cleaned)
	}
	rel, err := filepath.Rel(root, cleaned)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." {
		return "", errors.New("path lies outside checkout: " + raw)
	}
	if strings.HasPrefix(rel, "../") {
		return "", errors.New("path lies outside checkout: " + raw)
	}
	return rel, nil
}

// gitBlobAt returns the file contents at rev:rel, or "" when the file does not
// exist at rev (untracked or newly added).
func gitBlobAt(root, rev, rel string) (string, error) {
	if rev == "" {
		return "", nil
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitShowTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "show", rev+":"+rel) //nolint:gosec // rev and checkout-relative path inspected with git show
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if _, isExit := errors.AsType[*exec.ExitError](err); isExit {
			// Missing at rev (new/untracked file): treat every line as newly added.
			return "", nil
		}
		return "", err
	}
	return string(out), nil
}

func lineSpansFromAdded(added map[int]bool) []repochanged.LineSpan {
	if len(added) == 0 {
		return nil
	}
	lines := make([]int, 0, len(added))
	for line := range added {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	spans := make([]repochanged.LineSpan, 0, len(lines))
	start := lines[0]
	prev := lines[0]
	for _, line := range lines[1:] {
		if line == prev+1 {
			prev = line
			continue
		}
		spans = append(spans, repochanged.LineSpan{From: start, To: prev})
		start = line
		prev = line
	}
	spans = append(spans, repochanged.LineSpan{From: start, To: prev})
	return spans
}
