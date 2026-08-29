// Design: docs/architecture/core-design.md -- per-session recovery state
// Related: seed.go -- the other session lifecycle action
package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

const summaryGitTimeout = 60 * time.Second

var snapshotItem = regexp.MustCompile("^- `[^`]*`[ \\t]*$")

// SummaryReport describes one session-state update.
type SummaryReport struct {
	StateFile   string   `json:"state-file"`
	Written     bool     `json:"written"`
	Branch      string   `json:"branch"`
	LastCommit  string   `json:"last-commit,omitempty"`
	Spec        string   `json:"spec,omitempty"`
	Uncommitted []string `json:"uncommitted,omitempty"`
	Staged      []string `json:"staged,omitempty"`
}

// EndSummary updates the state file for the current native session. A clean
// checkout leaves the state file and compaction marker unchanged.
func EndSummary(root string, paths lepath.SessionPaths, at time.Time) (SummaryReport, error) {
	return endSummary(root, paths, at, gitText)
}

type gitQuery func(string, ...string) string

func endSummary(root string, paths lepath.SessionPaths, at time.Time, query gitQuery) (SummaryReport, error) {
	selectedSpec := selectedSpec(root, paths.ID)
	stateRel := stateFile(paths, selectedSpec)
	report := SummaryReport{
		StateFile:   filepath.ToSlash(stateRel),
		Branch:      query(root, "branch", "--show-current"),
		LastCommit:  query(root, "log", "-1", "--oneline"),
		Spec:        selectedSpec,
		Uncommitted: firstLines(query(root, "diff", "--name-only"), 20),
		Staged:      firstLines(query(root, "diff", "--cached", "--name-only"), 20),
	}
	if query(root, "status", "--porcelain") == "" {
		return report, nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	statePath := filepath.Join(root, stateRel)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		return report, fmt.Errorf("create session state directory: %w", err)
	}
	preserved := ""
	content, err := os.ReadFile(statePath) //nolint:gosec // the path is a session state file under the checkout root
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return report, fmt.Errorf("read session state %q: %w", filepath.ToSlash(stateRel), err)
		}
	} else {
		preserved = preserveState(string(content), 2)
	}
	newSnapshot := snapshotText(report, at)
	var state strings.Builder
	_, _ = state.WriteString("# Session State\n\n")
	_, _ = state.WriteString(newSnapshot)
	if preserved != "" {
		_, _ = state.WriteString("\n\n---\n")
		_, _ = state.WriteString(preserved)
	}
	_ = state.WriteByte('\n')
	if err := writeAtomic(statePath, []byte(state.String()), 0o644, ".session-state-*"); err != nil {
		return report, fmt.Errorf("write session state %q: %w", filepath.ToSlash(stateRel), err)
	}
	report.Written = true
	compaction := filepath.Join(root, ".claude", ".compaction-detected-"+paths.ID)
	if err := os.Remove(compaction); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return report, fmt.Errorf("remove compaction marker: %w", err)
		}
	}
	return report, nil
}

func selectedSpec(root, id string) string {
	marker := filepath.Join(root, "tmp", "session", ".session-"+id)
	content, err := os.ReadFile(marker) //nolint:gosec // the path is a session state file under the checkout root
	if err != nil {
		return ""
	}
	spec, _, _ := strings.Cut(string(content), "\n")
	if spec == "unassigned" {
		return ""
	}
	return spec
}

func stateFile(paths lepath.SessionPaths, spec string) string {
	name := "session-state-" + paths.ID + ".md"
	if spec != "" {
		stem := strings.TrimPrefix(spec, "spec-")
		stem = strings.TrimSuffix(stem, ".md")
		name = "session-state-" + stem + "-" + paths.ID + ".md"
	}
	return filepath.Join(paths.Dir, "state", name)
}

func snapshotText(report SummaryReport, at time.Time) string {
	var text strings.Builder
	_, _ = fmt.Fprintf(&text, "## Session: %s\n\n", at.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&text, "Branch: `%s`\n", report.Branch)
	if report.LastCommit != "" {
		_, _ = fmt.Fprintf(&text, "Last commit: %s\n", report.LastCommit)
	}
	if report.Spec != "" {
		_, _ = fmt.Fprintf(&text, "Spec: `%s`\n", report.Spec)
	}
	appendPaths := func(heading string, paths []string) {
		if len(paths) == 0 {
			return
		}
		_, _ = fmt.Fprintf(&text, "\n%s:\n", heading)
		for _, path := range paths {
			_, _ = fmt.Fprintf(&text, "- `%s`\n", path)
		}
	}
	appendPaths("Uncommitted", report.Uncommitted)
	appendPaths("Staged", report.Staged)
	return strings.TrimSuffix(text.String(), "\n")
}

func preserveState(content string, keep int) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	var snapshots []string
	var others []string
	kind := ""
	var current []string
	flush := func() {
		if kind == "" {
			return
		}
		if kind == snapshotKind {
			current = trimSnapshot(current)
			if len(current) != 0 {
				snapshots = append(snapshots, strings.Join(current, "\n"))
			}
		} else {
			current = trimBlank(current)
			if len(current) != 0 {
				others = append(others, strings.Join(current, "\n"))
			}
		}
		kind = ""
		current = nil
	}
	for _, line := range lines {
		if strings.TrimRight(line, " \t") == "# Session State" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "## Session:") {
			flush()
			kind = snapshotKind
			current = []string{line}
			continue
		}
		if kind == snapshotKind {
			if snapshotGrammar(line) {
				current = append(current, line)
				continue
			}
		}
		if kind == snapshotKind {
			flush()
		}
		if kind == "" {
			kind = "other"
		}
		current = append(current, line)
	}
	flush()
	if len(snapshots) > keep {
		snapshots = snapshots[:keep]
	}
	blocks := make([]string, 0, len(snapshots)+len(others))
	blocks = append(blocks, snapshots...)
	blocks = append(blocks, others...)
	return strings.Join(blocks, "\n\n---\n")
}

func snapshotGrammar(line string) bool {
	if strings.Trim(line, " \t") == "" {
		return true
	}
	if strings.TrimRight(line, " \t") == "---" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
		return true
	}
	if strings.HasPrefix(line, "Branch:") {
		return true
	}
	if strings.HasPrefix(line, "Last commit:") {
		return true
	}
	if strings.HasPrefix(line, "Spec:") {
		return true
	}
	trimmedRight := strings.TrimRight(line, " \t")
	if trimmedRight == "Uncommitted:" || trimmedRight == "Staged:" {
		return true
	}
	return snapshotItem.MatchString(line)
}

func trimSnapshot(lines []string) []string {
	for len(lines) != 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last != "" {
			if last != "---" {
				break
			}
		}
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimBlank(lines []string) []string {
	for len(lines) != 0 {
		if strings.TrimSpace(lines[0]) != "" {
			break
		}
		lines = lines[1:]
	}
	for len(lines) != 0 {
		if strings.TrimSpace(lines[len(lines)-1]) != "" {
			break
		}
		lines = lines[:len(lines)-1]
	}
	return lines
}

func firstLines(text string, limit int) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines
}

func gitText(root string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), summaryGitTimeout)
	defer cancel()
	//nolint:gosec // every invocation is a fixed read-only Git query in endSummary
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

func writeAtomic(path string, content []byte, mode os.FileMode, pattern string) error {
	file, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	written, err := file.Write(content)
	if err != nil {
		_ = file.Close()
		return err
	}
	if written != len(content) {
		_ = file.Close()
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
