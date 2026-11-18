// Design: docs/architecture/storage-backends.md -- snapshot the installed daemon store.
package cli

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// ErrSystemBackupUnavailable distinguishes an absent source from a failed copy.
var ErrSystemBackupUnavailable = errors.New("this daemon serves no store to back up")

// BackupSystemTo lends the installed live store to the bounded native backup
// producer. The caller MUST own path's private directory and remove it on any
// failure. The result includes every raw key and never invokes reload. Safe for
// concurrent use while the installed data target's store remains open.
func BackupSystemTo(ctx context.Context, path string, limit int64) error {
	target := dataRPC.Load()
	if target == nil {
		return ErrSystemBackupUnavailable
	}
	if target.Store == nil {
		return ErrSystemBackupUnavailable
	}
	_, err := storage.BackupBounded(ctx, target.Store, path, limit)
	return err
}
