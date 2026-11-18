// Design: docs/architecture/web-interface.md -- private current-system backup snapshots.
package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// SnapshotSystem retains one account-bound full-store download without creating
// an editor workspace or requiring a committed configuration. produce MUST obey
// limit before writing, close all handles before returning, retain neither path
// nor the owner, and never reenter this owner. The caller MUST Abort after its
// final Read, also on cancellation or failed transport writes. Safe concurrently.
func (m *Owner) SnapshotSystem(ctx context.Context, user string, produce func(path string, limit int64) error) (_ Transfer, retErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, err
	}
	if err := m.capacity(user, true); err != nil {
		return Transfer{}, err
	}
	limit := min(ArtifactBytesMax, OwnerBytesMax-m.bytes-m.debtBytes)
	if err := m.ensureRoot(); err != nil {
		return Transfer{}, storageFailure(err)
	}
	id, err := identity()
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	directory := filepath.Join(m.root, id)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return Transfer{}, storageFailure(err)
	}
	defer func() {
		if retErr != nil {
			if err := m.removeStage(directory, limit); err != nil {
				retErr = storageFailure(errors.Join(retErr, err))
			}
		}
	}()
	path := filepath.Join(directory, "snapshot")
	if err := produce(path, limit); err != nil {
		if _, ok := err.(*Refusal); ok { //nolint:errorlint // Only a direct producer refusal owns its complete policy category; joined storage failures must be reported.
			return Transfer{}, err
		}
		if errors.Is(err, context.Canceled) {
			return Transfer{}, err
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return Transfer{}, err
		}
		return Transfer{}, storageFailure(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	if info.Size() > limit {
		return Transfer{}, tooLarge()
	}
	if err := storage.CheckArtifact(path); err != nil {
		return Transfer{}, storageFailure(err)
	}
	if err := ctx.Err(); err != nil {
		return Transfer{}, err
	}
	m.sources[id] = &source{user: user, path: path, bytes: info.Size(), used: m.now()}
	m.bytes += info.Size()
	m.startWorker()
	return Transfer{ID: id, Total: info.Size()}, nil
}
