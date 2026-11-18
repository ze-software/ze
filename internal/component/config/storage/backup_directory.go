// Design: docs/architecture/storage-backends.md -- backup folders cannot occupy live store names.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// CheckBackupDirectory refuses the actual live store's tree and reserved
// entries before a caller creates its backup parent. Existing link prefixes
// are resolved; missing final directories remain eligible for later creation.
// The source MUST remain open through this check. Safe for concurrent use.
func CheckBackupDirectory(source Storage, parent string) error {
	owned, ok := ownedStore(source)
	if !ok {
		return fmt.Errorf("backup folder: unsupported storage %T", source)
	}
	if owned.tree == nil {
		return nil
	}
	resolved, err := resolveBackupDirectory(parent)
	if err != nil {
		return err
	}
	if reason := StoreOwnedPath(owned.tree.folder.Name(), resolved); reason != "" {
		return fmt.Errorf("backup folder %s occupies %s: %w", parent, reason, errors.ErrUnsupported)
	}
	return nil
}

func resolveBackupDirectory(parent string) (string, error) {
	path, err := filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	suffix := ""
	// Each missing ancestor removes one element of the finite absolute path.
	// EvalSymlinks resolves any existing prefix before StoreOwnedPath consumes it.
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(resolved, suffix), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		next := filepath.Dir(path)
		if next == path {
			return "", err
		}
		suffix = filepath.Join(filepath.Base(path), suffix)
		path = next
	}
}
