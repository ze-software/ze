// Design: docs/architecture/core-design.md -- which documents a review reads
// Overview: ste.go -- the checker this selection serves
//
// The tool reads three populations and names the selected one. The whole tree
// includes each writing surface in DEFAULT_GLOBS. The changed set differs from
// HEAD. The named set contains files from ONE commit, as the commit helper
// requires. Several sessions share this checkout, so commit attribution is the
// correct unit.
//
// Every git query FAILS CLOSED. The script converted git failures to an empty
// set, which makes the ratchet report "no habit grew". The journal row for this
// port records that fix.
package ste

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrNoGit says that git did not answer and the population is unknown. This is a
// refusal, not the empty set that caused the fail-open behavior.
var ErrNoGit = errors.New("ste: git could not name the changed files, so no ratchet can run")

// DefaultFiles answers every reviewable document in the tree, sorted, as
// repository-relative paths.
func DefaultFiles(root string) ([]string, error) {
	seen := make(map[string]bool)
	for _, pattern := range defaultGlobs {
		matched, err := globTree(root, pattern)
		if err != nil {
			return nil, err
		}
		for _, rel := range matched {
			seen[rel] = true
		}
	}

	out := make([]string, 0, len(seen))
	for rel := range seen {
		if !excluded(rel) {
			out = append(out, rel)
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessByPathParts(out[i], out[j]) })
	return out, nil
}

// lessByPathParts orders paths as Python Path does: COMPONENT BY COMPONENT.
//
// This difference affects the tree. By components, "cmd/ze" sorts before
// "cmd/ze-gok". By bytes, it sorts after because `-` is 0x2d and `/` is 0x2f.
// This reorders 41518 of 44957 findings. The JSON payloads would otherwise
// differ even when their rendered pages agreed.
func lessByPathParts(a, b string) bool {
	left := strings.Split(a, "/")
	right := strings.Split(b, "/")
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return left[index] < right[index]
		}
	}
	return len(left) < len(right)
}

// globTree answers the paths under root that match one glob.
//
// Each script pattern is `<prefix>/**/<name>` or a root `<name>`. `**` matches
// any directory count, including zero. pathlib Path.glob has this behavior, but
// filepath.Glob does not. Therefore, this code walks the tree.
func globTree(root, pattern string) ([]string, error) {
	prefix, name, recursive := strings.Cut(pattern, "/**/")
	if !recursive {
		prefix, name = "", pattern
	}

	base := root
	if prefix != "" {
		base = filepath.Join(root, filepath.FromSlash(prefix))
	}
	// A tree the pattern names and this checkout does not have contributes
	// nothing, which is what pathlib's glob answers for it. Anything else is a
	// filesystem this tool cannot read, and that is an error.
	if _, err := os.Stat(base); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var out []string
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ok, matchErr := filepath.Match(name, entry.Name())
		if matchErr != nil || !ok {
			return matchErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	}

	if !recursive {
		entries, err := os.ReadDir(base)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if ok, _ := filepath.Match(name, entry.Name()); ok {
				out = append(out, entry.Name())
			}
		}
		return out, nil
	}
	if err := filepath.WalkDir(base, walk); err != nil {
		return nil, err
	}
	return out, nil
}

// gitLines runs one git query in root and answers nonblank output lines. A git
// failure is an error, never an empty answer.
func gitLines(root string, args ...string) ([]string, error) {
	argv := append([]string{"-c", "core.quotePath=false"}, args...)
	// context.Background: no cancellation source exists for a build-host git
	// query. The action contract accepts no arguments (letools/leaction,
	// Action.Answer).
	cmd := exec.CommandContext(context.Background(), "git", argv...) // #nosec G204 -- this package's own fixed argument lists
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Join(ErrNoGit, err)
	}

	var lines []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if pyStrip(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// ChangedFiles answers the reviewable documents this working tree changed
// against HEAD, in git's own order.
func ChangedFiles(root string) ([]string, error) {
	lines, err := gitLines(root, "diff", "--name-only", "HEAD", "--")
	if err != nil {
		return nil, err
	}

	var out []string
	for _, line := range lines {
		name := pyStrip(line)
		if _, ok := surfaceOf(name); !ok || excluded(name) {
			continue
		}
		if isFile(filepath.Join(root, filepath.FromSlash(name))) {
			out = append(out, name)
		}
	}
	return out, nil
}

// pair is one reviewable file and the path its HEAD version lives at. The two
// differ for a renamed file, so moving a legacy document does not report its
// whole inherited content as new.
type pair struct {
	Current string
	Before  string
}

// Candidates answers the pairs the ratchet compares.
//
// named restricts the set to the repository-relative paths given, which is the
// commit-time form. An empty named reads the whole working tree.
func Candidates(root string, named []string) ([]pair, error) {
	if len(named) > 0 {
		renames, err := renamedPaths(root)
		if err != nil {
			return nil, err
		}

		// Strip a prefix, not a character set. Python `str.lstrip("./")` changed
		// ".claude/rules/x.md" to "claude/rules/x.md". The later file check then
		// failed and silently removed 20 tracked files from the gate.
		wanted := make([]string, 0, len(named))
		for _, path := range named {
			wanted = append(wanted, strings.TrimPrefix(path, "./"))
		}
		sort.Strings(wanted)

		var keep []pair
		for _, name := range dedupe(wanted) {
			if _, ok := surfaceOf(name); !ok || excluded(name) {
				continue
			}
			if !isFile(filepath.Join(root, filepath.FromSlash(name))) {
				continue
			}
			before := name
			if from, ok := renames[name]; ok {
				before = from
			}
			keep = append(keep, pair{Current: name, Before: before})
		}
		return keep, nil
	}

	status, err := gitLines(root, "diff", "--name-status", "-M", "HEAD", "--")
	if err != nil {
		return nil, err
	}

	var all []pair
	for _, line := range status {
		parts := strings.Split(line, "\t")
		code := parts[0]
		switch {
		case strings.HasPrefix(code, "D"): // deleted: nothing left to review
			continue
		case strings.HasPrefix(code, "R") && len(parts) >= 3:
			all = append(all, pair{Current: parts[2], Before: parts[1]})
		default:
			all = append(all, pair{Current: parts[len(parts)-1], Before: parts[len(parts)-1]})
		}
	}

	untracked, err := gitLines(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, name := range untracked {
		all = append(all, pair{Current: name, Before: name})
	}

	var keep []pair
	for _, candidate := range all {
		if _, ok := surfaceOf(candidate.Current); !ok || excluded(candidate.Current) {
			continue
		}
		if isFile(filepath.Join(root, filepath.FromSlash(candidate.Current))) {
			keep = append(keep, candidate)
		}
	}
	sort.Slice(keep, func(i, j int) bool {
		if keep[i].Current != keep[j].Current {
			return keep[i].Current < keep[j].Current
		}
		return keep[i].Before < keep[j].Before
	})
	return dedupePairs(keep), nil
}

// renamedPaths maps a file's current path to the path it had at HEAD.
func renamedPaths(root string) (map[string]string, error) {
	status, err := gitLines(root, "diff", "--name-status", "-M", "HEAD", "--")
	if err != nil {
		return nil, err
	}

	renames := make(map[string]string)
	for _, line := range status {
		parts := strings.Split(line, "\t")
		if strings.HasPrefix(parts[0], "R") && len(parts) >= 3 {
			renames[parts[2]] = parts[1]
		}
	}
	return renames, nil
}

// HeadText answers the file's content at HEAD, and "" when HEAD has no such
// file. A file that is new in this working tree has no baseline, and no
// baseline is a baseline of zero findings.
func HeadText(root, rel string) string {
	var tb textbuf.Buffer
	cmd := exec.CommandContext(context.Background(), "git", //nolint:noctx // see gitLines
		"-c", "core.quotePath=false", "show", tb.Str("HEAD:").Str(rel).String()) // #nosec G204 -- a repository-relative path from git's own output
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// isFile reports whether the path names a regular file, following a symlink the
// way Python's Path.is_file does.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// dedupe removes repeated entries from a sorted slice.
func dedupe(sorted []string) []string {
	out := sorted[:0:0]
	for index, value := range sorted {
		if index == 0 || value != sorted[index-1] {
			out = append(out, value)
		}
	}
	return out
}

// dedupePairs removes repeated entries from a sorted pair slice, which is what
// the script's `sorted(set(keep))` does.
func dedupePairs(sorted []pair) []pair {
	out := sorted[:0:0]
	for index, value := range sorted {
		if index == 0 || value != sorted[index-1] {
			out = append(out, value)
		}
	}
	return out
}
