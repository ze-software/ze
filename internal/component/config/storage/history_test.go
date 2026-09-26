// Design: docs/architecture/storage-backends.md -- content-addressed config history
package storage

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	historyStampA = time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	historyStampB = time.Date(2026, 9, 26, 11, 0, 0, 0, time.Local)
	historyStampC = time.Date(2026, 9, 26, 12, 0, 0, 0, time.Local)
)

// historyKeys returns the entry key of router.conf at stamp and the object key
// of data.
func historyKeys(stamp time.Time, data []byte) (entry, object string) {
	return "file/" + FormatVersionStamp(stamp) + "/router.conf", objectKey(contentDigest(data))
}

func requireAbsent(t *testing.T, s Storage, key string) {
	t.Helper()
	_, err := s.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist, key)
}

func requirePresent(t *testing.T, s Storage, key string) {
	t.Helper()
	_, err := s.ReadKey(key)
	require.NoError(t, err, key)
}

// TestWriteVersionStoresObject verifies AC-1: the entry holds sha256:<hex>,
// the object holds the bytes, on both encodings.
func TestWriteVersionStoresObject(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			data := []byte("router-id 192.0.2.1;\n")
			require.NoError(t, s.WriteVersion(backend.configPath(dir), data, historyStampA))

			entry, object := historyKeys(historyStampA, data)
			value, err := s.ReadKey(entry)
			require.NoError(t, err)
			assert.Equal(t, "sha256:"+contentDigest(data), string(value))
			assert.Len(t, value, 71)
			stored, err := s.ReadKey(object)
			require.NoError(t, err)
			assert.Equal(t, data, stored)
		})
	}
}

// TestWriteVersionDedups verifies AC-2: equal bytes under two stamps and two
// names are one object and three entries.
func TestWriteVersionDedups(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			other := filepath.Join(filepath.Dir(name), "other.conf")
			data := []byte("same\n")
			require.NoError(t, s.WriteVersion(name, data, historyStampA))
			require.NoError(t, s.WriteVersion(name, data, historyStampB))
			require.NoError(t, s.WriteVersion(other, data, historyStampA))

			objects, err := s.ListKeys("object/")
			require.NoError(t, err)
			assert.Len(t, objects, 1)
			entries, err := s.ListKeys("file/")
			require.NoError(t, err)
			assert.Len(t, entries, 3)
		})
	}
}

// recordingAccess records the order of the raw writes a guard performs.
type recordingAccess struct {
	keyAccess
	writes *[]string
}

func (r recordingAccess) WriteFile(key string, data []byte, mode fs.FileMode) error {
	*r.writes = append(*r.writes, key)
	return r.keyAccess.WriteFile(key, data, mode)
}

// TestWriteVersionObjectFirst verifies AC-1's ordering: the object frame is
// written before the entry, so a crash between them leaves an orphan object
// and never a dangling entry.
func TestWriteVersionObjectFirst(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir).(*store)
	g, err := s.acquire()
	require.NoError(t, err)
	var writes []string
	g.access = recordingAccess{keyAccess: g.access, writes: &writes}
	data := []byte("ordered\n")
	require.NoError(t, g.WriteVersion(filepath.Join(dir, "router.conf"), data, historyStampA))
	require.NoError(t, g.Release())

	entry, object := historyKeys(historyStampA, data)
	assert.Equal(t, []string{object, entry}, writes)
}

// TestWriteVersionRefusesWrongObject verifies AC-16: an existing object whose
// bytes hash to something else is refused before the entry is written.
func TestWriteVersionRefusesWrongObject(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			data := []byte("promised\n")
			entry, object := historyKeys(historyStampA, data)
			require.NoError(t, s.WriteKey(object, []byte("impostor\n")))

			err := s.WriteVersion(backend.configPath(dir), data, historyStampA)
			require.ErrorIs(t, err, ErrHistoryObject)
			assert.Contains(t, err.Error(), object)
			assert.Contains(t, err.Error(), contentDigest(data))
			assert.Contains(t, err.Error(), contentDigest([]byte("impostor\n")))
			requireAbsent(t, s, entry)
		})
	}
}

// TestReadVersionVerifiesHash verifies AC-3: a CRC-valid object holding the
// wrong bytes is an error through ReadVersion and ReadActiveConfig, never the
// wrong config.
func TestReadVersionVerifiesHash(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("good\n")
			stamp, _, err := EnsureActiveVersion(s, name, data, historyStampA)
			require.NoError(t, err)
			got, err := s.ReadVersion(name, stamp)
			require.NoError(t, err)
			assert.Equal(t, data, got)

			entry, object := historyKeys(historyStampA, data)
			require.NoError(t, s.WriteKey(object, []byte("rotten\n")))
			_, err = s.ReadVersion(name, stamp)
			require.ErrorIs(t, err, ErrHistoryObject)
			assert.NotErrorIs(t, err, fs.ErrNotExist)
			assert.Contains(t, err.Error(), entry)
			_, err = ReadActiveConfig(s, name)
			require.ErrorIs(t, err, ErrHistoryObject)
		})
	}
}

// TestReadVersionMissingObject verifies AC-3: an entry whose object is absent
// is an error naming the entry and the hash, and it keeps fs.ErrNotExist.
func TestReadVersionMissingObject(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("gone\n")
			require.NoError(t, s.WriteVersion(name, data, historyStampA))
			entry, object := historyKeys(historyStampA, data)
			require.NoError(t, s.RemoveKey(object))

			got, err := s.ReadVersion(name, FormatVersionStamp(historyStampA))
			require.ErrorIs(t, err, fs.ErrNotExist)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), entry)
			assert.Contains(t, err.Error(), "sha256:"+contentDigest(data))
		})
	}
}

// TestReadVersionRejectsBadEntry verifies AC-3 and the entry-value boundary:
// exactly `sha256:` plus 64 lowercase hex (71 characters) is a reference;
// 70, 72, uppercase, and a bare copy of config bytes are ErrHistoryObject.
func TestReadVersionRejectsBadEntry(t *testing.T) {
	digest := contentDigest([]byte("x"))
	cases := []struct {
		name  string
		value string
		want  error
	}{
		{"valid-71-no-object", "sha256:" + digest, fs.ErrNotExist},
		{"short-70", "sha256:" + digest[:63], ErrHistoryObject},
		{"long-72", "sha256:" + digest + "0", ErrHistoryObject},
		{"uppercase", "sha256:" + strings.ToUpper(digest), ErrHistoryObject},
		{"copy", "router-id 192.0.2.1;\n", ErrHistoryObject},
		{"empty", "", ErrHistoryObject},
	}
	for _, backend := range pointerTestStores() {
		for _, tc := range cases {
			t.Run(backend.name+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				s := backend.newStore(t, dir)
				entry, _ := historyKeys(historyStampA, nil)
				require.NoError(t, s.WriteKey(entry, []byte(tc.value)))
				_, err := s.ReadVersion(backend.configPath(dir), FormatVersionStamp(historyStampA))
				require.ErrorIs(t, err, tc.want)
				assert.Contains(t, err.Error(), entry)
			})
		}
	}
}

// TestReadFileEntryRaw verifies AC-4 and AC-7: ReadFile answers the stored
// entry value, and ListVersions still publishes the entry key as Path.
func TestReadFileEntryRaw(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("raw\n")
			require.NoError(t, s.WriteVersion(name, data, historyStampA))
			entry, _ := historyKeys(historyStampA, data)
			value, err := s.ReadFile(entry)
			require.NoError(t, err)
			assert.Equal(t, "sha256:"+contentDigest(data), string(value))

			versions, err := s.ListVersions(name)
			require.NoError(t, err)
			require.Len(t, versions, 1)
			assert.Equal(t, VersionInfo{Stamp: FormatVersionStamp(historyStampA), Date: historyStampA, Path: entry}, versions[0])
			got, err := ReadVersionEntry(s, name, versions[0].Path)
			require.NoError(t, err)
			assert.Equal(t, data, got)
			_, err = ReadVersionEntry(s, name, "file/active/router.conf")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})
	}
}

// TestSharedObjectSurvivesOneRemove verifies AC-5 and R-3: the object stays
// while one entry names it and goes with the last.
func TestSharedObjectSurvivesOneRemove(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("shared\n")
			require.NoError(t, s.WriteVersion(name, data, historyStampA))
			require.NoError(t, s.WriteVersion(name, data, historyStampB))
			entryA, object := historyKeys(historyStampA, data)
			entryB, _ := historyKeys(historyStampB, data)

			require.NoError(t, removeVersion(s, name, FormatVersionStamp(historyStampA)))
			requireAbsent(t, s, entryA)
			requirePresent(t, s, object)
			require.NoError(t, removeVersion(s, name, FormatVersionStamp(historyStampB)))
			requireAbsent(t, s, entryB)
			requireAbsent(t, s, object)
		})
	}
}

// TestRemoveChecksEveryName verifies AC-5 and A-3: an entry of another config
// name keeps the object, and a mutable file/active value holding the same
// digest text does not.
func TestRemoveChecksEveryName(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			other := filepath.Join(filepath.Dir(name), "other.conf")
			data := []byte("common\n")
			require.NoError(t, s.WriteVersion(name, data, historyStampA))
			require.NoError(t, s.WriteVersion(other, data, historyStampA))
			_, object := historyKeys(historyStampA, data)
			require.NoError(t, s.WriteKey("file/active/decoy.conf", []byte("sha256:"+contentDigest(data))))

			require.NoError(t, removeVersion(s, name, FormatVersionStamp(historyStampA)))
			requirePresent(t, s, object)
			require.NoError(t, removeVersion(s, other, FormatVersionStamp(historyStampA)))
			requireAbsent(t, s, object)
		})
	}
}

// TestPointedVersionRetainedWithObject verifies AC-5: a version a pointer
// names is retained, entry and object both, because the removal reported
// that it did not delete.
func TestPointedVersionRetainedWithObject(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("pinned\n")
			stamp, _, err := EnsureActiveVersion(s, name, data, historyStampA)
			require.NoError(t, err)

			g, err := s.AcquireLock(name)
			require.NoError(t, err)
			deleted, err := removeVersionLocked(s, g, name, stamp)
			require.NoError(t, g.Release())
			require.NoError(t, err)
			assert.False(t, deleted)
			entry, object := historyKeys(historyStampA, data)
			requirePresent(t, s, entry)
			requirePresent(t, s, object)
		})
	}
}

// TestClearCandidateDeletesObject verifies AC-6: the candidate's entry and its
// unreferenced object both go.
func TestClearCandidateDeletesObject(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("staged\n")
			_, err := WriteCandidateVersion(s, name, data, historyStampA)
			require.NoError(t, err)
			entry, object := historyKeys(historyStampA, data)
			requirePresent(t, s, object)

			require.NoError(t, ClearCandidate(s, name))
			requireAbsent(t, s, entry)
			requireAbsent(t, s, object)
		})
	}
}

// TestClearCandidateMissingEntry verifies AC-6 and R-10: a candidate pointer
// whose entry repair dropped clears cleanly, runs no sweep, and leaves the
// rollback version readable.
func TestClearCandidateMissingEntry(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			rollback := []byte("rollback\n")
			rollbackStamp, _, err := EnsureActiveVersion(s, name, rollback, historyStampA)
			require.NoError(t, err)
			_, err = WriteCandidateVersion(s, name, []byte("candidate\n"), historyStampB)
			require.NoError(t, err)
			candidateEntry, _ := historyKeys(historyStampB, nil)
			require.NoError(t, s.RemoveKey(candidateEntry))

			require.NoError(t, ClearCandidate(s, name))
			_, _, present, err := ReadCandidateConfig(s, name)
			require.NoError(t, err)
			assert.False(t, present)
			got, err := s.ReadVersion(name, rollbackStamp)
			require.NoError(t, err)
			assert.Equal(t, rollback, got)
		})
	}
}

// TestActiveMissingTargetClassification verifies AC-15: an active pointer
// whose object is absent keeps fs.ErrNotExist and names the pointer and the
// stamp; a wrong-hash object is not absence. Neither falls back to the mirror.
func TestActiveMissingTargetClassification(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			data := []byte("active\n")
			stamp, _, err := EnsureActiveVersion(s, name, data, historyStampA)
			require.NoError(t, err)
			require.NoError(t, s.WriteFile(name, []byte("mirror\n"), 0o600))
			_, object := historyKeys(historyStampA, data)

			require.NoError(t, s.WriteKey(object, []byte("rotten\n")))
			got, err := ReadActiveConfig(s, name)
			require.ErrorIs(t, err, ErrHistoryObject)
			assert.NotErrorIs(t, err, fs.ErrNotExist)
			assert.Nil(t, got)

			require.NoError(t, s.RemoveKey(object))
			got, err = ReadActiveConfig(s, name)
			require.ErrorIs(t, err, fs.ErrNotExist)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), "meta/config/router.conf/active")
			assert.Contains(t, err.Error(), stamp)
		})
	}
}

// TestRebuildPreservesValidRollback verifies AC-17 and R-11: promoting over an
// active pointer whose version does not resolve leaves the rollback pointer on
// the version that does, and never records the unresolved stamp.
func TestRebuildPreservesValidRollback(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			dir := t.TempDir()
			s := backend.newStore(t, dir)
			name := backend.configPath(dir)
			rollback := []byte("rollback R\n")
			active := []byte("active S\n")
			rebuilt := []byte("file F\n")
			rollbackStamp, _, err := EnsureActiveVersion(s, name, rollback, historyStampA)
			require.NoError(t, err)
			_, err = WriteCandidateVersion(s, name, active, historyStampB)
			require.NoError(t, err)
			require.NoError(t, PromoteCandidate(s, name))
			_, activeObject := historyKeys(historyStampB, active)
			require.NoError(t, s.RemoveKey(activeObject))

			stamp, err := WriteCandidateVersion(s, name, rebuilt, historyStampC)
			require.NoError(t, err)
			require.NoError(t, PromoteCandidate(s, name))

			got, err := ReadActiveConfig(s, name)
			require.NoError(t, err)
			assert.Equal(t, rebuilt, got)
			pointed, ok, err := readPointer(s, name, pointerRollback)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, rollbackStamp, pointed)
			assert.NotEqual(t, stamp, pointed)
			got, err = s.ReadVersion(name, rollbackStamp)
			require.NoError(t, err)
			assert.Equal(t, rollback, got)
		})
	}
}

// TestPromoteAbortsOnCorruptActive verifies the other arm of AC-17: a
// corrupt active version is not absence, so promotion aborts and moves
// nothing.
func TestPromoteAbortsOnCorruptActive(t *testing.T) {
	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	name := filepath.Join(dir, "router.conf")
	active := []byte("active S\n")
	activeStamp, _, err := EnsureActiveVersion(s, name, active, historyStampA)
	require.NoError(t, err)
	_, object := historyKeys(historyStampA, active)
	require.NoError(t, s.WriteKey(object, []byte("rotten\n")))
	_, err = WriteCandidateVersion(s, name, []byte("next\n"), historyStampB)
	require.NoError(t, err)

	err = PromoteCandidate(s, name)
	require.ErrorIs(t, err, ErrHistoryObject)
	pointed, ok, err := readPointer(s, name, pointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, activeStamp, pointed)
}

// TestImportObjectsFirst verifies AC-12: the walks order every object/* key
// before every other key, and a backup artifact reads its versions back
// through their objects.
func TestImportObjectsFirst(t *testing.T) {
	keys := []string{"file/20260926-100000.000/r.conf", "meta/x", "object/bb", "file/active/r.conf", "object/aa"}
	assert.Equal(t, []string{"object/bb", "object/aa", "file/20260926-100000.000/r.conf", "meta/x", "file/active/r.conf"}, objectsFirst(keys))

	dir := t.TempDir()
	s := newTreeStorage(t, dir)
	name := filepath.Join(dir, "router.conf")
	data := []byte("backed up\n")
	stamp, _, err := EnsureActiveVersion(s, name, data, historyStampA)
	require.NoError(t, err)
	artifact := filepath.Join(t.TempDir(), "backup.zefs")
	_, err = Backup(s, artifact, false)
	require.NoError(t, err)
	source, err := ReadRestoreSource(artifact, "", "router.conf")
	require.NoError(t, err)
	assert.Equal(t, data, source.Data)
	opened, err := OpenBlob(artifact, false)
	require.NoError(t, err)
	defer opened.Close() //nolint:errcheck // read-only artifact.
	got, err := opened.ReadVersion("router.conf", stamp)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

// TestRestoreConfigRefusesMissingObject verifies AC-13: a source whose active
// entry names an object the artifact lacks is refused before any write,
// naming the hash.
func TestRestoreConfigRefusesMissingObject(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "source.zefs")
	source, err := CreateBlob(artifact)
	require.NoError(t, err)
	data := []byte("lost\n")
	_, _, err = EnsureActiveVersion(source, "router.conf", data, historyStampA)
	require.NoError(t, err)
	_, object := historyKeys(historyStampA, data)
	require.NoError(t, source.RemoveKey(object))
	require.NoError(t, source.Close())

	_, err = ReadRestoreSource(artifact, "", "router.conf")
	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Contains(t, err.Error(), contentDigest(data))
	assert.False(t, errors.Is(err, ErrHistoryObject))
}

// BenchmarkWriteVersion measures A-4: hashing a few-KB config per commit.
func BenchmarkWriteVersion(b *testing.B) {
	s, err := Create(b.TempDir())
	require.NoError(b, err)
	defer s.Close() //nolint:errcheck // benchmark store.
	data := []byte(strings.Repeat("peer 192.0.2.1 { remote-as 65001; }\n", 100))
	stamp := historyStampA
	b.ResetTimer()
	for range b.N {
		stamp = stamp.Add(time.Millisecond)
		if err := s.WriteVersion("router.conf", data, stamp); err != nil {
			b.Fatal(err)
		}
	}
}
