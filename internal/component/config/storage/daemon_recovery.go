// Design: docs/architecture/storage-backends.md -- full restore after daemon ownership ends.
// Related: import.go -- the durable restore protocol used after close.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DaemonRecovery owns the live store across one daemon run. The outer launcher
// MUST close it after every borrower returns. Its zero value is uninitialized.
// Its owner methods are not safe for concurrent use; borrowers use the store.
type DaemonRecovery struct {
	source    *configStore
	admission *RecoveryAdmission
}

// RecoveryAdmission is borrowed by command handlers during the daemon run.
// Closing its owner seals further requests. Its zero value refuses admission.
// Safe for concurrent use, including a request racing the owner's close.
type RecoveryAdmission struct {
	mu    sync.Mutex
	owner *store
	state recoveryAdmissionState
}

// Admission is shared with callers that cannot have their aliases revoked.
// These private variants keep the accepted snapshot in its one valid state;
// the runtime seal still refuses a caller racing the owner's close.
type recoveryAdmissionState interface{ recoveryAdmissionState() }
type recoveryOpen struct{ folder *os.File }
type recoveryAccepted struct {
	folder   *os.File
	snapshot *recoverySnapshot
}
type recoverySealed struct{}

func (recoveryOpen) recoveryAdmissionState()     {}
func (recoveryAccepted) recoveryAdmissionState() {}
func (recoverySealed) recoveryAdmissionState()   {}

// ClosedRecovery is available only after the original store closed successfully.
// Its owner MUST call Apply for an accepted restart, or Discard for another exit.
// Its zero value refuses both operations.
// Not safe for concurrent use. Exactly one Apply or Discard MUST consume it.
type ClosedRecovery struct{ state closedRecoveryState }

type closedRecoveryState interface{ closedRecoveryState() }
type closedRecoveryEmpty struct{ folder *os.File }
type closedRecoveryAccepted struct {
	folder   *os.File
	snapshot *recoverySnapshot
}
type closedRecoveryConsumed struct{}

func (closedRecoveryEmpty) closedRecoveryState()    {}
func (closedRecoveryAccepted) closedRecoveryState() {}
func (closedRecoveryConsumed) closedRecoveryState() {}

// RecoveryReceipt describes the immutable snapshot accepted for the next restart.
type RecoveryReceipt struct {
	Bytes  int64  `json:"bytes"`
	Digest string `json:"digest"`
}

var errRecoveryNotInstalled = errors.New("full-store recovery lifecycle is not installed")

const recoveryStagePrefix = "database.recovery-tmp-"
const recoveryArtifactName = "snapshot.zefs"

// recoverySnapshot retains both directory authorities the existing importer
// needs. Its immutable private copy survives the artifact owner's shutdown.
type recoverySnapshot struct {
	directory string
	source    *importSource
	receipt   RecoveryReceipt
	target    string
	ownership recoveryOwnership
}

// NewDaemonRecovery transfers lifetime ownership of a bound live-tree handle.
// On failure the caller retains the handle and MUST close it.
func NewDaemonRecovery(source Storage) (*DaemonRecovery, error) {
	bound, ok := source.(*configStore)
	if !ok {
		return nil, recoveryFailure(errRecoveryNotInstalled)
	}
	if bound == nil {
		return nil, recoveryFailure(errRecoveryNotInstalled)
	}
	owned, ok := ownedStore(bound)
	if !ok {
		return nil, recoveryFailure(errRecoveryNotInstalled)
	}
	if owned == nil {
		return nil, recoveryFailure(errRecoveryNotInstalled)
	}
	owned.mu.RLock()
	defer owned.mu.RUnlock()
	if owned.closed {
		return nil, recoveryFailure(os.ErrClosed)
	}
	if owned.readonly {
		return nil, recoveryFailure(errors.New("the store is read-only"))
	}
	result := &DaemonRecovery{source: bound}
	// A blob or memory-backed runtime keeps its existing lifetime but cannot
	// admit a full live-tree replacement. Its normal start must still work.
	if owned.tree == nil {
		return result, nil
	}
	folder, err := duplicateFolder(owned.tree.folder)
	if err != nil {
		return nil, recoveryFailure(err)
	}
	admission := &RecoveryAdmission{owner: owned, state: recoveryOpen{folder: folder}}
	if !bound.recovery.CompareAndSwap(nil, admission) {
		return nil, recoveryFailure(errors.Join(errors.New("another daemon lifetime already owns this handle"), folder.Close()))
	}
	result.admission = admission
	return result, nil
}

// Borrow returns the same store handle every daemon service borrows. Its caller
// MUST finish using it before Close and MUST NOT close the borrowed handle.
func (owner *DaemonRecovery) Borrow() Storage {
	if owner == nil {
		return nil
	}
	if owner.source == nil {
		return nil
	}
	return owner.source
}

// RecoveryFor returns the command admission owned by this bound daemon handle,
// or nil when its outer launcher installed no recovery lifetime.
func RecoveryFor(source Storage) *RecoveryAdmission {
	bound, ok := source.(*configStore)
	if !ok {
		return nil
	}
	if bound == nil {
		return nil
	}
	return bound.recovery.Load()
}

// PendingRecoveryStage reports the first retained private recovery directory.
// An empty path means the complete directory walk found none, never a failed
// lookup. The doctor and admission share this authority. Blob/memory stores
// have no live-tree recovery directory. Safe for concurrent use.
func PendingRecoveryStage(source Storage) (string, error) {
	owned, ok := ownedStore(source)
	if !ok {
		return "", errors.ErrUnsupported
	}
	if owned == nil {
		return "", errors.ErrUnsupported
	}
	owned.mu.RLock()
	defer owned.mu.RUnlock()
	if owned.closed {
		return "", os.ErrClosed
	}
	if owned.tree == nil {
		return "", nil
	}
	return pendingRecoveryStage(owned.tree.folder)
}

// StagePath retains a verified private copy; the caller's source remains unchanged.
// A successful receipt is accepted and MUST survive caller cancellation afterward.
func (admission *RecoveryAdmission) StagePath(ctx context.Context, path string) (RecoveryReceipt, error) {
	if admission == nil {
		return RecoveryReceipt{}, recoveryFailure(errRecoveryNotInstalled)
	}
	if err := CheckArtifactPath(path); err != nil {
		return RecoveryReceipt{}, recoveryFailure(err)
	}
	if admission.owner == nil {
		return RecoveryReceipt{}, recoveryFailure(errRecoveryNotInstalled)
	}
	if err := refuseStoreName(admission.owner, path, "restore"); err != nil {
		return RecoveryReceipt{}, recoveryFailure(err)
	}
	return admission.StageWriter(ctx, func(target io.Writer) error {
		source, err := openImportSource(path)
		if err != nil {
			return err
		}
		owner, err := lockOwner(source.folder, filepath.Base(path)+".lock")
		if err != nil {
			return errors.Join(err, source.close())
		}
		file, err := openNode(source.folder, filepath.Base(path), false)
		if err != nil {
			return errors.Join(err, owner.Close(), source.close())
		}
		_, copyErr := io.Copy(target, file)
		return errors.Join(copyErr, file.Close(), owner.Close(), source.close())
	})
}

// StageWriter accepts bytes from another artifact owner into recovery-owned
// staging. write MUST return only after all its bytes have been written.
func (admission *RecoveryAdmission) StageWriter(ctx context.Context, write func(io.Writer) error) (RecoveryReceipt, error) {
	if admission == nil {
		return RecoveryReceipt{}, recoveryFailure(errRecoveryNotInstalled)
	}
	if ctx == nil {
		return RecoveryReceipt{}, recoveryFailure(errors.New("the request context is missing"))
	}
	if write == nil {
		return RecoveryReceipt{}, recoveryFailure(errors.New("the source writer is missing"))
	}
	admission.mu.Lock()
	defer admission.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return RecoveryReceipt{}, recoveryFailure(err)
	}
	var folder *os.File
	switch state := admission.state.(type) {
	case recoveryOpen:
		folder = state.folder
	case recoveryAccepted:
		return RecoveryReceipt{}, recoveryFailure(errors.New("a full-store recovery was already accepted"))
	case recoverySealed:
		return RecoveryReceipt{}, recoveryFailure(errors.New("the daemon store lifetime has ended"))
	case nil:
		return RecoveryReceipt{}, recoveryFailure(errRecoveryNotInstalled)
	}
	if folder == nil {
		return RecoveryReceipt{}, recoveryFailure(errRecoveryNotInstalled)
	}
	retained, err := pendingRecoveryStage(folder)
	if err != nil {
		return RecoveryReceipt{}, recoveryFailure(err)
	}
	if retained != "" {
		return RecoveryReceipt{}, recoveryFailure(fmt.Errorf("an earlier recovery stage remains at %s; resolve it before accepting another source", retained))
	}
	snapshot, err := buildRecoverySnapshot(ctx, folder, write)
	if err != nil {
		return RecoveryReceipt{}, recoveryFailure(err)
	}
	admission.state = recoveryAccepted{folder: folder, snapshot: snapshot}
	return snapshot.receipt, nil
}

// Close seals admission and closes the original owner. Only success yields
// a value that can apply an accepted recovery. It is safe to call once.
func (owner *DaemonRecovery) Close() (*ClosedRecovery, error) {
	if owner == nil {
		return nil, recoveryFailure(errRecoveryNotInstalled)
	}
	if owner.source == nil {
		return nil, recoveryFailure(errRecoveryNotInstalled)
	}
	var folder *os.File
	var snapshot *recoverySnapshot
	if owner.admission != nil {
		owner.admission.mu.Lock()
		switch state := owner.admission.state.(type) {
		case recoveryOpen:
			folder = state.folder
		case recoveryAccepted:
			folder, snapshot = state.folder, state.snapshot
		case recoverySealed:
			owner.admission.mu.Unlock()
			return nil, recoveryFailure(errors.New("the daemon store lifetime already ended"))
		case nil:
			owner.admission.mu.Unlock()
			return nil, recoveryFailure(errRecoveryNotInstalled)
		}
		owner.admission.state = recoverySealed{}
		owner.admission.mu.Unlock()
		owner.source.recovery.CompareAndSwap(owner.admission, nil)
	}
	source := owner.source
	owner.source = nil
	if err := source.Close(); err != nil {
		if snapshot != nil {
			return nil, retainedRecoveryFailure(errors.Join(err, snapshot.source.close(), folder.Close()), snapshot)
		}
		if folder != nil {
			err = errors.Join(err, folder.Close())
		}
		return nil, recoveryFailure(err)
	}
	if snapshot != nil {
		return &ClosedRecovery{state: closedRecoveryAccepted{folder: folder, snapshot: snapshot}}, nil
	}
	return &ClosedRecovery{state: closedRecoveryEmpty{folder: folder}}, nil
}

// Apply runs the existing durable restore only after successful owner close.
// Failure retains any source a durable import intent can still reference.
func (closed *ClosedRecovery) Apply() error {
	if closed == nil {
		return recoveryFailure(errRecoveryNotInstalled)
	}
	state := closed.state
	closed.state = closedRecoveryConsumed{}
	switch accepted := state.(type) {
	case closedRecoveryEmpty:
		if accepted.folder != nil {
			return accepted.folder.Close()
		}
		return nil
	case closedRecoveryAccepted:
		return applyRecovery(accepted.folder, accepted.snapshot)
	case closedRecoveryConsumed:
		return recoveryFailure(errors.New("the closed recovery was already consumed"))
	case nil:
		return recoveryFailure(errRecoveryNotInstalled)
	}
	return recoveryFailure(errRecoveryNotInstalled)
}

// Discard removes an accepted snapshot when the daemon did not finish its
// requested restart. Cleanup errors remain observable to the outer launcher.
func (closed *ClosedRecovery) Discard() error {
	if closed == nil {
		return recoveryFailure(errRecoveryNotInstalled)
	}
	state := closed.state
	closed.state = closedRecoveryConsumed{}
	switch accepted := state.(type) {
	case closedRecoveryEmpty:
		if accepted.folder != nil {
			return accepted.folder.Close()
		}
		return nil
	case closedRecoveryAccepted:
		return recoveryFailure(errors.Join(removeRecoverySnapshot(accepted.folder, accepted.snapshot), accepted.folder.Close()))
	case closedRecoveryConsumed:
		return recoveryFailure(errors.New("the closed recovery was already consumed"))
	case nil:
		return recoveryFailure(errRecoveryNotInstalled)
	}
	return recoveryFailure(errRecoveryNotInstalled)
}

func recoveryFailure(cause error) error {
	if cause == nil {
		return nil
	}
	return opererr.Wrap(cause, "full-store recovery could not complete",
		"an accepted full restore can replace the store only after the daemon and its original store owner finish cleanly", "ze doctor")
}

func retainedRecoveryFailure(cause error, snapshot *recoverySnapshot) error {
	return opererr.Wrap(cause, "the verified recovery source remains at "+snapshot.source.path,
		"full recovery via ze data restore did not finish cleanly; preserve this source and any database.import-intent, then run the command after all store owners stop", recoveryRestoreCommand(snapshot.target, snapshot.source.path))
}

func recoveryRestoreCommand(target, source string) string {
	var fix textbuf.Buffer
	fix.Str("ze data --path ").Str(configtext.QuoteCommandWord(target))
	fix.Str(" restore ").Str(configtext.QuoteCommandWord(source)).Str(" full")
	return fix.String()
}
