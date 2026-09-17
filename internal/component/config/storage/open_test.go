package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestOpenMissingDoesNotCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	for _, open := range []func(string) (Storage, error){Open, OpenReadOnly} {
		_, err := open(dir)
		require.ErrorIs(t, err, ErrNoStore)
		require.NoDirExists(t, dir)
	}
}

func TestOpenRefusesBlobStatOnly(t *testing.T) {
	for _, withTree := range []bool{false, true} {
		for _, kind := range []string{"regular", "fifo", "symlink", "directory"} {
			t.Run(kind+"-tree-"+map[bool]string{false: "absent", true: "present"}[withTree], func(t *testing.T) {
				dir := t.TempDir()
				if withTree {
					s := newTreeStorage(t, dir)
					require.NoError(t, s.Close())
				}
				path := filepath.Join(dir, "database.zefs")
				switch kind {
				case "regular":
					require.NoError(t, os.WriteFile(path, []byte("corrupt artifact"), 0o600))
				case "fifo":
					require.NoError(t, unix.Mkfifo(path, 0o600))
				case "symlink":
					require.NoError(t, os.Symlink(filepath.Join(dir, "missing-target"), path))
				case "directory":
					require.NoError(t, os.Mkdir(path, 0o700))
				}
				before, err := os.Lstat(path)
				require.NoError(t, err)
				for _, open := range []func(string) (Storage, error){Open, OpenReadOnly, Create} {
					s, openErr := open(dir)
					if s != nil {
						require.NoError(t, s.Close())
					}
					require.Error(t, openErr)
					assert.Contains(t, openErr.Error(), path)
					assert.Contains(t, openErr.Error(), "ze init --from")
				}
				after, err := os.Lstat(path)
				require.NoError(t, err)
				assert.True(t, os.SameFile(before, after))
				if !withTree {
					require.NoDirExists(t, filepath.Join(dir, "database"))
				}
				aside, err := filepath.Glob(path + ".replaced-*")
				require.NoError(t, err)
				assert.Empty(t, aside)
			})
		}
	}
}

func TestCreatePrivatePermissionsAndReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "config")
	s, err := Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	require.NoError(t, s.WriteFile("router.conf", []byte("secret"), 0o777))
	require.NoError(t, s.WriteKey("meta/deep/key", []byte("credential")))
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		want := fs.FileMode(0o600)
		if entry.IsDir() {
			want = 0o700
		}
		assert.Equal(t, want, info.Mode().Perm(), path)
		return nil
	}))
	require.NoFileExists(t, filepath.Join(dir, "router.conf"))
	require.NoFileExists(t, filepath.Join(dir, "router.conf.lock"))
	require.NoError(t, s.Close())
	reopened, err := Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	got, err := reopened.ReadFile("router.conf")
	require.NoError(t, err)
	assert.Equal(t, "secret", string(got))
}

// TestOpenFolderKeepsContainingMode opens a store whose containing directory
// is group/other-writable. The containing directory is not part of the store,
// so its mode is neither refused nor altered: the root's own 0700 and owner
// check is what bounds the store.
func TestOpenFolderKeepsContainingMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.Mkdir(dir, 0o775))
	require.NoError(t, os.Chmod(dir, 0o775))
	s, err := Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	require.NoError(t, s.WriteKey("meta/private/key", []byte("contained")))
	_, err = Open(dir)
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, s.Close())
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	got, err := reopened.ReadKey("meta/private/key")
	require.NoError(t, err)
	assert.Equal(t, "contained", string(got))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o775), info.Mode().Perm())
}

func TestOpenRejectsInsecureNodes(t *testing.T) {
	for _, node := range []string{"root", "directory", "file", "lock"} {
		t.Run(node, func(t *testing.T) {
			dir := t.TempDir()
			s := newTreeStorage(t, dir)
			require.NoError(t, s.WriteKey("meta/secret/key", []byte("secret")))
			require.NoError(t, s.Close())
			paths := map[string]string{
				"root":      filepath.Join(dir, "database"),
				"directory": filepath.Join(dir, "database", "meta", "secret"),
				"file":      filepath.Join(dir, "database", "meta", "secret", "key"),
				"lock":      filepath.Join(dir, "database.lock"),
			}
			path := paths[node]
			mode := fs.FileMode(0o755)
			if node == "file" || node == "lock" {
				mode = 0o644
			}
			require.NoError(t, os.Chmod(path, mode))
			_, err := Open(dir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), path)
			assert.ErrorIs(t, err, ErrPermissions)
		})
	}
}

// A refused open must not leave a lock file the operator never asked for.
func TestOpenRefusedLeavesNoLockFile(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	require.NoError(t, s.WriteKey("meta/secret/key", []byte("secret")))
	require.NoError(t, s.Close())
	lock := filepath.Join(dir, "database.lock")
	require.NoError(t, os.Remove(lock))
	require.NoError(t, os.Chmod(filepath.Join(dir, "database", "meta", "secret"), 0o755))
	_, err := Open(dir)
	require.ErrorIs(t, err, ErrPermissions)
	require.NoFileExists(t, lock)
}

func TestOpenRejectsSymlinksAndSpecialFiles(t *testing.T) {
	for _, node := range []string{"root", "directory", "key", "lock"} {
		for _, kind := range []string{"symlink", "fifo"} {
			t.Run(node+"-"+kind, func(t *testing.T) {
				dir := t.TempDir()
				s := newTreeStorage(t, dir)
				require.NoError(t, s.WriteKey("meta/secret/key", []byte("secret")))
				require.NoError(t, s.Close())
				paths := map[string]string{
					"root":      filepath.Join(dir, "database"),
					"directory": filepath.Join(dir, "database", "meta", "secret"),
					"key":       filepath.Join(dir, "database", "meta", "secret", "key"),
					"lock":      filepath.Join(dir, "database.lock"),
				}
				path := paths[node]
				require.NoError(t, os.Rename(path, path+".saved"))
				if kind == "symlink" {
					require.NoError(t, os.Symlink(path+".saved", path))
				} else {
					require.NoError(t, unix.Mkfifo(path, 0o600))
				}
				_, err := Open(dir)
				require.Error(t, err)
				assert.Contains(t, err.Error(), path)
			})
		}
	}
}

func TestOpenRejectsForeignOwnerEvenForRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing file ownership requires root")
	}
	for _, node := range []string{"folder", "root", "directory", "key", "lock"} {
		t.Run(node, func(t *testing.T) {
			dir := t.TempDir()
			s := newTreeStorage(t, dir)
			require.NoError(t, s.WriteKey("meta/secret/key", []byte("secret")))
			require.NoError(t, s.Close())
			paths := map[string]string{
				"folder":    dir,
				"root":      filepath.Join(dir, "database"),
				"directory": filepath.Join(dir, "database", "meta", "secret"),
				"key":       filepath.Join(dir, "database", "meta", "secret", "key"),
				"lock":      filepath.Join(dir, "database.lock"),
			}
			path := paths[node]
			require.NoError(t, os.Chown(path, 1, -1))
			t.Cleanup(func() { require.NoError(t, os.Chown(path, 0, -1)) })
			_, err := Open(dir)
			require.Error(t, err, "root must not bypass storage ownership checks")
			assert.Contains(t, err.Error(), path)
			assert.ErrorIs(t, err, ErrPermissions)
		})
	}
}

func TestTreeRejectsConfigOutsideItsFolder(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	inside := filepath.Join(dir, "router.conf")
	outside := filepath.Join(t.TempDir(), "router.conf")
	require.NoError(t, s.WriteFile(inside, []byte("inside"), 0))
	require.Error(t, s.WriteFile(outside, []byte("outside"), 0))
	_, err := s.ReadFile(outside)
	require.Error(t, err)
	require.Error(t, s.Remove(outside))
	require.Error(t, s.Rename(inside, outside))
	require.Error(t, s.Rename(outside, inside))
	require.Error(t, s.WriteVersion(outside, nil, time.Now()))
	_, err = s.ListVersions(outside)
	require.Error(t, err)
	_, err = s.AcquireLock(outside)
	require.Error(t, err)
	got, err := s.ReadFile(inside)
	require.NoError(t, err)
	assert.Equal(t, "inside", string(got))
	require.NoError(t, s.WriteFile("file/draft/nested/router.conf", []byte("namespaced"), 0))
	got, err = s.ReadKey("file/draft/nested/router.conf")
	require.NoError(t, err)
	assert.Equal(t, "namespaced", string(got))
}

func TestStorageReadOnlyAndOwnerContention(t *testing.T) {
	for _, backend := range []string{"tree", "blob"} {
		t.Run(backend, func(t *testing.T) {
			dir := t.TempDir()
			var writer Storage
			var openReader func() (Storage, error)
			var openWriter func() (Storage, error)
			lockPath := filepath.Join(dir, "database.lock")
			if backend == "tree" {
				writer = newTreeStorage(t, dir)
				openReader = func() (Storage, error) { return OpenReadOnly(dir) }
				openWriter = func() (Storage, error) { return Open(dir) }
			} else {
				writer = newBlobStorageAt(t, dir)
				path := filepath.Join(dir, "test.zefs")
				lockPath = path + ".lock"
				openReader = func() (Storage, error) { return OpenBlob(path, false) }
				openWriter = func() (Storage, error) { return OpenBlob(path, true) }
			}
			require.NoError(t, writer.WriteFile("router.conf", []byte("original"), 0))
			lockBefore, err := os.Stat(lockPath)
			require.NoError(t, err)
			_, err = openWriter()
			require.ErrorIs(t, err, ErrBusy)
			if backend == "blob" {
				_, err = openReader()
				require.ErrorIs(t, err, ErrBusy, "a mapped artifact reader must not overlap its writer")
				require.NoError(t, writer.Close())
			}
			reader, err := openReader()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })
			if backend == "blob" {
				_, err = openWriter()
				require.ErrorIs(t, err, ErrBusy, "a mapped artifact reader retains shared ownership")
				secondReader, readErr := openReader()
				require.NoError(t, readErr)
				require.NoError(t, secondReader.Close())
			}
			got, err := reader.ReadFile("router.conf")
			require.NoError(t, err)
			assert.Equal(t, "original", string(got))
			require.ErrorIs(t, reader.WriteFile("router.conf", nil, 0), ErrReadOnly)
			require.ErrorIs(t, reader.WriteKey("meta/new/key", nil), ErrReadOnly)
			require.ErrorIs(t, reader.Remove("router.conf"), ErrReadOnly)
			require.ErrorIs(t, reader.RemoveKey("file/active/router.conf"), ErrReadOnly)
			require.ErrorIs(t, reader.Rename("router.conf", "new.conf"), ErrReadOnly)
			require.ErrorIs(t, reader.WriteVersion("router.conf", nil, time.Now()), ErrReadOnly)
			_, err = reader.AcquireLock("router.conf")
			require.ErrorIs(t, err, ErrReadOnly)
			_, err = WriteCandidateVersion(reader, "router.conf", nil, time.Now())
			require.ErrorIs(t, err, ErrReadOnly)
			require.ErrorIs(t, ClearCandidate(reader, "router.conf"), ErrReadOnly)
			require.NoError(t, reader.Close())
			require.NoError(t, writer.Close())
			next, err := openWriter()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, next.Close()) })
			lockAfter, err := os.Stat(lockPath)
			require.NoError(t, err)
			assert.True(t, os.SameFile(lockBefore, lockAfter), "ownership uses one stable lock inode")
			got, err = next.ReadFile("router.conf")
			require.NoError(t, err)
			assert.Equal(t, "original", string(got))
		})
	}
}

func TestPopulatedPublicationAndReplacement(t *testing.T) {
	dir := t.TempDir()
	failure := errors.New("seed failed")
	_, err := CreatePopulated(dir, func(s Storage) error {
		require.NoDirExists(t, filepath.Join(dir, "database"))
		require.NoError(t, s.WriteKey("meta/seed/key", []byte("partial")))
		return failure
	})
	require.ErrorIs(t, err, failure)
	require.NoDirExists(t, filepath.Join(dir, "database"))
	first, err := CreatePopulated(dir, func(s Storage) error {
		return s.WriteKey("meta/seed/key", []byte("original"))
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, first.Close()) })
	_, err = ReplacePopulated(dir, func(Storage) error { t.Fatal("busy replacement ran seed callback"); return nil })
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, first.Close())
	_, err = CreatePopulated(dir, func(Storage) error { t.Fatal("existing creation ran seed callback"); return nil })
	require.ErrorIs(t, err, fs.ErrExist)
	_, err = ReplacePopulated(dir, func(s Storage) error {
		require.NoError(t, s.WriteKey("meta/seed/key", []byte("failed replacement")))
		return failure
	})
	require.ErrorIs(t, err, failure)
	original, err := Open(dir)
	require.NoError(t, err)
	got, err := original.ReadKey("meta/seed/key")
	require.NoError(t, err)
	assert.Equal(t, "original", string(got))
	require.NoError(t, original.Close())
	replacement, err := ReplacePopulated(dir, func(s Storage) error {
		return s.WriteKey("meta/seed/key", []byte("replacement"))
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, replacement.Close()) })
	got, err = replacement.ReadKey("meta/seed/key")
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(got))
	aside, err := filepath.Glob(filepath.Join(dir, "database.replaced-*"))
	require.NoError(t, err)
	require.Len(t, aside, 1)
	require.FileExists(t, filepath.Join(aside[0], "meta", "seed", "key"))
}

func TestOpenAcceptsOrdinaryConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o755))
	s := newTreeStorage(t, dir)
	require.NoError(t, s.WriteFile("router.conf", []byte("config"), 0))
	require.NoError(t, s.Close())
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	got, err := reopened.ReadFile("router.conf")
	require.NoError(t, err)
	assert.Equal(t, "config", string(got))
}

func TestOpenIgnoresUnpublishedStagingTrees(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"database.init-tmp-interrupted", "database.import-tmp-interrupted"} {
		stage := filepath.Join(dir, name)
		require.NoError(t, os.Mkdir(stage, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(stage, "partial"), []byte("never live"), 0o600))
	}
	_, err := Open(dir)
	require.ErrorIs(t, err, ErrNoStore)
	s := newTreeStorage(t, dir)
	keys, err := s.ListKeys("")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestTreeReadOnlySeesPublishedWrites(t *testing.T) {
	dir := t.TempDir()
	writer := newTreeStorage(t, dir)
	require.NoError(t, writer.WriteKey("meta/state/value", []byte("before")))
	reader, err := OpenReadOnly(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	before, err := reader.ReadKey("meta/state/value")
	require.NoError(t, err)
	require.NoError(t, writer.WriteKey("meta/state/value", []byte("after")))
	after, err := reader.ReadKey("meta/state/value")
	require.NoError(t, err)
	assert.Equal(t, "before", string(before))
	assert.Equal(t, "after", string(after))
	require.NoError(t, writer.RemoveKey("meta/state/value"))
	_, err = reader.ReadKey("meta/state/value")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// TestOpenRefusesUnfinishedImport proves the crash window between moving the
// old tree to database.replaced-* and publishing the stage: an intent beside
// an absent tree is ErrImportPending for every live opener and never
// ErrNoStore, so Create builds nothing over the operator's store, the refusal
// names the intent file and the repair, and ImportBlob still resumes. An intent
// that cannot be read refuses the same way.
func TestOpenRefusesUnfinishedImport(t *testing.T) {
	path, dir, values, intent := interruptedImport(t, "prepared")
	replaced := filepath.Join(dir, "database.replaced-20260101T000000.000000000")
	require.NoError(t, os.Mkdir(replaced, 0o700))
	seed := func(dir string) (Storage, error) { return CreatePopulated(dir, func(Storage) error { return nil }) }
	openers := []func(string) (Storage, error){Open, OpenReadOnly, Create, seed}
	for _, open := range openers {
		s, err := open(dir)
		if s != nil {
			require.NoError(t, s.Close())
		}
		require.ErrorIs(t, err, ErrImportPending)
		require.NotErrorIs(t, err, ErrNoStore)
		assert.Contains(t, err.Error(), filepath.Join(dir, "database.import-intent"))
		assert.Contains(t, err.Error(), "ze init --from "+path)
		require.NoDirExists(t, filepath.Join(dir, "database"))
		require.DirExists(t, replaced)
	}
	s, err := ImportBlob(path, dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	assertImportValues(t, s, values)
	require.NoFileExists(t, filepath.Join(dir, "database.import-intent"))
	require.NoDirExists(t, filepath.Join(dir, intent.Stage))
	require.DirExists(t, replaced)

	unreadable := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(unreadable, "database.import-intent"), []byte("not json"), 0o600))
	for _, open := range openers {
		s, err := open(unreadable)
		if s != nil {
			require.NoError(t, s.Close())
		}
		require.ErrorIs(t, err, ErrImportPending)
		require.NotErrorIs(t, err, ErrNoStore)
		assert.Contains(t, err.Error(), "cannot be read")
		require.NoDirExists(t, filepath.Join(unreadable, "database"))
	}
}
