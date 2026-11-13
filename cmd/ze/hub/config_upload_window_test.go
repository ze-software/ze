// Design: docs/architecture/appliance/remote-operations.md -- the daemon owns upload and confirmation lifetimes.
// Related: confirm_window.go -- stopped windows retain their rollback record.

package hub

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

const uploadWindowBefore = "set environment log level info\nset system dns resolv-conf-path \"\"\n"
const uploadWindowConfirmed = "set environment log level warn\nset system dns resolv-conf-path \"\"\n"
const uploadWindowAfter = "set environment log level error\nset system dns resolv-conf-path \"\"\n"

// uploadWindowFixture uses the actual mutation owner, window worker and
// canonical source publication. Its reload seam promotes the admitted candidate;
// the SSH receiver tests separately prove live daemon application.
type uploadWindowFixture struct {
	owner   *configMutationOwner
	window  *confirm.Window
	store   storage.Storage
	path    string
	reloads int
}

func newUploadWindowFixture(t *testing.T) *uploadWindowFixture {
	t.Helper()
	previous := daemonConfirmWindow.Swap(nil)
	t.Cleanup(func() { daemonConfirmWindow.Store(previous) })
	f := &uploadWindowFixture{owner: &configMutationOwner{}, path: "router.conf"}
	f.store = storage.BindConfigSource(storage.CreateMemory(), f.path, storage.ConfigSourceStored)
	if err := initializeConfigSource(f.store, f.path, []byte(uploadWindowBefore)); err != nil {
		t.Fatal(err)
	}
	f.window = startConfirmWindow(f.store, f.path, func() error {
		return f.owner.Reload(context.Background())
	})
	t.Cleanup(func() {
		stopConfirmWindow(f.window)
		f.owner.close()
		if err := f.store.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := f.owner.bind(f.store, f.path, f.window, func(context.Context) error {
		f.reloads++
		return promoteConfigCandidate(f.store, f.path)
	}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *uploadWindowFixture) open(t *testing.T) {
	t.Helper()
	if err := f.window.Confirmed("alice", time.Hour, false, confirm.Commit{
		Snapshot: func() ([]byte, error) {
			return storage.ReadConfigSource(f.store, f.path)
		},
		Apply: func() error {
			return writeRuntimeConfig(f.store, f.path, f.path, []byte(uploadWindowBefore), []byte(uploadWindowConfirmed), func() error {
				return f.owner.Reload(context.Background())
			})
		},
	}); err != nil {
		t.Fatal(err)
	}
	if f.reloads != 1 {
		t.Fatalf("confirmed commit reloads = %d, want 1", f.reloads)
	}
	f.requireSource(t, uploadWindowConfirmed)
}

func (f *uploadWindowFixture) requireSource(t *testing.T, want string) {
	t.Helper()
	for name, read := range map[string]func(storage.Storage, string) ([]byte, error){
		"selected": storage.ReadConfigSource,
		"active":   storage.ReadActiveConfig,
	} {
		content, err := read(f.store, f.path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != want {
			t.Errorf("%s source = %q, want %q", name, content, want)
		}
	}
}

// TestConfigUploadRetainsStoppedConfirmWindow exercises the actual interval
// after stopConfirmWindow returns and before the mutation owner's gate closes.
// Stop is the worker-join barrier; there is no scheduler delay or sleep. A
// pending rollback remains durable, so neither upload nor source commit may
// publish a change that boot recovery would silently discard.
func TestConfigUploadRetainsStoppedConfirmWindow(t *testing.T) {
	for _, operation := range []string{"upload", "source-commit"} {
		t.Run(operation, func(t *testing.T) {
			f := newUploadWindowFixture(t)
			f.open(t)
			stopConfirmWindow(f.window)
			if daemonConfirmWindow.Load() != nil {
				t.Fatal("stop did not withdraw the public window")
			}
			if _, err := f.window.Timeouts(); !errors.Is(err, confirm.ErrStopped) {
				t.Fatalf("window worker was not joined: %v", err)
			}
			var err error
			if operation == "upload" {
				err = f.owner.Upload(context.Background(), []byte(uploadWindowAfter))
			} else {
				err = f.owner.Commit(context.Background(), []byte(uploadWindowConfirmed), []byte(uploadWindowAfter))
			}
			if err == nil {
				t.Error("configuration write bypassed the stopped confirmed-commit window")
			}
			if f.reloads != 1 {
				t.Errorf("refused write reloaded %d times, want only the original confirmed commit", f.reloads)
			}
			f.requireSource(t, uploadWindowConfirmed)
			if _, _, present, err := storage.ReadCandidateConfig(f.store, f.path); err != nil {
				t.Fatal(err)
			} else if present {
				t.Error("refused write left a candidate")
			}
			record, err := confirm.NewStoreRecorder(f.store, f.path).Load()
			if err != nil {
				t.Fatal(err)
			}
			if record == nil {
				t.Fatal("shutdown lost the pending rollback record")
			}
			if record.User != "alice" {
				t.Errorf("pending record owner = %q, want alice", record.User)
			}
			if !bytes.Equal(record.Rollback, []byte(uploadWindowBefore)) {
				t.Error("refused write changed the original rollback")
			}
		})
	}
}

// TestConfigUploadWithoutPendingWindowApplies prevents blanket refusal from
// satisfying the stopped-window regression.
func TestConfigUploadWithoutPendingWindowApplies(t *testing.T) {
	f := newUploadWindowFixture(t)
	if err := f.owner.Upload(context.Background(), []byte(uploadWindowAfter)); err != nil {
		t.Fatal(err)
	}
	if f.reloads != 1 {
		t.Fatalf("accepted upload reloads = %d, want 1", f.reloads)
	}
	f.requireSource(t, uploadWindowAfter)
	record, err := confirm.NewStoreRecorder(f.store, f.path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if record != nil {
		t.Fatal("ordinary upload opened a confirmed-commit window")
	}
}

// TestConfigUploadWindowRevertCanReload proves that retaining write exclusion
// does not close the reload gate needed by a confirmed-commit revert.
func TestConfigUploadWindowRevertCanReload(t *testing.T) {
	f := newUploadWindowFixture(t)
	f.open(t)
	if err := f.window.Abort("alice"); err != nil {
		t.Fatal(err)
	}
	if f.reloads != 2 {
		t.Fatalf("confirmed commit and revert reloads = %d, want 2", f.reloads)
	}
	f.requireSource(t, uploadWindowBefore)
	record, err := confirm.NewStoreRecorder(f.store, f.path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if record != nil {
		t.Fatal("successful revert retained the pending rollback record")
	}
}
