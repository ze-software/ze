package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeImplementationFixture pins the native tables and render decisions
// that the retired cross-runtime oracle compared.
func TestNativeImplementationFixture(t *testing.T) {
	// Updated 2026-08-29: isArtifact became exported IsArtifact so
	// internal/le/sourcerewrite could delete its own copy of the predicate. The
	// judgement it makes is unchanged; only the name and its comment moved.
	//
	// The value is computed over the COMMITTED file set, not the working tree.
	// This digest covers every non-test file in the package, so a second
	// session's uncommitted edits to coverage.go and session_coverage.go were
	// in the working tree's answer and are not in the tree this commit makes.
	// Recording the working-tree value would have left HEAD red for a change
	// HEAD does not contain.
	const want = "3086e325c552cadad537584746a9473b4036e75a31e1a67c13d22b4073a41455"
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list rules sources: %v", err)
	}
	digest := sha256.New()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		digest.Write([]byte(filepath.Base(path)))
		digest.Write([]byte{0})
		digest.Write(content)
		digest.Write([]byte{0})
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		t.Fatalf("native rules fixture digest = %s, want %s; review the behavior change and update the owned fixture", got, want)
	}
}
