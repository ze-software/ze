// Design: docs/contributing/github-integration.md -- one private durable notification ledger per workflow.
package github

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

const notificationStateFile = "notification-state.json"
const notificationLockFile = "notification-state.lock"
const notificationReceiptBytesMax = 64 << 10

// fileNotificationStore anchors all file operations in an already-existing
// private directory. The zero value is invalid. Call Close after all operations.
// withLock is safe for concurrent use and refuses concurrent scope ownership.
type fileNotificationStore struct{ root *os.Root }

// newFileNotificationStore opens a private receipt directory. Callers MUST call
// Close after all store operations complete.
func newFileNotificationStore(directory string) (*fileNotificationStore, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("notification state directory must be a directory, without a symbolic link")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("notification state directory must be private to its owner")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	return &fileNotificationStore{root: root}, nil
}

// Close releases the directory handle. Callers MUST wait for all store
// operations to complete before calling it.
func (store *fileNotificationStore) Close() error {
	if store == nil {
		return errors.New("notification store was not constructed")
	}
	if store.root == nil {
		return errors.New("notification store has no directory handle")
	}
	return store.root.Close()
}

// initialize creates a deliberately empty receipt only. It never repairs,
// truncates or replaces an existing receipt, including a corrupt one.
func (store *fileNotificationStore) initialize(ctx context.Context, receipt notificationReceipt) error {
	if receipt.Mode != notificationReceiptReady {
		return errors.New("initial notification receipt is not ready")
	}
	if receipt.SeenRunID != 0 {
		return errors.New("initial notification receipt already names an observed run")
	}
	if receipt.FailureDigest != "" {
		return errors.New("initial notification receipt already claims a delivered failure")
	}
	return store.withLock(ctx, func(ledger notificationLedger) error {
		file, ok := ledger.(notificationFileLedger)
		if !ok {
			return errors.New("notification file store lost its locked ledger")
		}
		return file.publish(receipt, true)
	})
}

// withLock holds an exclusive OS lock across read, durable intent, send and
// durable result. It does not wait or retry when another process owns the scope.
func (store *fileNotificationStore) withLock(ctx context.Context, use func(notificationLedger) error) (result error) {
	if store == nil {
		return errors.New("notification store was not constructed")
	}
	if store.root == nil {
		return errors.New("notification store has no directory handle")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := notificationRootFile(store.root, notificationLockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("notification lock is not a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("notification lock is not private")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("notification scope could not be exclusively locked: %w", err)
	}
	// Closing the descriptor releases this lock, including when the callback fails.
	return use(notificationFileLedger{root: store.root})
}

type notificationFileLedger struct{ root *os.Root }

func (ledger notificationFileLedger) load() (notificationReceipt, error) {
	// A FIFO must not wait for a writer while holding the scope lock. Open
	// nonblocking, then qualify the actual descriptor before reading bytes.
	file, err := notificationRootFile(ledger.root, notificationStateFile, os.O_RDONLY, 0)
	if err != nil {
		return notificationReceipt{}, err
	}
	defer file.Close() //nolint:errcheck // A read-only close cannot qualify an acknowledged delivery.
	info, err := file.Stat()
	if err != nil {
		return notificationReceipt{}, err
	}
	if !info.Mode().IsRegular() {
		return notificationReceipt{}, errors.New("notification receipt is not a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return notificationReceipt{}, errors.New("notification receipt is not private")
	}
	body, err := io.ReadAll(io.LimitReader(file, notificationReceiptBytesMax+1))
	if err != nil {
		return notificationReceipt{}, err
	}
	if len(body) > notificationReceiptBytesMax {
		return notificationReceipt{}, errors.New("notification receipt exceeds its byte bound")
	}
	var receipt notificationReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		return receipt, errors.New("notification receipt is corrupt; reconcile its attempted delivery before replacement")
	}
	if err := notificationReceiptValid(receipt); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func (ledger notificationFileLedger) save(receipt notificationReceipt) error {
	return ledger.publish(receipt, false)
}

func (ledger notificationFileLedger) publish(receipt notificationReceipt, initial bool) (result error) {
	if err := notificationReceiptValid(receipt); err != nil {
		return err
	}
	if !initial {
		if _, err := ledger.load(); err != nil {
			return err
		}
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if len(body) > notificationReceiptBytesMax {
		return errors.New("notification receipt exceeds its byte bound")
	}
	name := ".notification-state-" + rand.Text()
	file, err := ledger.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err := ledger.root.Remove(name); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result = errors.Join(result, err)
			}
		}
	}()
	if _, err := file.Write(body); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if initial {
		// Link publishes without replacing an existing state file. Both names
		// remain anchored under the directory handle through publication.
		if err := ledger.root.Link(name, notificationStateFile); err != nil {
			return err
		}
	} else {
		if err := ledger.root.Rename(name, notificationStateFile); err != nil {
			return err
		}
	}
	directory, err := ledger.root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
