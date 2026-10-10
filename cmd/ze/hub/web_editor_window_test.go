//go:build ze_web

// Tests that the web's editors carry the daemon's confirmed-commit window.
//
// VALIDATES: spec-session-editor-file-mode-parity review round 3 ISSUE 1: a web
// user's `rollback <N>` during another user's window is refused with the
// window's refusal and writes nothing, because every editor newEditorFactory
// builds reads daemonConfirmWindow.
// METHOD: the production wiring of startWebServer (blob store, the real editor
// factory, EditorManager), a backup from a first commit, an open window in
// daemonConfirmWindow, then EditorManager.Rollback as verbRollback runs it.
package hub

import (
	"bytes"
	"errors"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

type discardRecorder struct{}

func (discardRecorder) Save(confirm.Pending) error { return nil }
func (discardRecorder) Clear() error               { return nil }

func TestWebRollbackRefusedDuringWindow(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("blob storage: %v", err)
	}
	configPath := "config.conf"
	if err := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	schema, err := zeconfig.YANGSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	mgr := zeweb.NewEditorManager(store, configPath, schema,
		newEditorFactory(zeconfigcmd.ValidateContent), newEditSessionFactory())

	// Two commits leave a backup of the first config to roll back to.
	for _, host := range []string{"first-host", "second-host"} {
		if err := mgr.SetValue("bob", []string{"system"}, "host", host); err != nil {
			t.Fatalf("set value: %v", err)
		}
		if _, err := mgr.CommitNow("bob"); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	backups, err := mgr.ListBackups("bob")
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) == 0 {
		t.Fatal("no backup to roll back to")
	}
	before, err := store.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	window := confirm.NewWindow(func([]byte) error { return nil }, discardRecorder{})
	t.Cleanup(window.Stop)
	commit := confirm.Commit{
		Snapshot: func() ([]byte, error) { return before, nil },
		Apply:    func() error { return nil },
	}
	if err := window.Confirmed("alice", time.Minute, false, commit); err != nil {
		t.Fatalf("open window: %v", err)
	}
	daemonConfirmWindow.Store(window)
	t.Cleanup(func() { daemonConfirmWindow.CompareAndSwap(window, nil) })

	err = mgr.Rollback("bob", backups[0].Path)
	var other *confirm.OtherUserError
	if !errors.As(err, &other) {
		t.Fatalf("web rollback during alice's window = %v; want the window's refusal", err)
	}
	if other.Owner != "alice" {
		t.Fatalf("refusal names %q; want alice", other.Owner)
	}
	after, err := store.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("a refused rollback wrote the config:\nbefore %q\nafter  %q", before, after)
	}
}
