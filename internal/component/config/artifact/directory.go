// Design: docs/architecture/web-interface.md -- platform-aware private backup storage.
package artifact

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/paths"
)

// configuredBackupParent selects one startup directory without creating a
// fallback after an explicit setting fails. Existing parents are never chmodded
// or owned by an artifact owner; only its random private child is cleaned up.
func configuredBackupParent() (string, error) {
	parent, err := backupDirectory(env.Get(paths.BackupDirectoryKey), paths.DefaultDataDir())
	if err != nil {
		return "", err
	}
	return parent, nil
}

func prepareBackupParent(parent string) error {
	if parent == "" {
		return nil
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return opererr.Wrap(err, "The backup folder could not be created or opened.",
			"No private backup files were created; choose a writable absolute directory, then commit the setting and restart Ze.",
			"set environment backup directory /absolute/backup-folder")
	}
	return nil
}

// SetDirectoryGuard binds the embedding server's live-store policy before
// attachment. The callback MUST remain usable until Close and MUST NOT reenter
// this owner. Existing roots are validated without moving or removing them.
// A refused binding leaves the previous policy unchanged. Safe concurrently.
func (m *Owner) SetDirectoryGuard(check func(string) error) error {
	if check == nil {
		return errors.New("a backup directory guard needs a policy callback")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root != "" {
		if err := check(filepath.Dir(m.root)); err != nil {
			return err
		}
	}
	m.directoryGuard = check
	return nil
}

// backupDirectory keeps the installed platform mapping in paths.DefaultDataDir;
// this policy adds only the backup subdirectory. Empty is the documented
// development-layout OS-temp choice, never a response to an invalid override.
func backupDirectory(explicit, data string) (string, error) {
	parent := explicit
	if parent == "" {
		if data == "" {
			return "", nil
		}
		parent = filepath.Join(data, "backups")
	}
	if !filepath.IsAbs(parent) {
		return "", opererr.New("The backup folder must be an absolute path.",
			"No private backup files were created; choose a path beginning with /, then commit the setting and restart Ze.",
			"set environment backup directory /absolute/backup-folder")
	}
	return parent, nil
}
