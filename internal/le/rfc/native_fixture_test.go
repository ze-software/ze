package rfc

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeImplementationFixture replaces the retired cross-runtime oracle.
// The digest pins every production byte that supplied its tables and edge-case
// decisions, while the behavioral tests in this package pin their outcomes.
func TestNativeImplementationFixture(t *testing.T) {
	// Re-sealed 2026-08-28, for a lint pass over this package that changed no
	// verdict. It deleted five superseded helpers no caller reached
	// (workflowIsScheduled, featureTags, functionNameAt, functionText, and the
	// three *Names accessors in audit.go), named the repeated string literals
	// the closed key sets and the selftest fixture share, restated one refusal
	// in reseal.go without inverting it, and corrected two UK spellings to US
	// English. Every deletion has a live equivalent named in its own commit
	// message.
	const want = "b024a46c97744bc2c814e7aafdd034794e73c8f8a8828147dd4562958289fbd7"
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list RFC sources: %v", err)
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
		t.Fatalf("native RFC fixture digest = %s, want %s; review the behavior change and update the owned fixture", got, want)
	}
}

func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve checkout root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("checkout root %s has no go.mod: %v", root, err)
	}
	return root
}
