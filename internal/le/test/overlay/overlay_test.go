// Design: docs/contributing/rfc-conformance-gates.md -- every carrier applies the break through a Go overlay
package overlay

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: A-4 -- an overlay can delete a file the tree holds and HEAD
// lacks: a File marked Absent maps its path to the empty backing path, writes
// no replacement, and the Go build then sees the package without that file.
// METHOD: Write over a two-file module with one file Absent and the other
// replaced, read the overlay JSON, then ask `go list` under that overlay which
// files the package compiles.
// PREVENTS: attribution of a session that added a file answering
// `undetermined`, because the HEAD build could not drop the added file.
func TestOverlayDeletesAFileHeadLacks(t *testing.T) {
	tree := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "overlay-absent-sid")
	sources := map[string]string{
		"go.mod":   "module example.com/absent\n\ngo 1.21\n",
		"kept.go":  "package absent\n\nfunc Kept() int { return 1 }\n",
		"added.go": "package absent\n\nfunc Added() int { return 2 }\n",
	}
	for name, content := range sources {
		if err := os.WriteFile(filepath.Join(tree, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const head = "package absent\n\nfunc Kept() int { return 3 }\n"
	overlay, err := Write(tree, "overlay-test",
		File{Rel: "kept.go", Content: head},
		File{Rel: "added.go", Absent: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct{ Replace map[string]string }
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	backing, held := parsed.Replace[filepath.Join(tree, "added.go")]
	if !held || backing != "" {
		t.Fatalf("added.go maps to %q (held %v), want the empty backing path that deletes it: %v",
			backing, held, parsed.Replace)
	}
	replacement := parsed.Replace[filepath.Join(tree, "kept.go")]
	if got, err := os.ReadFile(replacement); err != nil || string(got) != head {
		t.Fatalf("kept.go is replaced by %q, %v; want %q", got, err, head)
	}
	entries, err := os.ReadDir(filepath.Dir(overlay))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("the run directory holds %d files, want the overlay and one replacement", len(entries))
	}

	golang, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("no go binary on PATH to compile under the overlay: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), golang, "list", "-overlay="+overlay,
		"-f", "{{join .GoFiles \" \"}}", ".")
	cmd.Dir = tree
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off", "GO111MODULE=on")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list under the overlay: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "kept.go" {
		t.Fatalf("the package compiles %q under the overlay, want only kept.go", got)
	}
}

// VALIDATES: two runs whose files share a basename, written at once from one
// session scratch, each stage their own replacement and their own overlay, and
// each overlay still selects its own content.
// PREVENTS: one break compiling against another's broken-register.go because
// the replacement was keyed on the basename alone
// (concurrent-session-corruption journal, 2026-09-28).
func TestWriteStagesEachRunApart(t *testing.T) {
	tree := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "overlay-apart-sid")
	first, err := Write(tree, "overlay-test", File{Rel: "internal/a/register.go", Content: "package a\n"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(tree, "overlay-test", File{Rel: "internal/b/register.go", Content: "package b\n"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(first) == filepath.Dir(second) {
		t.Fatalf("both runs staged in %s", filepath.Dir(first))
	}
}
