// Design: docs/architecture/core-design.md -- component-group unit actions.
// Related: pkg.go -- PackageRuns, the judgment register.go installs in the hook
// Related: ../../hookruntime/rawheavy.go -- the raw-job refusal that reads it

package testunit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/hookruntime"
)

// TestRawJobAdviceUsesTheActionsOwnWords is review N6.
//
// VALIDATES: each label, the command and each keyword the raw-job refusal
// writes for the short package action (hookruntime.PackageActionWords) equals
// the constant this package's action table reads.
// PREVENTS: a renamed keyword or label here leaving the hook's copy behind,
// so the refusal advises a command the action refuses.
func TestRawJobAdviceUsesTheActionsOwnWords(t *testing.T) {
	want := map[string]string{
		"test-label": testLabel,
		"vet-label":  vetLabel,
		"command":    "./le " + Area + " " + pkgVerb,
		"path":       pathKeyword,
		"run":        runKeyword,
		"vet":        vetKeyword,
	}
	got := hookruntime.PackageActionWords()
	if len(got) != len(want) {
		t.Errorf("the hook copies %d words, the action declares %d: %v", len(got), len(want), got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s: the hook writes %q, the action reads %q", key, got[key], value)
		}
	}
}

// TestRawGoTestAdvisesTheShortFormOnlyWhereTheActionRuns is review N7.
//
// VALIDATES: through the Bash hook's raw-job check, with the judgment this
// package's init installs, a go test or go vet of a package the action runs
// is advised as the short form, and one the action refuses (a tree spanning
// groups, AC-12; a package behind the installer group's tags, AC-11) keeps
// the long admitted form.
// PREVENTS: advice whose literal command the action then refuses.
func TestRawGoTestAdvisesTheShortFormOnlyWhereTheActionRuns(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/ze-software/ze\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		command string
		use     string
	}{
		{"runs", `go test ./internal/le/job/`, "./le test unit pkg path ./internal/le/job/"},
		{"spans-groups", `go test ./internal/...`, "./le job run label unit-pkg command go test ./internal/..."},
		{"installer-group", `go test ./internal/install/x/`, "./le job run label unit-pkg command go test ./internal/install/x/"},
		{"vet-installer", `go vet ./internal/install/...`, "./le job run label vet-pkg command go vet ./internal/install/..."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := hookruntime.Payload{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": test.command},
			}
			code, message, found := hookruntime.Probe("bashRawHeavy", payload, root)
			if !found {
				t.Fatal("the hook serves no bashRawHeavy check")
			}
			if code != 2 {
				t.Fatalf("%q: code %d, want 2: %s", test.command, code, message)
			}
			if !strings.Contains(message, "  -- Use: "+test.use+"\n") {
				t.Errorf("%q: the refusal lacks %q: %s", test.command, test.use, message)
			}
		})
	}
}
