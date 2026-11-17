// Design: docs/architecture/storage-backends.md -- resolving retained private recovery.
package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func retainedManualRecovery(t *testing.T) (string, string, string) {
	t.Helper()
	source, dir, original := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	_, err = RecoveryFor(source).StagePath(context.Background(), original)
	require.NoError(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	competing, err := Open(dir)
	require.NoError(t, err)
	require.ErrorIs(t, closed.Apply(), ErrBusy)
	stage, err := PendingRecoveryStage(competing)
	require.NoError(t, err)
	require.NotEmpty(t, stage)
	require.NoError(t, competing.Close())
	return dir, filepath.Join(stage, recoveryArtifactName), original
}

// TestManualRecoveryConsumesOnlyAfterClose proves the explicit resume resolves
// its proven private stage after the imported owner closes, while the
// caller artifact survives and a new daemon can accept another recovery.
func TestManualRecoveryConsumesOnlyAfterClose(t *testing.T) {
	dir, snapshot, original := retainedManualRecovery(t)
	originalBytes, err := os.ReadFile(original)
	require.NoError(t, err)
	receipt, err := ResumeRecovery(snapshot, dir)
	require.NoError(t, err)
	require.Positive(t, receipt.Bytes)
	require.Len(t, receipt.Digest, 64)
	_, err = os.Stat(filepath.Dir(snapshot))
	require.ErrorIs(t, err, os.ErrNotExist)
	after, err := os.ReadFile(original)
	require.NoError(t, err)
	require.Equal(t, originalBytes, after)
	live, err := Open(dir)
	require.NoError(t, err)
	raw, err := live.ReadKey("meta/test/restored")
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 1, 128}, raw)
	owner, err := NewDaemonRecovery(BindConfigSource(live, "router.conf", ConfigSourceStored))
	require.NoError(t, err)
	_, err = RecoveryFor(owner.Borrow()).StagePath(context.Background(), original)
	require.NoError(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	require.NoError(t, closed.Discard())
}

// TestManualRecoveryRefusesForeignTarget proves a private source's label cannot
// authorize replacement of a different directory, even with a valid artifact.
func TestManualRecoveryRefusesForeignTarget(t *testing.T) {
	_, snapshot, _ := retainedManualRecovery(t)
	foreignDir := t.TempDir()
	foreign, err := Create(foreignDir)
	require.NoError(t, err)
	require.NoError(t, foreign.WriteKey("meta/foreign", []byte("untouched")))
	require.NoError(t, foreign.Close())
	_, err = ResumeRecovery(snapshot, foreignDir)
	require.Error(t, err)
	require.FileExists(t, snapshot)
	foreign, err = Open(foreignDir)
	require.NoError(t, err)
	raw, err := foreign.ReadKey("meta/foreign")
	require.NoError(t, err)
	require.Equal(t, "untouched", string(raw))
	require.NoError(t, foreign.Close())
}

// TestManualRecoveryRefusesChangedOwnership proves changed bytes, replacement
// inodes, unknown files and missing ownership records are never imported or
// removed merely because the directory has a recovery-looking name.
func TestManualRecoveryRefusesChangedOwnership(t *testing.T) {
	for _, change := range []string{"bytes", "inode", "extra-file", "missing-record", "unknown-record"} {
		t.Run(change, func(t *testing.T) {
			dir, snapshot, _ := retainedManualRecovery(t)
			stage := filepath.Dir(snapshot)
			switch change {
			case "bytes":
				file, err := OpenBlob(snapshot, true)
				require.NoError(t, err)
				require.NoError(t, file.WriteKey("meta/changed", []byte("valid changed artifact")))
				require.NoError(t, file.Close())
			case "inode":
				data, err := os.ReadFile(snapshot)
				require.NoError(t, err)
				replacement := filepath.Join(stage, "replacement")
				require.NoError(t, os.WriteFile(replacement, data, 0o600))
				require.NoError(t, os.Rename(replacement, snapshot))
			case "extra-file":
				require.NoError(t, os.WriteFile(filepath.Join(stage, "user-notes"), []byte("preserve me"), 0o600))
			case "missing-record":
				_ = os.Remove(filepath.Join(stage, "recovery-owner.json"))
			case "unknown-record":
				require.NoError(t, os.WriteFile(filepath.Join(stage, "recovery-owner.json"), []byte(`{"version":999}`), 0o600))
			}
			_, err := ResumeRecovery(snapshot, dir)
			require.Error(t, err)
			require.FileExists(t, snapshot)
			live, err := Open(dir)
			require.NoError(t, err)
			raw, err := live.ReadKey("meta/test/current")
			require.NoError(t, err)
			require.Equal(t, "original live bytes", string(raw))
			require.NoError(t, live.Close())
		})
	}
}

// TestManualRecoveryUsesDirectoryIdentityAfterRename proves the retained source
// follows its original directory identity, while an old pathname cannot name a
// replacement target. The operator can resolve using the actual moved path.
func TestManualRecoveryUsesDirectoryIdentityAfterRename(t *testing.T) {
	dir, snapshot, _ := retainedManualRecovery(t)
	moved := filepath.Join(t.TempDir(), "moved target")
	require.NoError(t, os.Rename(dir, moved))
	foreign, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, foreign.Close())
	movedSnapshot := filepath.Join(moved, filepath.Base(filepath.Dir(snapshot)), recoveryArtifactName)
	_, err = ResumeRecovery(movedSnapshot, dir)
	require.Error(t, err)
	require.FileExists(t, movedSnapshot)
	_, err = ResumeRecovery(movedSnapshot, moved)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Dir(movedSnapshot))
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestGenericRestoreKeepsPrivateRecoverySource proves the public keep-source
// contract still applies when a caller deliberately uses the generic API.
func TestGenericRestoreKeepsPrivateRecoverySource(t *testing.T) {
	dir, snapshot, _ := retainedManualRecovery(t)
	restored, err := RestoreBlob(snapshot, dir)
	require.NoError(t, err)
	require.NoError(t, restored.Close())
	require.FileExists(t, snapshot)
}

// TestManualRecoveryCloseFailurePreservesSource reaches the same completion
// boundary as ResumeRecovery using a real imported writer whose root close
// fails. Successful publication alone must not consume its recovery bytes.
func TestManualRecoveryCloseFailurePreservesSource(t *testing.T) {
	dir, path, _ := retainedManualRecovery(t)
	folder, err := openFolder(dir)
	require.NoError(t, err)
	snapshot, err := openRecoverySnapshot(folder, path)
	require.NoError(t, err)
	restored, err := applyRecoveryStore(folder, snapshot)
	require.NoError(t, err)
	require.NoError(t, restored.tree.root.Close())
	require.ErrorIs(t, completeRecoveryImport(folder, snapshot, restored), os.ErrClosed)
	require.FileExists(t, path)
	require.FileExists(t, filepath.Join(filepath.Dir(path), recoveryOwnershipName))
	_, err = ResumeRecovery(path, dir)
	require.NoError(t, err)
	require.NoDirExists(t, filepath.Dir(path))
}

// TestManualRecoveryReplaysOneIntentAfterRename proves failed application
// preserves the existing durable intent, and resume uses its original immutable
// source label through a newly opened actual directory after a parent rename.
func TestManualRecoveryReplaysOneIntentAfterRename(t *testing.T) {
	dir, path, _ := retainedManualRecovery(t)
	folder, err := openFolder(dir)
	require.NoError(t, err)
	snapshot, err := openRecoverySnapshot(folder, path)
	require.NoError(t, err)
	intent := importIntent{Policy: policyKeepSource, Source: snapshot.source.path, Digest: snapshot.receipt.Digest}
	intent.Tree, err = recordPrevious(folder, treeName, true, "recovery-test")
	require.NoError(t, err)
	intent.Seed, err = recordPrevious(folder, blobName, false, "recovery-test")
	require.NoError(t, err)
	intent.Stage, err = makeStage(folder, importStagePrefix)
	require.NoError(t, err)
	artifact, err := snapshot.source.open()
	require.NoError(t, err)
	built, err := buildStage(folder, intent.Stage, artifact)
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	intent.Device, intent.Inode = built.Device, built.Inode
	encoded, err := json.Marshal(intent)
	require.NoError(t, err)
	intentPath := filepath.Join(dir, importIntentName)
	require.NoError(t, os.WriteFile(intentPath, encoded, 0o600))
	require.NoError(t, folder.Sync())
	require.NoError(t, os.Rename(filepath.Join(dir, treeName), filepath.Join(dir, intent.Tree.Retired)))
	foreignTree := filepath.Join(dir, treeName)
	require.NoError(t, os.Mkdir(foreignTree, 0o700))
	marker := filepath.Join(foreignTree, "foreign-user-file")
	require.NoError(t, os.WriteFile(marker, []byte("keep"), 0o600))
	require.Error(t, applyRecovery(folder, snapshot))
	retainedIntent, err := os.ReadFile(intentPath)
	require.NoError(t, err)
	require.Equal(t, encoded, retainedIntent)
	require.FileExists(t, path)
	markerBytes, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "keep", string(markerBytes))
	// This directory was created immediately above by this test; after its
	// preservation assertion, remove that fixture to resolve the obstruction.
	require.NoError(t, os.Remove(marker))
	require.NoError(t, os.Remove(foreignTree))
	moved := filepath.Join(t.TempDir(), "moved-after-intent")
	require.NoError(t, os.Rename(dir, moved))
	foreign, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, foreign.WriteKey("meta/foreign", []byte("untouched")))
	require.NoError(t, foreign.Close())
	movedPath := filepath.Join(moved, filepath.Base(filepath.Dir(path)), recoveryArtifactName)
	_, err = ResumeRecovery(movedPath, moved)
	require.NoError(t, err)
	require.NoDirExists(t, filepath.Dir(movedPath))
	require.NoFileExists(t, filepath.Join(moved, importIntentName))
	require.DirExists(t, filepath.Join(moved, intent.Tree.Retired))
	stages, err := filepath.Glob(filepath.Join(moved, importStagePrefix+"*"))
	require.NoError(t, err)
	require.Empty(t, stages)
	foreign, err = Open(dir)
	require.NoError(t, err)
	foreignBytes, err := foreign.ReadKey("meta/foreign")
	require.NoError(t, err)
	require.Equal(t, "untouched", string(foreignBytes))
	require.NoError(t, foreign.Close())
}

// TestPartialRecoveryDebtRequiresPreservedRelocation proves incomplete cleanup
// cannot guess ownership or destroy remaining metadata. An owner can preserve
// that entire directory outside the config directory, then admit a new source.
func TestPartialRecoveryDebtRequiresPreservedRelocation(t *testing.T) {
	dir, path, original := retainedManualRecovery(t)
	current, err := RestoreBlob(path, dir)
	require.NoError(t, err)
	require.NoError(t, current.Close())
	stage := filepath.Dir(path)
	recordPath := filepath.Join(stage, recoveryOwnershipName)
	recordBytes, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	// This is a possible partial-cleanup state: the imported writer closed,
	// the private snapshot was removed, and the remaining cleanup failed.
	require.NoError(t, os.Remove(path))
	_, err = ResumeRecovery(path, dir)
	require.Error(t, err)
	current, err = Open(dir)
	require.NoError(t, err)
	pending, command, err := PendingRecoveryCommand(current)
	require.Error(t, err)
	require.Equal(t, stage, pending)
	require.Empty(t, command)
	after, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	require.Equal(t, recordBytes, after)
	require.NoError(t, current.Close())
	quarantine := filepath.Join(t.TempDir(), "preserved-owner-inspection")
	require.NoError(t, os.Rename(stage, quarantine))
	current, err = Open(dir)
	require.NoError(t, err)
	owner, err := NewDaemonRecovery(BindConfigSource(current, "router.conf", ConfigSourceStored))
	require.NoError(t, err)
	_, err = RecoveryFor(owner.Borrow()).StagePath(context.Background(), original)
	require.NoError(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	require.NoError(t, closed.Discard())
	retained, err := os.ReadFile(filepath.Join(quarantine, recoveryOwnershipName))
	require.NoError(t, err)
	require.Equal(t, recordBytes, retained)
}

// TestManualRecoveryRefusesUnsafeMetadata covers the on-disk ownership
// boundary independently of the artifact parser: malformed, enlarged, partial,
// symlinked and permissive records must preserve the source and old live keys.
func TestManualRecoveryRefusesUnsafeMetadata(t *testing.T) {
	for _, change := range []string{"malformed", "oversized", "unknown-field", "missing-field", "duplicate-field", "record-mode", "record-symlink", "snapshot-mode", "snapshot-symlink", "lock-inode", "target-identity", "stage-identity"} {
		t.Run(change, func(t *testing.T) {
			dir, snapshot, _ := retainedManualRecovery(t)
			stage := filepath.Dir(snapshot)
			recordPath := filepath.Join(stage, recoveryOwnershipName)
			content, err := os.ReadFile(recordPath)
			require.NoError(t, err)
			switch change {
			case "malformed":
				require.NoError(t, os.WriteFile(recordPath, []byte("{"), 0o600))
			case "oversized":
				require.NoError(t, os.WriteFile(recordPath, []byte(strings.Repeat(" ", recoveryOwnershipLimit+1)), 0o600))
			case "unknown-field":
				require.NoError(t, os.WriteFile(recordPath, append(append([]byte{}, content[:len(content)-1]...), []byte(`,"unknown":1}`)...), 0o600))
			case "missing-field":
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(content, &fields))
				delete(fields, "target-device")
				changed, err := json.Marshal(fields)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(recordPath, changed, 0o600))
			case "duplicate-field":
				changed := strings.Replace(string(content), `"version":1`, `"version":1,"version":1`, 1)
				require.NoError(t, os.WriteFile(recordPath, []byte(changed), 0o600))
			case "record-mode":
				require.NoError(t, os.Chmod(recordPath, 0o644))
			case "record-symlink":
				saved := filepath.Join(t.TempDir(), "original-record")
				require.NoError(t, os.Rename(recordPath, saved))
				require.NoError(t, os.Symlink(saved, recordPath))
			case "snapshot-mode":
				require.NoError(t, os.Chmod(snapshot, 0o644))
			case "snapshot-symlink":
				saved := filepath.Join(t.TempDir(), "original-snapshot")
				require.NoError(t, os.Rename(snapshot, saved))
				require.NoError(t, os.Symlink(saved, snapshot))
			case "lock-inode":
				lockPath := filepath.Join(stage, recoverySourceLockName)
				replacement := filepath.Join(t.TempDir(), "replacement-lock")
				require.NoError(t, os.WriteFile(replacement, nil, 0o600))
				require.NoError(t, os.Rename(replacement, lockPath))
			case "target-identity":
				var record recoveryOwnership
				require.NoError(t, json.Unmarshal(content, &record))
				record.TargetInode++
				changed, err := json.Marshal(record)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(recordPath, changed, 0o600))
			case "stage-identity":
				held := filepath.Join(t.TempDir(), "original-private-stage")
				require.NoError(t, os.Rename(stage, held))
				require.NoError(t, os.Mkdir(stage, 0o700))
				for _, name := range []string{recoveryArtifactName, recoverySourceLockName, recoveryOwnershipName} {
					data, err := os.ReadFile(filepath.Join(held, name))
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(filepath.Join(stage, name), data, 0o600))
				}
				require.FileExists(t, filepath.Join(held, recoveryArtifactName))
			}
			_, err = ResumeRecovery(snapshot, dir)
			require.Error(t, err)
			_, err = os.Lstat(snapshot)
			require.NoError(t, err)
			_, err = os.Lstat(recordPath)
			require.NoError(t, err)
			live, err := Open(dir)
			require.NoError(t, err)
			raw, err := live.ReadKey("meta/test/current")
			require.NoError(t, err)
			require.Equal(t, "original live bytes", string(raw))
			require.NoError(t, live.Close())
		})
	}
}
