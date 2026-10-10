// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Overview: confirm.go -- the window this record persists

package confirm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// pendingRecord is the stored form of Pending. It is JSON so the rollback
// bytes, which are a whole config, need no framing of their own.
type pendingRecord struct {
	User     string    `json:"user"`
	Deadline time.Time `json:"deadline"`
	Rollback []byte    `json:"rollback"`
}

// StoreRecorder keeps the open window's record under
// meta/config/<name>/confirm-pending in the daemon's store, beside the config's
// version pointers, so a restarted daemon finds it (AC-19). Safe for concurrent
// use as far as the store is.
type StoreRecorder struct {
	store storage.Storage
	key   string
}

var _ Recorder = (*StoreRecorder)(nil)

// NewStoreRecorder returns the recorder for the config at configPath.
func NewStoreRecorder(store storage.Storage, configPath string) *StoreRecorder {
	return &StoreRecorder{store: store, key: zefs.KeyConfigConfirmPending.Key(filepath.Base(configPath))}
}

// Save writes p as the open window's record, replacing any earlier one.
func (r *StoreRecorder) Save(p Pending) error {
	encoded, err := json.Marshal(pendingRecord(p))
	if err != nil {
		return fmt.Errorf("encode confirmed-commit record: %w", err)
	}
	if err := r.store.WriteKey(r.key, encoded); err != nil {
		return fmt.Errorf("persist confirmed-commit record: %w", err)
	}
	return nil
}

// Clear removes the record. No record is not an error.
func (r *StoreRecorder) Clear() error {
	err := r.store.RemoveKey(r.key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear confirmed-commit record: %w", err)
	}
	return nil
}

// WriteOutsideRecorded is WriteOutside for a writer that runs outside the
// daemon, such as `ze config rollback`, and so cannot reach its window: it
// reads the window's persisted record for configPath instead. A record means
// a window is open, or was left by a daemon that stopped during one and will
// revert it at start; either way the revert would wipe the write, so it is
// refused with OtherUserError naming the owner. A record that cannot be read
// refuses too, because the write could not be shown safe. The check is not
// atomic with apply: a window the daemon opens between the two is not seen.
func WriteOutsideRecorded(store storage.Storage, configPath string, apply func() error) error {
	pending, err := NewStoreRecorder(store, configPath).Load()
	if err != nil {
		return err
	}
	if pending != nil {
		return &OtherUserError{Owner: pending.User, Left: time.Until(pending.Deadline)}
	}
	return apply()
}

// Load reads the record a stopped daemon left. It answers nil only when no
// record exists; a record that does not decode, or names no user or deadline,
// is an error, because reading it as "no window" would boot the unconfirmed
// config.
func (r *StoreRecorder) Load() (*Pending, error) {
	encoded, err := r.store.ReadKey(r.key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // nil record, nil error: no window was open
	}
	if err != nil {
		return nil, fmt.Errorf("read confirmed-commit record: %w", err)
	}
	var record pendingRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return nil, fmt.Errorf("decode confirmed-commit record %s: %w", r.key, err)
	}
	if record.User == "" {
		return nil, fmt.Errorf("confirmed-commit record %s names no user", r.key)
	}
	if record.Deadline.IsZero() {
		return nil, fmt.Errorf("confirmed-commit record %s has no deadline", r.key)
	}
	pending := Pending(record)
	return &pending, nil
}
