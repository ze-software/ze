// Design: docs/architecture/storage-backends.md -- independent artifact editing
// Related: pointer.go -- shared version allocation and promotion
// Related: restore.go -- ordinary restore refuses an existing candidate

package storage

import (
	"errors"
	"fmt"
	"time"
)

// PublishArtifactConfig publishes a committed configuration inside an explicit
// writable blob artifact. It preserves the artifact's pending candidate, saved
// drafts and unrelated raw keys. Live tree stores are refused because their
// configuration owner must accept a change through its reload transaction.
//
// The caller MUST validate data and serialize its editor's review and export
// with this call. The store owns its guard for the whole publication; no
// candidate pointer is replaced, even temporarily. Safe for concurrent use.
func PublishArtifactConfig(target Storage, name string, data []byte) (stamp string, err error) {
	owned, ok := ownedStore(target)
	if !ok {
		return "", fmt.Errorf("publish backup configuration %s: explicit blob artifact required: %w", name, errors.ErrUnsupported)
	}
	if owned == nil {
		return "", fmt.Errorf("publish backup configuration %s: no artifact handle: %w", name, errors.ErrUnsupported)
	}
	if owned.blob == nil {
		return "", fmt.Errorf("publish backup configuration %s: a live tree is not a backup artifact: %w", name, errors.ErrUnsupported)
	}
	guard, err := target.AcquireLock(name)
	if err != nil {
		return "", err
	}
	defer func() { err = releaseGuard(guard, err) }()
	if err := checkArtifactCandidate(target, guard, name); err != nil {
		return "", err
	}
	stamp, err = writeNewVersionLocked(target, guard, name, data, time.Now())
	if err != nil {
		return "", err
	}
	if err := promoteVersionLocked(target, guard, name, stamp); err != nil {
		return "", withdrawArtifactVersion(target, guard, name, stamp, err)
	}
	return stamp, nil
}

// checkArtifactCandidate diagnoses unreadable pending state before publication.
// The pointer and its version remain unchanged; a backup may contain a pending
// edit without selecting it as the browser's committed starting configuration.
func checkArtifactCandidate(target Storage, guard WriteGuard, name string) error {
	pending, exists, err := readPointerLocked(target, guard, name, pointerCandidate)
	if err != nil {
		return fmt.Errorf("read backup configuration %s pending candidate: %w", name, err)
	}
	if !exists {
		return nil
	}
	if _, err := readVersionLocked(target, guard, name, pending); err != nil {
		return fmt.Errorf("read backup configuration %s pending candidate: %w", name, err)
	}
	return nil
}

// withdrawArtifactVersion removes an unaccepted new version, using the same
// active-pointer commit point as RestoreConfig. A failure after acceptance must
// retain the accepted version; the caller reports the storage error as returned.
func withdrawArtifactVersion(target Storage, guard WriteGuard, name, stamp string, failure error) error {
	active, exists, err := readPointerLocked(target, guard, name, pointerActive)
	if err != nil {
		return errors.Join(failure, err)
	}
	if exists {
		if active == stamp {
			return failure
		}
	}
	_, err = removeVersionLocked(target, guard, name, stamp)
	return errors.Join(failure, err)
}
