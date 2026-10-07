// Design: docs/contributing/feature-maturity.md -- the recorded green run Supported requires
// Related: runrecord.go -- the record that stores the id this computes
//
// An interop scenario is a directory, so the content a recorded run belongs to
// is the directory's git tree id: on a clean checkout, the id
// `git rev-parse HEAD:<dir>` prints. It is computed over the WORKING TREE, the
// way blobID computes a file's id: the files git would track there (tracked,
// or untracked and not ignored), each hashed as it is on disk now. An edited,
// added or deleted file changes the id before anything is committed, and a
// shallow CI checkout answers without history.

package feature

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Git tree entry modes, as git writes them into a tree object.
const (
	treeModeFile       = "100644"
	treeModeExecutable = "100755"
	treeModeSymlink    = "120000"
	treeModeDirectory  = "40000"
)

// scenarioTreeID answers the git tree id of the directory rel inside tree, over
// the working tree's present content.
func scenarioTreeID(tree, rel string) (string, error) {
	files, err := trackableFiles(tree, rel)
	if err != nil {
		return "", err
	}
	root := &treeNode{}
	prefix := strings.TrimSuffix(rel, "/") + "/"
	for _, file := range files {
		root.insert(strings.Split(strings.TrimPrefix(file, prefix), "/"), file)
	}
	id, err := root.objectID(tree)
	if err != nil {
		return "", err
	}
	if id == nil {
		return "", errors.New(rel + " holds no file git would track, so it has no content to identify")
	}
	return hex.EncodeToString(id), nil
}

// trackableFiles answers the repository-relative paths git would track under
// rel: every tracked path, and every untracked one the ignore rules keep. A
// tracked path deleted from the working tree is listed and skipped later.
func trackableFiles(tree, rel string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "ls-files", "-z", "--cached", "--others", //nolint:gosec // rel is a scenario directory a registered catalog answered, passed after "--"
		"--exclude-standard", "--", rel)
	cmd.Dir = tree
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("cannot list the files of " + rel + ": " + err.Error() + ": " +
			strings.TrimSpace(stderr.String()))
	}
	var files []string
	for file := range strings.SplitSeq(string(out), "\x00") {
		if file == "" {
			continue
		}
		files = append(files, file)
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

// treeNode is one directory of the tree being hashed.
type treeNode struct {
	files map[string]string // entry name -> repository-relative path
	dirs  map[string]*treeNode
}

// insert places the file at rel under the node by its path components.
func (n *treeNode) insert(components []string, rel string) {
	if len(components) == 1 {
		if n.files == nil {
			n.files = map[string]string{}
		}
		n.files[components[0]] = rel
		return
	}
	if n.dirs == nil {
		n.dirs = map[string]*treeNode{}
	}
	child, held := n.dirs[components[0]]
	if !held {
		child = &treeNode{}
		n.dirs[components[0]] = child
	}
	child.insert(components[1:], rel)
}

// treeEntry is one entry of a git tree object.
type treeEntry struct {
	mode string
	name string
	id   []byte
}

// objectID answers the raw tree id of the node, or nil when nothing under it
// exists in the working tree (git records no empty tree). The recursion depth
// is the directory nesting of a checked-out scenario directory: repository
// content Ze authors, never input from a peer.
func (n *treeNode) objectID(tree string) ([]byte, error) {
	entries := make([]treeEntry, 0, len(n.files)+len(n.dirs))
	for name, rel := range n.files {
		entry, present, err := fileEntry(tree, name, rel)
		if err != nil {
			return nil, err
		}
		if present {
			entries = append(entries, entry)
		}
	}
	for name, child := range n.dirs {
		id, err := child.objectID(tree)
		if err != nil {
			return nil, err
		}
		if id != nil {
			entries = append(entries, treeEntry{mode: treeModeDirectory, name: name, id: id})
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	// Git orders tree entries by name, comparing a directory's name as if it
	// ended in '/'.
	slices.SortFunc(entries, func(a, b treeEntry) int {
		return strings.Compare(a.sortKey(), b.sortKey())
	})
	var content bytes.Buffer
	for _, entry := range entries {
		content.WriteString(entry.mode)
		content.WriteByte(' ')
		content.WriteString(entry.name)
		content.WriteByte(0)
		content.Write(entry.id)
	}
	return gitObjectID("tree", content.Bytes()), nil
}

func (e treeEntry) sortKey() string {
	if e.mode == treeModeDirectory {
		return e.name + "/"
	}
	return e.name
}

// fileEntry answers the tree entry of the file at rel as it is on disk now.
// present is false for a tracked file deleted from the working tree.
func fileEntry(tree, name, rel string) (treeEntry, bool, error) {
	full := filepath.Join(tree, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return treeEntry{}, false, nil
	}
	if err != nil {
		return treeEntry{}, false, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return treeEntry{}, false, err
		}
		return treeEntry{mode: treeModeSymlink, name: name, id: gitObjectID("blob", []byte(target))}, true, nil
	}
	if !info.Mode().IsRegular() {
		return treeEntry{}, false, errors.New(rel + " is neither a file nor a symlink (a submodule?); " +
			"a scenario directory holds files")
	}
	content, err := os.ReadFile(full) //nolint:gosec // rel is a path git listed inside the scenario directory
	if err != nil {
		return treeEntry{}, false, err
	}
	mode := treeModeFile
	// Git records the executable mode from the owner's execute bit.
	if info.Mode().Perm()&0o100 != 0 {
		mode = treeModeExecutable
	}
	return treeEntry{mode: mode, name: name, id: gitObjectID("blob", content)}, true, nil
}
