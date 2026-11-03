// Design: docs/architecture/zefs-format.md -- acquired artifact descriptors.
package zefs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAcquiredArtifactSurvivesNameReplacement checks and loads the acquired
// inode after an unreadable replacement takes its former pathname.
func TestAcquiredArtifactSurvivesNameReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	seed, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.WriteFile("key", []byte("accepted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Test root.
	file, err := root.Open("blob")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("blob", "held"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unaccepted replacement"), 0o640); err != nil {
		t.Fatal(err)
	}
	report, err := CheckFile(file)
	if err != nil {
		file.Close() //nolint:errcheck // Failure cleanup.
		t.Fatal(err)
	}
	if !report.ContainerOK {
		t.Error("check consumed replacement")
	}
	store, err := OpenFileIn(root, "blob", file)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "accepted" {
		t.Errorf("loaded %q", got)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); err == nil {
		t.Error("transferred descriptor remains open")
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes) != "unaccepted replacement" {
		t.Errorf("replacement changed: %q", bytes)
	}
}

// TestAcquiredArtifactLimit rejects an oversized sparse file before mapping
// and closes transferred ownership on failure.
func TestAcquiredArtifactLimit(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Test root.
	file, err := root.OpenFile("large", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxImportSize + 1); err != nil {
		t.Fatal(err)
	}
	if store, err := OpenFileIn(root, "large", file); err == nil {
		store.Close() //nolint:errcheck // Wrongly admitted store.
		t.Fatal("accepted oversized artifact")
	}
	if _, err := file.Stat(); err == nil {
		t.Error("failed transfer leaked descriptor")
	}
}
