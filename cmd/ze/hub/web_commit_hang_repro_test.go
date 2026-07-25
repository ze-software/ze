//go:build ze_web

package hub

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// TestWebCommitHangRepro is the regression test for the web-only commit hang:
// POST /config/commit used to never return and wedged every subsequent request.
// Mirrors the production wiring in startWebServer: blob storage, the real
// editor factory with ValidateContent, no commit hook.
//
// VALIDATES: web-only commit completes, releases the store lock, and persists
// the committed value to the blob store (readable after the guard flushes).
// PREVENTS: CommitSession self-deadlock (spec-web-ui-integrity.md F1) freezing
// the whole web UI and silently losing the committed config (F2).
func TestWebCommitHangRepro(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
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

	if err := mgr.SetValue("insecure", []string{"system"}, "host", "audit-host"); err != nil {
		t.Fatalf("set value: %v", err)
	}

	// Run Commit directly; the go test -timeout flag produces a full
	// goroutine dump if this ever hangs again, pinpointing the deadlock.
	start := time.Now()
	if _, err := mgr.Commit("insecure"); err != nil {
		t.Fatalf("commit returned error after %v: %v", time.Since(start), err)
	}

	// F2: the committed value must be flushed to the blob store (the deadlock
	// previously prevented WriteLock.Release, so the write never reached disk).
	// A fresh read goes through the store's own lock, so it would itself hang
	// if the commit had not released the guard.
	data, err := store.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read committed config: %v", err)
	}
	if !strings.Contains(string(data), "audit-host") {
		t.Fatalf("committed value not persisted; config.conf = %q", string(data))
	}

	// The server stays responsive: a second commit on a fresh change must also
	// complete (the post-deadlock state used to wedge every later request).
	if err := mgr.SetValue("insecure", []string{"system"}, "host", "audit-host-2"); err != nil {
		t.Fatalf("set value 2: %v", err)
	}
	if _, err := mgr.Commit("insecure"); err != nil {
		t.Fatalf("second commit failed: %v", err)
	}
}
