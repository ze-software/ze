// Design: docs/contributing/committing.md -- committing some hunks of a shared file
//
// Package hunks builds the content a commit carries for a path when the commit
// takes only some of that path's hunks: HEAD's blob with one patch applied.
// Sessions share one checkout, so one file can hold two sessions' edits, each
// of which compiles only with that session's other files. Neither session can
// commit the whole file, and neither may write the other's hunks out of the
// working tree, so the blob is built in a private index and the working tree is
// never touched.
package hunks

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Selection is the content a prepared commit carries for each path whose hunks
// were selected, keyed by path.
//
// A gate that reads a selected path MUST read it through Read: the working-tree
// file holds hunks the commit does not carry. A nil Selection selects nothing,
// so Read then answers the working tree for every path.
type Selection map[string]Selected

// Selected is one selected path: the `git ls-files -s` line that names the
// blob the commit carries, and the patch file the commit named, which a gate
// quotes back when it tells the author which command to run again.
type Selected struct {
	Entry string
	Patch string
}

// Paths answers the selected paths in sorted order.
func (s Selection) Paths() []string {
	paths := make([]string, 0, len(s))
	for path := range s {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// Selected reports whether the commit carries path from a selected blob
// rather than from the working tree.
func (s Selection) Selected(path string) bool {
	_, selected := s[path]
	return selected
}

// Blob answers the object name the commit carries for a selected path. It
// MUST be called only for a path Selected reports. A path that is not
// selected, or whose entry is not a `git ls-files -s` line, is an error:
// answering the working tree there would judge content the commit does not
// carry.
func (s Selection) Blob(path string) (string, error) {
	selected, found := s[path]
	if !found {
		return "", errors.New("hunks " + path + ": the path is not selected, so it has no selected blob")
	}
	metadata, name, found := strings.Cut(selected.Entry, "\t")
	if !found {
		return "", errors.New("hunks " + path + ": the selected entry " + strconv.Quote(selected.Entry) +
			" is not a `git ls-files -s` line; this is a defect in ./le commit create, report it with this command line")
	}
	fields := strings.Fields(metadata)
	if len(fields) != 3 {
		return "", errors.New("hunks " + path + ": the selected entry " + strconv.Quote(selected.Entry) +
			" is not a `git ls-files -s` line; this is a defect in ./le commit create, report it with this command line")
	}
	if name != path {
		return "", errors.New("hunks " + path + ": the selected entry " + strconv.Quote(selected.Entry) +
			" names another path; this is a defect in ./le commit create, report it with this command line")
	}
	return fields[1], nil
}

// Read answers the bytes the commit carries for path: the selected blob when
// path is selected, otherwise the working-tree file under root. An absent
// working-tree file answers an error that wraps os.ErrNotExist, and a selected
// path whose entry names no blob answers Blob's error.
func (s Selection) Read(root, path string) ([]byte, error) {
	if s.Selected(path) {
		blob, err := s.Blob(path)
		if err != nil {
			return nil, err
		}
		out, err := git(root, nil, "cat-file", "blob", blob)
		if err != nil {
			return nil, errors.New("read the selected content of " + path + ": " + err.Error())
		}
		return out, nil
	}
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

// Content answers the bytes a commit naming `hunks path patchFile` would carry
// now: HEAD's blob of path with the patch applied, built the way Apply builds
// it. The review record pins these bytes, so the commit gate compares the
// selected blob with a pin taken over the same content.
func Content(root, path, patchFile string) ([]byte, error) {
	head, err := git(root, nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, errors.New("hunks " + path + ": read HEAD: " + err.Error())
	}
	entry, err := Apply(root, strings.TrimSpace(string(head)), path, patchFile)
	if err != nil {
		return nil, err
	}
	return Selection{path: {Entry: entry, Patch: patchFile}}.Read(root, path)
}

// Apply answers the `git ls-files -s` line for path once the patch in
// patchFile is applied to the content commit head holds for path.
//
// The patch applies to a private index read from head, never to the shared
// index or the working tree. It MUST change path and nothing else: a patch that
// does not apply, changes binary content, changes nothing, touches another
// path, or deletes path is refused, each with the step that fixes it. The caller MUST record head as the
// commit's prepared parent, so the script refuses when a peer changed path
// after the blob was built.
func Apply(root, head, path, patchFile string) (string, error) {
	if mustQuote(path) {
		return "", errors.New(quotedProblem(path))
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return "", err
	}
	directory, err := os.MkdirTemp(filepath.Join(root, "tmp"), "commit-hunks-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(directory) }()

	index := []string{"GIT_INDEX_FILE=" + filepath.Join(directory, "index")}
	if _, err := git(root, index, "read-tree", head); err != nil {
		return "", errors.New("hunks " + path + ": read the HEAD the patch applies to: " + err.Error())
	}
	short := head
	if len(short) > 10 {
		short = short[:10]
	}
	body, err := os.ReadFile(patchFile) //nolint:gosec // the patch is the file the caller named for this commit
	if err != nil {
		return "", errors.New("hunks " + path + ": read the patch " + patchFile + ": " + err.Error())
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return "", errors.New("hunks " + path + ": the patch " + patchFile + " is empty, so there is nothing to commit for " + path +
			"\n  save it with `git diff -- " + path + " > " + patchFile + "` and delete only the hunks this commit must not carry")
	}
	stats, err := git(root, nil, "apply", "--numstat", "--", patchFile)
	if err != nil {
		return "", errors.New("hunks " + path + ": the patch " + patchFile + " is not a patch git can read: " + err.Error() +
			"\n  save it with `git diff -- " + path + " > " + patchFile + "` and delete the hunks this commit must not carry")
	}
	if problem := binaryProblem(path, string(stats)); problem != "" {
		return "", errors.New(problem)
	}
	if _, err := git(root, index, "apply", "--cached", "--whitespace=nowarn", "--", patchFile); err != nil {
		return "", errors.New("hunks " + path + ": the patch " + patchFile + " does not apply to " + path +
			" as HEAD (" + short + ") holds it: " + err.Error() + "\n" +
			"  save it again from the current file with `git diff -- " + path + " > " + patchFile +
			"`, delete the hunks this commit must not carry, and run ./le commit create again")
	}
	changed, err := git(root, index, "diff", "--cached", "--name-only", "-z", head)
	if err != nil {
		return "", errors.New("hunks " + path + ": list what the patch changed: " + err.Error())
	}
	if problem := changedProblem(path, patchFile, string(changed)); problem != "" {
		return "", errors.New(problem)
	}
	listed, err := git(root, index, "-c", "core.quotePath=false", "ls-files", "-s", "--", path)
	if err != nil {
		return "", errors.New("hunks " + path + ": read the selected entry: " + err.Error())
	}
	entry := strings.TrimRight(string(listed), "\n")
	if entry == "" {
		return "", errors.New("hunks " + path + ": the patch " + patchFile + " deletes " + path +
			"; a deletion is committed with remove " + path + ", not hunks")
	}
	if strings.Contains(entry, "\n") {
		return "", errors.New(quotedProblem(path) + "\n  git staged: " + entry)
	}
	if !strings.HasSuffix(entry, "\t"+path) {
		return "", errors.New(quotedProblem(path) + "\n  git staged: " + entry)
	}
	return entry, nil
}

// mustQuote reports whether git quotes path in its output even with
// core.quotePath=false: a control character, a double quote or a backslash.
// The script names each path in a shell word and an index line git reads
// back, and a quoted name matches neither.
func mustQuote(path string) bool {
	for _, character := range []byte(path) {
		if character < 0x20 {
			return true
		}
		if character == 0x7f {
			return true
		}
		if character == '"' {
			return true
		}
		if character == '\\' {
			return true
		}
	}
	return false
}

// quotedProblem is the refusal for a path git must quote, with the fix.
func quotedProblem(path string) string {
	return "hunks " + path + ": git quotes this path in its output, so the commit script cannot name it. " +
		"Rename the file to a name with no control character, double quote or backslash, " +
		"commit the rename through ./le commit create (remove the old name, file the new one), then select its hunks again"
}

// binaryProblem answers the refusal for a patch that changes binary content,
// which `git apply --numstat` reports as "-" counts, or "" when every change
// is text. A binary change has no hunks to choose between, so the commit names
// the whole file instead.
func binaryProblem(path, stats string) string {
	for line := range strings.SplitSeq(strings.TrimSpace(stats), "\n") {
		if strings.HasPrefix(line, "-\t-\t") {
			return "hunks " + path + ": the patch changes binary content, which has no hunks to select; " +
				"commit the whole file with file " + path + " instead"
		}
	}
	return ""
}

// changedProblem judges the paths the patch changed, as `git diff --name-only
// -z` lists them, against the one path it was named for, and answers the
// refusal, or "" when the patch changed exactly that path. The names are read
// NUL-terminated because a path may hold a space or a newline, and -z prints
// each name verbatim, never quoted.
func changedProblem(path, patchFile, changed string) string {
	paths := make([]string, 0, 1)
	for name := range strings.SplitSeq(changed, "\x00") {
		if name != "" {
			paths = append(paths, name)
		}
	}
	if len(paths) == 0 {
		return "hunks " + path + ": the patch " + patchFile + " changes nothing against HEAD; " +
			"it is empty or every hunk is already committed, so there is nothing to commit for " + path
	}
	others := make([]string, 0, len(paths))
	for _, other := range paths {
		if other != path {
			others = append(others, other)
		}
	}
	if len(others) != 0 {
		return "hunks " + path + ": the patch " + patchFile + " also changes " + strings.Join(others, ", ") +
			"; name each file with its own hunks keyword and a patch holding only that file's hunks"
	}
	return ""
}

// git runs one git command in root with extra environment entries and answers
// its stdout, or an error carrying git's own words.
func git(root string, environment []string, args ...string) ([]byte, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- executable and verbs are closed; paths are argv.
	command.Dir = root
	command.Env = append(os.Environ(), environment...)
	var out, complaint bytes.Buffer
	command.Stdout = &out
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return nil, errors.New(strings.TrimSpace(complaint.String() + " " + err.Error()))
	}
	return out.Bytes(), nil
}
