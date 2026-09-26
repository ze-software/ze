package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

// restoreArtifact writes a blob artifact holding each config of configs as an
// active version, and each of mirrors as a file/active mirror with no pointer.
func restoreArtifact(t *testing.T, configs, mirrors map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.zefs")
	artifact, err := CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	for name, text := range configs {
		_, err := RestoreConfig(artifact, name, []byte(text))
		require.NoError(t, err)
	}
	for name, text := range mirrors {
		require.NoError(t, artifact.WriteKey(zefs.KeyFileActive.Key(name), []byte(text)))
	}
	require.NoError(t, artifact.Close())
	return path
}

// snapshotKeys returns every key and value of s.
func snapshotKeys(t *testing.T, s Storage) map[string]string {
	t.Helper()
	keys, err := s.ListKeys("")
	require.NoError(t, err)
	snapshot := make(map[string]string, len(keys))
	for _, key := range keys {
		data, err := s.ReadKey(key)
		require.NoError(t, err)
		snapshot[key] = string(data)
	}
	return snapshot
}

// TestRestoreConfigTouchesOnlyConfig verifies AC-7: the source config becomes
// a new version under the device's name, active moves to it, the previous
// active becomes rollback, the mirror holds the new bytes, and every other
// key is byte-unchanged.
// VALIDATES: AC-7, A-1.
// PREVENTS: a config restore that rewrites credentials, identity or history.
func TestRestoreConfigTouchesOnlyConfig(t *testing.T) {
	source := restoreArtifact(t, map[string]string{"ze.conf": "restored"}, nil)
	target := newTreeStorage(t, t.TempDir())
	previous, err := RestoreConfig(target, "ze.conf", []byte("current"))
	require.NoError(t, err)
	require.NoError(t, target.WriteKey("meta/auth/admin", []byte("secret")))
	require.NoError(t, target.WriteKey("meta/instance/name", []byte("ze")))
	before := snapshotKeys(t, target)

	selected, err := ReadRestoreSource(source, "", "ze.conf")
	require.NoError(t, err)
	assert.Equal(t, "ze.conf", selected.Name)
	time.Sleep(2 * time.Millisecond) // a distinct version stamp from the seed commit
	stamp, err := RestoreConfig(target, "ze.conf", selected.Data)
	require.NoError(t, err)

	after := snapshotKeys(t, target)
	versionKey := zefs.KeyFileVersion.Key(stamp, "ze.conf")
	changed := map[string]string{
		versionKey: "sha256:" + contentDigest([]byte("restored")),
		objectKey(contentDigest([]byte("restored"))): "restored",
		zefs.KeyConfigActive.Key("ze.conf"):          stamp + "\n",
		zefs.KeyConfigRollback.Key("ze.conf"):        previous + "\n",
		zefs.KeyFileActive.Key("ze.conf"):            "restored",
	}
	for key, want := range changed {
		assert.Equal(t, want, after[key], key)
	}
	_, candidate := after[zefs.KeyConfigCandidate.Key("ze.conf")]
	assert.False(t, candidate, "no candidate left behind")
	for key, value := range before {
		if _, isChanged := changed[key]; isChanged {
			continue
		}
		assert.Equal(t, value, after[key], key)
	}
	added := 0
	for key := range changed {
		if _, existed := before[key]; !existed {
			added++
		}
	}
	assert.Len(t, after, len(before)+added, "only the version entry, its object and the pointer keys are new")
}

// TestRestoreConfigFromMirror verifies R-3: an artifact with no active
// pointer restores from its file/active mirror.
// VALIDATES: R-3.
// PREVENTS: a seed or hand-built blob being unrestorable.
func TestRestoreConfigFromMirror(t *testing.T) {
	source := restoreArtifact(t, nil, map[string]string{"seed.conf": "seeded"})
	selected, err := ReadRestoreSource(source, "", "ze.conf")
	require.NoError(t, err)
	assert.Equal(t, "seed.conf", selected.Name)
	assert.Equal(t, "seeded", string(selected.Data))
}

// TestRestoreConfigName verifies R-4 selection: an explicit name wins, the
// device's name is chosen among several, and a missing name is refused
// listing what the artifact holds.
// VALIDATES: AC-8 (name refusal), AC-20, R-4.
// PREVENTS: a restore guessing which config the operator meant.
func TestRestoreConfigName(t *testing.T) {
	source := restoreArtifact(t, map[string]string{"a.conf": "A", "b.conf": "B"}, nil)
	selected, err := ReadRestoreSource(source, "b.conf", "a.conf")
	require.NoError(t, err)
	assert.Equal(t, "B", string(selected.Data))
	selected, err = ReadRestoreSource(source, "", "a.conf")
	require.NoError(t, err)
	assert.Equal(t, "A", string(selected.Data))
	_, err = ReadRestoreSource(source, "c.conf", "a.conf")
	require.ErrorIs(t, err, ErrRestoreSource)
	assert.Contains(t, err.Error(), "a.conf, b.conf")
}

// TestRestoreConfigAmbiguousSource verifies AC-20: two source names, none
// the device's, no name keyword: refused before any write, listing both
// source names and the device's name.
// VALIDATES: AC-20.
// PREVENTS: a silent pick among several configs.
func TestRestoreConfigAmbiguousSource(t *testing.T) {
	source := restoreArtifact(t, map[string]string{"a.conf": "A", "b.conf": "B"}, nil)
	_, err := ReadRestoreSource(source, "", "ze.conf")
	require.ErrorIs(t, err, ErrRestoreSource)
	for _, want := range []string{"a.conf", "b.conf", "ze.conf", "name <source-name>"} {
		assert.Contains(t, err.Error(), want)
	}
	empty := restoreArtifact(t, nil, nil)
	_, err = ReadRestoreSource(empty, "", "ze.conf")
	require.ErrorIs(t, err, ErrRestoreSource)
	assert.Contains(t, err.Error(), "no config")
}

// TestRestoreRefusesCorrupt verifies AC-6's check: an artifact failing
// zefs.Check is refused before anything reads it.
// VALIDATES: AC-6.
// PREVENTS: restoring from a truncated or foreign file.
func TestRestoreRefusesCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junk.zefs")
	require.NoError(t, os.WriteFile(path, []byte("not a blob"), 0o600))
	_, err := ReadRestoreSource(path, "", "ze.conf")
	require.ErrorIs(t, err, ErrCorrupt)
}

// TestRestoreConfigRefusesCandidate verifies a restore never promotes over a
// staged candidate: it refuses with ErrCandidateExists and writes nothing.
// VALIDATES: AC-7's "nothing else changes".
// PREVENTS: a restore discarding a commit somebody staged.
func TestRestoreConfigRefusesCandidate(t *testing.T) {
	target := newTreeStorage(t, t.TempDir())
	_, err := WriteCandidateVersion(target, "ze.conf", []byte("staged"), time.Now())
	require.NoError(t, err)
	before := snapshotKeys(t, target)
	_, err = RestoreConfig(target, "ze.conf", []byte("restored"))
	require.ErrorIs(t, err, ErrCandidateExists)
	assert.Equal(t, before, snapshotKeys(t, target))
}
