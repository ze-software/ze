// Design: docs/contributing/running-commands.md -- le test redwalk check
// Overview: redwalk.go -- the walk the break is applied to
//
// patch.go turns the break, a unified diff of tracked Go files, into the
// whole broken files a Go overlay replaces. It applies the diff in memory to
// the working-tree content and refuses on any mismatch: a hunk that does not
// match exactly is never placed by guess, because a break applied somewhere
// else is a different break.

package testredwalk

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/test/overlay"
)

// devNull is the path a git diff header names for a side that holds no file.
const devNull = "/dev/null"

// untrackedFunc answers which of rels, slash paths relative to tree, git does
// not track.
type untrackedFunc func(tree string, rels []string) ([]string, error)

// filePatch is the part of a break that changes one file.
type filePatch struct {
	source string
	target string
	hunks  []hunk
}

// hunk is one `@@` block of a unified diff.
type hunk struct {
	// oldStart is the first old line, counted from 1, or the line after which
	// a hunk with no old lines inserts.
	oldStart int
	oldCount int
	lines    []hunkLine
	// oldNoEOL and newNoEOL record `\ No newline at end of file` for each side.
	oldNoEOL bool
	newNoEOL bool
}

// hunkLine is one body line: its kind (' ', '-' or '+') and its text.
type hunkLine struct {
	kind byte
	text string
}

// parseBreak answers the broken files the patch produces from the working
// tree at tree. Every refusal names the file and the fix.
func parseBreak(tree string, patch []byte, untracked untrackedFunc) ([]overlay.File, error) {
	patches, err := splitPatch(string(patch))
	if err != nil {
		return nil, err
	}
	rels := make([]string, 0, len(patches))
	for _, one := range patches {
		rel, err := breakPath(tree, one)
		if err != nil {
			return nil, err
		}
		if slices.Contains(rels, rel) {
			return nil, errors.New(problem(rel, "the break changes this file twice",
				"save the break again as one `git diff` of the files it changes"))
		}
		rels = append(rels, rel)
	}
	missing, err := untracked(tree, rels)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, errors.New(problem(strings.Join(missing, ", "),
			"git does not track this file, and a break changes only tracked files inside the checkout",
			"break a tracked Go file instead"))
	}
	files := make([]overlay.File, 0, len(patches))
	for position, one := range patches {
		file, err := applyFile(tree, rels[position], one)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// breakPath answers the slash path one file patch changes, or why the walk
// refuses it. The checks run before any file is read.
func breakPath(tree string, one filePatch) (string, error) {
	if one.source == devNull {
		return "", errors.New(problem(one.target, "the break creates this file, and a break changes only tracked files",
			"break a tracked Go file instead"))
	}
	if one.target != devNull {
		if one.target != one.source {
			return "", errors.New(problem(one.source, "the break renames this file to "+one.target+
				", and a rename is not a break of the code", "save the break without the rename"))
		}
	}
	rel := one.source
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return "", errors.New(problem(rel, "the path leaves the checkout, and a break changes only files inside it",
			"name the file by its path inside the checkout, as `git diff` writes it"))
	}
	if filepath.Ext(rel) != ".go" {
		// D2 (b): a non-Go file is refused rather than copied into a private
		// root, which is a separate mechanism with its own spec.
		return "", errors.New(problem(rel, "this is not a Go file. A Go overlay replaces only the Go source "+
			"the compiler reads, so it cannot reach this file, and the walk never edits the shared tree",
			"break Go code instead; a walk of a .ci, a config or a file a test reads at run time is not supported"))
	}
	if err := requireInside(tree, rel); err != nil {
		return "", err
	}
	return rel, nil
}

// requireInside refuses a path whose real location, symlinks resolved, lies
// outside the checkout.
func requireInside(tree, rel string) error {
	root, err := filepath.EvalSymlinks(tree)
	if err != nil {
		return errors.New(problem(tree, "cannot resolve the checkout: "+err.Error(), "run the walk from the checkout"))
	}
	real, err := filepath.EvalSymlinks(filepath.Join(tree, filepath.FromSlash(rel)))
	if err != nil {
		return errors.New(problem(rel, "cannot read the file the break changes: "+err.Error(),
			"save the break again from the current tree with `git diff`"))
	}
	inside, err := filepath.Rel(root, real)
	if err != nil {
		return errors.New(problem(rel, "the path leaves the checkout, and a break changes only files inside it",
			"break a file inside the checkout"))
	}
	if !filepath.IsLocal(inside) {
		return errors.New(problem(rel, "the path leaves the checkout through a symlink, and a break changes "+
			"only files inside it", "break a file inside the checkout"))
	}
	return nil
}

// applyFile answers the overlay file one file patch produces.
func applyFile(tree, rel string, one filePatch) (overlay.File, error) {
	raw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(rel))) //nolint:gosec // rel was checked to lie inside the checkout
	if err != nil {
		return overlay.File{}, errors.New(problem(rel, "cannot read the file the break changes: "+err.Error(),
			"save the break again from the current tree with `git diff`"))
	}
	content := string(raw)
	broken, err := applyHunks(rel, content, one.hunks)
	if err != nil {
		return overlay.File{}, err
	}
	if one.target == devNull {
		if broken != "" {
			return overlay.File{}, errors.New(problem(rel, "the break deletes this file, but its hunks leave "+
				"lines in it", "save the break again with `git diff`"))
		}
		return overlay.File{Rel: rel, Absent: true}, nil
	}
	if broken == content {
		return overlay.File{}, errors.New(problem(rel, "the break leaves this file as it is, so there is no break to walk",
			"save a break that changes the code"))
	}
	return overlay.File{Rel: rel, Content: broken}, nil
}

// splitPatch reads every file patch of a git unified diff. The lines git
// writes between files (`diff --git`, `index`, modes) carry nothing the walk
// reads; a binary change and a quoted path are refused.
func splitPatch(patch string) ([]filePatch, error) {
	lines := strings.Split(patch, "\n")
	var patches []filePatch
	for at := 0; at < len(lines); {
		line := lines[at]
		if strings.HasPrefix(line, "Binary files ") {
			return nil, errors.New(problem("the break", "it changes binary content, which has no lines to apply",
				"break Go source instead"))
		}
		if strings.HasPrefix(line, "GIT binary patch") {
			return nil, errors.New(problem("the break", "it changes binary content, which has no lines to apply",
				"break Go source instead"))
		}
		if !strings.HasPrefix(line, "--- ") {
			at++
			continue
		}
		if at+1 >= len(lines) {
			return nil, errors.New(problem("the break", "a `--- ` line ends the patch with no `+++ ` line after it",
				"save the break again with `git diff`"))
		}
		if !strings.HasPrefix(lines[at+1], "+++ ") {
			return nil, errors.New(problem("the break", "a `--- ` line is not followed by a `+++ ` line",
				"save the break again with `git diff`"))
		}
		source, err := headerPath(line[len("--- "):], "a/")
		if err != nil {
			return nil, err
		}
		target, err := headerPath(lines[at+1][len("+++ "):], "b/")
		if err != nil {
			return nil, err
		}
		one := filePatch{source: source, target: target}
		at += 2
		for at < len(lines) && strings.HasPrefix(lines[at], "@@ ") {
			read, next, err := readHunk(lines, at)
			if err != nil {
				return nil, err
			}
			one.hunks = append(one.hunks, read)
			at = next
		}
		if len(one.hunks) == 0 {
			return nil, errors.New(problem(source, "the break names this file and holds no hunk for it",
				"save the break again with `git diff`"))
		}
		patches = append(patches, one)
	}
	if len(patches) == 0 {
		return nil, errors.New(problem("the break", "it holds no file change",
			"save the change you want to walk with `git diff -- <file> > <patch>`"))
	}
	return patches, nil
}

// headerPath answers the path one `---` or `+++` header names, with git's
// side prefix removed, or devNull.
func headerPath(text, prefix string) (string, error) {
	name, _, _ := strings.Cut(text, "\t")
	if name == devNull {
		return devNull, nil
	}
	if strings.HasPrefix(name, `"`) {
		return "", errors.New(problem(name, "git quoted this path because it holds unusual characters, "+
			"which a break path never needs", "break a file whose path git writes unquoted"))
	}
	rel, held := strings.CutPrefix(name, prefix)
	if !held {
		return "", errors.New(problem(name, "the header path has no "+prefix+" prefix, so this is not a "+
			"patch `git diff` wrote", "save the break with `git diff -- <file> > <patch>`"))
	}
	return rel, nil
}

// readHunk reads the hunk whose header is lines[at], and answers it with the
// index of the first line after it. The body is read by the header's counts,
// so a removed line that reads `--- x` is never taken for a file header.
func readHunk(lines []string, at int) (hunk, int, error) {
	read, newCount, err := parseHunkHeader(lines[at])
	if err != nil {
		return hunk{}, 0, err
	}
	oldLeft := read.oldCount
	newLeft := newCount
	var last byte
	at++
	for oldLeft > 0 || newLeft > 0 {
		if at >= len(lines) {
			return hunk{}, 0, errors.New(problem("the break", "a hunk ends before the lines its header counts",
				"save the break again with `git diff`"))
		}
		line := lines[at]
		at++
		if line == "" {
			// An editor strips the space of an empty context line.
			line = " "
		}
		kind := line[0]
		switch kind {
		case ' ':
			oldLeft--
			newLeft--
		case '-':
			oldLeft--
		case '+':
			newLeft--
		case '\\':
			read.markNoEOL(last)
			continue
		default:
			return hunk{}, 0, errors.New(problem("the break", "a hunk line opens with "+strconv.Quote(string(kind))+
				", which no unified diff line does", "save the break again with `git diff`"))
		}
		if oldLeft < 0 {
			return hunk{}, 0, errors.New(problem("the break", "a hunk holds more lines than its header counts",
				"save the break again with `git diff`"))
		}
		if newLeft < 0 {
			return hunk{}, 0, errors.New(problem("the break", "a hunk holds more lines than its header counts",
				"save the break again with `git diff`"))
		}
		read.lines = append(read.lines, hunkLine{kind: kind, text: line[1:]})
		last = kind
	}
	if at < len(lines) && strings.HasPrefix(lines[at], `\`) {
		read.markNoEOL(last)
		at++
	}
	return read, at, nil
}

// markNoEOL records `\ No newline at end of file` for the side the line before
// it belongs to.
func (h *hunk) markNoEOL(kind byte) {
	if kind != '+' {
		h.oldNoEOL = true
	}
	if kind != '-' {
		h.newNoEOL = true
	}
}

// parseHunkHeader reads `@@ -a[,b] +c[,d] @@`, answering the hunk with its old
// range and the new line count.
func parseHunkHeader(line string) (hunk, int, error) {
	refusal := errors.New(problem(line, "this is not a hunk header `@@ -a,b +c,d @@`",
		"save the break again with `git diff`"))
	body, held := strings.CutPrefix(line, "@@ -")
	if !held {
		return hunk{}, 0, refusal
	}
	ranges, _, closed := strings.Cut(body, " @@")
	if !closed {
		return hunk{}, 0, refusal
	}
	source, target, split := strings.Cut(ranges, " +")
	if !split {
		return hunk{}, 0, refusal
	}
	oldStart, oldCount, err := parseRange(source)
	if err != nil {
		return hunk{}, 0, refusal
	}
	_, newCount, err := parseRange(target)
	if err != nil {
		return hunk{}, 0, refusal
	}
	return hunk{oldStart: oldStart, oldCount: oldCount}, newCount, nil
}

// parseRange reads `a` or `a,b`; a range with no count holds one line.
func parseRange(text string) (int, int, error) {
	first, count, held := strings.Cut(text, ",")
	start, err := strconv.Atoi(first)
	if err != nil {
		return 0, 0, err
	}
	lines := 1
	if held {
		lines, err = strconv.Atoi(count)
		if err != nil {
			return 0, 0, err
		}
	}
	return start, lines, nil
}

// applyHunks answers content with every hunk applied in order. Each hunk must
// match the file exactly at the line its header names: no offset, no fuzz.
func applyHunks(rel, content string, hunks []hunk) (string, error) {
	lines, finalNewline := splitLines(content)
	resultNewline := finalNewline
	out := make([]string, 0, len(lines))
	cursor := 0
	for _, one := range hunks {
		start := one.oldStart - 1
		if one.oldCount == 0 {
			start = one.oldStart
		}
		if start < cursor {
			return "", errors.New(problem(rel, "the break's hunks overlap or are out of order",
				"save the break again with `git diff`"))
		}
		if start+one.oldCount > len(lines) {
			return "", errors.New(mismatch(rel, "a hunk reaches past the end of the file"))
		}
		out = append(out, lines[cursor:start]...)
		position := start
		for _, line := range one.lines {
			if line.kind == '+' {
				out = append(out, line.text)
				continue
			}
			if lines[position] != line.text {
				var tb textbuf.Buffer
				return "", errors.New(mismatch(rel, tb.Str("line ").Int(int64(position+1)).Str(" reads ").
					Quoted(lines[position]).Str(", and the break expects ").Quoted(line.text).String()))
			}
			if line.kind == ' ' {
				out = append(out, line.text)
			}
			position++
		}
		cursor = position
		if position == len(lines) {
			if one.oldNoEOL == finalNewline {
				return "", errors.New(mismatch(rel, "the break and the file disagree on whether the file "+
					"ends with a newline"))
			}
			resultNewline = !one.newNoEOL
			continue
		}
		if one.oldNoEOL {
			return "", errors.New(mismatch(rel, "a hunk marks a line inside the file as its last"))
		}
		if one.newNoEOL {
			return "", errors.New(mismatch(rel, "a hunk marks a line inside the file as its last"))
		}
	}
	out = append(out, lines[cursor:]...)
	if len(out) == 0 {
		return "", nil
	}
	joined := strings.Join(out, "\n")
	if resultNewline {
		joined += "\n"
	}
	return joined, nil
}

// splitLines answers content's lines without their newlines, and whether the
// last line ends with one. An empty file holds no line.
func splitLines(content string) ([]string, bool) {
	if content == "" {
		return nil, true
	}
	trimmed, finalNewline := strings.CutSuffix(content, "\n")
	return strings.Split(trimmed, "\n"), finalNewline
}

// gitUntracked answers which of rels git does not track in tree.
func gitUntracked(tree string, rels []string) ([]string, error) {
	argv := append([]string{"--literal-pathspecs", "ls-files", "-z", "--"}, rels...)
	cmd := exec.CommandContext(context.Background(), "git", argv...) //nolint:gosec // rels are checked paths inside the checkout
	cmd.Dir = tree
	listed, err := cmd.Output()
	if err != nil {
		return nil, errors.New(problem(tree, "cannot ask git which files it tracks: "+err.Error(),
			"run the walk from a git checkout"))
	}
	tracked := strings.Split(strings.TrimSuffix(string(listed), "\x00"), "\x00")
	var missing []string
	for _, rel := range rels {
		if !slices.Contains(tracked, rel) {
			missing = append(missing, rel)
		}
	}
	return missing, nil
}

// problem words one refusal: the subject, what is wrong, then the fix on a
// line of its own.
func problem(subject, what, fix string) string {
	var tb textbuf.Buffer
	return tb.Str(subject).Str(": ").Str(what).Str("\n  fix: ").Str(fix).String()
}

// mismatch words a break that does not apply to the file as the tree holds it.
func mismatch(rel, what string) string {
	var reason, fix textbuf.Buffer
	return problem(rel, reason.Str("the break does not apply to the file as the working tree holds it: ").
		Str(what).String(), fix.Str("save the break again from the current file with `git diff -- ").
		Str(rel).Str(" > <patch>`").String())
}
