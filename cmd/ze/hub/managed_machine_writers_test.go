// Design: docs/architecture/fleet-config.md -- the machine writers a managed device's freeze admits

package hub

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/pkg/zefs"
)

// machineWriterBooted is the configuration the managed device of these tests
// runs before a machine writer changes it.
const machineWriterBooted = "booted"

// managedDeviceStore is the store of a managed device as its daemon holds it:
// bound to its stored configuration, with meta/instance/managed true. With
// versioned, the configuration has its active version, as after the boot
// record. It answers the handle and the configuration path.
func managedDeviceStore(t *testing.T, versioned bool) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ze.conf")
	store := storage.BindConfigSource(newTestStore(t, dir), path, storage.ConfigSourceStored)
	require.NoError(t, store.WriteFile(path, []byte(machineWriterBooted), 0o600))
	if versioned {
		_, _, err := storage.EnsureActiveVersion(store, path, []byte(machineWriterBooted), time.Now().Add(-time.Hour))
		require.NoError(t, err)
	}
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))
	return store, path
}

// requireOperatorFrozen is the control of every case below: the store really
// is frozen, so a machine writer that completes was admitted, not unguarded.
func requireOperatorFrozen(t *testing.T, store storage.Storage, path string) {
	t.Helper()
	_, err := storage.WriteCandidateVersion(store, path, []byte("operator"), time.Now())
	require.ErrorIs(t, err, storage.ErrConfigFrozen, "the store of this test is not frozen for an operator")
}

// TestMachineWritersOnManagedStore proves that each writer that changes a
// device's own configuration with no operator deciding it completes on a
// managed device, whose freeze refuses an operator's change.
//
// Each case runs the production function on a store bound to its stored
// configuration with meta/instance/managed true, after the control showed
// that the same store refuses an operator's staging.
//
// VALIDATES: AC-17 and A-3: the hub push, the confirmed-commit revert
// (running and at boot), the signal reload, the boot record and schema
// recovery.
// PREVENTS: a managed device that refuses its own hub push, cannot revert an
// unconfirmed commit, or stops at boot on a recovery it owes.
func TestMachineWritersOnManagedStore(t *testing.T) {
	t.Run("hub push", func(t *testing.T) {
		store, path := managedDeviceStore(t, true)
		requireOperatorFrozen(t, store, path)
		client := &managed.ClientConfig{}
		wireManagedCommit(client, store, path, func() error {
			return storage.PromoteCandidate(store, path)
		}, nil, nil)

		require.NoError(t, client.OnCommit([]byte("pushed")))

		active, err := storage.ReadActiveConfig(store, path)
		require.NoError(t, err)
		assert.Equal(t, "pushed", string(active))
	})

	t.Run("confirmed-commit revert on a running daemon", func(t *testing.T) {
		store, path := managedDeviceStore(t, true)
		requireOperatorFrozen(t, store, path)
		revert := confirmRevert(store, path, func() error {
			return storage.PromoteCandidate(store, path)
		})

		require.NoError(t, revert([]byte("rollback")))

		active, err := storage.ReadActiveConfig(store, path)
		require.NoError(t, err)
		assert.Equal(t, "rollback", string(active))
	})

	t.Run("confirmed-commit revert at boot", func(t *testing.T) {
		store, path := managedDeviceStore(t, true)
		requireOperatorFrozen(t, store, path)
		// The stopped daemon left its interrupted commit staged and its
		// window recorded.
		_, err := storage.StageMachineCandidate(store, path, []byte("interrupted"), time.Now())
		require.NoError(t, err)
		recorder := confirm.NewStoreRecorder(store, path)
		require.NoError(t, recorder.Save(confirm.Pending{User: "alice", Deadline: time.Now(), Rollback: []byte("rollback")}))

		require.NoError(t, recoverConfirmWindow(store, path))

		booted, err := storage.ReadConfigSource(store, path)
		require.NoError(t, err)
		assert.Equal(t, "rollback", string(booted))
		record, err := recorder.Load()
		require.NoError(t, err)
		assert.Nil(t, record, "the window's record survived its revert")
	})

	t.Run("signal reload", func(t *testing.T) {
		store, path := managedDeviceStore(t, true)
		requireOperatorFrozen(t, store, path)

		require.NoError(t, stageSIGHUPCandidate(store, path))

		staged, _, present, err := storage.ReadCandidateConfig(store, path)
		require.NoError(t, err)
		require.True(t, present, "the signal reload staged no candidate")
		assert.Equal(t, machineWriterBooted, string(staged))
	})

	t.Run("boot record", func(t *testing.T) {
		store, path := managedDeviceStore(t, false)

		require.NoError(t, recordBootConfig(store, path, []byte(machineWriterBooted)))

		versions, err := store.ListVersions(path)
		require.NoError(t, err)
		require.Len(t, versions, 1, "the boot record versioned nothing")
		requireOperatorFrozen(t, store, path)
	})

	t.Run("schema recovery", func(t *testing.T) {
		store, path := managedDeviceStore(t, true)
		requireOperatorFrozen(t, store, path)

		require.NoError(t, publishRecoveredConfig(store, path, []byte(machineWriterBooted), []byte("migrated")))

		booted, err := storage.ReadConfigSource(store, path)
		require.NoError(t, err)
		assert.Equal(t, "migrated", string(booted))
	})
}
