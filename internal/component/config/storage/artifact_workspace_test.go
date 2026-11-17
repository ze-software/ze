// Design: docs/architecture/storage-backends.md -- independent artifact editing

package storage

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

// TestArtifactConfigNames exercises the shared pointer-and-mirror census,
// including duplicate declarations and an unrelated namespace.
func TestArtifactConfigNames(t *testing.T) {
	path := restoreArtifact(t, map[string]string{"router.conf": "active"}, map[string]string{"seed.conf": "mirror"})
	artifact, err := OpenBlob(path, true, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	require.NoError(t, artifact.WriteKey("unknown/router.conf", []byte("not a config")))
	names, err := ArtifactConfigNames(artifact)
	require.NoError(t, err)
	assert.Equal(t, []string{"router.conf", "seed.conf"}, names)
}

// TestArtifactConfigNamesReadFailure refuses an incomplete name census rather
// than offering a successful empty configuration picker.
func TestArtifactConfigNamesReadFailure(t *testing.T) {
	readErr := errors.New("artifact key listing failed")
	names, err := ArtifactConfigNames(artifactListingFailure{err: readErr})
	require.ErrorIs(t, err, readErr)
	assert.Nil(t, names)
}

type artifactListingFailure struct {
	Storage
	err error
}

func (source artifactListingFailure) ListKeys(string) ([]string, error) {
	return nil, source.err
}

// TestPublishArtifactConfigPreservesPendingRecords edits the committed source
// while a different candidate and saved drafts are already in the artifact.
// Every unrelated raw key must survive, and the normal restore reader must
// resolve the edited active version after the artifact is closed.
func TestPublishArtifactConfigPreservesPendingRecords(t *testing.T) {
	path := restoreArtifact(t, map[string]string{"router.conf": "committed", "other.conf": "other"}, nil)
	artifact, err := OpenBlob(path, true, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	pending, err := WriteCandidateVersion(artifact, "router.conf", []byte("pending candidate"), time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	pendingKey := zefs.KeyConfigCandidate.Key("router.conf")
	require.NoError(t, artifact.WriteKey(pendingKey, []byte(pending+"\n\n")))
	for key, data := range map[string][]byte{
		"file/draft/router.conf":    []byte("saved draft"),
		"file/changes/alice/router": []byte("saved account changes"),
		"meta/auth/admin":           {0, 1, 255, '\n'},
		"unknown/binary":            {0, 255, 0, 128},
		// cli.DraftPath and cli.ChangePath use plain names, which storage
		// resolves under file/active rather than the draft namespace.
		"file/active/router.conf.draft":        []byte("editor draft"),
		"file/active/router.conf.change.alice": []byte("editor account changes"),
		zefs.KeyLocalAdminPassword.Key():       []byte("private password record"),
	} {
		require.NoError(t, artifact.WriteKey(key, data))
	}
	before := snapshotKeys(t, artifact)
	stamp, err := PublishArtifactConfig(artifact, "router.conf", []byte("edited committed"))
	require.NoError(t, err)
	after := snapshotKeys(t, artifact)
	want := maps.Clone(before)
	want[zefs.KeyConfigActive.Key("router.conf")] = stamp + "\n"
	want[zefs.KeyConfigRollback.Key("router.conf")] = before[zefs.KeyConfigActive.Key("router.conf")]
	want[zefs.KeyFileActive.Key("router.conf")] = "edited committed"
	want[zefs.KeyFileVersion.Key(stamp, "router.conf")] = "sha256:" + contentDigest([]byte("edited committed"))
	want[objectKey(contentDigest([]byte("edited committed")))] = "edited committed"
	assert.Equal(t, want, after, "only the selected committed version, pointers and mirror may change")
	require.NoError(t, artifact.Close())
	selected, err := ReadRestoreSource(path, "router.conf", "")
	require.NoError(t, err)
	assert.Equal(t, "edited committed", string(selected.Data))
	require.NoError(t, CheckArtifact(path))
}

// TestPublishArtifactConfigWithoutCandidate retains the absent-candidate state
// while a mirror-only source gains normal version and rollback pointers.
func TestPublishArtifactConfigWithoutCandidate(t *testing.T) {
	path := restoreArtifact(t, nil, map[string]string{"router.conf": "mirror"})
	artifact, err := OpenBlob(path, true, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	_, err = PublishArtifactConfig(artifact, "router.conf", []byte("edited"))
	require.NoError(t, err)
	_, err = artifact.ReadKey(zefs.KeyConfigCandidate.Key("router.conf"))
	require.ErrorIs(t, err, fs.ErrNotExist)
	active, err := ReadActiveConfig(artifact, "router.conf")
	require.NoError(t, err)
	assert.Equal(t, "edited", string(active))
}

// TestArtifactLegacyAdoptionPreservesOccupiedHistory commits over a mirror-only
// source at an occupied timestamp and checks that adopting its rollback leaves
// every older version and the pending candidate's raw records unchanged.
func TestArtifactLegacyAdoptionPreservesOccupiedHistory(t *testing.T) {
	path := restoreArtifact(t, nil, map[string]string{"router.conf": "legacy mirror"})
	artifact, err := OpenBlob(path, true, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	fixed := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	pending, err := WriteCandidateVersion(artifact, "router.conf", []byte("pending intent"), fixed)
	require.NoError(t, err)
	require.NoError(t, artifact.WriteVersion("router.conf", []byte("older history"), fixed.Add(-time.Millisecond)))
	before := snapshotKeys(t, artifact)
	var committed string
	func() {
		guard, err := artifact.AcquireLock("router.conf")
		require.NoError(t, err)
		defer func() { require.NoError(t, guard.Release()) }()
		committed, err = writeNewVersionLocked(artifact, guard, "router.conf", []byte("edited committed"), fixed)
		require.NoError(t, err)
		require.Equal(t, FormatVersionStamp(fixed.Add(time.Millisecond)), committed)
		require.NoError(t, promoteVersionLocked(artifact, guard, "router.conf", committed))
	}()
	after := snapshotKeys(t, artifact)
	for key, value := range before {
		if key == zefs.KeyFileActive.Key("router.conf") {
			continue
		}
		assert.Equal(t, value, after[key], "preexisting record %s", key)
	}
	pendingData, err := artifact.ReadVersion("router.conf", pending)
	require.NoError(t, err)
	assert.Equal(t, "pending intent", string(pendingData))
	activeData, err := ReadActiveConfig(artifact, "router.conf")
	require.NoError(t, err)
	assert.Equal(t, "edited committed", string(activeData))
	rollback, present, err := readPointer(artifact, "router.conf", pointerRollback)
	require.NoError(t, err)
	require.True(t, present)
	assert.Less(t, rollback, committed, "adopted mirror precedes the commit")
	rollbackData, err := artifact.ReadVersion("router.conf", rollback)
	require.NoError(t, err)
	assert.Equal(t, "legacy mirror", string(rollbackData))
}

// TestPublishArtifactConfigRefusesLiveStore keeps the artifact-only operation
// from bypassing a live configuration owner's reload and authorization path.
func TestPublishArtifactConfigRefusesLiveStore(t *testing.T) {
	live := newTreeStorage(t, t.TempDir())
	_, err := RestoreConfig(live, "router.conf", []byte("live"))
	require.NoError(t, err)
	before := snapshotKeys(t, live)
	_, err = PublishArtifactConfig(live, "router.conf", []byte("wrong target"))
	require.ErrorIs(t, err, errors.ErrUnsupported)
	assert.Equal(t, before, snapshotKeys(t, live))
}

// TestPublishArtifactConfigRefusesMissingArtifact handles both absent-interface
// and absent concrete handles before any publication code is reachable.
func TestPublishArtifactConfigRefusesMissingArtifact(t *testing.T) {
	for _, missing := range []Storage{nil, (*store)(nil)} {
		_, err := PublishArtifactConfig(missing, "router.conf", []byte("edited"))
		require.ErrorIs(t, err, errors.ErrUnsupported)
	}
}

// TestPublishArtifactConfigRefusesReadOnlyArtifact preserves every key when the
// caller did not acquire a writable artifact handle.
func TestPublishArtifactConfigRefusesReadOnlyArtifact(t *testing.T) {
	path := restoreArtifact(t, map[string]string{"router.conf": "committed"}, nil)
	artifact, err := OpenBlob(path, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	before := snapshotKeys(t, artifact)
	_, err = PublishArtifactConfig(artifact, "router.conf", []byte("edited"))
	require.ErrorIs(t, err, ErrReadOnly)
	assert.Equal(t, before, snapshotKeys(t, artifact))
}

// TestPublishArtifactConfigRejectsInvalidPendingPointer diagnoses pending
// metadata before changing the committed configuration or any raw key.
func TestPublishArtifactConfigRejectsInvalidPendingPointer(t *testing.T) {
	for _, pending := range []string{"invalid stamp", "20261117-080000.000\n"} {
		t.Run(pending, func(t *testing.T) {
			path := restoreArtifact(t, map[string]string{"router.conf": "committed"}, nil)
			artifact, err := OpenBlob(path, true, zefs.Spare(0))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, artifact.Close()) })
			require.NoError(t, artifact.WriteKey(zefs.KeyConfigCandidate.Key("router.conf"), []byte(pending)))
			before := snapshotKeys(t, artifact)
			_, err = PublishArtifactConfig(artifact, "router.conf", []byte("edited"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "pending candidate")
			assert.Equal(t, before, snapshotKeys(t, artifact))
		})
	}
}

// TestPublishArtifactConfigRefusesClosedArtifact returns the original ownership
// failure rather than attempting to publish through a released handle.
func TestPublishArtifactConfigRefusesClosedArtifact(t *testing.T) {
	path := restoreArtifact(t, map[string]string{"router.conf": "committed"}, nil)
	artifact, err := OpenBlob(path, true, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	_, err = PublishArtifactConfig(artifact, "router.conf", []byte("edited"))
	require.ErrorIs(t, err, fs.ErrClosed)
	selected, err := ReadRestoreSource(path, "router.conf", "")
	require.NoError(t, err)
	assert.Equal(t, "committed", string(selected.Data))
}

// TestCheckArtifactRejectsCorruption runs the same complete artifact verdict
// used by restore, for a valid artifact and malformed magic or payload.
func TestCheckArtifactRejectsCorruption(t *testing.T) {
	valid := restoreArtifact(t, map[string]string{"router.conf": "committed"}, nil)
	require.NoError(t, CheckArtifact(valid))
	data, err := os.ReadFile(valid)
	require.NoError(t, err)
	for _, offset := range []int{0, len(data) - 1} {
		broken := append([]byte(nil), data...)
		broken[offset] ^= 255
		path := filepath.Join(t.TempDir(), "broken.zefs")
		require.NoError(t, os.WriteFile(path, broken, 0o600))
		require.ErrorIs(t, CheckArtifact(path), ErrCorrupt)
	}
}

// TestArtifactPublicationFailurePreservesPendingRecords injects a failed
// active-pointer write on either side of acceptance through a real artifact
// guard. Withdrawal must remove only a rejected version and retain accepted
// content, without changing the pending candidate or editor records.
func TestArtifactPublicationFailurePreservesPendingRecords(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		name := "before active publication"
		if accepted {
			name = "after active publication"
		}
		t.Run(name, func(t *testing.T) {
			artifact, _, before := artifactPendingFixture(t)
			failure := errors.New("active publication failed")
			var stamp string
			func() {
				guard, err := artifact.AcquireLock("router.conf")
				require.NoError(t, err)
				defer func() { require.NoError(t, guard.Release()) }()
				fault := &artifactPublicationFailureGuard{WriteGuard: guard, failure: failure, accepted: accepted}
				stamp, err = writeNewVersionLocked(artifact, fault, "router.conf", []byte("edited"), time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC))
				require.NoError(t, err)
				err = promoteVersionLocked(artifact, fault, "router.conf", stamp)
				require.ErrorIs(t, err, failure)
				require.True(t, fault.reached, "the failure must occur at the active pointer")
				err = withdrawArtifactVersion(artifact, fault, "router.conf", stamp, err)
				require.ErrorIs(t, err, failure)
			}()
			after := snapshotKeys(t, artifact)
			for key, value := range before {
				if key == zefs.KeyConfigActive.Key("router.conf") {
					continue
				}
				if key == zefs.KeyConfigRollback.Key("router.conf") {
					continue
				}
				assert.Equal(t, value, after[key], "preexisting record %s", key)
			}
			data, err := artifact.ReadVersion("router.conf", stamp)
			if accepted {
				require.NoError(t, err)
				assert.Equal(t, "edited", string(data))
				assert.Equal(t, stamp+"\n", after[zefs.KeyConfigActive.Key("router.conf")])
				return
			}
			require.ErrorIs(t, err, fs.ErrNotExist)
			assert.NotContains(t, after, zefs.KeyFileVersion.Key(stamp, "router.conf"))
			assert.NotContains(t, after, objectKey(contentDigest([]byte("edited"))))
			assert.Equal(t, before[zefs.KeyConfigActive.Key("router.conf")], after[zefs.KeyConfigActive.Key("router.conf")])
		})
	}
}

type artifactPublicationFailureGuard struct {
	WriteGuard
	failure  error
	accepted bool
	reached  bool
}

func (g *artifactPublicationFailureGuard) WriteFile(name string, data []byte, mode fs.FileMode) error {
	if name != zefs.KeyConfigActive.Key("router.conf") {
		return g.WriteGuard.WriteFile(name, data, mode)
	}
	g.reached = true
	if g.accepted {
		if err := g.WriteGuard.WriteFile(name, data, mode); err != nil {
			return err
		}
	}
	return g.failure
}

// TestPublishArtifactConfigReportsReleaseFailure obstructs the task-owned
// artifact pathname before save. Only guard release flushes blob writes, so
// publication must report that flush failure while retaining pending bytes.
func TestPublishArtifactConfigReportsReleaseFailure(t *testing.T) {
	artifact, path, before := artifactPendingFixture(t)
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.Rename(path, path+".original"))
	require.NoError(t, os.Mkdir(path, 0o700))
	stamp, err := PublishArtifactConfig(artifact, "router.conf", []byte("edited"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release storage lock")
	assert.NotEmpty(t, stamp, "publication reaches the release boundary")
	after := snapshotKeys(t, artifact)
	for key, value := range before {
		if key == zefs.KeyConfigActive.Key("router.conf") {
			continue
		}
		if key == zefs.KeyConfigRollback.Key("router.conf") {
			continue
		}
		if key == zefs.KeyFileActive.Key("router.conf") {
			continue
		}
		assert.Equal(t, value, after[key], "preexisting record %s", key)
	}
	preserved, err := os.ReadFile(path + ".original")
	require.NoError(t, err)
	assert.Equal(t, original, preserved)
}

// artifactPendingFixture retains real editor intent and private keys alongside
// a committed version and a different pending candidate for failure tests.
func artifactPendingFixture(t *testing.T) (Storage, string, map[string]string) {
	t.Helper()
	path := restoreArtifact(t, map[string]string{"router.conf": "committed"}, nil)
	artifact, err := OpenBlob(path, true, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	_, err = WriteCandidateVersion(artifact, "router.conf", []byte("pending"), time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	for key, value := range map[string][]byte{
		"file/active/router.conf.draft":        []byte("draft"),
		"file/active/router.conf.change.alice": []byte("alice changes"),
		zefs.KeyLocalAdminPassword.Key():       []byte("private password"),
		"unknown/binary":                       {0, 255, 128, 0},
	} {
		require.NoError(t, artifact.WriteKey(key, value))
	}
	return artifact, path, snapshotKeys(t, artifact)
}

// TestArtifactVersionAllocationRefusesIncompleteCensus fails the guarded key
// census for both commit allocation and mirror adoption. Neither failure may
// guess an unused timestamp or change any artifact key.
func TestArtifactVersionAllocationRefusesIncompleteCensus(t *testing.T) {
	for _, adoption := range []bool{false, true} {
		name := "new commit"
		if adoption {
			name = "mirror adoption"
		}
		t.Run(name, func(t *testing.T) {
			path := restoreArtifact(t, nil, map[string]string{"router.conf": "legacy"})
			artifact, err := OpenBlob(path, true, zefs.Spare(0))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, artifact.Close()) })
			fixed := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
			candidate, err := WriteCandidateVersion(artifact, "router.conf", []byte("candidate"), fixed)
			require.NoError(t, err)
			before := snapshotKeys(t, artifact)
			failure := errors.New("history listing failed")
			func() {
				guard, err := artifact.AcquireLock("router.conf")
				require.NoError(t, err)
				defer func() { require.NoError(t, guard.Release()) }()
				fault := artifactCensusFailureGuard{WriteGuard: guard, failure: failure}
				if adoption {
					require.ErrorIs(t, promoteVersionLocked(artifact, fault, "router.conf", candidate), failure)
					return
				}
				_, err = writeNewVersionLocked(artifact, fault, "router.conf", []byte("new"), fixed)
				require.ErrorIs(t, err, failure)
			}()
			assert.Equal(t, before, snapshotKeys(t, artifact))
		})
	}
}

type artifactCensusFailureGuard struct {
	WriteGuard
	failure error
}

func (g artifactCensusFailureGuard) ListKeys(string) ([]string, error) {
	return nil, g.failure
}
