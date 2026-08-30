// Design: internal/le/hookruntime/postwrite.go -- immediate journal row validation
package journal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidationProblem is one actionable defect in an edited journal shard.
type ValidationProblem struct {
	Line    int    `json:"line,omitempty"`
	Kind    string `json:"kind"`
	Content string `json:"content,omitempty"`
	Message string `json:"message"`
}

// ValidationReport is the read-only verdict for exactly one journal shard.
type ValidationReport struct {
	Path     string              `json:"path"`
	Rows     int                 `json:"rows"`
	Problems []ValidationProblem `json:"problems,omitempty"`
}

// ExitCode returns zero for a readable contract and one for malformed content.
func (r ValidationReport) ExitCode() int {
	if len(r.Problems) != 0 {
		return 1
	}
	return 0
}

// Text renders the diagnosis consumed by the post-write hook.
func (r ValidationReport) Text() string {
	var text textbuf.Buffer
	if len(r.Problems) == 0 {
		return text.Str("journal: ").Str(r.Path).Str(" is valid (").
			Int(int64(r.Rows)).Str(" row(s))\n").String()
	}
	text.Str("journal: ").Int(int64(len(r.Problems))).Str(" problem(s) in ").
		Str(r.Path).Byte('\n')
	for _, problem := range r.Problems {
		text.Str("  ").Str(problem.Kind)
		if problem.Line != 0 {
			text.Str(" at line ").Int(int64(problem.Line))
		}
		text.Str(": ").Str(problem.Message)
		if problem.Content != "" {
			content := problem.Content
			if len(content) > 110 {
				content = content[:110]
			}
			text.Str("\n    ").Str(content)
		}
		text.Byte('\n')
	}
	return text.Str("  A row is exactly | Date | Spec | Surface | Symptom | Fix |. ").
		Str("Raw pipes add cells; a backslash does not escape one. Keep one table in the file.\n").
		Str("  The Spec cell is a review-artifact key. Write - for no spec, or comma-separated ").
		Str("spec stems with an optional trailing (note). See plan/journal/README.md.\n").String()
}

// ValidateFile validates exactly one edited journal class file without reading
// other shards or creating commit/session artifacts.
func ValidateFile(root, rawPath string) (ValidationReport, error) {
	relative, err := journalRelativePath(root, rawPath)
	if err != nil {
		return ValidationReport{}, err
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		return ValidationReport{}, fmt.Errorf("open journal checkout: %w", err)
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(relative))
	closeErr := repository.Close()
	if readErr != nil {
		return ValidationReport{}, fmt.Errorf("read journal file %s: %w", relative, readErr)
	}
	if closeErr != nil {
		return ValidationReport{}, fmt.Errorf("close journal checkout: %w", closeErr)
	}
	report := ValidationReport{Path: relative}
	headerFound := false
	for number, line := range strings.Split(string(content), "\n") {
		lineNumber := number + 1
		if cells, valid := exactJournalCells(line); valid && isJournalHeader(cells) {
			headerFound = true
			continue
		}
		row, candidate := parseJournalRow(line)
		if !candidate {
			continue
		}
		if row.malformed {
			report.Problems = append(report.Problems, ValidationProblem{
				Line: lineNumber, Kind: "malformed-row", Content: strings.TrimSpace(line),
				Message: "row is not the five cells | Date | Spec | Surface | Symptom | Fix |",
			})
			continue
		}
		report.Rows++
		if _, err := time.Parse(time.DateOnly, row.cells[0]); err != nil {
			report.Problems = append(report.Problems, ValidationProblem{
				Line: lineNumber, Kind: "invalid-date", Content: row.cells[0],
				Message: "Date must be YYYY-MM-DD",
			})
		}
		if _, readable := specStems(row.cells[1]); !readable {
			report.Problems = append(report.Problems, ValidationProblem{
				Line: lineNumber, Kind: "unreadable-spec", Content: row.cells[1],
				Message: "Spec names no safe spec stem; use - for no spec or a stem with an optional trailing (note)",
			})
		}
	}
	if !headerFound {
		report.Problems = append([]ValidationProblem{{
			Kind:    "missing-header",
			Message: "file has no | Date | Spec | Surface | Symptom | Fix | table header",
		}}, report.Problems...)
	}
	return report, nil
}

func exactJournalCells(line string) ([5]string, bool) {
	var cells [5]string
	fields := strings.Split(line, "|")
	if len(fields) != 7 || strings.TrimSpace(fields[0]) != "" || strings.TrimSpace(fields[6]) != "" {
		return cells, false
	}
	for index := range cells {
		cells[index] = strings.TrimSpace(fields[index+1])
	}
	return cells, true
}

func journalRelativePath(root, rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" || strings.ContainsAny(rawPath, "\x00\r\n") {
		return "", errors.New("journal validate requires one safe file path")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := rawPath
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(root, filepath.Clean(path))
		if err != nil {
			return "", err
		}
	}
	path = filepath.Clean(path)
	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("journal path is outside the checkout: %s", rawPath)
	}
	relative := filepath.ToSlash(path)
	if !strings.HasPrefix(relative, journalPrefix) || !strings.HasSuffix(relative, markdownExt) ||
		filepath.Base(relative) == journalReadme {
		return "", fmt.Errorf("not a journal class file: %s", rawPath)
	}
	// root itself can sit under a path the OS canonicalises (macOS resolves
	// /var/folders/... to /private/var/folders/...), which is not a symlink
	// the relative path traverses. Resolving root once, before joining,
	// isolates the check to symlinks introduced inside the relative path.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve journal checkout %s: %w", root, err)
	}
	full := filepath.Join(resolvedRoot, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", fmt.Errorf("inspect journal file %s: %w", relative, err)
	}
	if filepath.Clean(resolved) != filepath.Clean(full) {
		return "", fmt.Errorf("journal path traverses a symbolic link: %s", relative)
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", fmt.Errorf("inspect journal file %s: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("journal path is not a regular file: %s", relative)
	}
	return relative, nil
}
