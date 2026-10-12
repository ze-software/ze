// VALIDATES: a verify worktree's gokrazy module cache is filled from the
// source checkout's by links, the tracked files of the commit under test are
// never replaced, and a source with no cache is reported rather than silent.
// PREVENTS: every appliance build in a verify worktree resolving its modules
// offline from an empty cache (plan/journal/gate-verdict-depends-on-the-machine.md,
// 2026-10-12).

package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModcacheFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The source holds a tracked gokrazy tree plus ignored modules beside it and
// under it; the worktree holds only the tracked files. Each ignored entry must
// arrive as a link, and each tracked file must keep the worktree's content.
func TestAWorktreeLinksTheCheckoutGokrazyModcache(t *testing.T) {
	root := t.TempDir()
	path := t.TempDir()
	source := filepath.Join(root, modcacheDir)
	target := filepath.Join(path, modcacheDir)
	gokrazy := filepath.Join("github.com", "gokrazy", "gokrazy@v1")

	writeModcacheFile(t, filepath.Join(source, ".gitignore"), "source")
	writeModcacheFile(t, filepath.Join(source, gokrazy, "main.go"), "source")
	writeModcacheFile(t, filepath.Join(source, gokrazy, "docs", "a.md"), "ignored")
	writeModcacheFile(t, filepath.Join(source, "github.com", "rtr7", "kernel@v1", "go.mod"), "ignored")
	writeModcacheFile(t, filepath.Join(source, "cache", "download", "x.zip"), "ignored")
	writeModcacheFile(t, filepath.Join(target, ".gitignore"), "tracked")
	writeModcacheFile(t, filepath.Join(target, gokrazy, "main.go"), "tracked")

	line := sharedModcacheLink(root, path)
	if !strings.Contains(line, "linked 3 entries from "+source) {
		t.Fatalf("line = %q, want three links from %s", line, source)
	}
	for _, linked := range []string{
		"cache",
		filepath.Join("github.com", "rtr7"),
		filepath.Join(gokrazy, "docs"),
	} {
		info, err := os.Lstat(filepath.Join(target, linked))
		if err != nil {
			t.Fatalf("%s: %v", linked, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is %v, want a symlink", linked, info.Mode())
		}
	}
	for _, tracked := range []string{".gitignore", filepath.Join(gokrazy, "main.go")} {
		content, err := os.ReadFile(filepath.Join(target, tracked))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "tracked" {
			t.Fatalf("%s = %q, want the worktree's tracked content", tracked, content)
		}
	}
	content, err := os.ReadFile(filepath.Join(target, "cache", "download", "x.zip"))
	if err != nil {
		t.Fatalf("a module read through the link: %v", err)
	}
	if string(content) != "ignored" {
		t.Fatalf("x.zip = %q, want the checkout's content", content)
	}
}

// A checkout with no gokrazy cache cannot fill the worktree's. The line names
// the failure and the command that populates the cache in that checkout.
func TestAWorktreeReportsAnUnlinkableGokrazyModcache(t *testing.T) {
	root := t.TempDir()
	line := sharedModcacheLink(root, t.TempDir())
	if !strings.Contains(line, "no such file or directory") {
		t.Fatalf("line = %q, want the read failure", line)
	}
	if !strings.Contains(line, "run `./le setup install` in "+root) {
		t.Fatalf("line = %q, want the fix naming the checkout", line)
	}
}
