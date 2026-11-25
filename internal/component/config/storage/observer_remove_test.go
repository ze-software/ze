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
