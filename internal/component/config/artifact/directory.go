// Design: docs/architecture/web-interface.md -- platform-aware private backup storage.
package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/paths"
)

// configuredBackupParent selects the effective directory without creating a
// fallback after an explicit setting fails. The OS-temp default uses the same
// absolute, normalized path policy. Existing parents are never chmodded or
// owned by an artifact owner; only its random private child is cleaned up.
func configuredBackupParent(configured string) (string, error) {
	explicit := env.Get(paths.BackupDirectoryKey)
	if explicit == "" {
		explicit = configured
	}
	parent, err := backupDirectory(explicit, paths.DefaultDataDir())
	if err != nil {
		return "", err
	}
	if parent == "" {
		return backupDirectory(os.TempDir(), "")
	}
	return parent, nil
}

func prepareBackupParent(parent string) error {
	if parent == "" {
		return nil
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return opererr.Wrap(err, "The backup folder could not be created or opened.",
			"No private backup files were created; choose a writable absolute directory and commit the setting.",
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
// Parent traversal is refused before cleaning; normalized paths keep owned root
// identities consistent with paths built beneath them. Ordinary links remain
// eligible for the live-store and private-root admission checks.
func backupDirectory(explicit, data string) (string, error) {
	parent := explicit
	defaulted := parent == ""
	if parent == "" {
		if data == "" {
			return "", nil
		}
		parent = data
	}
	if !filepath.IsAbs(parent) {
		return "", opererr.New("The backup folder must be an absolute path.",
			"No private backup files were created; choose a path beginning with / and commit the setting.",
			"set environment backup directory /absolute/backup-folder")
	}
	for component := range strings.SplitSeq(parent, string(filepath.Separator)) {
		if component == ".." {
			return "", opererr.New("The backup folder cannot contain parent traversal.",
				"A .. path component can conceal a live-store or private-workspace ancestor; choose its absolute location directly.",
				"set environment backup directory /absolute/backup-folder")
		}
	}
	parent = filepath.Clean(parent)
	if defaulted {
		parent = filepath.Join(parent, "backups")
	}
	return parent, nil
}

// CaptureDirectory retains the already running configuration without probing
// its folder. A later commit can repair an unusable old location. The caller
// MUST attach the owner before serving it. Safe for concurrent use.
func (m *Owner) CaptureDirectory(configured string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.directoryConfig = configured
}

// DirectoryPreparation is a checked parent selection bound to one owner and
// config generation. Its zero value is invalid; use PrepareDirectory.
type DirectoryPreparation struct {
	owner              *Owner
	parent, configured string
	generation         uint64
}

// PrepareDirectory checks the selected folder using the daemon's own account.
// The permission probe is closed and removed before returning; no private root
// is allocated and no effective selection changes until Complete accepts it.
func (m *Owner) PrepareDirectory(ctx context.Context, configured string, generation uint64) (*DirectoryPreparation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.closed {
		return nil, errors.New("the backup owner has closed")
	}
	if err := m.retryCleanupDebt(); err != nil {
		return nil, storageFailure(err)
	}
	parent, err := configuredBackupParent(configured)
	if err != nil {
		return nil, err
	}
	if err := m.checkOwnedParent(parent); err != nil {
		return nil, err
	}
	if m.directoryGuard != nil {
		if err := m.directoryGuard(parent); err != nil {
			return nil, storageFailure(err)
		}
	}
	if err := prepareBackupParent(parent); err != nil {
		return nil, storageFailure(err)
	}
	probe, err := os.CreateTemp(parent, ".ze-backup-directory-check-")
	if err != nil {
		return nil, storageFailure(err)
	}
	_, writeErr := probe.Write([]byte{0})
	closeErr := probe.Close()
	cleanupErr := m.removeStage(probe.Name(), 1)
	if err := errors.Join(writeErr, closeErr, cleanupErr); err != nil {
		return nil, storageFailure(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &DirectoryPreparation{owner: m, parent: parent, configured: configured, generation: generation}, nil
}

// Complete accepts only a checked parent. Activation is an in-memory swap;
// retired-root cleanup can report a separate failure after that accepted swap.
// A rejected or stale preparation never changes the effective selection.
func (p *DirectoryPreparation) Complete(_ context.Context, accepted bool) error {
	if !accepted {
		return nil
	}
	m := p.owner
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.generation < m.directoryGeneration {
		return nil
	}
	if m.closed {
		return errors.New("the backup owner closed before directory acceptance")
	}
	m.directoryGeneration = p.generation
	m.directoryConfig = p.configured
	if m.root != "" {
		if filepath.Dir(m.root) != p.parent {
			m.root = ""
		}
	}
	m.parentTemp = p.parent
	if err := m.pruneRetiredRoots(); err != nil {
		return storageFailure(err)
	}
	return nil
}
