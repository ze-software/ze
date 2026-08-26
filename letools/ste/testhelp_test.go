// Helpers the package's own cases share: a git fixture, and the JSON probe the
// kebab-case cases read.

package ste

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitFixture writes a repository, commits it, and answers its root. The ratchet
// compares against HEAD, so a fixture with no commit has no baseline.
func gitFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		writeFixtureFile(t, root, rel, body)
	}
	runGit(t, root, "init", "--quiet")
	runGit(t, root, "add", "--all")
	runGit(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=test",
		"commit", "--quiet", "-m", "fixture")
	return root
}

// writeFixtureFile writes one file of a fixture, making its directory.
func writeFixtureFile(t *testing.T, root, rel, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixture file: %v", err)
	}
}

// runGit runs one git command in the fixture and fails the test when it does
// not succeed.
func runGit(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// marshalForTest encodes a payload the way leroot encodes it.
func marshalForTest(t *testing.T, payload any) []byte {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	return raw
}

// jsonKeys answers every object key in a JSON document, at every depth.
func jsonKeys(raw []byte) []string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	var out []string
	var walk func(any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				out = append(out, key)
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(decoded)
	return out
}
