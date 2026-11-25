// VALIDATES: an observer can read a durable runtime-key deletion after guard release.
// PREVENTS: stale registry history and configuration pushes firing on config deletion.

package storage

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObserverRuntimeRemovalAfterUnlock(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			store := backend.newStore(t, t.TempDir())
			const key = "meta/fleet/device/edge-01/record"
			const config = "file/active/client-edge-01.conf"
			require.NoError(t, store.WriteKey(key, []byte("history")))
			require.NoError(t, store.WriteKey(config, []byte("config")))
			var observed []string
			store.SetWriteObserver(func(removed string) {
				_, err := store.ReadKey(removed)
				require.ErrorIs(t, err, fs.ErrNotExist)
				observed = append(observed, removed)
			})
			guard, err := store.AcquireLock(config)
			require.NoError(t, err)
			require.NoError(t, guard.RemoveKey(config))
			require.NoError(t, guard.RemoveKey(key))
			require.Empty(t, observed)
			require.NoError(t, guard.Release())
			require.Equal(t, []string{key}, observed)
		})
	}
}

func TestRuntimeObserverUsesGuardOwnershipAndKeepsWriteObserver(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			store := backend.newStore(t, t.TempDir())
			const key = "meta/fleet/device/edge/record"
			const prefix = "meta/fleet/device/"
			var events []string
			stopFirst := store.SetRuntimeObserver(prefix, func(string, bool) {
				t.Error("replaced observer received a publication")
			})
			stopSecond := store.SetRuntimeObserver(prefix, func(changed string, removed bool) {
				require.Equal(t, key, changed)
				require.True(t, removed)
				events = append(events, "runtime delete")
			})
			stopFirst()
			store.SetWriteObserver(func(changed string) {
				if changed == key {
					// The existing observer may re-enter the real store.
					_, _ = store.ReadKey(changed)
					events = append(events, "after unlock")
				}
			})
			guard, err := store.AcquireLock(key)
			require.NoError(t, err)
			guard.SetRuntimeObserver(func(changed string, removed bool) {
				require.Equal(t, key, changed)
				require.False(t, removed)
				events = append(events, "owner write")
			})
			require.NoError(t, guard.WriteKey(key, []byte("saved")))
			require.Empty(t, events)
			require.NoError(t, guard.Release())
			require.NoError(t, store.WriteKey("meta/unrelated/state", []byte("other")))
			require.NoError(t, store.RemoveKey(key))
			stopSecond()
			require.NoError(t, store.WriteKey(key, []byte("new")))
			require.Equal(t, []string{"owner write", "after unlock", "runtime delete", "after unlock", "after unlock"}, events)
		})
	}
}
