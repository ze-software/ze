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
	"strings"
)

// Selection is the content a prepared commit carries for each path whose hunks
// were selected: the path's `git ls-files -s` line, which names the blob.
//
// A gate that reads a selected path MUST read it through Read: the working-tree
// file holds hunks the commit does not carry. A nil Selection selects nothing,
// so Read then answers the working tree for every path.
type Selection map[string]string

// Paths answers the selected paths in sorted order.
func (s Selection) Paths() []string {
	paths := make([]string, 0, len(s))
	for path := range s {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// Blob answers the object name the commit carries for path, and false when the
// path is not selected.
func (s Selection) Blob(path string) (string, bool) {
	entry, selected := s[path]
	if !selected {
		return "", false
	}
	fields := strings.Fields(entry)
	if len(fields) < 2 {
		return "", false
	}
	return fields[1], true
}

// Read answers the bytes the commit carries for path: the selected blob when
// path is selected, otherwise the working-tree file under root. An absent
// working-tree file answers an error that wraps os.ErrNotExist.
func (s Selection) Read(root, path string) ([]byte, error) {
	if blob, selected := s.Blob(path); selected {
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

// Apply answers the `git ls-files -s` line for path once the patch in
// patchFile is applied to the content commit head holds for path.
//
// The patch applies to a private index read from head, never to the shared
// index or the working tree. It MUST change path and nothing else: a patch that
// does not apply, changes nothing, touches another path, or deletes path is
// refused, each with the step that fixes it. The caller MUST record head as the
// commit's prepared parent, so the script refuses when a peer changed path
// after the blob was built.
func Apply(root, head, path, patchFile string) (string, error) {
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
	if _, err := git(root, index, "apply", "--cached", "--whitespace=nowarn", "--", patchFile); err != nil {
		return "", errors.New("hunks " + path + ": the patch " + patchFile + " does not apply to " + path +
			" as HEAD (" + short + ") holds it: " + err.Error() + "\n" +
			"  save it again from the current file with `git diff -- " + path + " > " + patchFile +
			"`, delete the hunks this commit must not carry, and run ./le commit create again")
	}
	changed, err := git(root, index, "-c", "core.quotePath=false", "diff", "--cached", "--name-only", head)
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
	if strings.Contains(entry, "\n") || !strings.HasSuffix(entry, "\t"+path) {
		return "", errors.New("hunks " + path + ": git staged an entry this commit cannot name: " + entry +
			"; a path git must quote cannot be committed by a script")
	}
	return entry, nil
}

// changedProblem judges the paths the patch changed against the one path it
// was named for, and answers the refusal, or "" when the patch changed exactly
// that path.
func changedProblem(path, patchFile, changed string) string {
	paths := strings.Fields(strings.TrimSpace(changed))
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
