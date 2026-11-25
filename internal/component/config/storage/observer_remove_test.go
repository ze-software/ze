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
			stopFirst.Unsubscribe()
			store.SetWriteObserver(func(changed string) {
				if changed == key {
					// The existing observer may re-enter the real store.
					_, _ = store.ReadKey(changed)
					events = append(events, "after unlock")
				}
			})
			guard, err := store.AcquireLock(key)
			require.NoError(t, err)
			guard.SetRuntimeObserver(stopSecond, func(changed string, removed bool) {
				require.Equal(t, key, changed)
				require.False(t, removed)
				events = append(events, "owner write")
			})
			require.NoError(t, guard.WriteKey(key, []byte("saved")))
			require.Empty(t, events)
			require.NoError(t, guard.Release())
			require.NoError(t, store.WriteKey("meta/unrelated/state", []byte("other")))
			require.NoError(t, store.RemoveKey(key))
			stopSecond.Unsubscribe()
			require.NoError(t, store.WriteKey(key, []byte("new")))
			require.Equal(t, []string{"owner write", "after unlock", "runtime delete", "after unlock", "after unlock"}, events)
		})
	}
}

// Every matching runtime owner sees committed changes. A guard publishes
// directly to its writer exactly once, even with no registered subscribers.
func TestRuntimeObserverFanout(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			store := backend.newStore(t, t.TempDir())
			const prefix = "meta/fleet/device/"
			const key = prefix + "edge/record"
			const health = "meta/fleet/health/edge"
			type event struct {
				source  string
				key     string
				removed bool
			}
			var events []event
			record := func(source string) RuntimeObserver {
				return func(key string, removed bool) {
					events = append(events, event{source, key, removed})
				}
			}
			guard, err := store.AcquireLock(key)
			require.NoError(t, err)
			guard.SetRuntimeObserver(nil, record("guard"))
			require.NoError(t, guard.WriteKey(key, []byte("first")))
			require.NoError(t, guard.Release())
			require.Equal(t, []event{{"guard", key, false}}, events)

			owner := store.SetRuntimeObserver(prefix, record("device"))
			all := store.SetRuntimeObserver("meta/fleet/", record("fleet"))
			defer all.Unsubscribe()
			peer := store.SetRuntimeObserver(prefix, record("peer"))
			defer peer.Unsubscribe()
			stats := store.SetRuntimeObserver("meta/fleet/health/", record("health"))
			defer stats.Unsubscribe()
			events = nil
			require.NoError(t, store.WriteKey(key, []byte("external")))
			require.Equal(t, []event{{"device", key, false}, {"fleet", key, false}, {"peer", key, false}}, events)
			events = nil
			guard, err = store.AcquireLock(key)
			require.NoError(t, err)
			guard.SetRuntimeObserver(owner, record("guard"))
			require.NoError(t, guard.RemoveKey(key))
			require.NoError(t, guard.Release())
			require.Equal(t, []event{{"guard", key, true}, {"fleet", key, true}, {"peer", key, true}}, events)
			owner.Unsubscribe()
			owner.Unsubscribe()
			events = nil
			require.NoError(t, store.WriteKey(key, []byte("new")))
			require.NoError(t, store.WriteKey(health, []byte("up")))
			require.Equal(t, []event{{"fleet", key, false}, {"peer", key, false}, {"fleet", health, false}, {"health", health, false}}, events)
		})
	}
}

func TestWriteObserverDetachPreservesReplacement(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			store := backend.newStore(t, t.TempDir())
			var seen []string
			var detachReplacement func()
			detach := store.SetWriteObserver(func(string) {
				seen = append(seen, "first")
				// After unlock, the observer may replace itself.
				detachReplacement = store.SetWriteObserver(func(string) { seen = append(seen, "replacement") })
			})
			require.NoError(t, store.WriteFile("client-edge.conf", []byte("first"), 0600))
			detach()
			require.NoError(t, store.WriteFile("client-edge.conf", []byte("second"), 0600))
			detachReplacement()
			require.NoError(t, store.WriteFile("client-edge.conf", []byte("third"), 0600))
			require.Equal(t, []string{"first", "replacement"}, seen)
		})
	}
}
