package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentCreateRetainsOneOwner(t *testing.T) {
	dir := t.TempDir()
	type result struct {
		storage Storage
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			s, err := Create(dir)
			results <- result{storage: s, err: err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	for _, r := range []result{first, second} {
		if r.storage != nil {
			t.Cleanup(func() { require.NoError(t, r.storage.Close()) })
		}
	}
	if first.err != nil {
		first, second = second, first
	}
	require.NoError(t, first.err)
	require.NotNil(t, first.storage)
	require.ErrorIs(t, second.err, ErrBusy)
	require.Nil(t, second.storage)
	require.NoError(t, first.storage.WriteKey("meta/winner/key", []byte("winner")))
	require.NoError(t, first.storage.Close())
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	got, err := reopened.ReadKey("meta/winner/key")
	require.NoError(t, err)
	assert.Equal(t, "winner", string(got))
}

func TestCreatePopulatedDoesNotReplaceEmptyTree(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "database")
	require.NoError(t, os.Mkdir(root, 0o700))
	before, err := os.Stat(root)
	require.NoError(t, err)
	_, err = CreatePopulated(dir, func(Storage) error {
		t.Fatal("creation must refuse an existing empty tree before populating")
		return nil
	})
	require.ErrorIs(t, err, fs.ErrExist)
	after, err := os.Stat(root)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after), "an empty live tree is not disposable staging")
	s, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	keys, err := s.ListKeys("")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestTreeDirectoryBarrierFailureDoesNotPublishCandidate(t *testing.T) {
	for _, boundary := range []string{"new ancestor", "post rename parent"} {
		t.Run(boundary, func(t *testing.T) {
			dir := t.TempDir()
			s := newTreeStorage(t, dir)
			oldStamp := "20260524-090000.000"
			newStamp := "20260524-100000.000"
			_, _, err := EnsureActiveVersion(s, "router.conf", []byte("active"), mustParseVersionStamp(t, oldStamp))
			require.NoError(t, err)
			if boundary == "post rename parent" {
				require.NoError(t, s.WriteVersion("other.conf", []byte("retain parent"), mustParseVersionStamp(t, newStamp)))
			}
			concrete, ok := s.(*store)
			require.True(t, ok)
			failure := errors.New("injected directory sync failure")
			failed := false
			concrete.tree.sync = func(file *os.File) error {
				info, statErr := file.Stat()
				if statErr != nil {
					return statErr
				}
				if info.IsDir() {
					if boundary == "new ancestor" && filepath.Base(file.Name()) == "file" {
						failed = true
						return failure
					}
					if boundary == "post rename parent" && filepath.Base(file.Name()) == newStamp {
						if _, statErr := os.Stat(filepath.Join(file.Name(), "router.conf")); statErr != nil {
							return statErr
						}
						failed = true
						return failure
					}
				}
				return file.Sync()
			}
			t.Cleanup(func() { concrete.tree.sync = nil })
			var observed []string
			s.SetWriteObserver(func(key string) { observed = append(observed, key) })
			_, err = WriteCandidateVersion(s, "router.conf", []byte("candidate"), mustParseVersionStamp(t, newStamp))
			require.ErrorIs(t, err, failure)
			require.True(t, failed, "the selected durability barrier must be reached")
			assert.Empty(t, observed)
			_, present, err := readPointer(s, "router.conf", pointerCandidate)
			require.NoError(t, err)
			assert.False(t, present, "a failed version barrier must never be followed by a candidate pointer")
			active, present, err := readPointer(s, "router.conf", pointerActive)
			require.NoError(t, err)
			require.True(t, present)
			assert.Equal(t, oldStamp, active)
			got, err := ReadActiveConfig(s, "router.conf")
			require.NoError(t, err)
			assert.Equal(t, "active", string(got))
		})
	}
}

func TestTreeSupportsKeysDeeperThanSixtyFourComponents(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	key := "meta/" + strings.Repeat("d/", 80) + "value"
	require.NoError(t, s.WriteKey(key, []byte("deep value")))
	got, err := s.ReadKey(key)
	require.NoError(t, err)
	assert.Equal(t, "deep value", string(got))
	require.NoError(t, s.Close())
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	keys, err := reopened.ListKeys("meta/")
	require.NoError(t, err)
	assert.Equal(t, []string{key}, keys)
	got, err = reopened.ReadKey(key)
	require.NoError(t, err)
	assert.Equal(t, "deep value", string(got))
	require.NoError(t, reopened.RemoveKey(key))
	keys, err = reopened.ListKeys("")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestTreePublicationRetainsRenamedParent(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "publish"
		if fail {
			name = "cleanup"
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			dir, moved := filepath.Join(base, "config"), filepath.Join(base, "moved")
			require.NoError(t, os.Mkdir(dir, 0o700))
			folder, err := openFolder(dir)
			require.NoError(t, err)
			owner, err := lockOwner(folder, "database.lock")
			require.NoError(t, err)
			t.Cleanup(func() { _ = folder.Close(); _ = owner.Close() })
			require.NoError(t, os.Rename(dir, moved))
			require.NoError(t, os.Mkdir(dir, 0o700))
			// A replacement pathname is neither the publication destination nor
			// a reason to refuse creation in the already-owned directory.
			require.NoError(t, os.Mkdir(filepath.Join(dir, "database"), 0o700))
			decoy := filepath.Join(dir, "database", "unrelated")
			require.NoError(t, os.WriteFile(decoy, []byte("retain"), 0o600))
			failure := errors.New("population failed")
			var stageName string
			s, err := populateOwned(folder, owner, func(s Storage) error {
				staged, ok := s.(*store)
				require.True(t, ok)
				stageName = filepath.Base(staged.tree.root.Name())
				require.NoError(t, os.Mkdir(filepath.Join(dir, stageName), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, stageName, "unrelated"), []byte("retain stage"), 0o600))
				if err := s.WriteKey("meta/test/key", []byte("owned")); err != nil {
					return err
				}
				if fail {
					return failure
				}
				return nil
			}, false)
			if fail {
				require.ErrorIs(t, err, failure)
				require.NoDirExists(t, filepath.Join(moved, stageName))
				require.NoDirExists(t, filepath.Join(moved, "database"))
			} else {
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, s.Close()) })
				require.NoError(t, s.WriteKey("meta/test/key", []byte("updated")))
				got, err := s.ReadKey("meta/test/key")
				require.NoError(t, err)
				assert.Equal(t, "updated", string(got))
				require.DirExists(t, filepath.Join(moved, "database"))
				require.NoDirExists(t, filepath.Join(moved, stageName))
			}
			got, err := os.ReadFile(decoy)
			require.NoError(t, err)
			assert.Equal(t, "retain", string(got))
			got, err = os.ReadFile(filepath.Join(dir, stageName, "unrelated"))
			require.NoError(t, err)
			assert.Equal(t, "retain stage", string(got))
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			assert.Len(t, entries, 2, "staging must not create files in the replacement parent")
		})
	}
}

func TestImportPublicationRetainsRenamedParent(t *testing.T) {
	source, values := importFixture(t)
	base := t.TempDir()
	dir, moved := filepath.Join(base, "config"), filepath.Join(base, "moved")
	require.NoError(t, os.Mkdir(dir, 0o700))
	folder, err := openFolder(dir)
	require.NoError(t, err)
	owner, err := lockOwner(folder, "database.lock")
	require.NoError(t, err)
	t.Cleanup(func() { _ = folder.Close(); _ = owner.Close() })
	require.NoError(t, os.Rename(dir, moved))
	require.NoError(t, os.Mkdir(dir, 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "database"), 0o700))
	decoy := filepath.Join(dir, "database", "unrelated")
	require.NoError(t, os.WriteFile(decoy, []byte("retain"), 0o600))
	s, err := importOwned(source, folder, owner, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	assertImportValues(t, s, values)
	require.DirExists(t, filepath.Join(moved, "database"))
	require.NoFileExists(t, source)
	require.NoFileExists(t, filepath.Join(moved, "database.import-intent"))
	got, err := os.ReadFile(decoy)
	require.NoError(t, err)
	assert.Equal(t, "retain", string(got))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "database", entries[0].Name())
}

func TestWriteConfigFileRejectsSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	path := filepath.Join(target, "router.conf")
	require.NoError(t, os.WriteFile(path, []byte("retain"), 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	require.Error(t, WriteConfigFile(filepath.Join(link, "router.conf"), []byte("replace")))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "retain", string(got))
}

func TestFailedFrameCleanupRetainsRenamedParent(t *testing.T) {
	base := t.TempDir()
	dir, moved := filepath.Join(base, "config"), filepath.Join(base, "moved")
	s := newTreeStorage(t, dir)
	failure := errors.New("frame sync failed")
	var stageName string
	concrete, ok := s.(*store)
	require.True(t, ok)
	concrete.tree.sync = func(file *os.File) error {
		if !strings.HasPrefix(filepath.Base(file.Name()), ".ze-storage-") {
			return file.Sync()
		}
		stageName = filepath.Base(file.Name())
		require.NoError(t, os.Rename(dir, moved))
		require.NoError(t, os.Mkdir(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, stageName), []byte("retain"), 0o600))
		return failure
	}
	require.ErrorIs(t, s.WriteKey("meta/test/key", []byte("not published")), failure)
	_, err := s.ReadKey("meta/test/key")
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.NoFileExists(t, filepath.Join(moved, stageName))
	got, err := os.ReadFile(filepath.Join(dir, stageName))
	require.NoError(t, err)
	assert.Equal(t, "retain", string(got))
}
