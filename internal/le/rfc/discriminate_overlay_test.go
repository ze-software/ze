// Design: docs/architecture/core-design.md -- observing the red a record rests on
package rfc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: two observations of producers that share a basename, run at once
// from one session scratch, each stage their own replacement and their own
// overlay, and each overlay still selects its own break.
// PREVENTS: the IKE break compiling against the LDP agent's broken-register.go
// because the replacement was keyed on the basename alone
// (concurrent-session-corruption journal, 2026-09-28).
func TestWriteOverlayStagesEachRunApart(t *testing.T) {
	tree := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "overlay-sid")
	first, err := writeOverlay(tree, overlayFile{rel: "internal/a/register.go", content: "package a\n"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeOverlay(tree, overlayFile{rel: "internal/b/register.go", content: "package b\n"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("both runs wrote overlay %s", first)
	}
	for overlay, want := range map[string]string{first: "package a\n", second: "package b\n"} {
		raw, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct{ Replace map[string]string }
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		for _, replacement := range parsed.Replace {
			got, err := os.ReadFile(replacement)
			if err != nil || string(got) != want {
				t.Fatalf("overlay %s selects %s holding %q, %v; want %q", filepath.Base(overlay), replacement, got, err, want)
			}
		}
	}
}
