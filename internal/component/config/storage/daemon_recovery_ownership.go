// Design: docs/architecture/storage-backends.md -- source-owned private recovery.
// Related: import.go -- the only durable publication/replay protocol.
package storage

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const recoveryOwnershipName = "recovery-owner.json"
const recoverySourceLockName = recoveryArtifactName + ".lock"
const recoveryOwnershipLimit = 4096

// recoveryOwnership proves which private bytes and destination this stage
// owns. It is metadata only: importIntent remains the sole publication journal.
// Source and Target are immutable labels; identities authorize the operation.
type recoveryOwnership struct {
	Version        int    `json:"version"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	TargetDevice   uint64 `json:"target-device"`
	TargetInode    uint64 `json:"target-inode"`
	StageDevice    uint64 `json:"stage-device"`
	StageInode     uint64 `json:"stage-inode"`
	SnapshotDevice uint64 `json:"snapshot-device"`
	SnapshotInode  uint64 `json:"snapshot-inode"`
	LockDevice     uint64 `json:"lock-device"`
	LockInode      uint64 `json:"lock-inode"`
	Bytes          int64  `json:"bytes"`
	Digest         string `json:"digest"`
}

// recoveryProof borrows the destination directory until its source closes.
// importOwnedSource verifies it while holding its existing source owner lock.
type recoveryProof struct {
	parent *os.File
	record recoveryOwnership
}

func recoveryIdentity(file *os.File) (nodeIdentity, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return nodeIdentity{}, err
	}
	return nodeIdentity{Device: uint64(stat.Dev), Inode: stat.Ino}, nil //nolint:unconvert // Dev differs between supported hosts.
}

func writeRecoveryOwnership(parent *os.File, snapshot *recoverySnapshot) error {
	source := snapshot.source
	lock, err := lockOwner(source.folder, recoverySourceLockName)
	if err != nil {
		return err
	}
	lockID, identityErr := recoveryIdentity(lock)
	if err := errors.Join(identityErr, lock.Close()); err != nil {
		return err
	}
	targetID, err := recoveryIdentity(parent)
	if err != nil {
		return err
	}
	stageID, err := recoveryIdentity(source.folder)
	if err != nil {
		return err
	}
	snapshotID, exists, err := identify(source.folder, recoveryArtifactName, false)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("the verified recovery snapshot disappeared")
	}
	record := recoveryOwnership{Version: 1, Source: source.path, Target: snapshot.target,
		TargetDevice: targetID.Device, TargetInode: targetID.Inode,
		StageDevice: stageID.Device, StageInode: stageID.Inode,
		SnapshotDevice: snapshotID.Device, SnapshotInode: snapshotID.Inode,
		LockDevice: lockID.Device, LockInode: lockID.Inode,
		Bytes: snapshot.receipt.Bytes, Digest: snapshot.receipt.Digest}
	content, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(content) > recoveryOwnershipLimit {
		return errors.New("the recovery ownership record exceeds its size limit")
	}
	fd, err := unix.Openat(int(source.folder.Fd()), recoveryOwnershipName,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(source.folder.Name(), recoveryOwnershipName))
	n, writeErr := file.Write(content)
	if writeErr == nil {
		if n != len(content) {
			writeErr = io.ErrShortWrite
		}
	}
	if err := errors.Join(writeErr, file.Sync(), file.Close(), source.folder.Sync(), parent.Sync()); err != nil {
		return err
	}
	snapshot.ownership = record
	source.recovery = &recoveryProof{parent: parent, record: record}
	return nil
}

func readRecoveryOwnership(folder *os.File) (recoveryOwnership, error) {
	file, err := openNode(folder, recoveryOwnershipName, false)
	if err != nil {
		return recoveryOwnership{}, err
	}
	content, readErr := io.ReadAll(io.LimitReader(file, recoveryOwnershipLimit+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return recoveryOwnership{}, err
	}
	if len(content) > recoveryOwnershipLimit {
		return recoveryOwnership{}, errors.New("oversized recovery ownership record")
	}
	var record recoveryOwnership
	if err := json.Unmarshal(content, &record); err != nil {
		return recoveryOwnership{}, err
	}
	canonical, err := json.Marshal(record)
	if err != nil {
		return recoveryOwnership{}, err
	}
	// Only Ze writes this private format. Exact canonical bytes reject unknown,
	// missing, duplicate and reordered fields rather than guessing an owner.
	if !bytes.Equal(content, canonical) {
		return recoveryOwnership{}, errors.New("noncanonical or unrecognized recovery ownership fields")
	}
	if record.Version != 1 {
		return recoveryOwnership{}, errors.New("unrecognized recovery ownership version")
	}
	if !filepath.IsAbs(record.Source) {
		return recoveryOwnership{}, errors.New("invalid recovery source label")
	}
	if filepath.Clean(record.Source) != record.Source {
		return recoveryOwnership{}, errors.New("invalid recovery source label")
	}
	if !filepath.IsAbs(record.Target) {
		return recoveryOwnership{}, errors.New("invalid recovery target label")
	}
	if filepath.Clean(record.Target) != record.Target {
		return recoveryOwnership{}, errors.New("invalid recovery target label")
	}
	if filepath.Base(record.Target) != treeName {
		return recoveryOwnership{}, errors.New("invalid recovery target name")
	}
	stage := filepath.Base(folder.Name())
	if !strings.HasPrefix(stage, recoveryStagePrefix) {
		return recoveryOwnership{}, errors.New("invalid recovery stage name")
	}
	if record.Source != filepath.Join(filepath.Dir(record.Target), stage, recoveryArtifactName) {
		return recoveryOwnership{}, errors.New("recovery source and target labels disagree")
	}
	if record.Bytes <= 0 {
		return recoveryOwnership{}, errors.New("invalid recovery snapshot size")
	}
	if record.Bytes > blobImportMax {
		return recoveryOwnership{}, errors.New("invalid recovery snapshot size")
	}
	digest, err := hex.DecodeString(record.Digest)
	if err != nil {
		return recoveryOwnership{}, err
	}
	if len(digest) != 32 {
		return recoveryOwnership{}, errors.New("invalid recovery snapshot digest")
	}
	return record, nil
}

func verifyRecoverySource(source *importSource, proof *recoveryProof) error {
	record, err := readRecoveryOwnership(source.folder)
	if err != nil {
		return err
	}
	if record != proof.record {
		return errors.New("the recovery ownership record changed after verification")
	}
	target, err := recoveryIdentity(proof.parent)
	if err != nil {
		return err
	}
	if target != (nodeIdentity{Device: record.TargetDevice, Inode: record.TargetInode}) {
		return errors.New("the recovery source belongs to another target directory")
	}
	stage, err := recoveryIdentity(source.folder)
	if err != nil {
		return err
	}
	if stage != (nodeIdentity{Device: record.StageDevice, Inode: record.StageInode}) {
		return errors.New("the recovery source directory was replaced")
	}
	named, exists, err := identify(proof.parent, filepath.Base(source.folder.Name()), true)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("the recovery directory is no longer at its owned target entry")
	}
	if named != stage {
		return errors.New("the recovery directory is no longer at its owned target entry")
	}
	if err := verifyRecoveryEntries(source.folder); err != nil {
		return err
	}
	file, err := openNode(source.folder, recoveryArtifactName, false)
	if err != nil {
		return err
	}
	identity, identityErr := recoveryIdentity(file)
	info, statErr := file.Stat()
	if err := errors.Join(identityErr, statErr, file.Close()); err != nil {
		return err
	}
	if identity != (nodeIdentity{Device: record.SnapshotDevice, Inode: record.SnapshotInode}) {
		return errors.New("the recovery snapshot file was replaced")
	}
	if info.Size() != record.Bytes {
		return errors.New("the recovery snapshot size changed")
	}
	digest, err := blobDigestIn(source.folder, recoveryArtifactName)
	if err != nil {
		return err
	}
	if digest != record.Digest {
		return errors.New("the recovery snapshot bytes changed")
	}
	lock, err := openNode(source.folder, recoverySourceLockName, false)
	if err != nil {
		return err
	}
	lockID, identityErr := recoveryIdentity(lock)
	lockInfo, statErr := lock.Stat()
	if err := errors.Join(identityErr, statErr, lock.Close()); err != nil {
		return err
	}
	if lockID != (nodeIdentity{Device: record.LockDevice, Inode: record.LockInode}) {
		return errors.New("the recovery source lock was replaced")
	}
	if lockInfo.Size() != 0 {
		return errors.New("the recovery source lock contains unowned bytes")
	}
	return nil
}

func verifyRecoveryEntries(folder *os.File) error {
	return verifyRecoveryNames(folder, true)
}

func verifyRecoveryNames(folder *os.File, complete bool) error {
	listing, err := duplicateFolder(folder)
	if err != nil {
		return err
	}
	entries, readErr := listing.ReadDir(4)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(readErr, listing.Close()); err != nil {
		return err
	}
	if len(entries) > 3 {
		return errors.New("the recovery stage contains unowned entries; preserve the whole directory")
	}
	if complete {
		if len(entries) != 3 {
			return errors.New("the recovery stage contains missing or unowned entries; preserve the whole directory")
		}
	}
	for _, entry := range entries {
		switch entry.Name() {
		case recoveryArtifactName, recoverySourceLockName, recoveryOwnershipName:
		default:
			return fmt.Errorf("the recovery stage contains unowned entry %q; preserve the whole directory", entry.Name())
		}
	}
	return nil
}
