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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gitenv"
	lepath "github.com/ze-software/ze/internal/le/le/path"
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
	var state textbuf.Buffer
	state.Reset().Str("# Session State\n\n").Str(newSnapshot)
	if preserved != "" {
		state.Str("\n\n---\n").Str(preserved)
	}
	state.Byte('\n')
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

// selectedSpec answers the spec this process's identity claimed: a subagent's
// own claim, else its session's, which is the claim it was spawned under.
func selectedSpec(root, id string) string {
	agent, err := lepath.Agent()
	if err != nil {
		return ""
	}
	if agent != "" {
		if spec := claimedSpec(root, id, agent); spec != "" {
			return spec
		}
	}
	return claimedSpec(root, id, "")
}

func claimedSpec(root, id, agent string) string {
	marker, err := lepath.SpecClaimMarker(root, id, agent)
	if err != nil {
		return ""
	}
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

// blockSeparator is the column-zero line endSummary writes between blocks.
const blockSeparator = "---"

const sharedCheckoutHeading = "Shared-checkout changes (session attribution unavailable):"

func snapshotText(report SummaryReport, at time.Time) string {
	var text textbuf.Buffer
	text.Reset().Str("## Session: ").Str(at.Format(time.RFC3339)).Str("\n\n")
	text.Str("Branch: `").Str(report.Branch).Str("`\n")
	if report.LastCommit != "" {
		text.Str("Last commit: ").Str(report.LastCommit).Byte('\n')
	}
	if report.Spec != "" {
		text.Str("Spec: `").Str(report.Spec).Str("`\n")
	}
	text.Byte('\n').Str(sharedCheckoutHeading).Byte('\n')
	appendPaths := func(heading string, paths []string) {
		if len(paths) == 0 {
			return
		}
		text.Byte('\n').Str(heading).Str(":\n")
		for _, path := range paths {
			text.Str("- `").Str(path).Str("`\n")
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
	// blockStart is true where endSummary can have begun a block: the top of
	// the file, after the "# Session State" title, and after a column-zero
	// "---" separator, with only blank lines since. A "## Session:" line inside
	// a handoff is the handoff's own text (one that quotes the heading), so it
	// starts a snapshot only at a block start or inside another snapshot.
	blockStart := true
	// fence is the marker of the Markdown code fence an authored block holds
	// open, or empty outside one. Every line inside a fence is quoted text: a
	// quoted "## Session:" heading, "---" separator or title neither opens a
	// snapshot nor splits the block that quotes it.
	fence := ""
	for index, line := range lines {
		// Only the first line is endSummary's title. The same heading further
		// down is authored text and stays in its block.
		if index == 0 && strings.TrimRight(line, " \t") == "# Session State" {
			continue
		}
		if fence != "" {
			current = append(current, line)
			if closesFence(line, fence) {
				fence = ""
			}
			continue
		}
		if opening := fenceMarker(line); opening != "" {
			if kind == snapshotKind {
				flush()
			}
			if kind == "" {
				kind = "other"
			}
			fence = opening
			blockStart = false
			current = append(current, line)
			continue
		}
		// The separator endSummary writes between blocks ends the block it
		// follows. Kept inside the block, it would be written again by the join
		// below, and every rotation would add one more.
		if strings.TrimRight(line, " \t") == blockSeparator {
			flush()
			blockStart = true
			continue
		}
		if snapshotBoundary(line, kind, blockStart) {
			flush()
			kind = snapshotKind
			current = []string{line}
			blockStart = false
			continue
		}
		blockStart = nextBlockStart(line, blockStart)
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

// snapshotBoundary reports whether line opens a snapshot endSummary wrote:
// the heading snapshotText emits, met at a block start or after another
// snapshot. The same heading inside a handoff block is that block's text.
func snapshotBoundary(line, kind string, blockStart bool) bool {
	if !strings.HasPrefix(line, "## Session:") {
		return false
	}
	if kind == snapshotKind {
		return true
	}
	return blockStart
}

// nextBlockStart carries the block-start state past one line that is neither
// a separator nor a fence, which preserveState handles first: a blank line
// keeps it and any other line ends it.
func nextBlockStart(line string, blockStart bool) bool {
	if strings.TrimSpace(line) == "" {
		return blockStart
	}
	return false
}

// fenceMarker returns the run of backticks or tildes that opens or closes a
// Markdown code fence on line, or "" when line is no fence. CommonMark allows
// at most three spaces of indentation and needs at least three marker
// characters, and a backtick fence's info string holds no backtick, which is
// what keeps an inline span such as ```x``` ordinary text.
func fenceMarker(line string) string {
	text := strings.TrimLeft(line, " ")
	if len(line)-len(text) > 3 {
		return ""
	}
	if text == "" {
		return ""
	}
	marker := text[0]
	if marker != '`' {
		if marker != '~' {
			return ""
		}
	}
	width := 0
	for width < len(text) {
		if text[width] != marker {
			break
		}
		width++
	}
	if width < 3 {
		return ""
	}
	if marker == '`' {
		if strings.ContainsRune(text[width:], '`') {
			return ""
		}
	}
	return text[:width]
}

// closesFence reports whether line closes the fence opening began: the same
// marker character, a run at least as long, and nothing after it but spaces.
func closesFence(line, opening string) bool {
	closing := fenceMarker(line)
	if len(closing) < len(opening) {
		return false
	}
	if closing[0] != opening[0] {
		return false
	}
	return strings.TrimSpace(line) == closing
}

func snapshotGrammar(line string) bool {
	if strings.Trim(line, " \t") == "" {
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
	if trimmedRight == sharedCheckoutHeading {
		return true
	}
	if trimmedRight == "Uncommitted:" || trimmedRight == "Staged:" {
		return true
	}
	return snapshotItem.MatchString(line)
}

func trimSnapshot(lines []string) []string {
	for len(lines) != 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last != "" {
			if last != blockSeparator {
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
	// The deadline SIGKILLs git, and a status killed while it refreshes the index
	// leaves an orphaned .git/index.lock that blocks every session's git writes.
	cmd.Env = gitenv.ReadOnly(os.Environ())
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
