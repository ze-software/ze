// Design: docs/guide/config-editor.md — source authority and competing editors
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// A stored mirror must not hide the explicit file, and an external edit must
// survive a competing loose-file commit unchanged.
func TestLooseEditorSourceAuthority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "router.conf")
	original := "bgp { router-id 192.0.2.1; }\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	if err := store.WriteFile(path, []byte("bgp { router-id 192.0.2.99; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := NewLooseFileEditor(store, path)
	if err != nil {
		t.Fatal(err)
	}
	if ed.OriginalContent() != original {
		t.Fatalf("explicit file hidden by stored mirror: %q", ed.OriginalContent())
	}
	if err := ed.SetValue([]string{"bgp"}, "router-id", "192.0.2.2"); err != nil {
		t.Fatal(err)
	}
	external := "bgp { router-id 192.0.2.3; }\n"
	if err := os.WriteFile(path, []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ed.Save(); err == nil {
		t.Fatal("competing external edit was overwritten")
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != external {
		t.Fatalf("external config lost: %s", actual)
	}
	versions, err := store.ListVersions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 0 {
		t.Fatal("refused commit recorded a version")
	}
}

// The daemon publication callback must finish before an editor clears its draft
// and must remain the only writer: a refusal leaves the committed store intact.
func TestEditorCommitPublisherRefusal(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	original := "bgp { router-id 192.0.2.1; }\n"
	if err := store.WriteFile("router.conf", []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := NewEditorWithStorage(store, "router.conf")
	if err != nil {
		t.Fatal(err)
	}
	ed.SetCommitWriter(func(expected, content []byte) error {
		if string(expected) != original {
			t.Errorf("wrong source baseline: %s", expected)
		}
		return os.ErrPermission
	})
	if err := ed.SetValue([]string{"bgp"}, "router-id", "192.0.2.2"); err != nil {
		t.Fatal(err)
	}
	if _, err := ed.Save(); err == nil {
		t.Fatal("refused publication reported success")
	}
	actual, err := store.ReadFile("router.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != original {
		t.Fatalf("publisher bypassed: %s", actual)
	}
	if !ed.Dirty() || !strings.Contains(ed.WorkingContent(), "192.0.2.2") {
		t.Fatal("refused publication lost pending edit")
	}
}
