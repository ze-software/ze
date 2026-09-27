// Design: docs/architecture/testing/verify-freshness-scope.md -- the change set, in lines
//
// lines.go answers the change set at the finest grain a gate can judge: the
// lines the working tree added or modified since the last pushed commit behind
// HEAD (LinesSinceUpstream), and the paths changed since that same commit
// (PathsSince). The package and path answers in selector.go say which files
// moved against HEAD. A style gate over a tree that
// already holds thousands of instances needs more, because touching one line
// of a file must not make every old instance in that file due.

package repochanged

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// LineSpan is one run of new-side lines, both ends inclusive, numbered from 1.
type LineSpan struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// ChangedLines maps a checkout-relative, slash-separated path to the lines the
// working tree added or modified in it against a base commit. A path absent
// from the map did not change. An untracked file holds one span covering every
// line, because every line of it is new.
type ChangedLines map[string][]LineSpan

// Touches reports whether any line from first to last, inclusive, of path is a
// changed line.
func (c ChangedLines) Touches(path string, first, last int) bool {
	for _, span := range c[path] {
		if span.To < first {
			continue
		}
		if span.From > last {
			continue
		}
		return true
	}
	return false
}

// errDiffShape names a unified diff this parser cannot read. A diff it cannot
// read is refused rather than read as "nothing changed", because a gate over
// an empty change set passes everything.
var errDiffShape = errors.New("unreadable git diff output")

// LineBase names the commit a line-level change set was diffed against, and
// why that commit.
type LineBase struct {
	Commit string `json:"commit"`
	Reason string `json:"reason"`
}

// errNoUpstreamBase says no pushed commit behind HEAD could be found, so the
// unpushed range has no start.
var errNoUpstreamBase = errors.New("no pushed commit behind HEAD to diff against")

// upstreamFallback is the ref the base falls back to when the branch tracks
// no upstream. A detached checkout, which is what `le verify worktree` judges,
// tracks none.
const upstreamFallback = "refs/remotes/origin/main"

// LinesSinceUpstream answers the lines changed between the last pushed commit
// behind HEAD and the working tree: every unpushed commit, the staged and
// unstaged edits, and each untracked file. A push is where the gates are owed
// (ai/rules/pre-release.md), so the unpushed range is what a gate must judge.
// A clean checkout of pushed code answers nothing, which is correct, because
// that code was judged before it was pushed.
//
// The base is the merge base of HEAD and the branch's upstream, or of HEAD and
// origin/main when no upstream resolves. It reads refs only, never a file
// under tmp/, so a detached worktree that shares refs with its source checkout
// answers the same range. With neither ref, or no common history, it returns
// errNoUpstreamBase naming why: an empty base must never read as "nothing
// changed".
func LinesSinceUpstream(root string) (ChangedLines, LineBase, error) {
	base, err := upstreamBase(root)
	if err != nil {
		return nil, LineBase{}, err
	}
	changed, err := linesAgainst(root, base.Commit, nil)
	if err != nil {
		return nil, LineBase{}, err
	}
	return changed, base, nil
}

// LinesSinceCommit answers the lines the working tree changed since one
// commit, in every path. A reader that asks "did this declaration move after
// commit C" diffs from C rather than from the upstream base.
func LinesSinceCommit(root, commit string) (ChangedLines, error) {
	return linesAgainst(root, commit, nil)
}

// LinesInCommit answers the lines one commit changed against its first parent,
// numbered in that commit's own copy of each file. A reader maps them onto the
// declarations of that copy to learn which symbols the commit touched.
func LinesInCommit(root, commit string) (ChangedLines, error) {
	out, err := runGit(root, gitDiff, commit+"^", commit, "-U0", "--inter-hunk-context=0", "--no-color",
		"--no-ext-diff", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--")
	if err != nil {
		return nil, err
	}
	return parseZeroContextDiff(out)
}

// PathsSince answers every path the working tree changed since base, once
// each and sorted: the paths git diffs against the base commit, a deleted path
// included, and every untracked path. It is the file set that pairs with the
// line set LinesSinceUpstream answered for the same base, so a change in the
// range is never missing from the file set.
//
// The reverse does not hold. Renames are not detected here, so a renamed file
// answers both its old path, which the change deleted, and its new one, while
// the line reader detects the rename: a pure rename, like a mode-only change,
// is a path here with no changed line there.
func PathsSince(root string, base LineBase) ([]string, error) {
	// A zero base names no commit. Diffing against it would be a git error at
	// best, and a caller that built one skipped LinesSinceUpstream.
	if base.Commit == "" {
		return nil, fmt.Errorf("%w: the base names no commit", errNoUpstreamBase)
	}
	paths, err := runGitQueries(root, [][]string{
		{gitDiff, gitNameOnly, "-z", "--no-renames", base.Commit, "--"},
		untrackedPathsQuery(),
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

// upstreamBase answers the merge base of HEAD and the first of the upstream
// and origin/main that resolves.
func upstreamBase(root string) (LineBase, error) {
	ref, err := upstreamRef(root)
	if err != nil {
		return LineBase{}, err
	}
	// merge-base exits 1 when the two share no commit, which a shallow clone
	// cut above the fork point also produces.
	out, err := runGit(root, "merge-base", "HEAD", ref)
	if err != nil {
		return LineBase{}, fmt.Errorf("%w: HEAD and %s share no commit this repository holds, as in a shallow clone (%w)", errNoUpstreamBase, ref, err)
	}
	return LineBase{Commit: strings.TrimSpace(out), Reason: "the merge base of HEAD and " + ref + ", so every unpushed commit is judged"}, nil
}

// upstreamRef answers the branch's upstream, or origin/main when the branch
// tracks none.
func upstreamRef(root string) (string, error) {
	if out, err := runGit(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		return strings.TrimSpace(out), nil
	}
	if _, err := runGit(root, "rev-parse", "--verify", "--quiet", upstreamFallback+"^{commit}"); err != nil {
		return "", fmt.Errorf("%w: the branch tracks no upstream and %s does not resolve", errNoUpstreamBase, upstreamFallback)
	}
	return upstreamFallback, nil
}

// linesAgainst answers the lines the working tree changed against one commit.
//
// One diff of the commit against the working tree rather than the two diffs
// WorkingTreePaths runs, because line numbers are only comparable within one
// coordinate system. The staged diff numbers its lines in the index, and a file
// edited again after staging holds those lines somewhere else in the working
// tree.
func linesAgainst(root, commit string, paths []string) (ChangedLines, error) {
	// Every option that changes the output shape is passed here, so a user's
	// diff.noprefix, color or external diff setting cannot change what this
	// parser reads. diff.interHunkContext matters most: set to 10 on the
	// owner's machine, it merges hunks up to ten lines apart even under -U0,
	// and the unchanged lines between them then read as changed.
	diff := []string{gitDiff, commit, "-U0", "--inter-hunk-context=0", "--no-color", "--no-ext-diff",
		"--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--"}
	out, err := runGit(root, append(diff, paths...)...)
	if err != nil {
		return nil, err
	}
	changed, err := parseZeroContextDiff(out)
	if err != nil {
		return nil, err
	}

	untracked, err := runGitQueries(root, [][]string{append(untrackedPathsQuery(), paths...)})
	if err != nil {
		return nil, err
	}
	for _, path := range untracked {
		changed[path] = []LineSpan{{From: 1, To: math.MaxInt}}
	}
	return changed, nil
}

// hunkHeader reads the new-side span of a unified diff hunk header.
var hunkHeader = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

// parseZeroContextDiff reads the new-side spans out of a `git diff -U0`.
//
// A hunk of zero new lines is a deletion. It takes the line the deletion
// follows, so removing one fact from a multi-line condition still counts as a
// change to that condition.
func parseZeroContextDiff(diff string) (ChangedLines, error) {
	changed := ChangedLines{}
	current := ""
	deleted := false
	// inHeader is true from a file's `diff --git` line to its first hunk. A
	// `+++ ` line is a target path only there: inside a hunk it is an added
	// line whose text starts with `++ `.
	inHeader := false
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			inHeader = true
			current = ""
			deleted = false
			continue
		}
		if !inHeader {
			if !strings.HasPrefix(line, "@@ ") {
				continue
			}
		}
		if target, isTarget := strings.CutPrefix(line, "+++ "); isTarget {
			path, err := diffTargetPath(target)
			if err != nil {
				return nil, err
			}
			current = path
			deleted = path == ""
			continue
		}
		match := hunkHeader.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		inHeader = false
		if deleted {
			// A deleted file holds no new line to judge.
			continue
		}
		if current == "" {
			return nil, fmt.Errorf("%w: a hunk header with no target file: %q", errDiffShape, line)
		}
		span, keep, err := hunkSpan(match)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errDiffShape, err)
		}
		if !keep {
			continue
		}
		changed[current] = append(changed[current], span)
	}
	return changed, nil
}

// hunkSpan turns one matched hunk header into its new-side span. keep is false
// for a deletion at the head of a file, which follows no line.
func hunkSpan(match []string) (LineSpan, bool, error) {
	from, err := strconv.Atoi(match[1])
	if err != nil {
		return LineSpan{}, false, err
	}
	count := 1
	if match[2] != "" {
		count, err = strconv.Atoi(match[2])
		if err != nil {
			return LineSpan{}, false, err
		}
	}
	if count > 0 {
		return LineSpan{From: from, To: from + count - 1}, true, nil
	}
	if from == 0 {
		return LineSpan{}, false, nil
	}
	return LineSpan{From: from, To: from}, true, nil
}

// diffTargetPath reads the path out of a `+++` line. It answers "" for
// /dev/null, which is a deleted file.
func diffTargetPath(target string) (string, error) {
	if target == "/dev/null" {
		return "", nil
	}
	// Git ends an unquoted path holding a space with a TAB, so a reader
	// splitting on whitespace cannot mistake the space for the end.
	target = strings.TrimSuffix(target, "\t")
	if strings.HasPrefix(target, `"`) {
		// Git C-quotes a path holding a byte it will not print raw, and the
		// escapes it writes are a subset of Go's.
		unquoted, err := strconv.Unquote(target)
		if err != nil {
			return "", fmt.Errorf("%w: a quoted path Go cannot read: %s", errDiffShape, target)
		}
		target = unquoted
	}
	path, hasPrefix := strings.CutPrefix(target, "b/")
	if !hasPrefix {
		return "", fmt.Errorf("%w: a target path without the b/ prefix: %s", errDiffShape, target)
	}
	return path, nil
}
