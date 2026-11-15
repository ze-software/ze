// Design: docs/contributing/github-integration.md -- nonregular receipts fail before reading or delivery.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// notificationReceiptEntryStore marks entry after the real context/lock guard.
// The FIFO test's deadline cannot turn an unentered load into a passing refusal.
type notificationReceiptEntryStore struct {
	store   *fileNotificationStore
	entered chan struct{}
}

func (store notificationReceiptEntryStore) withLock(ctx context.Context, use func(notificationLedger) error) error {
	return store.store.withLock(ctx, func(ledger notificationLedger) error {
		close(store.entered)
		return use(ledger)
	})
}

// TestNotificationFileStoreRejectsFIFOWithoutWaitingForWriter requires the
// delivery path to reject a real private FIFO with no writer. The bounded
// reader is always joined: the counterfactual blocking implementation is
// released by a nonblocking read/write descriptor before the test reports red.
func TestNotificationFileStoreRejectsFIFOWithoutWaitingForWriter(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, notificationStateFile)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newFileNotificationStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	sends := 0
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		sends++
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	entered := make(chan struct{})
	done := make(chan error, 1)
	joined := false
	// One owned reader lifecycle; the release below joins it on both the
	// correct path and the blocking implementation used for discrimination.
	go func() {
		_, deliveryErr := deliverNotification(ctx, notificationReceiptEntryStore{store: store, entered: entered}, preview, approval, sender)
		done <- deliveryErr
	}()
	t.Cleanup(func() {
		if joined {
			return
		}
		cancel()
		release, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Errorf("could not release the owned FIFO reader during cleanup: %v", err)
			return
		}
		<-done
		if err := release.Close(); err != nil {
			t.Error(err)
		}
	})
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("delivery did not enter the receipt's real locked load")
	}
	// This is a deadlock bound, not a latency claim. It starts only after the
	// context guard and lock acquisition, so an expired guard cannot pass.
	bound := time.NewTimer(10 * time.Second)
	defer bound.Stop()
	select {
	case deliveryErr := <-done:
		joined = true
		if deliveryErr == nil {
			t.Error("a FIFO receipt authorized delivery")
		}
		if errors.Is(deliveryErr, context.DeadlineExceeded) {
			t.Error("context expiry masked the nonregular receipt check")
		}
	case <-bound.C:
		// O_RDWR does not wait for a peer. Holding this writer releases a
		// blocking O_RDONLY open even if that reader has not yet been scheduled.
		release, openErr := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if openErr != nil {
			t.Fatalf("could not release the counterfactual FIFO reader: %v", openErr)
		}
		deliveryErr := <-done
		joined = true
		if err := release.Close(); err != nil {
			t.Error(err)
		}
		if deliveryErr == nil {
			t.Error("a FIFO receipt authorized delivery after release")
		}
		t.Error("receipt load waited for a FIFO writer while holding the delivery lock")
	}
	if sends != 0 {
		t.Errorf("nonregular receipt reached sender %d times", sends)
	}
	locked := false
	if err := store.withLock(t.Context(), func(notificationLedger) error { locked = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !locked {
		t.Error("nonregular receipt left the delivery lock held")
	}
}

// TestNotificationFileStoreRejectsOtherNonRegularReceipts proves the descriptor
// regular-file check and no-follow rule also refuse directories and symlinks.
func TestNotificationFileStoreRejectsOtherNonRegularReceipts(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.Chmod(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, notificationStateFile)
			if kind == "directory" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(directory, "ordinary-receipt")
				preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
				receipt, err := newNotificationReceipt(preview, approval)
				if err != nil {
					t.Fatal(err)
				}
				body, err := json.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, body, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			store, err := newFileNotificationStore(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
			sends := 0
			sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
				sends++
				return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
			})
			if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err == nil {
				t.Error("nonregular receipt authorized delivery")
			}
			if sends != 0 {
				t.Errorf("nonregular receipt reached sender %d times", sends)
			}
		})
	}
}
