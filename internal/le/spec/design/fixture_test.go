// Fixture helpers the `le spec design` wiring tests share. Every test runs
// against a throwaway checkout named by ZE_REPO_ROOT, so no record lands in
// the real tmp/spec-design/.
package specdesign_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// The widget fixture: one code file, the design page its header names, the
// record the page's decision row cites, the system door, and a spec in the
// fixes category whose Files to Modify names the code file.
const (
	widgetCode     = "internal/component/widget/widget.go"
	widgetPage     = "docs/architecture/widget.md"
	widgetRecord   = "plan/records/widget.md"
	widgetDoor     = "ai/doors/system.md"
	widgetSpec     = "plan/release/fixes/spec-widget.md"
	widgetDecision = "The widget answers data and the surface renders it"
)

// widgetPageText carries one decision row inside its Design decisions section.
const widgetPageText = "# Widget\n\nHow the widget answers.\n\n## Design decisions\n\n" +
	"| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
	"| " + widgetDecision + " | Rendering text in the widget | " + widgetRecord + " | 2026-08-21 |\n" +
	"\n## Wire format\n\nThe answer is JSON.\n"

// widgetSpecText is a design spec whose Design documents table gives the page
// and its decision row a verdict.
const widgetSpecText = "# Spec: widget\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n" +
	"| Priority | P2 |\n| Updated | 2026-11-19 |\n\n## Task\n\nRender the widget in the surface.\n\n" +
	"## Files to Modify\n- `" + widgetCode + "` - render in the surface\n\n" +
	"## Design documents\n\n| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |\n" +
	"|---|---|---|---|---|---|\n" +
	"| `" + widgetPage + "` | Design decisions | Design header of `" + widgetCode + "` | \"" + widgetDecision + "\" | agrees | The surface renders |\n"

// useRoot points le's checkout resolution at root for the test's duration.
func useRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// writeTree writes every file of files under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// widgetRoot lays out the widget fixture in a fresh checkout and selects it.
func widgetRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		widgetCode:   "// Design: " + widgetPage + " -- the widget\n\npackage widget\n",
		widgetPage:   widgetPageText,
		widgetRecord: "# Widget record\n",
		widgetDoor:   "# System door\n\n<!-- door-base: pkg cmd/ze internal -->\n",
		widgetSpec:   widgetSpecText,
	})
	useRoot(t, root)
	return root
}

// captureStderr runs fn with os.Stderr replaced by a file and answers what fn
// wrote there.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("create the capture file: %v", err)
	}
	saved := os.Stderr
	os.Stderr = file
	fn()
	os.Stderr = saved
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read the capture file: %v", err)
	}
	return string(data)
}

// answerText is the answer as the JSON renderer would publish it, so a test
// asserts on what a caller reads rather than on a Go type a later phase
// chooses.
func answerText(t *testing.T, answer any) string {
	t.Helper()
	data, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("the answer does not render as JSON: %v", err)
	}
	return string(data)
}

// requireContains fails the test for each want that text does not hold.
func requireContains(t *testing.T, what, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not name %q:\n%s", what, want, text)
		}
	}
}

// requireAbsent fails the test when path exists under root.
func requireAbsent(t *testing.T, root, path string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
		t.Errorf("%s was written", path)
	}
}
