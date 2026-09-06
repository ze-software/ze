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
	add := append([]string{"add", "-f", "--"}, paths...)
	if _, err := gitIndexOutput(root, index, add...); err != nil {
		return nil, fmt.Errorf("snapshot the content of this commit: %w", err)
	}
	// quotePath=false keeps a non-ASCII path as the bytes it is. A path git
	// would still quote is refused below rather than written into the script,
	// where the quoting and the heredoc disagree about what the path is.
	listed, err := gitIndexOutput(root, index, "-c", "core.quotePath=false", "ls-files", "-s")
	if err != nil {
		return nil, fmt.Errorf("read the snapshot of this commit: %w", err)
	}
	return checkSnapshot(paths, listed)
}

// checkSnapshot pairs the entries git wrote against the paths that were asked
// for, and refuses anything it cannot pair.
//
// A missing entry means the script would commit a path it names with no
// content, and a quoted path means the script and git disagree about the name.
// Both are silent in the commit and loud here.
func checkSnapshot(paths []string, listed string) ([]string, error) {
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}
	entries := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for line := range strings.SplitSeq(strings.TrimRight(listed, "\n"), "\n") {
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
			return nil, fmt.Errorf("git staged no content for %s; a path git must quote cannot be committed by a script", path)
		}
	}
	return entries, nil
}

// gitIndexOutput runs git over the private index at indexPath and answers what
// it wrote to stdout.
func gitIndexOutput(root, indexPath string, args ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- executable and verbs are closed; paths are argv.
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexPath)
	var out, complaint bytes.Buffer
	command.Stdout = &out
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(complaint.String() + " " + err.Error()))
	}
	return out.String(), nil
}
