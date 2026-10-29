// Design: docs/architecture/storage-backends.md -- readers and the write lock.
package storage

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/statestore"
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

// TestConfigurationGuardThatReachesRuntimeStateHoldsIt proves a guard that
// walks the whole store still excludes runtime-state writers once it lists.
//
// Method: a backup lists every key under one guard and copies them, so the
// copy must not change under it. The guard lists the store, then a runtime
// write starts from another goroutine. It must not land while the guard is
// held, and it must land once the guard is released.
func TestConfigurationGuardThatReachesRuntimeStateHoldsIt(t *testing.T) {
	for _, backend := range []struct {
		name string
		open func(*testing.T, string) Storage
	}{{name: "tree", open: newTreeStorage}, {name: "memory", open: newMemoryStorage}} {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.open(t, t.TempDir())
			guard, err := s.AcquireLock("")
			require.NoError(t, err)
			_, err = guard.ListKeys("")
			require.NoError(t, err)

			written := make(chan error, 1)
			go func() { // test-only worker: one write, joined through written.
				written <- s.WriteKey("meta/ospf/auth/boot-count", []byte{0, 0, 0, 1})
			}()
			select {
			case err := <-written:
				t.Errorf("a runtime write landed under a guard that listed the store: %v", err)
			case <-time.After(300 * time.Millisecond):
			}
			require.NoError(t, guard.Release())
			require.NoError(t, <-written)
		})
	}
}

// TestRuntimeGuardRefusesConfigurationKeys proves a guard acquired for a
// runtime-state key cannot reach the configuration space, which it holds no
// lock over.
func TestRuntimeGuardRefusesConfigurationKeys(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	guard, err := s.AcquireLock("meta/ospf/auth/boot-count")
	require.NoError(t, err)
	require.NoError(t, guard.WriteKey("meta/ospf/auth/boot-count", []byte{0, 0, 0, 1}))
	_, err = guard.ListKeys("meta/ospf/")
	require.NoError(t, err)
	const refusal = "acquired for runtime state"
	assert.ErrorContains(t, guard.WriteKey("meta/config/router.conf/active", []byte("x")), refusal)
	_, err = guard.ListKeys("")
	assert.ErrorContains(t, err, refusal)
	_, err = guard.ListKeys("meta/c")
	assert.ErrorContains(t, err, refusal)
	_, err = guard.ReadFile("router.conf")
	assert.ErrorContains(t, err, refusal)
	_, err = guard.List("router.conf")
	assert.ErrorContains(t, err, refusal)
	assert.Contains(t, panicText(func() { guard.Has("router.conf") }), refusal,
		"Has on a configuration name under a runtime guard must not answer false as if the key were absent")
	require.NoError(t, guard.Release())
}

// panicText runs fn and returns what it panicked with, or "" when it returned.
func panicText(fn func()) (text string) {
	defer func() {
		if r := recover(); r != nil {
			text = fmt.Sprint(r)
		}
	}()
	fn()
	return ""
}

// TestRuntimeWriteDoesNotWaitForADirectorySync proves a runtime-state write
// whose directories exist does not wait for a configuration guard's directory
// sync.
//
// Method: the boot record creates file/<stamp>/ on every version it writes,
// and a slow disk makes the sync of the directory it created under take
// seconds, while a plugin's runtime-state write carries a deadline of its own
// (OSPF increments meta/ospf/auth/boot-count under five seconds). The test blocks the first directory sync after a configuration
// guard starts creating a directory, then increments an existing runtime-state
// counter through statestore from another goroutine: it must answer while the
// sync is still blocked.
func TestRuntimeWriteDoesNotWaitForADirectorySync(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	owned, ok := s.(*store)
	require.True(t, ok, "tree storage is not *store")
	statestore.SetStore(s)
	t.Cleanup(func() { statestore.SetStore(nil) })
	_, err := statestore.Increment("meta/ospf/auth/boot-count")
	require.NoError(t, err)

	blocked, release := make(chan struct{}), make(chan struct{})
	// An atomic flag rather than sync.Once: Once.Do makes every later caller
	// wait for the first, which is the very wait under test.
	var armed atomic.Bool
	armed.Store(true)
	owned.tree.sync = func(file *os.File) error {
		if armed.CompareAndSwap(true, false) {
			close(blocked)
			<-release
		}
		return file.Sync()
	}
	written := make(chan error, 1)
	go func() { // test-only worker: one configuration write, joined through written.
		written <- s.WriteKey("meta/config/router.conf/candidate", []byte("pending"))
	}()
	<-blocked

	counted := make(chan error, 1)
	go func() { // test-only worker: one increment, joined through counted.
		_, err := statestore.Increment("meta/ospf/auth/boot-count")
		counted <- err
	}()
	select {
	case err := <-counted:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Error("a runtime-state write waited for a configuration guard's directory sync")
		close(release)
		<-counted
		require.NoError(t, <-written)
		return
	}
	close(release)
	require.NoError(t, <-written)
	value, err := statestore.Increment("meta/ospf/auth/boot-count")
	require.NoError(t, err)
	assert.Equal(t, uint32(3), value)
}

// TestRenameBesideRuntimeWrites proves Rename reads the published metadata
// safely while runtime-state guards publish theirs. Run it under -race: the
// two never wait for each other's write lock, so only mu orders the map.
func TestRenameBesideRuntimeWrites(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	require.NoError(t, s.WriteFile("a.conf", []byte("x"), 0o600))
	stop := make(chan struct{})
	written := make(chan error, 1)
	go func() { // test-only worker: runtime writes until stop, joined through written.
		for i := 0; ; i++ {
			select {
			case <-stop:
				written <- nil
				return
			default:
			}
			if err := s.WriteKey("meta/ospf/auth/boot-count", []byte{0, 0, 0, byte(i)}); err != nil {
				written <- err
				return
			}
		}
	}()
	for range 30 {
		require.NoError(t, s.Rename("a.conf", "b.conf"))
		require.NoError(t, s.Rename("b.conf", "a.conf"))
	}
	close(stop)
	require.NoError(t, <-written)
}

// TestDirectoryCreationBesidePruning proves the two key spaces create and prune
// their directories under meta at once without an error.
//
// Method: a configuration writer and a runtime-state writer each write a key
// in a directory of their own under meta, then remove it, which prunes the
// directory and tries meta itself. Neither waits for the other's lock, so
// only layout orders creating meta against pruning it: without it, a write
// can find meta gone after opening it, or fail with EEXIST creating it.
func TestDirectoryCreationBesidePruning(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	cycle := func(key string, done chan<- error) {
		for range 50 {
			if err := s.WriteKey(key, []byte("x")); err != nil {
				done <- err
				return
			}
			if err := s.RemoveKey(key); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}
	configuration, runtime := make(chan error, 1), make(chan error, 1)
	go cycle("meta/config/router.conf/candidate", configuration) // test-only worker, joined below.
	go cycle("meta/ospf/auth/boot-count", runtime)               // test-only worker, joined below.
	require.NoError(t, <-configuration)
	require.NoError(t, <-runtime)
}
