// Design: docs/architecture/storage-backends.md -- indexed rollback through versioned publication.
// Related: history_revision.go -- the exact existing history-order authority.
// Related: restore.go -- the shared guarded commit and withdrawal boundaries.

package storage

import (
	"fmt"

	"github.com/ze-software/ze/pkg/zefs"
)

// RollbackConfig publishes a selected stored revision as a new active version,
// retaining the previous active version as rollback. Selection, verification,
// pending-state admission and publication share one configuration guard.
// source MUST be an open writable owner handle. The selected version and new
// active stamp are returned for the caller's result; no history index is guessed.
//
// A daemon's active source MUST use ReadRollbackRevision followed by its normal
// stage/reload and live confirmed-window fence instead. Local callers also use
// confirm.WriteOutsideRecorded for the persisted window's operator explanation;
// this owner operation refuses any surviving confirm or file-commit intent.
// Safe for concurrent use. The caller retains ownership of source.
func RollbackConfig(source Storage, name string, revision int) (selected VersionInfo, stamp string, err error) {
	base, err := StoredConfigName(source, name)
	if err != nil {
		return VersionInfo{}, "", err
	}
	held, err := source.AcquireLock(name)
	if err != nil {
		return VersionInfo{}, "", err
	}
	defer func() { err = releaseGuard(held, err) }()
	if err := admitConfigRollbackLocked(held, base); err != nil {
		return VersionInfo{}, "", err
	}
	selected, data, err := readRollbackRevisionLocked(held, base, revision)
	if err != nil {
		return VersionInfo{}, "", err
	}
	stamp, err = restoreConfigLocked(source, held, base, data)
	return selected, stamp, err
}

// admitConfigRollbackLocked is the pre-write identity fence shared by indexed
// publication and the active-source selection that feeds the daemon reload.
func admitConfigRollbackLocked(held WriteGuard, name string) error {
	for _, intent := range []struct{ key, explanation string }{
		{zefs.KeyConfigConfirmPending.Key(name), "confirmed-commit record"},
		{zefs.KeyConfigFileCommit.Key(name), "file commit"},
	} {
		if err := refuseConfigIntentLocked(held, intent.key, intent.explanation); err != nil {
			return fmt.Errorf("rollback configuration %s: %w", name, err)
		}
	}
	return nil
}

func (s *store) RollbackConfig(name string, revision int) (VersionInfo, string, error) {
	return RollbackConfig(s, name, revision)
}
