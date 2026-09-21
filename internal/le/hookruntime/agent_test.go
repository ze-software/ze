package hookruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewAgentHasNoModelRestriction(t *testing.T) {
	// VALIDATES: the agent entry point accepts review on any model or without model metadata.
	// PREVENTS: reinstating a model requirement through the dispatcher.
	root := t.TempDir()
	transcript := filepath.Join(root, "session.jsonl")
	for _, model := range []string{"claude-sonnet-4", "gpt-6-astra", ""} {
		t.Run(model, func(t *testing.T) {
			body := "{\"message\":{\"model\":\"" + model + "\"}}\n"
			if err := os.WriteFile(transcript, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			code, _, message := runHook(t, root, "pretool-agent-skill", map[string]any{
				"tool_name": "Agent", "transcript_path": transcript,
				"tool_input": map[string]any{"prompt": "/ze-review"},
			})
			if code != 0 || message != "" {
				t.Fatalf("review spawn: code=%d message=%q", code, message)
			}
		})
	}
}
