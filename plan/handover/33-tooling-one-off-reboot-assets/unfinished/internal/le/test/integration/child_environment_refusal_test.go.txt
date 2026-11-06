// Design: docs/architecture/testing/qemu-integration.md -- attributed child launch.
// Related: dockerpeers.go -- runRequiringDocker refuses an unattributable child.

package testintegration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// environmentRefusingCleaner records that attribution was attempted while
// retaining the ordinary fake cleaner's observations of every cleanup route.
type environmentRefusingCleaner struct {
	*fakeCleaner
	refusal error
	calls   int
}

func (c *environmentRefusingCleaner) ChildEnvironment([]string) ([]string, error) {
	c.calls++
	return nil, c.refusal
}

// TestDockerChildEnvironmentRefusalPreventsLaunch first proves the actual
// runner can launch a marker writer. Refused attribution must then prevent
// that writer from starting, report its cause and perform no orphan cleanup.
// MUTATION: ignoring ChildEnvironment's error starts the second marker writer.
func TestDockerChildEnvironmentRefusalPreventsLaunch(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	directory := t.TempDir()
	const writer = `printf '%s' "$2" > "$1"`
	const payload = "child marker reached"
	positiveMarker := filepath.Join(directory, "positive marker '$();.txt")
	positive := &fakeCleaner{}
	_, positiveCode := runRequiringDocker(t.Context(), dockerPeersVerb,
		fixedArgv([]string{"sh", "-c", writer, "owned-marker-writer", positiveMarker, payload}), root, positive)
	if positiveCode != 0 {
		t.Fatalf("positive marker writer code = %d, want 0", positiveCode)
	}
	written, err := os.ReadFile(positiveMarker)
	if err != nil {
		t.Fatalf("positive child marker: %v", err)
	}
	if string(written) != payload {
		t.Fatalf("positive child marker = %q, want %q", written, payload)
	}
	if positive.ownedCalls != 0 {
		t.Errorf("positive run owned cleanup calls = %d, want 0", positive.ownedCalls)
	}
	if len(positive.dry) != 0 {
		t.Errorf("positive run cleanup calls = %v, want none", positive.dry)
	}

	refusalMarker := filepath.Join(directory, "refused marker '$();.txt")
	const cause = "fixture launch attribution refused: no trusted child ownership"
	refused := &environmentRefusingCleaner{fakeCleaner: &fakeCleaner{}, refusal: errors.New(cause)}
	diagnosticPath := filepath.Join(directory, "attribution-error")
	diagnostics, err := os.Create(diagnosticPath)
	if err != nil {
		t.Fatalf("create the refusal diagnostic file: %v", err)
	}
	previousStderr := os.Stderr
	t.Cleanup(func() {
		os.Stderr = previousStderr
		if err := diagnostics.Close(); err != nil {
			t.Errorf("close the refusal diagnostic file: %v", err)
		}
	})
	os.Stderr = diagnostics
	report, code := runRequiringDocker(t.Context(), dockerPeersVerb,
		fixedArgv([]string{"sh", "-c", writer, "owned-marker-writer", refusalMarker, payload}), root, refused)
	os.Stderr = previousStderr
	if refused.calls != 1 {
		t.Fatalf("child attribution calls = %d, want 1", refused.calls)
	}
	if report != nil {
		t.Errorf("refused launch report = %#v, want nil", report)
	}
	if code == 0 {
		t.Error("refused launch returned success")
	}
	if _, err := os.Stat(refusalMarker); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused child marker stat = %v, want no marker", err)
	}
	if refused.ownedCalls != 0 {
		t.Errorf("refused launch owned cleanup calls = %d, want 0", refused.ownedCalls)
	}
	if len(refused.dry) != 0 {
		t.Errorf("refused launch cleanup calls = %v, want none", refused.dry)
	}
	diagnostic, err := os.ReadFile(diagnosticPath)
	if err != nil {
		t.Fatalf("read the refusal diagnostic: %v", err)
	}
	if !strings.Contains(string(diagnostic), cause) {
		t.Errorf("refusal diagnostic = %q, want the actual cause %q", diagnostic, cause)
	}
}
