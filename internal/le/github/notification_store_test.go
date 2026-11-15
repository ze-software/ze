// Design: docs/contributing/github-integration.md -- locked, durable notification receipts.
package github

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestNotificationFileStorePersistsIntentAndStopsAfterRestart observes the
// actual state file during a send, then reopens it after an ambiguous result.
func TestNotificationFileStorePersistsIntentAndStopsAfterRestart(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newFileNotificationStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.withLock(t.Context(), func(ledger notificationLedger) error {
		_, err := ledger.load()
		if err == nil {
			t.Error("missing state was treated as an empty ledger")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.initialize(t.Context(), receipt); err != nil {
		t.Fatal(err)
	}
	if err := store.initialize(t.Context(), receipt); err == nil {
		t.Error("initialization replaced existing state")
	}
	sends := 0
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		sends++
		root, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		observed, loadErr := (notificationFileLedger{root: root}).load()
		closeErr := root.Close()
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if observed.Mode != notificationReceiptInFlight {
			t.Error("POST could precede durable intent")
		}
		if observed.AttemptedRunID != 101 {
			t.Error("durable intent lost the selected run")
		}
		return notificationSendResult{Outcome: notificationSendAmbiguous}, errors.New("lost acknowledgement")
	})
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err == nil {
		t.Error("ambiguous send succeeded")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = newFileNotificationStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err == nil {
		t.Error("restart retried an ambiguous send")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Errorf("sends = %d, want exactly one", sends)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("store retained %d files, want receipt and lock", len(entries))
	}
}

// TestNotificationFileStoreRefusesCorruptionAndConcurrentOwnership proves
// malformed state and an already-held scope cannot authorize another sender.
func TestNotificationFileStoreRefusesCorruptionAndConcurrentOwnership(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	first, err := newFileNotificationStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := first.Close(); err != nil {
			t.Error(err)
		}
	})
	second, err := newFileNotificationStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := second.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := first.withLock(t.Context(), func(notificationLedger) error {
		called := false
		err := second.withLock(t.Context(), func(notificationLedger) error { called = true; return nil })
		if err == nil {
			t.Error("another descriptor acquired the occupied scope")
		}
		if called {
			t.Error("the losing lock caller entered the ledger")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, notificationStateFile), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := first.initialize(t.Context(), receipt); err == nil {
		t.Error("initialization repaired corrupt state")
	}
	called := false
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		called = true
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	if _, err := deliverNotification(t.Context(), first, preview, approval, sender); err == nil {
		t.Error("corrupt state authorized delivery")
	}
	if called {
		t.Error("sender ran with corrupt state")
	}
	if _, err := newFileNotificationStore(filepath.Join(directory, "missing")); err == nil {
		t.Error("missing private state directory was created automatically")
	}
}
