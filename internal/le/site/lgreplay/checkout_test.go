package sitelgreplay

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestReadCheckoutIgnoresTheRecordingFile proves that the recording an earlier
// run left behind does not make the checkout uncommitted, and that any other
// uncommitted path still does.
//
// VALIDATES: a second `le site lg-replay record` on the same commit runs, so
// two recordings of one commit can be compared.
// PREVENTS: the recorder refusing its own output as an uncommitted change,
// which made every re-record need the trial keyword.
func TestReadCheckoutIgnoresTheRecordingFile(t *testing.T) {
	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // literal arguments of this test
		command.Dir = root
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	writeFile := func(relative, text string) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	runGit("init", "--quiet")
	writeFile("code.go", "package code\n")
	runGit("add", "code.go")
	runGit("commit", "--quiet", "--message", "code")

	writeFile(File, "{}\n")
	recorded, err := readCheckout(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(recorded.uncommitted) != 0 || recorded.provenance.TreeDirty {
		t.Fatalf("the recording file alone made the checkout uncommitted: %q", recorded.uncommitted)
	}

	writeFile("code.go", "package code\n\nvar changed bool\n")
	recorded, err = readCheckout(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(recorded.uncommitted) != 1 || !recorded.provenance.TreeDirty {
		t.Fatalf("a changed source file must make the checkout uncommitted, got %q", recorded.uncommitted)
	}
}
