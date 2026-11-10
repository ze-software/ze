// Design: docs/contributing/committing.md -- the content a prepared commit carries
//
// Detail: the snapshot is read when `./le commit create` runs and committed
// when the generated script runs. Those are two different moments in a checkout
// several sessions write at once, and binding the content to the FIRST of them
// is what stops a commit carrying an edit its author never saw
// (plan/journal/concurrent-session-corruption.md).
package commit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/hunks"
)

// snapshotIndexEntries answers one `git ls-files -s` line for each path, over
// the content that path holds now.
//
// The staging runs in a private index under tmp/, so the shared index is not
// written and no other session can see this call happen. The blobs land in the
// object database, where the generated script reads them back by hash.
func snapshotIndexEntries(root string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	directory, err := os.MkdirTemp(filepath.Join(root, "tmp"), "commit-snapshot-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	index := filepath.Join(directory, "index")

	// -f is what lets a TRACKED file under an ignored directory be staged. Git
	// reports the whole pathspec as ignored and exits 1 without it, even though
	// it staged every path. Forcing costs nothing here: every path passed
	// validateAddPath, which already refuses a path git ignores with the index
	// consulted.
	add := append([]string{"--literal-pathspecs", "add", "-f", "--"}, paths...)
	if _, err := gitIndexOutput(root, index, add...); err != nil {
		return nil, fmt.Errorf("snapshot the content of this commit: %w", err)
	}
	// NUL framing preserves raw path bytes; Git's quoted display is not a path.
	listed, err := gitIndexOutput(root, index, "ls-files", "-s", "-z")
	if err != nil {
		return nil, fmt.Errorf("read the snapshot of this commit: %w", err)
	}
	return checkSnapshot(paths, listed)
}

// checkSnapshot pairs the entries git wrote against the paths that were asked
// for, and refuses anything it cannot pair.
//
// A missing entry means the script would carry a named path with no content.
func checkSnapshot(paths []string, listed string) ([]string, error) {
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}
	entries := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for line := range strings.SplitSeq(listed, "\x00") {
		if line == "" {
			continue
		}
		_, path, found := strings.Cut(line, "\t")
		if !found {
			return nil, fmt.Errorf("git ls-files wrote an entry with no path: %q", line)
		}
		if !wanted[path] {
			return nil, fmt.Errorf("snapshot holds a path this commit does not name: %q", path)
		}
		seen[path] = true
		entries = append(entries, line)
	}
	for _, path := range paths {
		if !seen[path] {
			return nil, fmt.Errorf("git staged no content for %s", path)
		}
	}
	return entries, nil
}

// gitIndexOutput runs git over the private index at indexPath and answers what
// it wrote to stdout.
func gitIndexOutput(root, indexPath string, args ...string) (string, error) {
	return gitIndexInput(root, indexPath, "", args...)
}

func gitIndexInput(root, indexPath, input string, args ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- executable and verbs are closed; paths are argv.
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexPath)
	command.Stdin = strings.NewReader(input)
	var out, complaint bytes.Buffer
	command.Stdout = &out
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(complaint.String() + " " + err.Error()))
	}
	return out.String(), nil
}

// prospectiveContent retains one preparation's immutable inputs. Its zero value
// names no Git objects and MUST NOT be passed to content gates.
type prospectiveContent struct {
	head       string
	before     string
	tree       string
	scriptPath string
	script     string
}

// prospectiveTree overlays the immutable selections on the recorded parent.
// Earlier script blocks are applied in their publication order, never exported.
// Script text MUST be the unchanged text captured by appendScript.
func prospectiveTree(root, head, scriptPath, script string, entries, removed []string) (prospectiveContent, error) {
	directory, err := os.MkdirTemp(filepath.Join(root, "tmp"), "commit-tree-")
	if err != nil {
		return prospectiveContent{}, err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	index := filepath.Join(directory, "index")
	if _, err := gitIndexOutput(root, index, "read-tree", head); err != nil {
		return prospectiveContent{}, err
	}
	overlay := func(entry string) error {
		if _, _, err := snapshotEntryFields(entry); err != nil {
			return err
		}
		_, err := gitIndexInput(root, index, entry+"\x00", "update-index", "-z", "--index-info")
		return err
	}
	var removalError error
	remove := func(paths []string) {
		if removalError != nil {
			return
		}
		if len(paths) == 0 {
			return
		}
		_, removalError = gitIndexOutput(root, index, append([]string{"update-index", "--force-remove", "--"}, paths...)...)
	}
	if err := walkScriptBlocks(scriptPath, script, overlay, remove); err != nil {
		return prospectiveContent{}, err
	}
	if removalError != nil {
		return prospectiveContent{}, removalError
	}
	before := head
	if script != "" {
		before, err = gitIndexOutput(root, index, "write-tree")
		if err != nil {
			return prospectiveContent{}, err
		}
	}
	for _, entry := range entries {
		if err := overlay(entry); err != nil {
			return prospectiveContent{}, err
		}
	}
	remove(removed)
	if removalError != nil {
		return prospectiveContent{}, removalError
	}
	tree, err := gitIndexOutput(root, index, "write-tree")
	if err != nil {
		return prospectiveContent{}, err
	}
	return prospectiveContent{head: head, before: strings.TrimSpace(before), tree: strings.TrimSpace(tree),
		scriptPath: scriptPath, script: script}, nil
}

// treeSelection resolves each review path from one immutable prospective tree.
func treeSelection(root, tree string, paths []string) (hunks.Selection, error) {
	selection := make(hunks.Selection, len(paths))
	for _, path := range paths {
		selection[path] = hunks.Selected{Entry: "0 " + strings.Repeat("0", 40) + " 0\t" + path}
	}
	if len(paths) == 0 {
		return selection, nil
	}
	listed, err := gitOutput(root, append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", tree, "--"}, paths...)...)
	if err != nil {
		return nil, err
	}
	for entry := range strings.SplitSeq(listed, "\x00") {
		if entry == "" {
			continue
		}
		metadata, path, found := strings.Cut(entry, "\t")
		if !found {
			return nil, fmt.Errorf("prospective tree entry has no path: %q", entry)
		}
		fields := strings.Fields(metadata)
		if len(fields) != 3 {
			return nil, fmt.Errorf("prospective tree entry is malformed: %q", entry)
		}
		if fields[1] != "blob" {
			return nil, fmt.Errorf("prospective review path %s is not a blob", path)
		}
		selection[path] = hunks.Selected{Entry: fields[0] + " " + fields[2] + " 0\t" + path}
	}
	return selection, nil
}
