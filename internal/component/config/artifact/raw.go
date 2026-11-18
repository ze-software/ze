// Design: docs/architecture/web-interface.md -- owner-held native artifact maintenance.
package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// SnapshotKey reads and retains one immutable raw-key result while holding the
// same owner. No uncharged result escapes between a read and its admission.
// The caller MUST Abort the transfer after its final Read.
func (m *Owner) SnapshotKey(ctx context.Context, user, id, key string) (Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, err
	}
	w, err := m.find(user, id)
	if err != nil {
		return Transfer{}, err
	}
	source, err := storage.OpenBlob(w.path, false)
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	content, readErr := source.ReadKey(key)
	if err := errors.Join(readErr, source.Close()); err != nil {
		return Transfer{}, err
	}
	return m.retain(user, content)
}

// WriteKeyFromSource publishes one explicit raw key from complete private bytes.
// The source remains retained. No handle or uncharged byte copy leaves the owner.
func (m *Owner) WriteKeyFromSource(ctx context.Context, user, id, key, transfer string) (int, error) {
	return m.ChangeStoreFromSource(ctx, user, id, transfer, func(target storage.Storage, content []byte) error {
		return target.WriteKey(key, content)
	})
}

// ChangeStoreFromSource lends complete private source bytes and a writable
// staging store to one native operation. The callback MUST NOT retain either
// value or reenter this Owner. The caller MUST Abort the source after use;
// publication never consumes it. Source admission, staging and quota accounting
// hold the same owner lock, so expiry cannot replace the input during a write.
func (m *Owner) ChangeStoreFromSource(ctx context.Context, user, id, transfer string, change func(storage.Storage, []byte) error) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return 0, err
	}
	w, err := m.find(user, id)
	if err != nil {
		return 0, err
	}
	content, err := m.readBytes(user, transfer)
	if err != nil {
		return 0, err
	}
	err = m.changeStore(ctx, w, func(target storage.Storage) error { return change(target, content) })
	return len(content), err
}

// ViewSource lends an immutable private file to a native inspection callback.
// id names an artifact or completed disk transfer owned by this account. The
// callback MUST NOT retain the path, modify it or reenter this Owner. The owner
// lock holds expiry, removal and publication until inspection returns.
func (m *Owner) ViewSource(ctx context.Context, user, id string, inspect func(string) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return err
	}
	if w, found := m.items[id]; found {
		if w.user != user {
			return missing()
		}
		w.used = m.now()
		return inspect(w.path)
	}
	source, found := m.sources[id]
	if !found {
		return missing()
	}
	if source.user != user {
		return missing()
	}
	source.used = m.now()
	return inspect(source.path)
}

// ProduceArtifact retains a native salvage as an independent artifact. The
// callback receives the unchanged source and a new private output filename;
// it MUST close every native handle before returning and retain neither path.
// Complete frame integrity, close, size and quota checks precede publication.
// A salvage with no configuration is still useful and remains downloadable.
func (m *Owner) ProduceArtifact(ctx context.Context, user, transfer string, produce func(source, output string) error) (_ Info, retErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Info{}, err
	}
	source, found := m.sources[transfer]
	if !found {
		return Info{}, missing()
	}
	if source.user != user {
		return Info{}, missing()
	}
	if err := m.capacity(user, false); err != nil {
		return Info{}, err
	}
	if err := m.ensureRoot(); err != nil {
		return Info{}, storageFailure(err)
	}
	id, err := identity()
	if err != nil {
		return Info{}, storageFailure(err)
	}
	directory := filepath.Join(m.root, id)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return Info{}, storageFailure(err)
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, m.removeStage(directory, 2*ArtifactBytesMax))
		}
	}()
	path := filepath.Join(directory, "backup.zefs")
	if err := produce(source.path, path); err != nil {
		return Info{}, err
	}
	if err := storage.CheckArtifact(path); err != nil {
		return Info{}, err
	}
	target, err := storage.OpenBlob(path, false)
	if err != nil {
		return Info{}, err
	}
	names, namesErr := storage.ArtifactConfigNames(target)
	if err := errors.Join(namesErr, target.Close()); err != nil {
		return Info{}, err
	}
	if len(names) > ConfigMax {
		return Info{}, tooLarge()
	}
	info, err := os.Stat(path)
	if err != nil {
		return Info{}, err
	}
	if info.Size() > ArtifactBytesMax {
		return Info{}, tooLarge()
	}
	if m.bytes+info.Size()+m.debtBytes > OwnerBytesMax {
		return Info{}, tooLarge()
	}
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	w := &workspace{id: id, user: user, label: "repaired.zefs", path: path, bytes: info.Size(), configs: names, used: m.now()}
	m.items[id] = w
	m.bytes += info.Size()
	source.used = m.now()
	return workspaceInfo(w), nil
}
