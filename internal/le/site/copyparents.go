// Design: docs/contributing/gh-pages.md -- publication parents are directories, never aliases.
package site

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// checkCopyParent inspects the finite relative parent path without creating
// anything. A missing ancestor means no deeper destination parent exists yet.
func checkCopyParent(root *os.Root, directory string) error {
	if !filepath.IsLocal(directory) {
		return fmt.Errorf("site destination parent %q is not local to its output root", directory)
	}
	prefix := "."
	for component := range strings.SplitSeq(filepath.Clean(directory), string(filepath.Separator)) {
		prefix = filepath.Join(prefix, component)
		info, err := root.Lstat(prefix)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("site destination parent %s is a symlink; choose an output with real parent directories", prefix)
		}
		if !info.IsDir() {
			return fmt.Errorf("site destination parent %s is not a directory; choose an output with real parent directories", prefix)
		}
	}
	return nil
}

// checkTreeCopyParents refuses every selected parent before the first overlay
// write. Its traversal is bounded by the same finite source tree as copyTree.
func checkTreeCopyParents(source string, target *os.Root, keep func(string) bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if !keep(filepath.ToSlash(relative)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return checkCopyParent(target, relative)
		}
		return checkCopyParent(target, filepath.Dir(relative))
	})
}

// openCopyParent acquires each parent component separately and pins its identity
// before continuing. Callers MUST close the returned directory after publication.
func openCopyParent(root *os.Root, directory string) (*os.Root, error) {
	if !filepath.IsLocal(directory) {
		return nil, fmt.Errorf("site destination parent %q is not local to its output root", directory)
	}
	parent, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	// Each component belongs to the finite relative filesystem path.
	for component := range strings.SplitSeq(filepath.Clean(directory), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		child, err := openCopyDirectory(parent, component)
		parent.Close() //nolint:errcheck // Directory reads and acquisition own their verdicts.
		if err != nil {
			return nil, err
		}
		parent = child
	}
	return parent, nil
}

func openCopyDirectory(parent *os.Root, name string) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if os.IsNotExist(err) {
		if err := parent.Mkdir(name, 0o755); err != nil {
			if !os.IsExist(err) {
				return nil, err
			}
		}
		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("site destination parent %s is a symlink; choose an output with real parent directories", name)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("site destination parent %s is not a directory; choose an output with real parent directories", name)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := child.Stat(".")
	if err != nil {
		child.Close() //nolint:errcheck // The failed identity read owns the verdict.
		return nil, err
	}
	if !os.SameFile(info, opened) {
		child.Close() //nolint:errcheck // The identity mismatch owns the verdict.
		return nil, fmt.Errorf("site destination parent %s changed during acquisition; retry with stable output directories", name)
	}
	return child, nil
}
