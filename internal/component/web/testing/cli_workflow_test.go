// Design: docs/architecture/testing/runner-architecture.md -- explicit browser workflows.
package webtesting

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: default key actions still wait, while an explicitly targeted
// overlapping action returns after the real browser key event.
func TestPressExplicitOverlapPreservesDefaultWait(t *testing.T) {
	log := installFakeAgentBrowser(t)
	b := newBrowser("https://127.0.0.1:1234")
	if err := b.Open("/cli"); err != nil {
		t.Fatal(err)
	}
	for _, overlap := range []bool{false, true} {
		values := map[string]string{"id": "terminal-input", "key": "Enter"}
		if overlap {
			values["wait"] = "none"
		}
		before := len(readAgentLog(t, log))
		if err := executeAction(b, &WBAction{Kind: "press", Values: values}); err != nil {
			t.Fatal(err)
		}
		want := []string{"focus #terminal-input", "press Enter"}
		if !overlap {
			want = append(want, "eval "+inflightIdleExpr)
		}
		if got := readAgentLog(t, log)[before:]; !slices.Equal(got, want) {
			t.Fatalf("overlap=%v commands %q, want %q", overlap, got, want)
		}
	}
	for _, values := range []map[string]string{
		{"key": "Enter", "wait": "none"},
		{"id": "terminal-input", "key": "Enter", "wait": "later"},
	} {
		if err := executeAction(nil, &WBAction{Kind: "press", Values: values}); err == nil {
			t.Fatal("invalid wait accepted")
		}
	}
}

// VALIDATES: upload reaches the browser with an existing regular file; missing
// inputs are refused before a browser action can silently succeed.
func TestUploadSelectsActualRegularFile(t *testing.T) {
	log := installFakeAgentBrowser(t)
	b := newBrowser("https://127.0.0.1:1234")
	if err := b.Open("/cli"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "candidate.conf")
	if err := os.WriteFile(file, []byte("set bgp router-id 192.0.2.9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := len(readAgentLog(t, log))
	if err := executeAction(b, &WBAction{Kind: "upload", Values: map[string]string{"id": "cli-load-file", "file": file}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"upload #cli-load-file " + file, "eval " + inflightIdleExpr}
	if got := readAgentLog(t, log)[before:]; !slices.Equal(got, want) {
		t.Fatalf("commands %q, want %q", got, want)
	}
	for _, path := range []string{"", file + "-missing", filepath.Dir(file)} {
		if err := b.uploadID("cli-load-file", path); err == nil {
			t.Fatalf("invalid upload %q accepted", path)
		}
	}
	if err := b.uploadID("", file); err == nil {
		t.Fatal("upload without target accepted")
	}
}

// VALIDATES: every authored element condition is understood. A misspelled or
// unsupported qualifier must fail even when another qualifier would pass.
func TestElementExpectationRefusesUnsupportedQualifiers(t *testing.T) {
	for _, values := range []map[string]string{
		{}, {"id": ""}, {"class": "visible"}, {"not-class": "hidden"},
		{"id": "terminal-input", "cl" + "as": "visible"},
		{"id": "terminal-input", "aria-label": "Command"},
	} {
		if err := checkElement(nil, &WBExpectation{Values: values}); err == nil || !strings.Contains(err.Error(), "element expectation") {
			t.Fatalf("invalid qualifiers %v: %v", values, err)
		}
	}
}
