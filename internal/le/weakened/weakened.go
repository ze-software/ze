// Design: scripts/dev/check_weakened_tests.py -- HEAD/worktree comparison and pairing
// Related: ledger.go -- the fail-closed contract reader.
// Related: detector.go, scope.go -- verdict production and unit naming.
package weakened

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

// Request is the exact change population the check judges. Paths and Removed
// come from one commit, not from a scan of the shared worktree.
type Request struct {
	Root    string   `json:"root"`
	Paths   []string `json:"paths,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Anchor  string   `json:"anchor,omitempty"`
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
		result.Anchor = "HEAD"
	}
	given := uniquePaths(request.Paths, request.Removed)
	result.PathsGiven = len(given)
	for _, path := range given {
		if isTestPath(path) {
			result.PathsJudged++
		}
	}
	findings, errors := weakenedTests(request.Root, request.Paths, request.Removed, result.Anchor)
	result.Findings = findings
	if len(errors) != 0 {
		result.Problems = errors
		return result
	}
	if len(findings) == 0 {
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
	result.Problems = slices.Concat(parseProblems, unmatchedProblems(parsed, findings))
	return result
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

func weakenedTests(root string, paths, removed []string, anchor string) ([]Finding, []string) {
	findings := make([]Finding, 0)
	problems := make([]string, 0)
	seen := make(map[string]bool, len(paths)+len(removed))
	removedSet := make(map[string]bool, len(removed))
	var text textbuf.Buffer
	for _, path := range removed {
		removedSet[path] = true
	}
	for _, path := range slices.Concat(paths, removed) {
		if seen[path] || !isTestPath(path) {
			continue
		}
		seen[path] = true
		oldText, problem := baselineText(root, path, anchor)
		if problem != "" {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).Str(problem).String())
			continue
		}
		if strings.TrimSpace(oldText) == "" {
			continue
		}
		newText := ""
		if !removedSet[path] {
			newText = worktreeText(root, path)
		}
		packageDir := filepath.Dir(path)
		packageName := filepath.Base(packageDir)
		if packageDir == "." {
			packageName = ""
		}
		for _, verdict := range weakenedUnits(path, oldText, newText) {
			findings = append(findings, Finding{
				Path: path, Package: packageName, Name: verdict.name, Details: verdict.details,
			})
		}
	}
	return findings, problems
}

func baselineText(root, path, anchor string) (string, string) {
	var text textbuf.Buffer
	verified, started := gitStatus(root, "rev-parse", "--verify",
		text.Str(anchor).Str("^{commit}").Slice())
	if !started {
		return "", "git could not start, so nothing was compared"
	}
	if verified != 0 {
		return "", text.Reset().Str(anchor).
			Str(" does not resolve to a commit, so nothing was compared").String()
	}
	present, started := gitStatus(root, "cat-file", "-e",
		text.Reset().Str(anchor).Byte(':').Str(path).Slice())
	if !started {
		return "", "git could not start, so nothing was compared"
	}
	if present != 0 {
		return "", ""
	}
	stdout, stderr, code, started := gitCapture(root, "show",
		text.Reset().Str(anchor).Byte(':').Str(path).Slice())
	if !started {
		return "", "git could not start, so nothing was compared"
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
		if len(hits) > 1 {
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
	at := strings.LastIndexByte(rowName, '.')
	if at < 0 {
		return finding.Name == rowName
	}
	return finding.Name == rowName[at+1:] && finding.Package == rowName[:at]
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
