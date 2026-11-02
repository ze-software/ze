// Design: docs/architecture/core-design.md -- native Claude lifecycle hooks
package hookruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the bash-timestamp hook prints one "▶ HH:MM:SS" system message,
// stays quiet inside the gap, prints again once the gap has passed, honors
// CLAUDE_BASH_STAMP_QUIET, and never blocks.
// PREVENTS: a timestamp on every Bash call flooding the transcript, and a
// display hook that refuses a command.
func TestBashTimestampPrintsOncePerQuietGap(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{"session_id": "stamp-probe", "tool_name": "Bash"}
	marker := filepath.Join(root, "tmp", "session", ".bash-stamp-stamp-probe")

	code, output, _ := runHook(t, root, "bash-timestamp", payload)
	if code != 0 {
		t.Fatalf("first call: code = %d, want 0", code)
	}
	var message struct {
		System string `json:"systemMessage"`
	}
	if err := json.Unmarshal([]byte(output), &message); err != nil {
		t.Fatalf("first call printed no JSON system message: %q: %v", output, err)
	}
	if !strings.HasPrefix(message.System, "▶ ") {
		t.Fatalf("system message = %q, want a ▶ timestamp", message.System)
	}
	if _, err := time.Parse(time.TimeOnly, strings.TrimPrefix(message.System, "▶ ")); err != nil {
		t.Fatalf("system message %q carries no HH:MM:SS time: %v", message.System, err)
	}

	code, output, _ = runHook(t, root, "bash-timestamp", payload)
	if code != 0 {
		t.Fatalf("second call: code = %d, want 0", code)
	}
	if output != "" {
		t.Fatalf("second call inside the quiet gap printed %q, want nothing", output)
	}

	old := strconv.AppendInt(nil, time.Now().Add(-2*bashStampQuietDefault).Unix(), 10)
	if err := os.WriteFile(marker, old, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, output, _ = runHook(t, root, "bash-timestamp", payload); output == "" {
		t.Fatal("call after the quiet gap printed nothing, want a timestamp")
	}

	t.Setenv(bashStampQuietEnv, "0")
	if _, output, _ = runHook(t, root, "bash-timestamp", payload); output == "" {
		t.Fatalf("call with %s=0 printed nothing, want a timestamp", bashStampQuietEnv)
	}
}
