// Design: docs/architecture/storage-backends.md -- resolving retained private recovery.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RecoverySource recognizes the private recovery namespace, not ownership.
// Callers MUST send every matching path to ResumeRecovery; a missing or invalid
// record must never fall through to generic RestoreBlob.
func RecoverySource(path string) bool {
	return strings.HasPrefix(filepath.Base(filepath.Dir(filepath.Clean(path))), recoveryStagePrefix)
}

// ResumeRecovery completes an explicitly selected, source-owned private
// recovery through the normal durable importer. Success means the imported
// writer closed and the proven private source was consumed. Generic caller
// artifacts continue to use RestoreBlob and its keep-source contract.
func ResumeRecovery(path, dir string) (RecoveryReceipt, error) {
	if !RecoverySource(path) {
		return RecoveryReceipt{}, recoveryFailure(errors.New("the source is not a private recovery snapshot"))
	}
	if filepath.Base(path) != recoveryArtifactName {
		return RecoveryReceipt{}, recoveryFailure(errors.New("the private recovery source is not snapshot.zefs"))
	}
	folder, err := openFolder(dir)
	if err != nil {
		return RecoveryReceipt{}, recoveryFailure(err)
	}
	snapshot, err := openRecoverySnapshot(folder, path)
	if err != nil {
		return RecoveryReceipt{}, recoveryFailure(errors.Join(err, folder.Close()))
	}
	receipt := snapshot.receipt
	if err := applyRecovery(folder, snapshot); err != nil {
		return RecoveryReceipt{}, err
	}
	return receipt, nil
}

func openRecoverySnapshot(parent *os.File, path string) (_ *recoverySnapshot, retErr error) {
	source, err := openImportSource(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, source.close())
		}
	}()
	record, err := readRecoveryOwnership(source.folder)
	if err != nil {
		return nil, err
	}
	proof := &recoveryProof{parent: parent, record: record}
	if err := verifyRecoverySource(source, proof); err != nil {
		return nil, err
	}
	// The immutable source label is also the existing import intent's key.
	// I/O continues through the newly retained actual directory/root, so a
	// parent-directory rename cannot redirect it or prevent intent replay.
	source.path = record.Source
	source.recovery = proof
	return &recoverySnapshot{directory: filepath.Base(source.folder.Name()), source: source,
		target: filepath.Join(parent.Name(), treeName), ownership: record,
		receipt: RecoveryReceipt{Bytes: record.Bytes, Digest: record.Digest}}, nil
}

// PendingRecoveryCommand returns the safe completion command for the first
// retained stage under the actual store directory authority. Invalid or
// unowned stages return an error and remain untouched for owner inspection.
func PendingRecoveryCommand(source Storage) (_ string, _ string, retErr error) {
	owned, ok := ownedStore(source)
	if !ok {
		return "", "", errors.ErrUnsupported
	}
	if owned == nil {
		return "", "", errors.ErrUnsupported
	}
	owned.mu.RLock()
	defer owned.mu.RUnlock()
	if owned.closed {
		return "", "", os.ErrClosed
	}
	if owned.tree == nil {
		return "", "", nil
	}
	parent := owned.tree.folder
	stage, err := pendingRecoveryStage(parent)
	if err != nil {
		return "", "", err
	}
	if stage == "" {
		return "", "", nil
	}
	folder, err := openNode(parent, filepath.Base(stage), true)
	if err != nil {
		return stage, "", err
	}
	root, err := blobRoot(folder)
	if err != nil {
		return stage, "", errors.Join(err, folder.Close())
	}
	held := &importSource{path: filepath.Join(folder.Name(), recoveryArtifactName), folder: folder, root: root}
	defer func() { retErr = errors.Join(retErr, held.close()) }()
	record, err := readRecoveryOwnership(folder)
	if err != nil {
		return stage, "", fmt.Errorf("private recovery ownership is not proven: %w", err)
	}
	if err := verifyRecoverySource(held, &recoveryProof{parent: parent, record: record}); err != nil {
		return stage, "", err
	}
	return stage, recoveryRestoreCommand(filepath.Join(parent.Name(), treeName), held.path), nil
}
