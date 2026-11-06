// Design: website/AI.md -- artifact seeding preserves each valid destination until replacement.
package site

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArtifactSeedFailureKeepsPreviousPublication drives the seeding entry with
// an escaping source link and observes the old target after copying is refused.
func TestArtifactSeedFailureKeepsPreviousPublication(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("foreign bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "page.html")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "page.html"), []byte("valid previous page"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := seedArtifact(Paths{Output: target}, source); err == nil {
		t.Fatal("seeding accepted an escaping source link")
	}
	content, err := os.ReadFile(filepath.Join(target, "page.html"))
	if err != nil {
		t.Fatalf("failed seed removed the valid destination: %v", err)
	}
	if string(content) != "valid previous page" {
		t.Fatalf("failed seed changed valid destination: %q", content)
	}
}

// TestArtifactSeedPrunesOnlyAfterSuccessfulOverlay checks a successful seed
// replaces file content, removes stale trees and source-only files, preserves
// Git metadata, and unlinks a stale symlink without touching its foreign target.
func TestArtifactSeedPrunesOnlyAfterSuccessfulOverlay(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	outside := t.TempDir()
	for _, path := range []string{
		filepath.Join(source, "assets"),
		filepath.Join(target, "assets"),
		filepath.Join(target, "obsolete"),
		filepath.Join(target, ".git"),
	} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(source, "assets", "kept.txt"): "new seeded bytes",
		filepath.Join(source, "CACHEDIR.TAG"):       "source-only",
		filepath.Join(target, "assets", "kept.txt"): "old seeded bytes",
		filepath.Join(target, "obsolete", "page"):   "obsolete",
		filepath.Join(target, "CACHEDIR.TAG"):       "source-only",
		filepath.Join(target, ".git", "config"):     "preserved metadata",
		filepath.Join(outside, "sentinel"):          "foreign bytes",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(target, "stale-link")); err != nil {
		t.Fatal(err)
	}
	if err := seedArtifact(Paths{Output: target}, source); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(target, "assets", "kept.txt"): "new seeded bytes",
		filepath.Join(target, ".git", "config"):     "preserved metadata",
		filepath.Join(outside, "sentinel"):          "foreign bytes",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != want {
			t.Errorf("%s: got %q, want %q", path, content, want)
		}
	}
	for _, name := range []string{"obsolete", "CACHEDIR.TAG", "stale-link"} {
		if _, err := os.Lstat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Errorf("seed retained stale %s: %v", name, err)
		}
	}
}
