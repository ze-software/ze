// Design: docs/architecture/testing/test-health.md -- HEAD/worktree comparison and pairing
// Related: ledger.go -- the fail-closed contract reader.
// Related: detector.go, scope.go -- verdict production and unit naming.
package testweakened

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const cannotRunPrefix = "check could not run: "

// RenamePair is one old/new test path pair from the prospective commit's
// isolated index. Score is Git's similarity percentage.
type RenamePair struct {
	OldPath string `json:"old-path"`
	NewPath string `json:"new-path"`
	Score   int    `json:"score"`
}

// Request is the exact change population the check judges. Paths and Removed
// come from one commit, not from a scan of the shared worktree.
type Request struct {
	Root        string       `json:"root"`
	Paths       []string     `json:"paths,omitempty"`
	Removed     []string     `json:"removed,omitempty"`
	RenamePairs []RenamePair `json:"rename-pairs,omitempty"`
	Anchor      string       `json:"anchor,omitempty"`
}

// Finding is one weakened unit and the ordered detector verdicts for it.
type Finding struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Details []string `json:"details"`
}

// Result is the structured answer from Check.
type Result struct {
	Contract    string    `json:"contract"`
	Anchor      string    `json:"anchor,omitempty"`
	Rows        int       `json:"rows"`
	PathsGiven  int       `json:"paths-given"`
	PathsJudged int       `json:"paths-judged"`
	Findings    []Finding `json:"findings,omitempty"`
	Problems    []string  `json:"problems,omitempty"`
	Comparison  bool      `json:"comparison"`
}

// Check performs either the live no-path shape check or one exact HEAD/worktree
// comparison. Every fixture and the command action use this function.
func Check(request Request) Result {
	return check(request, true)
}

// CheckCommit judges a prospective commit. When ledgerCarried is false, rows
// present only in the shared worktree cannot accept the commit's weakening.
func CheckCommit(request Request, ledgerCarried bool) Result {
	return check(request, ledgerCarried)
}

func check(request Request, ledgerCarried bool) Result {
	result := Result{Contract: ContractPath}
	if request.Root == "" {
		var text textbuf.Buffer
		result.Problems = []string{text.Str(cannotRunPrefix).
			Str("the checkout root is empty").String()}
		return result
	}
	if len(request.Paths) == 0 && len(request.Removed) == 0 {
		rows, problems := readLedger(request.Root)
		result.Rows = len(rows)
		result.Problems = problems
		return result
	}

	result.Comparison = true
	result.Anchor = request.Anchor
	if result.Anchor == "" {
		result.Anchor = headRevision
	}
	given := uniquePaths(request.Paths, request.Removed)
	result.PathsGiven = len(given)
	for _, path := range given {
		if isTestPath(path) {
			result.PathsJudged++
		}
	}
	findings, errors := weakenedTests(
		request.Root, request.Paths, request.Removed, request.RenamePairs, result.Anchor,
	)
	result.Findings = findings
	if len(errors) != 0 {
		result.Problems = errors
		return result
	}
	if len(findings) == 0 {
		return result
	}

	if !ledgerCarried {
		result.Problems = unmatchedProblems(nil, findings)
		var text textbuf.Buffer
		result.Problems = append(result.Problems, text.Str("this commit weakens ").
			Int(int64(len(findings))).Str(" test(s) and does not carry ").
			Str(ContractPath).Str(". The row is in the working tree only, so git history ").
			Str("would hold the weakening with no reason beside it. Name the file too:\n    file ").
			Str(ContractPath).String())
		return result
	}
	rows, exists, readProblem := loadLedger(request.Root)
	if readProblem != "" {
		result.Problems = []string{readProblem}
		return result
	}
	if !exists {
		var page textbuf.Buffer
		page.Str(ContractPath).Str(" does not exist, and this commit weakens ").
			Int(int64(len(findings))).Str(" test(s). Write it:\n")
		for index, finding := range findings {
			if index != 0 {
				page.Byte('\n')
			}
			page.Str("    ").Str(rowToWrite(finding, false))
		}
		result.Problems = []string{page.String()}
		return result
	}
	parsed, parseProblems := parseLedger(rows, ContractPath)
	result.Rows = len(parsed)
	result.Problems = slices.Concat(parseProblems,
		liveScopeProblems(request.Removed, parsed, findings), unmatchedProblems(parsed, findings))
	return result
}

// liveScopeProblems refuses a path scope that covers a path this commit keeps.
//
// A scoped row states one reason for every finding under a path, which is
// honest only when the commit RETIRES that path: `scripts/**` covers a tree
// this commit deletes, so nothing can be weakened there again. Over a live
// tree the same row is an escape hatch. `internal/le/**` would parse, match
// every finding under the tooling tree, and accept each one silently, and no
// leftover-row pressure would remove it because it keeps matching.
//
// The test is the commit's own removal list rather than the filesystem: a
// retired tree still has a directory on disk until git prunes it, and a
// finding for a file this commit merely edits is exactly what must not be
// swallowed.
func liveScopeProblems(removed []string, rows []Row, findings []Finding) []string {
	retired := make(map[string]bool, len(removed))
	for _, path := range removed {
		retired[path] = true
	}
	problems := make([]string, 0)
	var text textbuf.Buffer
	for _, row := range rows {
		if !isScopedRowName(row.Name) {
			continue
		}
		for _, finding := range findings {
			scoped, matches := scopedRowMatches(row.Name, finding)
			if !scoped || !matches || retired[finding.Path] {
				continue
			}
			problems = append(problems, text.Reset().Str(ContractPath).Byte(':').
				Int(int64(row.Line)).Str(" scopes ").Str(row.Name).
				Str(", which covers ").Str(finding.Path).
				Str(", a path this commit keeps. A path scope states one reason for every ").
				Str("finding under it, which is honest only for a path the commit retires. ").
				Str("Name that test instead.").String())
			break
		}
	}
	return problems
}

// ExitCode preserves the producer's three-way verdict: clean, problem, or no
// comparison. A cannot-run result is never interpreted as a clean population.
func (r Result) ExitCode() int {
	for _, problem := range r.Problems {
		if strings.HasPrefix(problem, cannotRunPrefix) {
			return 2
		}
	}
	if len(r.Problems) != 0 {
		return 1
	}
	return 0
}

func (r Result) Text() string {
	var page textbuf.Buffer
	page.SetColor(slogutil.UseColor(os.Stdout))
	color := textbuf.C
	switch r.ExitCode() {
	case 0:
		if !r.Comparison {
			page.Colored(color.BrightGreen).Str("Weakened-test check: ").Str(ContractPath).
				Str(" parses (").Int(int64(r.Rows)).Str(" row(s)).").
				Colored(color.Reset).Byte('\n')
			return page.String()
		}
		page.Colored(color.BrightGreen).Str("Weakened-test check: clean (").
			Int(int64(r.PathsJudged)).Str(" of ").Int(int64(r.PathsGiven)).
			Str(" path(s) are tests, judged against ").Str(r.Anchor).Str(").").
			Colored(color.Reset).Byte('\n')
	case 2:
		page.Colored(color.BoldRed).Str("Weakened-test check: CANNOT RUN.").
			Colored(color.Reset).Byte('\n')
		for _, problem := range r.Problems {
			page.Str("  ").Str(problem).Byte('\n')
		}
	default:
		page.Colored(color.BoldRed).Str("Weakened-test check: ").Int(int64(len(r.Problems))).
			Str(" problem(s).").Colored(color.Reset).Str("\n\n")
		for _, problem := range r.Problems {
			page.Str("  ").Str(problem).Byte('\n')
		}
	}
	return page.String()
}

func weakenedTests(
	root string,
	paths, removed []string,
	renamePairs []RenamePair,
	anchor string,
) ([]Finding, []string) {
	findings := make([]Finding, 0)
	problems := make([]string, 0)
	seen := make(map[string]bool, len(paths)+len(removed))
	removedSet := make(map[string]bool, len(removed))
	var text textbuf.Buffer
	for _, path := range removed {
		removedSet[path] = true
	}
	for _, pair := range renamePairs {
		if seen[pair.OldPath] || seen[pair.NewPath] {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).
				Str("rename path appears in more than one pair, so no rename was compared").String())
			continue
		}
		seen[pair.OldPath] = true
		seen[pair.NewPath] = true
		pairFindings, problem := comparePath(root, pair.OldPath, pair.NewPath, anchor)
		if problem != "" {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).Str(problem).String())
			continue
		}
		findings = append(findings, pairFindings...)
	}
	for _, path := range slices.Concat(paths, removed) {
		if seen[path] || !isTestPath(path) {
			continue
		}
		seen[path] = true
		newPath := path
		if removedSet[path] {
			newPath = ""
		}
		pathFindings, problem := comparePath(root, path, newPath, anchor)
		if problem != "" {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).Str(problem).String())
			continue
		}
		findings = append(findings, pathFindings...)
	}
	return findings, problems
}

func comparePath(root, oldPath, newPath, anchor string) ([]Finding, string) {
	oldText, problem := baselineText(root, oldPath, anchor)
	if problem != "" {
		return nil, problem
	}
	if strings.TrimSpace(oldText) == "" {
		return nil, ""
	}
	newText := ""
	path := oldPath
	if newPath != "" {
		path = newPath
		newText = worktreeText(root, newPath)
	}
	packageDir := filepath.Dir(path)
	packageName := filepath.Base(packageDir)
	if packageDir == "." {
		packageName = ""
	}
	verdicts := weakenedUnits(path, oldText, newText)
	findings := make([]Finding, 0, len(verdicts))
	for _, verdict := range verdicts {
		findings = append(findings, Finding{
			Path: path, Package: packageName, Name: verdict.name, Details: verdict.details,
		})
	}
	return findings, ""
}

func baselineText(root, path, anchor string) (string, string) {
	var text textbuf.Buffer
	verified, started := gitStatus(root, "rev-parse", "--verify",
		text.Str(anchor).Str("^{commit}").Slice())
	if !started {
		return "", gitCannotStart
	}
	if verified != 0 {
		return "", text.Reset().Str(anchor).
			Str(" does not resolve to a commit, so nothing was compared").String()
	}
	present, missingDetail, presentCode, started := gitCapture(root, "ls-tree", "--name-only",
		anchor, "--", path)
	if !started {
		return "", gitCannotStart
	}
	if presentCode != 0 {
		return "", text.Reset().Str("git ls-tree ").Str(anchor).Str(" -- ").Str(path).
			Str(" failed: ").Str(strings.TrimSpace(missingDetail)).String()
	}
	if strings.TrimSpace(present) == "" {
		return "", ""
	}
	stdout, stderr, code, started := gitCapture(root, "show",
		text.Reset().Str(anchor).Byte(':').Str(path).Slice())
	if !started {
		return "", gitCannotStart
	}
	if code != 0 {
		return "", text.Reset().Str("git show ").Str(anchor).Byte(':').Str(path).
			Str(" failed: ").Str(strings.TrimSpace(stderr)).String()
	}
	return stdout, ""
}

func worktreeText(root, path string) string {
	content, err := readRepositoryFile(root, path)
	if err != nil {
		return ""
	}
	return string(content)
}

func readLedger(root string) ([]Row, []string) {
	ledger, exists, problem := loadLedger(root)
	if problem != "" {
		return nil, []string{problem}
	}
	if !exists {
		var text textbuf.Buffer
		return nil, []string{text.Str(ContractPath).
			Str(" is missing. The commit gate reads it, so a commit that weakens a test has nowhere to record the reason.").
			String()}
	}
	return parseLedger(ledger, ContractPath)
}

func loadLedger(root string) (string, bool, string) {
	content, err := readRepositoryFile(root, ContractPath)
	if err == nil {
		return string(content), true, ""
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", false, ""
	}
	var text textbuf.Buffer
	return "", false, text.Str(cannotRunPrefix).Str("cannot read ").Str(ContractPath).
		Str(": ").Err(err).String()
}

func readRepositoryFile(root, path string) ([]byte, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(path))
	closeErr := repository.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return content, nil
}

// distinctPackages counts the package names among the findings a row hit.
//
// It decides whether the ambiguity the caller found is one the OPERATOR can
// answer. A qualified row names package.Test, so it separates two findings
// only when their packages are named differently. Two packages can share a
// name (internal/component/iface/cli and internal/component/tacacs/cli are
// both `cli`), and then no row text separates them: parseLedger refuses a
// second row under the same name ("one test, one reason"), and a path scope
// is accepted only for a path the commit RETIRES, which a surviving test file
// is not. Demanding a row per package there is a demand nothing can meet, so
// one row answers both occurrences and its reason covers them.
func distinctPackages(findings []Finding, hits []int) int {
	seen := make(map[string]bool, len(hits))
	for _, hit := range hits {
		seen[findings[hit].Package] = true
	}
	return len(seen)
}

func unmatchedProblems(rows []Row, findings []Finding) []string {
	problems := make([]string, 0)
	claimed := make([]bool, len(findings))
	var text textbuf.Buffer
	for _, row := range rows {
		hits := make([]int, 0, 1)
		for index, finding := range findings {
			if rowMatches(row.Name, finding) {
				hits = append(hits, index)
			}
		}
		if len(hits) == 0 {
			problems = append(problems, text.Reset().Str(ContractPath).Byte(':').Int(int64(row.Line)).
				Str(" names ").Str(row.Name).
				Str(", which this commit does not weaken. A row left over from the last commit accepts nothing here; delete it.").
				String())
			continue
		}
		// A scoped row matching many findings is the feature, not an
		// ambiguity: that is what lets one row explain a whole retired tree.
		// Only an exact-name row matching more than one finding is a
		// collision (the same test name in two packages), and only that
		// shape needs the operator to write one qualified row per package.
		if len(hits) > 1 && !isScopedRowName(row.Name) && distinctPackages(findings, hits) > 1 {
			text.Reset().Str(ContractPath).Byte(':').Int(int64(row.Line)).
				Str(" names ").Str(row.Name).
				Str(", which this commit weakens in ").Int(int64(len(hits))).Str(" packages: ")
			for index, hit := range hits {
				if index != 0 {
					text.Str(", ")
				}
				finding := findings[hit]
				text.Str(finding.Package).Str(" (").Str(finding.Path).Byte(')')
			}
			text.Str(".\n    Write package.TestName, one row each:")
			for _, hit := range hits {
				text.Str("\n    ").Str(rowToWrite(findings[hit], true))
			}
			problems = append(problems, text.String())
		}
		// A finding claimed by both a scoped row and an exact row is not an
		// error: each row's hits are computed independently over the full
		// finding population, so neither row can be made to look empty (and
		// so fail the leftover-row check) by the other row's existence, and
		// the finding is simply claimed twice, which claimed[] treats as one.
		for _, hit := range hits {
			claimed[hit] = true
		}
	}
	for index, finding := range findings {
		if claimed[index] {
			continue
		}
		text.Reset().Str(finding.Path).Str(" weakens ").Str(finding.Name).Str(" and ").
			Str(ContractPath).Str(" has no row for it:\n")
		for _, detail := range finding.Details {
			text.Str("    - ").Str(detail).Byte('\n')
		}
		qualify := findingNameCount(findings, finding.Name) > 1
		text.Str("    Add the row, then commit the file with the change:\n    ").
			Str(rowToWrite(finding, qualify))
		problems = append(problems, text.String())
	}
	return problems
}

func rowMatches(rowName string, finding Finding) bool {
	if scoped, matches := scopedRowMatches(rowName, finding); scoped {
		return matches
	}
	at := strings.LastIndexByte(rowName, '.')
	if at < 0 {
		return finding.Name == rowName
	}
	return finding.Name == rowName[at+1:] && finding.Package == rowName[:at]
}

// scopedRowMatches reports whether rowName is a path-scoped row and, if it
// is, whether it covers finding.Path. A row Name containing a slash is
// always a path scope, never a test name: a Go test identifier can never
// contain one, and a package-qualified name (pkg.Name) never does either, so
// the two grammars cannot collide. Two forms: a bare path
// ("internal/le/pylint/pylint_test.go") matches that one file exactly, for a
// migration that retires a whole file's worth of findings under one honest
// reason; a path ending "/**" ("scripts/**") matches every finding whose
// Path sits inside that tree, for a migration that retires the tree itself.
func scopedRowMatches(rowName string, finding Finding) (scoped, matches bool) {
	if !strings.Contains(rowName, "/") {
		return false, false
	}
	if tree, isTree := strings.CutSuffix(rowName, "/**"); isTree {
		return true, strings.HasPrefix(finding.Path, tree+"/")
	}
	return true, finding.Path == rowName
}

// isScopedRowName reports whether name uses the path-scope grammar rather
// than the exact test-name grammar. See scopedRowMatches.
func isScopedRowName(name string) bool {
	return strings.Contains(name, "/")
}

func rowToWrite(finding Finding, qualify bool) string {
	var text textbuf.Buffer
	text.Str("| ")
	if qualify {
		text.Str(finding.Package).Byte('.')
	}
	return text.Str(finding.Name).
		Str(" | <what left the suite, and why the commit is correct without it> |").String()
}

func findingNameCount(findings []Finding, name string) int {
	count := 0
	for _, finding := range findings {
		if finding.Name == name {
			count++
		}
	}
	return count
}

func isTestPath(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	if (strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et")) &&
		(strings.HasPrefix(path, "test/") || strings.Contains(path, "/test/")) {
		return true
	}
	return isPythonTest(path)
}

func uniquePaths(first, second []string) []string {
	paths := make([]string, 0, len(first)+len(second))
	seen := make(map[string]bool, len(first)+len(second))
	for _, path := range slices.Concat(first, second) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func gitStatus(root string, args ...string) (int, bool) {
	_, _, code, started := gitCapture(root, args...)
	return code, started
}

func gitCapture(root string, args ...string) (string, string, int, bool) {
	// #nosec G204 -- Git is the fixed executable and every call site supplies one of this package's closed verbs; path and ref operands remain distinct argv, never shell code.
	command := exec.CommandContext(context.Background(), "git", args...)
	command.Dir = root
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0, true
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return stdout.String(), stderr.String(), exit.ExitCode(), true
	}
	return stdout.String(), err.Error(), -1, false
}

// The revision this detector compares against, and the two reasons it reports
// when git answers nothing it can compare.
const (
	headRevision        = "HEAD"
	gitCannotStart      = "git could not start, so nothing was compared"
	gitRenameUnreadable = "Git returned malformed or ambiguous rename status, so no rename was compared"
)
