// Design: docs/contributing/running-commands.md -- session password admission.
package session

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

// TestSeedPasswordRejectsHardlink preserves an outside inode's bytes and mode
// when the existing password name is a hard link to it.
func TestSeedPasswordRejectsHardlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "sentinel")
	content := []byte("outside-password\n")
	if err := os.WriteFile(outside, content, 0o640); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	if err := os.Mkdir(config, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(config, ".dev-password")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Test directory.
	password, err := ensurePassword(root, rand.Reader)
	if err == nil {
		t.Errorf("accepted hard-linked password %q", password)
	}
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("outside bytes changed: %q", got)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("outside mode changed: %v", info.Mode())
	}
}
