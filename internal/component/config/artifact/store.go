// Design: docs/architecture/web-interface.md -- native operations over one privately owned artifact.
// Related: edit.go -- full-text editing uses the same complete-artifact publication.
package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// ViewStore lends a read-only handle to an authenticated operation while the
// workspace is held. The callback MUST NOT retain the handle or call this Owner.
// Its result MUST own any bytes it retains. Close joins before this returns.
// Safe for concurrent use with other owner operations.
func (m *Owner) ViewStore(ctx context.Context, user, id string, view func(storage.Storage) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return err
	}
	w, err := m.find(user, id)
	if err != nil {
		return err
	}
	source, err := storage.OpenBlob(w.path, false)
	if err != nil {
		return storageFailure(err)
	}
	return errors.Join(view(source), source.Close())
}

// ChangeStore lends a writable staging handle, never the retained original.
// The callback MUST NOT retain the handle or call this Owner. A refused change,
// cancellation, close failure or oversized artifact leaves the original intact.
// The complete successful artifact replaces it after Close and quota checks.
// Safe for concurrent use with other owner operations.
func (m *Owner) ChangeStore(ctx context.Context, user, id string, change func(storage.Storage) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return err
	}
	w, err := m.find(user, id)
	if err != nil {
		return err
	}
	return m.changeStore(ctx, w, change)
}

// Text returns independent completed text bytes belonging to this account.
// The caller MUST Abort the source after use; reading never consumes its token.
// Safe for concurrent use with other owner operations.
func (m *Owner) Text(ctx context.Context, user, id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return nil, err
	}
	return m.readText(user, id)
}

func (m *Owner) readText(user, id string) ([]byte, error) {
	source, found := m.sources[id]
	if !found {
		return nil, missing()
	}
	if source.user != user {
		return nil, missing()
	}
	if source.bytes > TextBytesMax {
		return nil, tooLarge()
	}
	content, err := os.ReadFile(source.path)
	if err != nil {
		return nil, storageFailure(err)
	}
	if len(content) > TextBytesMax {
		return nil, tooLarge()
	}
	return content, nil
}

// changeStore is the sole complete-artifact mutation boundary. Its caller MUST
// hold m.mu and have admitted both the account and the selected workspace.
func (m *Owner) changeStore(ctx context.Context, w *workspace, change func(storage.Storage) error) (err error) {
	if err := m.retryCleanupDebt(); err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(w.path), "save.zefs")
	defer func() { err = errors.Join(err, m.removeStage(path, 2*ArtifactBytesMax)) }()
	if err := export(w, path); err != nil {
		return err
	}
	target, err := storage.OpenBlob(path, true, zefs.Spare(0))
	if err != nil {
		return err
	}
	changeErr := change(target)
	names, namesErr := storage.ArtifactConfigNames(target)
	if err := errors.Join(changeErr, namesErr, target.Close()); err != nil {
		return err
	}
	if len(names) > ConfigMax {
		return tooLarge()
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > ArtifactBytesMax {
		return tooLarge()
	}
	if m.bytes-w.bytes+info.Size()+m.debtBytes > OwnerBytesMax {
		return tooLarge()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(path, w.path); err != nil {
		return err
	}
	m.bytes += info.Size() - w.bytes
	w.bytes = info.Size()
	w.configs = names
	return nil
}
