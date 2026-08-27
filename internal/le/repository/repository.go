// Design: docs/architecture/core-design.md -- the post-verify repository checks
//
// Package repository is the post-verify validation gate: five checks, each
// derived from a documented defect pattern in
// plan/learned/RECURRING-PATTERNS.md.
//
// Three checks read the complete tree. They find numbered source anchors, stale
// source paths, and acceptance criteria without demonstrations. Two checks read
// only session changes. They find uncalled exported symbols and CLI commands
// without .ci coverage.
//
// The two gates select between those scopes. `le repository check` gets the
// changed set from git and judges a developer's tree. `le repository
// tree-check` declares an EMPTY changed set and runs inside the pre-commit gate.
// Changed-file checks require complete files, but several sessions share this
// checkout. Verify CAN otherwise fail work from another session.
//
// Detail: wiring.go -- the cross-package caller search
// Detail: report.go -- what the checks answer
package repository

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// These patterns match the script character for character. Thus, the migration
// compares each with the Python module's compiled pattern.
const (
	// SourceAnchorPattern reads the path out of a `<!-- source: ... -->`
	// anchor.
	SourceAnchorPattern = `<!--\s*source:\s*(\S+)\s+--`
	// SourceAnchorLinePattern matches an anchor whose path carries a line
	// number, which rots the moment the file is edited.
	SourceAnchorLinePattern = `<!--\s*source:\s*\S+\.go:\d+\s`
	// ACRowPattern reads one acceptance-criteria row of a spec table.
	ACRowPattern = `^\|\s*(AC-\d+)\s*\|([^|]*)\|([^|]*)\|([^|]*)\|`
	// SpecStatusPattern reads a spec's Status cell.
	SpecStatusPattern = `(?m)^\|\s*Status\s*\|\s*(\S+)\s*\|`
	// RegisterPattern reads the command name out of a MustRegister* call.
	RegisterPattern = `MustRegister\w+\(\s*"([^"]+)"`
	// TrailingLinePattern matches the optional `:123` suffix on an anchor path.
	TrailingLinePattern = `:\d+$`
)

var (
	sourceAnchorRe     = regexp.MustCompile(SourceAnchorPattern)
	sourceAnchorLineRe = regexp.MustCompile(SourceAnchorLinePattern)
	acRowRe            = regexp.MustCompile(ACRowPattern)
	specStatusRe       = regexp.MustCompile(SpecStatusPattern)
	registerRe         = regexp.MustCompile(RegisterPattern)
	trailingLineRe     = regexp.MustCompile(TrailingLinePattern)
)

// CLIPaths are the trees whose Go files register CLI commands, so a change
// under one of them owes a .ci test naming the command.
var CLIPaths = [...]string{
	"internal/component/cli/",
	"internal/component/cmd/",
	"internal/plugins/",
}

// Findings have two severities, but current checks emit only ISSUE. The script
// has an unused WARN branch, and this port preserves it so a future check has a
// warning result.
const (
	severityIssue = "ISSUE"
	severityWarn  = "WARN"
)

// ChangedFiles answers the files this working tree changed against HEAD, plus
// the untracked ones, sorted and deduplicated.
//
// A failed git command is an error, not an empty contribution. The script
// ignores a nonzero status, so an unavailable git leaves an empty changed set.
// Both changed-file checks then pass without their population. No output
// distinguishes that failure from a clean tree.
func ChangedFiles(ctx context.Context, tree string) ([]string, error) {
	commands := [][]string{
		{"git", "diff", "--name-only", "HEAD"},
		{"git", "ls-files", "--others", "--exclude-standard"},
	}

	seen := make(map[string]bool)
	var files []string
	for _, argv := range commands {
		//nolint:gosec // a fixed git argv over the tree the caller named
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = tree
		out, err := cmd.Output()
		if err != nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("repository: ").Join(argv, " ").Str(" failed: ").Err(err).String())
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			files = append(files, line)
		}
	}
	sort.Strings(files)
	return files, nil
}

// markdownUnder answers every .md file under one subtree of the tree,
// tree-relative and ordered by path COMPONENT.
//
// Python sorted() compares pathlib.Path component tuples. Thus, `docs/a/x.md`
// precedes `docs/a-b/x.md`, unlike plain string order. Finding order controls
// which item a reader sees first, so this port preserves component order.
func markdownUnder(tree, sub string) ([]string, error) {
	dir := filepath.Join(tree, sub)
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var found []string
	err = filepath.WalkDir(dir, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(tree, name)
		if relErr != nil {
			return relErr
		}
		found = append(found, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortByComponent(found)
	return found, nil
}

// sortByComponent orders paths the way Python orders pathlib.Path values: by
// path component rather than by the whole string.
func sortByComponent(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		left, right := strings.Split(paths[i], "/"), strings.Split(paths[j], "/")
		for k := 0; k < len(left) && k < len(right); k++ {
			if left[k] != right[k] {
				return left[k] < right[k]
			}
		}
		return len(left) < len(right)
	})
}

// readLines answers one tree-relative file's lines.
//
// An unreadable file is an ERROR. The script skips it and adds no finding, so
// the gate looks cleaner over an incomplete tree. A lower count looks like a
// pass, and the output does not name the skipped file.
func readLines(tree, rel string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(rel))) //nolint:gosec // a file of the tree the caller named
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(raw), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// CheckSourceAnchorLineNumbers reports every documentation source anchor whose
// path carries a line number.
func CheckSourceAnchorLineNumbers(tree string) ([]Finding, error) {
	docs, err := markdownUnder(tree, "docs")
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, rel := range docs {
		lines, readErr := readLines(tree, rel)
		if readErr != nil {
			return nil, readErr
		}
		for i, line := range lines {
			if !sourceAnchorLineRe.MatchString(line) {
				continue
			}
			anchorPath := "unknown"
			if match := sourceAnchorRe.FindStringSubmatch(line); match != nil {
				anchorPath = match[1]
			}
			var tb textbuf.Buffer
			findings = append(findings, Finding{
				Severity: severityIssue, File: rel, Line: i + 1,
				Message: tb.Str("source anchor ").Str(anchorPath).
					Str(" contains line number; use path only (line numbers rot)").String(),
			})
		}
	}
	return findings, nil
}

// CheckSourceAnchorStalePaths reports every documentation source anchor naming
// an in-repository file that no longer exists.
//
// An anchor outside the repository names a SIBLING checkout. Whether it
// resolves depends on the reader's checkout layout, not documentation age. The
// check ignores home-relative paths, absolute paths, URLs, and `../` climbs
// because all have that property.
func CheckSourceAnchorStalePaths(tree string) ([]Finding, error) {
	docs, err := markdownUnder(tree, "docs")
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, rel := range docs {
		lines, readErr := readLines(tree, rel)
		if readErr != nil {
			return nil, readErr
		}
		for i, line := range lines {
			match := sourceAnchorRe.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			anchorPath := match[1]
			if isExternalAnchor(anchorPath) {
				continue
			}
			clean := trailingLineRe.ReplaceAllString(anchorPath, "")
			if strings.HasPrefix(path.Clean(clean), "..") {
				continue
			}
			if _, statErr := os.Lstat(filepath.Join(tree, filepath.FromSlash(clean))); statErr == nil {
				continue
			}
			var tb textbuf.Buffer
			findings = append(findings, Finding{
				Severity: severityIssue, File: rel, Line: i + 1,
				Message: tb.Str("source anchor points to non-existent file: ").Str(clean).String(),
			})
		}
	}
	return findings, nil
}

// isExternalAnchor reports the anchors that point outside this repository and
// therefore document a provenance no check here can verify.
func isExternalAnchor(anchorPath string) bool {
	for _, prefix := range [...]string{"http://", "https://", "~", "/"} {
		if strings.HasPrefix(anchorPath, prefix) {
			return true
		}
	}
	return !strings.Contains(anchorPath, "/")
}

// CheckSpecACCompleteness reports every acceptance criterion of an in-progress
// spec whose Demonstrated By column is empty.
func CheckSpecACCompleteness(tree string) ([]Finding, error) {
	dir := filepath.Join(tree, "plan")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "spec-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	var findings []Finding
	for _, name := range names {
		rel := path.Join("plan", name)
		raw, readErr := os.ReadFile(filepath.Join(tree, "plan", name)) //nolint:gosec // a spec of the tree the caller named
		if readErr != nil {
			return nil, readErr
		}
		status := specStatusRe.FindStringSubmatch(string(raw))
		if len(status) < 2 || status[1] != "in-progress" {
			continue
		}

		inAudit := false
		for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
			if strings.Contains(line, "### Acceptance Criteria") {
				inAudit = true
				continue
			}
			if inAudit && strings.HasPrefix(line, "### ") {
				inAudit = false
				continue
			}
			if !inAudit {
				continue
			}
			row := acRowRe.FindStringSubmatch(line)
			if len(row) < 4 || strings.TrimSpace(row[3]) != "" {
				continue
			}
			var tb textbuf.Buffer
			findings = append(findings, Finding{
				Severity: severityIssue, File: rel, Line: i + 1,
				Message: tb.Str(strings.TrimSpace(row[1])).Str(" has empty 'Demonstrated By' column").String(),
			})
		}
	}
	return findings, nil
}

// CheckCLIHandlerCoverage reports every CLI command a changed file registers
// that no .ci test mentions.
func CheckCLIHandlerCoverage(tree string, changed []string) ([]Finding, error) {
	var cliFiles []string
	for _, rel := range changed {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		for _, prefix := range CLIPaths {
			if strings.HasPrefix(rel, prefix) {
				cliFiles = append(cliFiles, rel)
				break
			}
		}
	}
	if len(cliFiles) == 0 {
		return nil, nil
	}

	testDir := filepath.Join(tree, "test")
	if info, err := os.Stat(testDir); err != nil || !info.IsDir() {
		return nil, nil //nolint:nilerr // a tree with no test/ has no .ci corpus to read
	}

	var corpus string
	var findings []Finding
	for _, rel := range cliFiles {
		raw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(rel))) //nolint:gosec // a changed file of the tree the caller named
		if errors.Is(err, fs.ErrNotExist) {
			continue // a changed file that was deleted registers nothing
		}
		if err != nil {
			return nil, err
		}

		commands := registerRe.FindAllStringSubmatch(string(raw), -1)
		if len(commands) == 0 {
			continue
		}
		if corpus == "" {
			corpus, err = readCITests(testDir)
			if err != nil {
				return nil, err
			}
		}
		for _, command := range commands {
			if strings.Contains(corpus, command[1]) {
				continue
			}
			var tb textbuf.Buffer
			findings = append(findings, Finding{
				Severity: severityIssue, File: rel, Line: 0,
				Message: tb.Str("CLI command '").Str(command[1]).Str("' has no .ci test mentioning it").String(),
			})
		}
	}
	return findings, nil
}

// readCITests answers every .ci test's text, joined the way the script joined
// them so a command's presence is asked of one document.
func readCITests(testDir string) (string, error) {
	var parts []string
	err := filepath.WalkDir(testDir, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ci") {
			return nil
		}
		raw, readErr := os.ReadFile(name) //nolint:gosec // a test of the tree the caller named
		if readErr != nil {
			return readErr
		}
		parts = append(parts, string(raw))
		return nil
	})
	if err != nil {
		return "", err
	}
	// A tree whose .ci corpus is genuinely empty would leave this string
	// empty, and the caller would then read it as "not yet loaded" and walk
	// again. One newline costs nothing and states that the walk happened.
	var tb textbuf.Buffer
	return tb.Join(parts, "\n").Byte('\n').String(), nil
}

// Run runs every check over the tree, in the script's order, and answers the
// findings with the changed set they were selected by.
func Run(ctx context.Context, tree string, changed []string) (Report, error) {
	report := Report{Changed: changed, Findings: []Finding{}}
	if report.Changed == nil {
		report.Changed = []string{}
	}

	steps := []func() ([]Finding, error){
		func() ([]Finding, error) { return CheckSourceAnchorLineNumbers(tree) },
		func() ([]Finding, error) { return CheckSourceAnchorStalePaths(tree) },
		func() ([]Finding, error) { return CheckCrossPackageWiring(ctx, tree, changed) },
		func() ([]Finding, error) { return CheckSpecACCompleteness(tree) },
		func() ([]Finding, error) { return CheckCLIHandlerCoverage(tree, changed) },
	}
	for _, step := range steps {
		findings, err := step()
		if err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, findings...)
	}

	for _, finding := range report.Findings {
		switch finding.Severity {
		case severityIssue:
			report.Issues++
		case severityWarn:
			report.Warnings++
		}
	}
	return report, nil
}
