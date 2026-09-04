// Design: docs/contributing/committing.md -- what a prospective commit carries
// Related: prepare.go -- Prepared.Added, the paths these describe
// Related: input.go -- validateAddPath, which admits a path before it is measured

package commit

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// FileStat is how much of one file a prospective commit carries, measured
// against HEAD rather than against what the author remembers editing.
//
// It exists because this repository is a shared checkout and `le commit create`
// stages the WORKING TREE version of every path it is given. A path another
// session has edited carries that session's work too, and nothing said so. Twice
// in one night the same author swept in another session's uncommitted change:
// once a consumer whose producer was not committed, which broke the tracked
// build at HEAD, and once twenty lines of another session's documentation into a
// commit that meant to add one row. The second happened an hour after the first
// was written up, by an author who had just prescribed "diff every file you
// name" as the remedy, which is the evidence that a habit was not one.
//
// The gate that already exists refuses another session's STAGED files. It cannot
// be extended to unstaged edits, because nothing here knows which hunks this
// author wrote. So this does not refuse anything. It prints, which is enough:
// an author who believes a file is a one-line change and reads `+20 -0` beside
// it has the question put in front of them without having to think to ask it.
type FileStat struct {
	Path string `json:"path"`
	// Added and Deleted are line counts from `git diff --numstat HEAD`. Both
	// are -1 when the count is unavailable: a binary file, which numstat
	// reports as `-`, or a git invocation that failed.
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	// New marks a path that HEAD does not have, where every line is an
	// addition and a diffstat says nothing useful.
	New bool `json:"new,omitempty"`
}

// statUnavailable is the Added/Deleted value for a count git did not give.
const statUnavailable = -1

// fileStats measures each path against HEAD, in the order given. A path git
// cannot describe still gets a row: silence about a file is what this exists to
// remove, so an unmeasurable file says so rather than being dropped.
func fileStats(root string, paths []string) []FileStat {
	stats := make([]FileStat, 0, len(paths))
	for _, path := range paths {
		stats = append(stats, fileStat(root, path))
	}
	return stats
}

func fileStat(root, path string) FileStat {
	command := exec.CommandContext(context.Background(), "git", "diff", "--numstat", "HEAD", "--", path) // #nosec G204 -- fixed Git query; path is an argv operand.
	command.Dir = root
	out, err := command.Output()
	if err != nil {
		return FileStat{Path: path, Added: statUnavailable, Deleted: statUnavailable}
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		// No diff against HEAD. Either the file is unchanged, or it is
		// untracked and HEAD has never seen it; `git diff` reports neither.
		return FileStat{Path: path, New: isUntracked(root, path)}
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return FileStat{Path: path, Added: statUnavailable, Deleted: statUnavailable}
	}
	return FileStat{Path: path, Added: countOrUnavailable(fields[0]), Deleted: countOrUnavailable(fields[1])}
}

// countOrUnavailable reads a numstat column. numstat writes `-` for a binary
// file, which is a count that does not exist rather than a zero.
func countOrUnavailable(field string) int {
	value, err := strconv.Atoi(field)
	if err != nil {
		return statUnavailable
	}
	return value
}

func isUntracked(root, path string) bool {
	command := exec.CommandContext(context.Background(), "git", "ls-files", "--error-unmatch", "--", path) // #nosec G204 -- fixed Git query; path is an argv operand.
	command.Dir = root
	return command.Run() != nil
}

// StatText renders the measured paths as one line each, widest count first so
// the column reads straight down. It answers the empty string for no paths, so
// a caller can append it unconditionally.
func StatText(stats []FileStat) string {
	if len(stats) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("files=\n")
	for _, stat := range stats {
		builder.WriteString("  ")
		builder.WriteString(statCounts(stat))
		builder.WriteString("  ")
		builder.WriteString(stat.Path)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func statCounts(stat FileStat) string {
	switch {
	case stat.New:
		return "    new"
	case stat.Added == statUnavailable || stat.Deleted == statUnavailable:
		return "      ?"
	default:
		counts := "+" + strconv.Itoa(stat.Added) + " -" + strconv.Itoa(stat.Deleted)
		for len(counts) < 7 {
			counts = " " + counts
		}
		return counts
	}
}
