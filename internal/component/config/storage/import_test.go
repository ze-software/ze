package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

func importFixture(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	s, err := CreateBlob(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	values := map[string][]byte{
		"file/active/router.conf":              []byte("router-id 192.0.2.1\n"),
		"file/20260524-100000.000/router.conf": []byte("historical config"),
		"meta/config/router.conf/active":       []byte("20260524-100000.000"),
		"meta/auth/local/password":             {0, 1, 2, 0xff, '\n'},
		"meta/empty/value":                     {},
		"custom/unregistered/nested/key":       []byte("unknown keys must survive"),
	}
	for key, value := range values {
		require.NoError(t, s.WriteKey(key, value))
	}
	require.NoError(t, s.Close())
	return path, values
}

func assertImportValues(t *testing.T, s Storage, values map[string][]byte) {
	t.Helper()
	wantKeys := make([]string, 0, len(values))
	for key, want := range values {
		wantKeys = append(wantKeys, key)
		got, err := s.ReadKey(key)
		require.NoError(t, err, key)
		assert.True(t, bytes.Equal(want, got), "%s: got %x, want %x", key, got, want)
	}
	slices.Sort(wantKeys)
	keys, err := s.ListKeys("")
	require.NoError(t, err)
	assert.Equal(t, wantKeys, keys)
}

func TestImportEqualsSource(t *testing.T) {
	path, values := importFixture(t)
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	dir := t.TempDir()
	s, err := ImportBlob(path, dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	assertImportValues(t, s, values)
	require.NoFileExists(t, path)
	require.NoFileExists(t, filepath.Join(dir, "database.import-intent"))
	aside, err := filepath.Glob(path + ".replaced-*")
	require.NoError(t, err)
	require.Len(t, aside, 1)
	archived, err := os.ReadFile(aside[0])
	require.NoError(t, err)
	assert.Equal(t, original, archived)
	require.NoError(t, s.Close())
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	assertImportValues(t, reopened, values)
}

func TestImportRefusesCorruptSource(t *testing.T) {
	for _, kind := range []string{"empty", "entry crc"} {
		t.Run(kind, func(t *testing.T) {
			path, _ := importFixture(t)
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			if kind == "empty" {
				content = nil
			} else {
				offset := bytes.Index(content, []byte("historical config"))
				require.NotEqual(t, -1, offset)
				content[offset] ^= 1
			}
			require.NoError(t, os.WriteFile(path, content, 0o600))
			dir := t.TempDir()
			_, err = ImportBlob(path, dir)
			require.Error(t, err)
			require.NoDirExists(t, filepath.Join(dir, "database"))
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(content), string(got))
			aside, err := filepath.Glob(path + ".replaced-*")
			require.NoError(t, err)
			assert.Empty(t, aside)
		})
	}
}

// interruptedImport installs the durable state at an import crash boundary.
// Recovery is exercised through ImportBlob, not the private recovery functions.
func interruptedImport(t *testing.T, phase string) (string, string, map[string][]byte, importIntent) {
	t.Helper()
	path, values := importFixture(t)
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	for key, value := range values {
		require.NoError(t, s.WriteKey(key, value))
	}
	require.NoError(t, s.Close())
	root := filepath.Join(dir, "database")
	var stat unix.Stat_t
	require.NoError(t, unix.Stat(root, &stat))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	digest := sha256.Sum256(data)
	intent := importIntent{
		Source:  path,
		Digest:  hex.EncodeToString(digest[:]),
		Stage:   "database.import-tmp-interrupted",
		Archive: path + ".replaced-interrupted",
		Device:  uint64(stat.Dev),
		Inode:   stat.Ino,
	}
	if phase == "prepared" {
		require.NoError(t, os.Rename(root, filepath.Join(dir, intent.Stage)))
	}
	if phase == "retired" {
		require.NoError(t, os.Rename(path, intent.Archive))
	}
	encoded, err := json.Marshal(intent)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "database.import-intent"), encoded, 0o600))
	return path, dir, values, intent
}

func TestImportResumesCrashBoundaries(t *testing.T) {
	for _, phase := range []string{"prepared", "published", "retired"} {
		t.Run(phase, func(t *testing.T) {
			path, dir, values, intent := interruptedImport(t, phase)
			s, err := ImportBlob(path, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, s.Close()) })
			assertImportValues(t, s, values)
			require.NoFileExists(t, path)
			archives, err := filepath.Glob(path + ".replaced-*")
			require.NoError(t, err)
			require.Len(t, archives, 1)
			if phase != "prepared" {
				assert.Equal(t, intent.Archive, archives[0])
			}
			require.NoFileExists(t, filepath.Join(dir, "database.import-intent"))
			require.NoDirExists(t, filepath.Join(dir, intent.Stage))
			require.NoError(t, s.Close())
			reopened, err := Open(dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			assertImportValues(t, reopened, values)
		})
	}
}

func TestImportRecoveryRefusesChangedIdentityOrBytes(t *testing.T) {
	for _, change := range []string{"source", "destination value", "destination keys", "destination inode", "destination device"} {
		t.Run(change, func(t *testing.T) {
			path, dir, _, intent := interruptedImport(t, "published")
			switch change {
			case "source":
				s, err := OpenBlob(path, true)
				require.NoError(t, err)
				require.NoError(t, s.WriteKey("meta/changed/key", []byte("changed source")))
				require.NoError(t, s.Close())
			case "destination value", "destination keys":
				s, err := Open(dir)
				require.NoError(t, err)
				key := "file/active/router.conf"
				if change == "destination keys" {
					key = "meta/extra/key"
				}
				require.NoError(t, s.WriteKey(key, []byte("local edit")))
				require.NoError(t, s.Close())
			case "destination inode":
				root := filepath.Join(dir, "database")
				require.NoError(t, os.Rename(root, root+".unrelated"))
				s := newTreeStorage(t, dir)
				require.NoError(t, s.WriteKey("meta/unrelated/key", []byte("keep")))
				require.NoError(t, s.Close())
			case "destination device":
				intent.Device++
				encoded, err := json.Marshal(intent)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(dir, "database.import-intent"), encoded, 0o600))
			}
			_, err := ImportBlob(path, dir)
			require.Error(t, err)
			require.FileExists(t, path)
			require.NoFileExists(t, intent.Archive)
			require.FileExists(t, filepath.Join(dir, "database.import-intent"))
			if change == "destination inode" {
				s, err := OpenReadOnly(dir)
				require.NoError(t, err)
				defer s.Close() //nolint:errcheck // read-only fixture cleanup.
				got, err := s.ReadKey("meta/unrelated/key")
				require.NoError(t, err)
				assert.Equal(t, "keep", string(got))
			}
		})
	}
}

func TestImportDoesNotReplaceUnrelatedStore(t *testing.T) {
	path, _ := importFixture(t)
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	require.NoError(t, s.WriteKey("meta/owner/key", []byte("keep")))
	_, err := ImportBlob(path, dir)
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, s.Close())
	_, err = ImportBlob(path, dir)
	require.ErrorIs(t, err, fs.ErrExist)
	require.FileExists(t, path)
	reopened, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	got, err := reopened.ReadKey("meta/owner/key")
	require.NoError(t, err)
	assert.Equal(t, "keep", string(got))
}

func TestTreeCorruptionAndRepairCanBeOpened(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	require.NoError(t, s.WriteKey("meta/good/key", []byte("survivor")))
	require.NoError(t, s.WriteKey("meta/bad/key", []byte("corrupt me")))
	path := filepath.Join(dir, "database", "meta", "bad", "key")
	frame, err := os.ReadFile(path)
	require.NoError(t, err)
	offset := bytes.Index(frame, []byte("corrupt me"))
	require.NotEqual(t, -1, offset)
	frame[offset] ^= 1
	require.NoError(t, os.WriteFile(path, frame, 0o600))
	assert.True(t, s.Exists("meta/bad/key"), "a corrupt key is present, not absent")
	got, err := s.ReadKey("meta/bad/key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "meta/bad/key")
	assert.Empty(t, got)
	require.NoError(t, s.Close())
	output := t.TempDir()
	report, err := zefs.RepairPath(filepath.Join(dir, "database"), filepath.Join(output, "database"))
	require.NoError(t, err)
	assert.Equal(t, []string{"meta/good/key"}, report.Recovered)
	repaired, err := Open(output)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repaired.Close()) })
	got, err = repaired.ReadKey("meta/good/key")
	require.NoError(t, err)
	assert.Equal(t, "survivor", string(got))
	_, err = repaired.ReadKey("meta/bad/key")
	require.ErrorIs(t, err, fs.ErrNotExist)
	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, frame, unchanged)
}

func TestTreeFailedWriteBarrierDoesNotNotify(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	require.NoError(t, s.WriteFile("router.conf", []byte("original"), 0))
	concrete, ok := s.(*store)
	require.True(t, ok)
	failure := errors.New("injected file sync failure")
	concrete.tree.sync = func(file *os.File) error {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			return failure
		}
		return file.Sync()
	}
	t.Cleanup(func() { concrete.tree.sync = nil })
	var observed []string
	s.SetWriteObserver(func(key string) { observed = append(observed, key) })
	require.ErrorIs(t, s.WriteFile("router.conf", []byte("not durable"), 0), failure)
	assert.Empty(t, observed)
	got, err := s.ReadFile("router.conf")
	require.NoError(t, err)
	assert.Equal(t, "original", string(got))
}
