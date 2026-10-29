// Design: docs/architecture/storage-backends.md -- readers and the write lock.
package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readsUnderHeldGuard is what one reader goroutine saw while a guard was held.
type readsUnderHeldGuard struct {
	data   []byte
	exists bool
	keys   []string
	err    error
}

// TestReadsDoNotWaitForAWriteGuard proves an unguarded read is answered while
// another writer holds the store's write guard.
//
// Method: the boot record holds the write guard across all of its syncs once
// the hub serves, and every plugin state read used to wait for it: OSPF's
// graceful restart read then passed its five second deadline at startup and
// the plugin stopped. The test holds a guard, writes through it, and reads a
// key the guard does not touch from another goroutine. The read, the
// existence probe and the listing must answer before the guard is released,
// and the listing shows the committed key. Run under -race, the memory
// encoding also proves its reads and the guard's write do not race.
func TestReadsDoNotWaitForAWriteGuard(t *testing.T) {
	backends := []struct {
		name string
		open func(*testing.T, string) Storage
	}{
		{name: "tree", open: newTreeStorage},
		{name: "memory", open: newMemoryStorage},
	}
	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.open(t, t.TempDir())
			require.NoError(t, s.WriteKey("state/ospf/restart-fact", []byte("fact")))
			guard, err := s.AcquireLock("")
			require.NoError(t, err)

			answered := make(chan readsUnderHeldGuard, 1)
			go func() { // test-only worker: one set of reads, joined through answered.
				var seen readsUnderHeldGuard
				seen.data, seen.err = s.ReadKey("state/ospf/restart-fact")
				seen.exists = s.Exists("meta/never-written")
				if seen.err == nil {
					seen.keys, seen.err = s.ListKeys("state/")
				}
				answered <- seen
			}()
			require.NoError(t, guard.WriteKey("meta/config/router.conf/active", []byte("pending")))

			var seen readsUnderHeldGuard
			select {
			case seen = <-answered:
			case <-time.After(5 * time.Second):
				t.Error("a read waited for a write guard that never touched its key")
				require.NoError(t, guard.Release())
				seen = <-answered
			}
			require.NoError(t, guard.Release())
			require.NoError(t, seen.err)
			assert.Equal(t, "fact", string(seen.data))
			assert.False(t, seen.exists)
			assert.Equal(t, []string{"state/ospf/restart-fact"}, seen.keys)
		})
	}
}

// TestListKeysToleratesPrunedDirectories proves an unguarded listing of the
// tree answers while a writer removes keys and so prunes their directories.
//
// Method: ListKeys no longer waits for a guard, so it walks the tree while
// another writer removes keys. Removing a/b/c prunes the emptied a/b and a, and
// a walk that read a directory entry just before the prune then opens a node
// that is gone. One goroutine writes and removes a/b/c in a bounded loop, each
// step under its own guard, while the test lists the whole store repeatedly.
// Every listing must succeed and hold the two keys present throughout. With
// the ENOENT tolerance in treeEncoding.list removed, a listing fails with
// "no such file or directory".
func TestListKeysToleratesPrunedDirectories(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	require.NoError(t, s.WriteKey("keep/x", []byte("x")))
	require.NoError(t, s.WriteKey("z/y", []byte("y")))

	const churnMax = 1_000_000
	const listings = 3000
	stop := make(chan struct{})
	churned := make(chan error, 1)
	go func() { // test-only worker: one churn loop, joined through churned.
		for range churnMax {
			select {
			case <-stop:
				churned <- nil
				return
			default:
			}
			if err := s.WriteKey("a/b/c", []byte("c")); err != nil {
				churned <- err
				return
			}
			if err := s.RemoveKey("a/b/c"); err != nil {
				churned <- err
				return
			}
		}
		churned <- nil
	}()

	var listErr error
	for range listings {
		keys, err := s.ListKeys("")
		if err != nil {
			listErr = err
			break
		}
		if !assert.Subset(t, keys, []string{"keep/x", "z/y"}) {
			break
		}
	}
	close(stop)
	require.NoError(t, <-churned)
	require.NoError(t, listErr, "a listing failed while a writer pruned directories")
}
