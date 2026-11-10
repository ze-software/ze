// Design: docs/contributing/spec-workflow.md -- shared checkout recovery evidence
// VALIDATES: the real summary entry names foreign edits without claiming session attribution.
// PREVENTS: a session snapshot silently presenting the whole checkout as its own work.
package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestSessionStateLabelsSharedCheckout drives EndSummary over real committed
// originals changed by two declared writers. With no attribution mechanism,
// both paths remain visible under an explicit shared-checkout label.
func TestSessionStateLabelsSharedCheckout(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"own.go", "foreign.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package original\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "own.go", "foreign.go"},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "committed originals"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v\n%s", args, err, output)
		}
	}
	for _, edit := range []struct{ path, writer string }{
		{"own.go", "this_session"},
		{"foreign.go", "another_session"},
	} {
		if err := os.WriteFile(filepath.Join(root, edit.path), []byte("package "+edit.writer+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := lepath.SessionPaths{ID: "attribution-proof", Dir: "tmp/session/2026-11-04-attribution-proof"}
	report, err := EndSummary(root, paths, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !report.Written {
		t.Fatal("real dirty checkout produced no summary")
	}
	body, err := os.ReadFile(filepath.Join(root, report.StateFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "Shared-checkout changes (session attribution unavailable):") {
		t.Errorf("whole-tree edits lack their attribution limit:\n%s", text)
	}
	for _, path := range []string{"own.go", "foreign.go"} {
		if !strings.Contains(text, "- `"+path+"`") {
			t.Errorf("real changed path %s missing:\n%s", path, text)
		}
	}
	for minute := 1; minute <= 3; minute++ {
		if _, err := EndSummary(root, paths, time.Date(2026, 11, 4, 0, minute, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	body, err = os.ReadFile(filepath.Join(root, report.StateFile))
	if err != nil {
		t.Fatal(err)
	}
	text = string(body)
	if got := strings.Count(text, "## Session:"); got != 3 {
		t.Errorf("rotation retained %d snapshots, want 3:\n%s", got, text)
	}
	if got := strings.Count(text, "Shared-checkout changes (session attribution unavailable):"); got != 3 {
		t.Errorf("rotation retained %d attribution labels, want one per retained snapshot:\n%s", got, text)
	}
	for _, path := range []string{"own.go", "foreign.go"} {
		if got := strings.Count(text, "- `"+path+"`"); got != 3 {
			t.Errorf("rotation retained %d occurrences of %s, want one per retained snapshot:\n%s", got, path, text)
		}
	}
}
