// Design: docs/architecture/web-interface.md -- bounded retired private backup roots.
package artifact

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/opererr"
)

// pruneRetiredRoots removes only empty roots allocated by this owner. Occupied
// roots are bounded by the workspace/source/upload quotas. A failed removal
// stays in roots and fences further allocation through retryCleanupDebt.
// The caller MUST hold m.mu.
func (m *Owner) pruneRetiredRoots() error {
	var failures []error
	for root := range m.roots {
		if root == m.root {
			continue
		}
		if m.rootInUse(root) {
			continue
		}
		if err := m.removeAll(root); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(m.roots, root)
	}
	return errors.Join(failures...)
}

func (m *Owner) rootInUse(root string) bool {
	for _, w := range m.items {
		if filepath.Dir(filepath.Dir(w.path)) == root {
			return true
		}
	}
	for _, u := range m.uploads {
		if filepath.Dir(filepath.Dir(u.path)) == root {
			return true
		}
	}
	for _, s := range m.sources {
		if filepath.Dir(filepath.Dir(s.path)) == root {
			return true
		}
	}
	return filepath.Dir(m.cleanupDebt) == root
}

// checkOwnedParent prevents a new root nested in an old one, whose eventual
// cleanup would otherwise remove live descendants. The configured path's depth
// and the quota-bounded owned root set bound both loops; Stat follows existing
// link prefixes without creating any missing directories. Caller holds m.mu.
func (m *Owner) checkOwnedParent(parent string) error {
	for directory := parent; ; directory = filepath.Dir(directory) {
		info, err := os.Stat(directory)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		} else {
			for root := range m.roots {
				owned, err := os.Stat(root)
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					return err
				}
				if os.SameFile(info, owned) {
					return opererr.New("The backup folder cannot be inside a private backup workspace.",
						"The existing workspace and downloads must survive future cleanup; their private roots cannot become parent folders.",
						"set environment backup directory /absolute/backup-folder")
				}
			}
		}
		if filepath.Dir(directory) == directory {
			return nil
		}
	}
}
