package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

// TestBackupCarriesEveryKey verifies AC-1: the artifact holds every key of the
// source byte-equal, nested history and unknown namespaces included, from
// either encoding.
//
// VALIDATES: AC-1, the walk is guard.ListKeys("") then guard.ReadKey.
// PREVENTS: a backup built on List, which drops everything below one level.
func TestBackupCarriesEveryKey(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			source := backend.newStore(t, t.TempDir())
			seedGuardRawKeys(t, source)
			path := filepath.Join(t.TempDir(), "out.zefs")
			result, err := Backup(source, path, zefs.Spare(0))
			require.NoError(t, err)
			assert.Equal(t, path, result.Path)
			assert.Equal(t, len(guardRawKeys), result.Keys)

			artifact, err := OpenBlob(path, false)
			require.NoError(t, err)
			defer func() { require.NoError(t, artifact.Close()) }()
			keys, err := artifact.ListKeys("")
			require.NoError(t, err)
			assert.Len(t, keys, len(guardRawKeys))
			for key, value := range guardRawKeys {
				got, err := artifact.ReadKey(key)
				require.NoError(t, err, key)
				assert.Equal(t, value, string(got), key)
			}
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, result.Bytes, info.Size())
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}
}

// TestBackupExactFit verifies AC-1 and AC-2: every key and data slot is
// exact-fit at spare 0, and used plus the percentage at spare 25.
//
// VALIDATES: AC-1, AC-2, the artifact's policy is the one the caller states.
// PREVENTS: an artifact padded by the zefs default.
func TestBackupExactFit(t *testing.T) {
	source := newTreeStorage(t, t.TempDir())
	seedGuardRawKeys(t, source)
	for _, percent := range []int{0, 25} {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("spare-%d.zefs", percent))
		_, err := Backup(source, path, zefs.Spare(percent))
		require.NoError(t, err)
		report, err := zefs.Check(path)
		require.NoError(t, err)
		require.Zero(t, report.CorruptEntries)
		require.Equal(t, len(guardRawKeys), report.TotalEntries)
		for _, entry := range report.Entries {
			assert.Equal(t, len(entry.Key)+len(entry.Key)*percent/100, entry.KeyCapacity, entry.Key)
			assert.Equal(t, entry.Size+entry.Size*percent/100, entry.Capacity, entry.Key)
		}
	}
	_, err := Backup(source, filepath.Join(t.TempDir(), "bad.zefs"), zefs.Spare(101))
	require.ErrorContains(t, err, "0 to 100")
}

// TestBackupUnderLock verifies R-1: a pair of keys written together under one
// guard is in every backup together or not at all, while writers race the walk.
//
// VALIDATES: AC-4's consistency half, the walk holds one guard throughout.
// PREVENTS: a backup that straddles a commit's version and pointer.
func TestBackupUnderLock(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			source := backend.newStore(t, t.TempDir())
			stop := make(chan struct{})
			var writers sync.WaitGroup
			writers.Go(func() {
				for round := 0; ; round++ {
					select {
					case <-stop:
						return
					default:
					}
					guard, err := source.AcquireLock("writer")
					if err != nil {
						t.Error(err)
						return
					}
					value := []byte(fmt.Sprintf("round-%d", round))
					writeErr := errors.Join(
						guard.WriteFile(fmt.Sprintf("file/v%04d/router.conf", round), value, 0),
						guard.WriteFile("meta/pointer/router.conf", value, 0),
					)
					if err := errors.Join(writeErr, guard.Release()); err != nil {
						t.Error(err)
						return
					}
				}
			})
			dir := t.TempDir()
			for i := range 10 {
				path := filepath.Join(dir, fmt.Sprintf("b%d.zefs", i))
				_, err := Backup(source, path, zefs.Spare(0))
				require.NoError(t, err)
				artifact, err := OpenBlob(path, false)
				require.NoError(t, err)
				pointer, pointerErr := artifact.ReadKey("meta/pointer/router.conf")
				versions, err := artifact.ListKeys("file/v")
				require.NoError(t, err)
				if pointerErr == nil {
					latest, err := artifact.ReadKey(versions[len(versions)-1])
					require.NoError(t, err)
					assert.Equal(t, string(pointer), string(latest), "pointer and newest version disagree")
				} else {
					assert.Empty(t, versions, "versions without their pointer")
				}
				require.NoError(t, artifact.Close())
			}
			close(stop)
			writers.Wait()
		})
	}
}

// TestBackupRefusesStorePaths verifies R-10's list for the offline walk: every
// name the live store owns in its folder is refused, and nothing is written.
//
// VALIDATES: R-10, AC-4's exclusion list, before any write.
// PREVENTS: a backup that corrupts, blocks or unlocks the store it copies.
func TestBackupRefusesStorePaths(t *testing.T) {
	dir := t.TempDir()
	source := newTreeStorage(t, dir)
	seedGuardRawKeys(t, source)
	for _, name := range []string{
		"database", "database/meta/out.zefs", "database.zefs", "database.lock", "out.zefs.lock",
		"database.import-intent", "database.replaced-20260926", "database.zefs.replaced-20260926",
		"database.init-tmp-1", "database.import-tmp-1",
	} {
		_, err := Backup(source, filepath.Join(dir, name), zefs.Spare(0))
		require.ErrorIs(t, err, errors.ErrUnsupported, name)
	}
	_, err := os.Lstat(filepath.Join(dir, "database.zefs"))
	require.ErrorIs(t, err, os.ErrNotExist)
	result, err := Backup(source, filepath.Join(dir, "beside.zefs"), zefs.Spare(0))
	require.NoError(t, err)
	assert.Equal(t, len(guardRawKeys), result.Keys)
	_, err = Backup(source, filepath.Join(dir, "beside.zefs"), zefs.Spare(0))
	require.ErrorIs(t, err, os.ErrExist)
}
