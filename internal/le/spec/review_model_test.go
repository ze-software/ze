package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewCommandsAcceptAnyModel(t *testing.T) {
	// VALIDATES: recording and checking reviews do not depend on a model family.
	// PREVENTS: review evidence being refused because of informational metadata.
	root := reviewFixture(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "session")
	dir := transcriptDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"claude-sonnet-4", "gpt-6-astra", ""} {
		t.Run(model, func(t *testing.T) {
			body := "{\"message\":{\"model\":\"" + model + "\"}}\n"
			if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			payload, code := answerReview(root, "session", []string{
				"record", "spec", "demo", "verdict", "clean", "rounds", "1", "file", "pkg/a.go",
			})
			if code != 0 {
				t.Fatalf("record = (%#v, %d)", payload, code)
			}
			artifact, ok := payload.(reviewArtifact)
			if !ok || artifact.Model != model {
				t.Fatalf("model metadata = %#v, want %q", payload, model)
			}
			payload, code = answerReview(root, "session", []string{
				"check", "spec", "demo", "file", "pkg/a.go",
			})
			if code != 0 {
				t.Fatalf("check = (%#v, %d)", payload, code)
			}
		})
	}
}
