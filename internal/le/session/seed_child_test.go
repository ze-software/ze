// Design: docs/contributing/running-commands.md -- session seed child-storage confinement.
package session

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// TestSeedNativeChildStorageReplacement drives a real ze init child after the
// config directory becomes a different regular directory. Supply the native ze
// artifact after -args --; its bytes are hard-linked, never rebuilt by this test.
// Refusal must precede any child write in the replacement directory.
func TestSeedNativeChildStorageReplacement(t *testing.T) {
	args := flag.Args()
	if len(args) != 1 {
		t.Skip("supply one native Ze artifact with -args -- <absolute-ze-path>")
	}
	if !filepath.IsAbs(args[0]) {
		t.Fatal("native Ze artifact path must be absolute")
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "seed-child-owner")
	root := t.TempDir()
	rel := filepath.Join("tmp", "session", "2026-11-03-160000")
	dir := filepath.Join(root, rel)
	config := filepath.Join(dir, "etc", "ze")
	binary := filepath.Join(rel, "bin", "ze")
	for _, path := range []string{config, filepath.Join(dir, "bin")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".session-id"), []byte("seed-child-owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(args[0], filepath.Join(root, binary)); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	report, code, err := seedStoreAfterEntry(root, binary, streams{Out: &output, Err: &output}, func() {
		if err := os.Rename(config, config+".accepted"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(config, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config, "sentinel"), []byte("foreign directory\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("native seed exit=%d child=%d seeded=%v error=%v output=%s", code, report.ChildCode, report.Seeded, err, output.String())
	entries, err := os.ReadDir(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "sentinel" {
			t.Errorf("native child wrote foreign entry %s", entry.Name())
		}
	}
	content, err := os.ReadFile(filepath.Join(config, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "foreign directory\n" {
		t.Errorf("foreign sentinel bytes changed: %q", content)
	}
	info, err := os.Stat(filepath.Join(config, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("foreign sentinel mode changed: %v", info.Mode())
	}
}
