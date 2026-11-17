// Design: docs/architecture/storage-backends.md -- immutable accepted recovery source.
// Related: daemon_recovery.go -- lifetime states and the outer close boundary.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func pendingRecoveryStage(parent *os.File) (_ string, retErr error) {
	listing, err := duplicateFolder(parent)
	if err != nil {
		return "", err
	}
	defer func() { retErr = errors.Join(retErr, listing.Close()) }()
	// The directory's finite entry census bounds the walk; each batch holds
	// at most 128 names, including when unrelated files fill the directory.
	for {
		entries, readErr := listing.ReadDir(128)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), recoveryStagePrefix) {
				return filepath.Join(parent.Name(), entry.Name()), nil
			}
		}
		if errors.Is(readErr, io.EOF) {
			return "", nil
		}
		if readErr != nil {
			return "", readErr
		}
	}
}

func buildRecoverySnapshot(ctx context.Context, parent *os.File, write func(io.Writer) error) (_ *recoverySnapshot, retErr error) {
	directory, err := makeStage(parent, recoveryStagePrefix)
	if err != nil {
		return nil, err
	}
	folder, err := openNode(parent, directory, true)
	if err != nil {
		return nil, errors.Join(err, removeStage(parent, directory))
	}
	root, err := blobRoot(folder)
	if err != nil {
		return nil, errors.Join(err, folder.Close(), removeStage(parent, directory))
	}
	snapshot := &recoverySnapshot{directory: directory, target: filepath.Join(parent.Name(), treeName),
		source: &importSource{path: filepath.Join(folder.Name(), recoveryArtifactName), folder: folder, root: root}}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, removeRecoverySnapshot(parent, snapshot))
		}
	}()
	fd, err := unix.Openat(int(folder.Fd()), recoveryArtifactName,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), snapshot.source.path)
	writer := &recoveryWriter{ctx: ctx, file: file}
	writeErr := write(writer)
	err = errors.Join(writeErr, file.Sync(), file.Close())
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	artifact, err := snapshot.source.open()
	if err != nil {
		return nil, err
	}
	if err := artifact.Close(); err != nil {
		return nil, err
	}
	digest, err := blobDigestIn(folder, recoveryArtifactName)
	if err != nil {
		return nil, err
	}
	snapshot.receipt = RecoveryReceipt{Bytes: writer.bytes, Digest: digest}
	if err := writeRecoveryOwnership(parent, snapshot); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// recoveryWriter enforces the existing import decoder's byte bound and checks
// cancellation at every source write. It borrows its file until staging ends.
type recoveryWriter struct {
	ctx   context.Context
	file  *os.File
	bytes int64
}

func (writer *recoveryWriter) Write(content []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(content)) > blobImportMax-writer.bytes {
		return 0, fmt.Errorf("the recovery source exceeds the %d-byte import limit", blobImportMax)
	}
	n, err := writer.file.Write(content)
	writer.bytes += int64(n)
	return n, err
}

// applyRecovery reuses the importer's retained-directory keep-source protocol.
// Its distinct caller type proves the original owner already closed; a racing
// new owner still refuses through the existing lifetime lock.
func applyRecovery(folder *os.File, snapshot *recoverySnapshot) error {
	restored, err := applyRecoveryStore(folder, snapshot)
	if err != nil {
		return retainedRecoveryFailure(errors.Join(err, snapshot.source.close(), folder.Close()), snapshot)
	}
	return completeRecoveryImport(folder, snapshot, restored)
}

func applyRecoveryStore(folder *os.File, snapshot *recoverySnapshot) (*store, error) {
	owner, err := lockOwner(folder, lockName)
	if err != nil {
		return nil, err
	}
	restored, err := importOwnedSource(snapshot.source, folder, owner, policyKeepSource, true)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return restored, nil
}

// completeRecoveryImport consumes private bytes only after the actual imported
// writer closed successfully. An unsuccessful close never grants cleanup.
func completeRecoveryImport(folder *os.File, snapshot *recoverySnapshot, restored *store) error {
	if err := restored.Close(); err != nil {
		return retainedRecoveryFailure(errors.Join(err, snapshot.source.close(), folder.Close()), snapshot)
	}
	return recoveryFailure(errors.Join(removeRecoverySnapshot(folder, snapshot), folder.Close()))
}

func removeRecoverySnapshot(parent *os.File, snapshot *recoverySnapshot) (retErr error) {
	defer func() { retErr = errors.Join(retErr, snapshot.source.close()) }()
	if snapshot.ownership.Version == 0 {
		// Only the still-running builder reaches this state. It created this
		// exact directory, and no durable ownership has yet been accepted.
		if err := verifyRecoveryNames(snapshot.source.folder, false); err != nil {
			return err
		}
		return removeRecoveryNodes(parent, snapshot)
	}
	lock, err := lockOwner(snapshot.source.folder, recoverySourceLockName)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, lock.Close()) }()
	proof := &recoveryProof{parent: parent, record: snapshot.ownership}
	if err := verifyRecoverySource(snapshot.source, proof); err != nil {
		return err
	}
	return removeRecoveryNodes(parent, snapshot)
}

func removeRecoveryNodes(parent *os.File, snapshot *recoverySnapshot) error {
	held, err := recoveryIdentity(snapshot.source.folder)
	if err != nil {
		return err
	}
	named, exists, err := identify(parent, snapshot.directory, true)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("the recovery stage entry disappeared; preserve its retained directory")
	}
	if named != held {
		return errors.New("the recovery stage entry was replaced; preserve both directories")
	}
	for _, name := range []string{recoveryArtifactName, recoverySourceLockName, recoveryOwnershipName} {
		if err := unix.Unlinkat(int(snapshot.source.folder.Fd()), name, 0); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	if err := unix.Unlinkat(int(parent.Fd()), snapshot.directory, unix.AT_REMOVEDIR); err != nil {
		return err
	}
	return parent.Sync()
}
