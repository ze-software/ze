// Design: docs/architecture/storage-backends.md -- bounded system backup snapshots.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/zefs"
)

// ErrBackupAllowance means the complete exact-fit artifact cannot fit the
// admitted snapshot allowance, including conservative native framing overhead.
var ErrBackupAllowance = errors.New("the complete backup exceeds its byte allowance")

// BackupBounded creates an exact-fit artifact only after its complete guarded
// source fits limit. It never replaces an existing file. The caller MUST own
// and remove the private output directory on every error, including a canceled
// context after publication. Safe for concurrent use with the source store.
func BackupBounded(ctx context.Context, source Storage, path string, limit int64) (BackupResult, error) {
	if limit < backupEnvelopeAllowance {
		return BackupResult{}, ErrBackupAllowance
	}
	return backup(ctx, source, path, false, limit, zefs.Spare(0))
}

// Native framing has two netcapstrings per key and one each for magic and
// container. A netcapstring's 19-digit capacity/used, 2-digit width, separators,
// CRC and terminators need at most 53 bytes (pkg/zefs/netcapstring.go). Round up
// to 128 per key pair and 256 for the four-byte magic/container/trailing newline.
// This is deliberately conservative; it is not another wire-size encoder.
const (
	backupEntryAllowance    int64 = 128
	backupEnvelopeAllowance int64 = 256
)

func checkBackupAllowance(ctx context.Context, source WriteGuard, keys []string, limit int64) error {
	remaining := limit - backupEnvelopeAllowance
	// The source guard freezes this finite key census through the later copy.
	// Subtraction checks each contribution before arithmetic can overflow.
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if backupEntryAllowance > remaining {
			return ErrBackupAllowance
		}
		remaining -= backupEntryAllowance
		if int64(len(key)) > remaining {
			return ErrBackupAllowance
		}
		remaining -= int64(len(key))
		content, err := source.ReadKey(key)
		if err != nil {
			return fmt.Errorf("backup: read %s: %w", key, err)
		}
		if int64(len(content)) > remaining {
			return ErrBackupAllowance
		}
		remaining -= int64(len(content))
	}
	return ctx.Err()
}
