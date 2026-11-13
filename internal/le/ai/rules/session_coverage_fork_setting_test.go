// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: session_coverage.go -- nativeTranscriptPath, held here to the session id and the known session

package airules

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNativeTranscriptSourceIgnoresTheForkSetting asks nativeTranscriptPath
// for the session's transcript with the harness's fork setting at each value
// an owner can give it. The known session is passed in, so the answer does
// not depend on the process tree of the test runner.
//
// VALIDATES: CLAUDE_CODE_FORK_SUBAGENT changes no answer, and with no session
// id in the environment the session le already knows decides: a known session
// with a transcript answers that transcript although a newer neighbor's file
// sits beside it; a known session with no transcript file answers no path,
// as a session id in the environment does for a missing file; a lookup that
// fails answers no path; and only a process with no known session falls back
// to the newest transcript. A session id in the environment wins over all of
// it, which is how a subagent's le finds its parent's transcript.
// PREVENTS: the setting read as a mark of a fork's process, and a process
// whose session is known measuring a neighboring session's transcript.
func TestNativeTranscriptSourceIgnoresTheForkSetting(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	// Setenv registers the restore, and Unsetenv then removes the variable.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	if err := os.Unsetenv("CLAUDE_CODE_SESSION_ID"); err != nil {
		t.Fatal(err)
	}

	dir := transcriptDirectory(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	named := filepath.Join(dir, "named.jsonl")
	newest := filepath.Join(dir, "newest.jsonl")
	for _, path := range []string{named, newest} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	if err := os.Chtimes(named, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newest, now, now); err != nil {
		t.Fatal(err)
	}
	knows := func(id string, err error) func(string) (string, error) {
		return func(string) (string, error) { return id, err }
	}

	for _, value := range []string{"1", "0"} {
		t.Run("set to "+value, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", value)
			if got := nativeTranscriptPath(root, knows("named", nil)); got != named {
				t.Errorf("known session with a transcript: got %q, want its own %q, not the newer neighbor", got, named)
			}
			if got := nativeTranscriptPath(root, knows("absent", nil)); got != "" {
				t.Errorf("known session with no transcript file: got %q, want no path", got)
			}
			if got := nativeTranscriptPath(root, knows("", errors.New("unreadable identity cache"))); got != "" {
				t.Errorf("a failed lookup of the known session: got %q, want no path", got)
			}
			if got := nativeTranscriptPath(root, knows("../named", nil)); got != "" {
				t.Errorf("a known session id that is no file name: got %q, want no path", got)
			}
			if got := nativeTranscriptPath(root, knows("", nil)); got != newest {
				t.Errorf("no known session: got %q, want the newest transcript %q", got, newest)
			}
			t.Setenv("CLAUDE_CODE_SESSION_ID", "named")
			if got := nativeTranscriptPath(root, knows("newest", nil)); got != named {
				t.Errorf("session id in the environment: got %q, want %q", got, named)
			}
		})
	}
}
