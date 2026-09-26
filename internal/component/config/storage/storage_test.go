// Design: docs/architecture/storage-backends.md -- shared storage contract.
package storage

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTreeStorage(t *testing.T, dir string) Storage {
	t.Helper()
	s, err := Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	return s
}

func newBlobStorageAt(t *testing.T, dir string) Storage {
	t.Helper()
	s, err := CreateBlob(filepath.Join(dir, "test.zefs"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	return s
}

func TestStorageConformance(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			t.Run("operations", func(t *testing.T) {
				dir := t.TempDir()
				s := backend.newStore(t, dir)
				name := backend.configPath(dir)
				assert.False(t, s.Exists(name))
				_, err := s.ReadFile(name)
				require.ErrorIs(t, err, fs.ErrNotExist)
				require.ErrorIs(t, s.Remove(name), fs.ErrNotExist)
				_, err = s.Stat(name)
				require.ErrorIs(t, err, fs.ErrNotExist)
				require.ErrorIs(t, s.Rename(name, "missing.conf"), fs.ErrNotExist)
				require.NoError(t, s.WriteFile(name, []byte("first"), 0))
				assert.True(t, s.Exists(name))
				require.NoError(t, s.WriteFile(name, []byte("replacement is longer"), 0o600))
				got, err := s.ReadFile(name)
				require.NoError(t, err)
				assert.Equal(t, "replacement is longer", string(got))
				require.NoError(t, s.WriteFile("other.conf", []byte("independent"), 0))
				require.NoError(t, s.Rename(name, "renamed.conf"))
				assert.False(t, s.Exists(name))
				got, err = s.ReadFile("renamed.conf")
				require.NoError(t, err)
				assert.Equal(t, "replacement is longer", string(got))
				got, err = s.ReadFile("other.conf")
				require.NoError(t, err)
				assert.Equal(t, "independent", string(got))
				require.NoError(t, s.Rename("renamed.conf", "renamed.conf"))
				require.NoError(t, s.Remove("renamed.conf"))
				assert.False(t, s.Exists("renamed.conf"))
			})
			t.Run("guard", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				require.NoError(t, s.WriteFile("router.conf", []byte("original"), 0))
				g, err := s.AcquireLock("router.conf")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, g.Release()) })
				assert.True(t, g.Has("router.conf"))
				assert.False(t, g.Has("missing.conf"))
				got, err := g.ReadFile("router.conf")
				require.NoError(t, err)
				assert.Equal(t, "original", string(got))
				_, err = g.ReadFile("missing.conf")
				require.ErrorIs(t, err, fs.ErrNotExist)
				require.ErrorIs(t, g.Remove("missing.conf"), fs.ErrNotExist)
				g.SetModifier("alice")
				require.NoError(t, g.WriteFile("router.conf.draft", []byte("modified"), 0))
				got, err = g.ReadFile("router.conf.draft")
				require.NoError(t, err)
				assert.Equal(t, "modified", string(got))
				keys, err := g.List("file/active")
				require.NoError(t, err)
				assert.Equal(t, []string{"file/active/router.conf", "file/active/router.conf.draft"}, keys)
				require.NoError(t, g.Remove("router.conf.draft"))
				assert.False(t, g.Has("router.conf.draft"))
				keys, err = g.List("file/active")
				require.NoError(t, err)
				assert.Equal(t, []string{"file/active/router.conf"}, keys)
				require.NoError(t, g.Release())
				require.NoError(t, g.Release())
				assert.False(t, s.Exists("router.conf.draft"))
				require.ErrorIs(t, g.WriteFile("late.conf", nil, 0), fs.ErrClosed)
				_, err = g.ReadFile("router.conf")
				require.ErrorIs(t, err, fs.ErrClosed)
				require.ErrorIs(t, g.Remove("router.conf"), fs.ErrClosed)
				_, err = g.List("file/active")
				require.ErrorIs(t, err, fs.ErrClosed)
			})
			t.Run("owned reads", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				input := []byte("original")
				require.NoError(t, s.WriteKey("meta/owned/value", input))
				input[0] = 'X'
				got, err := s.ReadKey("meta/owned/value")
				require.NoError(t, err)
				assert.Equal(t, "original", string(got))
				got[0] = 'Y'
				copyRead, err := s.ReadFile("meta/owned/value")
				require.NoError(t, err)
				assert.Equal(t, "original", string(copyRead))
				require.NoError(t, s.WriteKey("meta/owned/value", []byte("new")))
				assert.Equal(t, "original", string(copyRead), "unlocked reads survive subsequent writes")
				require.NoError(t, s.Close())
				assert.Equal(t, "original", string(copyRead), "unlocked reads survive Close")
			})
			t.Run("raw and config lists", func(t *testing.T) {
				dir := t.TempDir()
				s := backend.newStore(t, dir)
				values := map[string]string{
					"file/active/router.conf":       "router",
					"file/active/router.conf.draft": "draft",
					"file/active/notes.txt":         "notes",
					"file/active/nested/site.conf":  "nested",
					"meta/ssh/host/user":            "alice",
				}
				for key, value := range values {
					require.NoError(t, s.WriteKey(key, []byte(value)))
				}
				keys, err := s.ListKeys("")
				require.NoError(t, err)
				assert.Equal(t, []string{"file/active/nested/site.conf", "file/active/notes.txt", "file/active/router.conf", "file/active/router.conf.draft", "meta/ssh/host/user"}, keys)
				keys, err = s.ListKeys("file/active/")
				require.NoError(t, err)
				assert.Equal(t, []string{"file/active/nested/site.conf", "file/active/notes.txt", "file/active/router.conf", "file/active/router.conf.draft"}, keys)
				for _, prefix := range []string{"file/active", "file/active/", dir} {
					keys, err = s.List(prefix)
					require.NoError(t, err)
					assert.Equal(t, []string{"file/active/notes.txt", "file/active/router.conf", "file/active/router.conf.draft"}, keys)
					for _, key := range keys {
						got, readErr := s.ReadFile(key)
						require.NoError(t, readErr)
						assert.Equal(t, values[key], string(got))
					}
				}
				keys, err = s.List("meta/ssh/host")
				require.NoError(t, err)
				assert.Equal(t, []string{"meta/ssh/host/user"}, keys)
				require.NoError(t, s.RemoveKey("meta/ssh/host/user"))
				keys, err = s.ListKeys("meta/ssh/")
				require.NoError(t, err)
				assert.Empty(t, keys)
			})
			t.Run("file directory conflicts and pruning", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				require.NoError(t, s.WriteKey("meta/branch/leaf", []byte("leaf")))
				require.Error(t, s.WriteKey("meta/branch", []byte("not a directory")))
				got, err := s.ReadKey("meta/branch/leaf")
				require.NoError(t, err)
				assert.Equal(t, "leaf", string(got))
				require.NoError(t, s.RemoveKey("meta/branch/leaf"))
				require.NoError(t, s.WriteKey("meta/branch", []byte("now a file")))
				require.Error(t, s.WriteKey("meta/branch/child", []byte("blocked")))
				got, err = s.ReadKey("meta/branch")
				require.NoError(t, err)
				assert.Equal(t, "now a file", string(got))
				require.NoError(t, s.RemoveKey("meta/branch"))
				require.NoError(t, s.WriteKey("meta/branch/child", []byte("directory again")))
				keys, err := s.ListKeys("")
				require.NoError(t, err)
				assert.Equal(t, []string{"meta/branch/child"}, keys)
			})
			t.Run("invalid raw keys", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				for _, key := range []string{"", ".", "../escape", "/absolute", "meta//double", "meta/../escape", "meta/trailing/"} {
					require.Error(t, s.WriteKey(key, []byte("invalid")), key)
					_, err := s.ReadKey(key)
					require.Error(t, err, key)
					require.Error(t, s.RemoveKey(key), key)
				}
				keys, err := s.ListKeys("")
				require.NoError(t, err)
				assert.Empty(t, keys)
			})
			t.Run("versions and metadata", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				versions, err := s.ListVersions("router.conf")
				require.NoError(t, err)
				assert.Empty(t, versions)
				older := mustParseVersionStamp(t, "20260318-100000.000")
				newer := mustParseVersionStamp(t, "20260319-113000.500")
				require.NoError(t, s.WriteVersion("router.conf", []byte("old"), older))
				g, err := s.AcquireLock("router.conf")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, g.Release()) })
				g.SetModifier("alice")
				before := time.Now()
				require.NoError(t, g.WriteFile("router.conf", []byte("current"), 0))
				require.NoError(t, g.WriteVersion("router.conf", []byte("new"), newer))
				require.NoError(t, g.Release())
				meta, err := s.Stat("router.conf")
				require.NoError(t, err)
				assert.Equal(t, "alice", meta.ModifiedBy)
				assert.False(t, meta.ModTime.Before(before))
				assert.False(t, meta.ModTime.After(time.Now()))
				require.NoError(t, s.Rename("router.conf", "renamed.conf"))
				renamed, err := s.Stat("renamed.conf")
				require.NoError(t, err)
				assert.Equal(t, meta, renamed)
				require.NoError(t, s.WriteVersion("other.conf", []byte("unrelated"), newer))
				require.NoError(t, s.WriteKey("file/not-a-stamp/router.conf", []byte("not history")))
				versions, err = s.ListVersions("router.conf")
				require.NoError(t, err)
				require.Len(t, versions, 2)
				assert.Equal(t, VersionInfo{Stamp: "20260319-113000.500", Date: newer, Path: "file/20260319-113000.500/router.conf"}, versions[0])
				assert.Equal(t, VersionInfo{Stamp: "20260318-100000.000", Date: older, Path: "file/20260318-100000.000/router.conf"}, versions[1])
				for i, want := range []string{"new", "old"} {
					got, readErr := s.ReadFile(versions[i].Path)
					require.NoError(t, readErr)
					assert.Equal(t, want, string(got))
				}
				meta, err = s.Stat(versions[0].Path)
				require.NoError(t, err)
				assert.Equal(t, FileMeta{ModTime: newer, ModifiedBy: "alice"}, meta)
			})
			t.Run("observer after release", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				var observed []string
				s.SetWriteObserver(func(key string) {
					got, err := s.ReadFile(key)
					assert.NoError(t, err)
					assert.Equal(t, "committed", string(got))
					observed = append(observed, key)
				})
				g, err := s.AcquireLock("router.conf")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, g.Release()) })
				require.NoError(t, g.WriteFile("router.conf", []byte("committed"), 0))
				assert.Empty(t, observed, "observers must not run inside the guard")
				require.NoError(t, g.Release())
				assert.Equal(t, []string{"file/active/router.conf"}, observed)
				require.NoError(t, s.WriteFile("other.conf", []byte("committed"), 0))
				assert.Equal(t, []string{"file/active/router.conf", "file/active/other.conf"}, observed)
				s.SetWriteObserver(nil)
				require.NoError(t, s.WriteFile("quiet.conf", []byte("not observed"), 0))
				assert.Len(t, observed, 2)
			})
			t.Run("guard serialization", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				first, err := s.AcquireLock("router.conf")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, first.Release()) })
				done := make(chan error, 1)
				go func() {
					second, lockErr := s.AcquireLock("router.conf")
					if lockErr != nil {
						done <- lockErr
						return
					}
					writeErr := second.WriteFile("router.conf", []byte("second"), 0)
					releaseErr := second.Release()
					if writeErr != nil {
						done <- writeErr
						return
					}
					done <- releaseErr
				}()
				require.NoError(t, first.WriteFile("router.conf", []byte("first"), 0))
				require.NoError(t, first.Release())
				require.NoError(t, <-done)
				got, err := s.ReadFile("router.conf")
				require.NoError(t, err)
				assert.Equal(t, "second", string(got))
			})
			t.Run("closed handle", func(t *testing.T) {
				s := backend.newStore(t, t.TempDir())
				require.NoError(t, s.Close())
				require.NoError(t, s.Close())
				_, err := s.ReadKey("meta/key")
				require.ErrorIs(t, err, fs.ErrClosed)
				require.ErrorIs(t, s.WriteKey("meta/key", nil), fs.ErrClosed)
				require.ErrorIs(t, s.RemoveKey("meta/key"), fs.ErrClosed)
				_, err = s.ListKeys("")
				require.ErrorIs(t, err, fs.ErrClosed)
				_, err = s.Stat("router.conf")
				require.ErrorIs(t, err, fs.ErrClosed)
				_, err = s.AcquireLock("router.conf")
				require.ErrorIs(t, err, fs.ErrClosed)
			})
		})
	}
}

func TestVersionStampRoundTrip(t *testing.T) {
	original := time.Date(2026, 3, 18, 10, 30, 45, 123_000_000, time.Local)
	stamp := FormatVersionStamp(original)
	assert.Equal(t, "20260318-103045.123", stamp)
	parsed, err := parseVersionStamp(stamp)
	require.NoError(t, err)
	assert.Equal(t, original.Truncate(time.Millisecond), parsed)
}

func TestParseVersionStampRejectsInvalid(t *testing.T) {
	for _, stamp := range []string{"../../../etc/shadow", "20260318-100000.1234", "20260318-100000.-01", "20260318-100000.abc"} {
		_, err := parseVersionStamp(stamp)
		require.Error(t, err, stamp)
	}
}

// VALIDATES: CheckName accepts a config whose directory is the store folder
// under a DIFFERENT SPELLING, and still refuses one that is really elsewhere.
//
// The guard compared filepath.Abs strings until 2026-09-18, and filepath.Abs
// resolves no symlink. On darwin os.MkdirTemp answers /var/folders/... while
// the framed-tree opener canonicalizes the same directory to /private/var/...,
// so every caller that passed an absolute config path was refused with "is
// outside store folder" about a file that was inside it. The editor suite,
// 165 of its 170 tests, failed on exactly that.
func TestCheckNameAcceptsAnotherSpellingOfTheStoreFolder(t *testing.T) {
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	if resolved == dir {
		// A host whose temporary directory crosses no symlink cannot pose the
		// question, so the symlink is made rather than assumed.
		resolved = filepath.Join(t.TempDir(), "store")
		require.NoError(t, os.Mkdir(resolved, 0o750))
		dir = filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(resolved, dir))
	}

	s := newTreeStorage(t, resolved)
	store, ok := s.(*store)
	require.True(t, ok, "Create returned %T, want *store", s)

	require.NoError(t, store.CheckName(filepath.Join(resolved, "ze.conf")),
		"the canonical spelling of the store folder is the store folder")
	require.NoError(t, store.CheckName(filepath.Join(dir, "ze.conf")),
		"another spelling of the store folder is the same directory")
	require.Error(t, store.CheckName(filepath.Join(t.TempDir(), "ze.conf")),
		"a config in another directory stays refused")
}

// guardRawKeys is the key set the guarded raw-key tests seed: nested history,
// a namespace outside meta/ and file/, and names that share a partial prefix.
var guardRawKeys = map[string]string{
	"meta/instance/name":              "r1",
	"meta/instances":                  "partial-prefix sibling",
	"file/active/router.conf":         "active",
	"file/20260926-101500.000/r.conf": "history",
	"file/draft/router.conf":          "draft",
	"filer/other":                     "shares fil",
	"object/ab/cdef":                  "future namespace",
}

func seedGuardRawKeys(t *testing.T, s Storage) {
	t.Helper()
	for key, value := range guardRawKeys {
		require.NoError(t, s.WriteKey(key, []byte(value)))
	}
}

// withinDeadline runs call and fails the test when it does not return in
// time: a guard method that re-takes the store's lock blocks forever, so a
// deadlock surfaces as this timeout.
func withinDeadline(t *testing.T, what string, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		call()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not return while the guard is held: deadlock", what)
	}
}

// TestGuardReadKeyNoDeadlock verifies AC-21: WriteGuard.ReadKey answers what
// Storage.ReadKey answers, inside a held guard, on both encodings.
//
// VALIDATES: AC-21, the guarded raw read reaches the encoding without locking again.
// PREVENTS: a backup walk that deadlocks on its own guard.
func TestGuardReadKeyNoDeadlock(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.newStore(t, t.TempDir())
			seedGuardRawKeys(t, s)
			g, err := s.AcquireLock("backup")
			require.NoError(t, err)
			for key, value := range guardRawKeys {
				var got []byte
				var readErr error
				withinDeadline(t, "ReadKey", func() { got, readErr = g.ReadKey(key) })
				require.NoError(t, readErr, key)
				assert.Equal(t, value, string(got), key)
			}
			_, err = g.ReadKey("meta/absent")
			require.ErrorIs(t, err, fs.ErrNotExist)
			_, err = g.ReadKey("../escape")
			require.Error(t, err)
			require.NoError(t, g.Release())
			_, err = g.ReadKey("meta/instance/name")
			require.ErrorIs(t, err, fs.ErrClosed)
		})
	}
}

// TestGuardListKeysRecursive verifies AC-21: WriteGuard.ListKeys answers the
// whole key space, nested history and unknown namespaces included, exactly as
// Storage.ListKeys does outside the guard, and sees the guard's own writes.
//
// VALIDATES: AC-21, the backup walk's enumeration.
// PREVENTS: a walk built on List, which answers only immediate file children.
func TestGuardListKeysRecursive(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.newStore(t, t.TempDir())
			seedGuardRawKeys(t, s)
			for _, prefix := range []string{"", "file/", "meta/", "object/"} {
				want, err := s.ListKeys(prefix)
				require.NoError(t, err)
				g, err := s.AcquireLock("backup")
				require.NoError(t, err)
				var got []string
				var listErr error
				withinDeadline(t, "ListKeys", func() { got, listErr = g.ListKeys(prefix) })
				require.NoError(t, listErr, prefix)
				require.NoError(t, g.Release())
				assert.Equal(t, want, got, "prefix %q", prefix)
			}
			all, err := s.ListKeys("")
			require.NoError(t, err)
			assert.Len(t, all, len(guardRawKeys))

			g, err := s.AcquireLock("backup")
			require.NoError(t, err)
			require.NoError(t, g.WriteFile("meta/added/inside", []byte("x"), 0))
			got, err := g.ListKeys("meta/added/")
			require.NoError(t, err)
			assert.Equal(t, []string{"meta/added/inside"}, got)
			require.NoError(t, g.Release())
		})
	}
}

// TestGuardListKeysPartialPrefix verifies AC-21: the prefix is literal, so a
// partial name segment matches every key that string-starts with it.
//
// VALIDATES: AC-21, ListKeys("fil") and ListKeys("meta/inst").
// PREVENTS: a prefix resolved to a directory name, as List resolves it.
func TestGuardListKeysPartialPrefix(t *testing.T) {
	cases := map[string][]string{
		"fil": {
			"file/20260926-101500.000/r.conf", "file/active/router.conf",
			"file/draft/router.conf", "filer/other",
		},
		"meta/inst": {"meta/instance/name", "meta/instances"},
		"absent/":   {},
	}
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			s := backend.newStore(t, t.TempDir())
			seedGuardRawKeys(t, s)
			g, err := s.AcquireLock("backup")
			require.NoError(t, err)
			defer func() { require.NoError(t, g.Release()) }()
			for prefix, want := range cases {
				got, err := g.ListKeys(prefix)
				require.NoError(t, err, prefix)
				assert.ElementsMatch(t, want, got, "prefix %q", prefix)
			}
			_, err = g.ListKeys("../")
			require.Error(t, err)
		})
	}
}

// TestGuardListKeysSorted verifies AC-21: both encodings answer the same keys
// in the same sorted order.
//
// VALIDATES: AC-21, ordering is part of the contract.
// PREVENTS: a backup whose key order depends on the source encoding.
func TestGuardListKeysSorted(t *testing.T) {
	var answers [][]string
	for _, backend := range pointerTestStores() {
		s := backend.newStore(t, t.TempDir())
		seedGuardRawKeys(t, s)
		g, err := s.AcquireLock("backup")
		require.NoError(t, err)
		got, err := g.ListKeys("")
		require.NoError(t, err)
		require.NoError(t, g.Release())
		assert.True(t, slices.IsSorted(got), "%s: %v", backend.name, got)
		answers = append(answers, got)
	}
	require.Len(t, answers, 2)
	assert.Equal(t, answers[0], answers[1])
}
