// Tests for the pending record the daemon keeps in its store.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-19 at the store: a
// window's record survives a restart. The boot revert over a real store is
// tested where it lives, cmd/ze/hub (TestRecoverConfirmWindow).
// PREVENTS: a record that loses the user, the deadline or the rollback bytes,
// and a Load that reads "no record" when the record is corrupt.
package confirm

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

func newStoreForTest(t *testing.T) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ze.conf")
	store, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store, configPath
}

// TestStoreRecorderRoundTrip saves a record, loads it back from a recorder
// built afresh (as a restarted daemon builds it), and clears it.
func TestStoreRecorderRoundTrip(t *testing.T) {
	store, configPath := newStoreForTest(t)

	empty, err := NewStoreRecorder(store, configPath).Load()
	require.NoError(t, err)
	assert.Nil(t, empty, "a store with no window has no record")

	deadline := time.Now().Add(42 * time.Second).Round(time.Millisecond)
	want := Pending{User: "alice", Deadline: deadline, Rollback: []byte("router-id 1.2.3.4;\n")}
	require.NoError(t, NewStoreRecorder(store, configPath).Save(want))

	got, err := NewStoreRecorder(store, configPath).Load()
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.User, got.User)
	assert.True(t, want.Deadline.Equal(got.Deadline), "deadline %v, got %v", want.Deadline, got.Deadline)
	assert.Equal(t, want.Rollback, got.Rollback)

	require.NoError(t, NewStoreRecorder(store, configPath).Clear())
	cleared, err := NewStoreRecorder(store, configPath).Load()
	require.NoError(t, err)
	assert.Nil(t, cleared)
	require.NoError(t, NewStoreRecorder(store, configPath).Clear(), "clearing no record is not an error")
}

// TestStoreRecorderCorruptRecordIsAnError proves a record that does not decode
// is reported, never read as "no window": a silent nil would boot the
// unconfirmed config.
func TestStoreRecorderCorruptRecordIsAnError(t *testing.T) {
	store, configPath := newStoreForTest(t)
	require.NoError(t, store.WriteKey(zefs.KeyConfigConfirmPending.Key(filepath.Base(configPath)), []byte("{not json")))
	got, err := NewStoreRecorder(store, configPath).Load()
	require.Error(t, err)
	assert.Nil(t, got)
}
