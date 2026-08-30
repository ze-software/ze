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
	// Updated 2026-08-30: the hook table gate was repointed at
	// `.claude/hooks/README.md`. The four `| Check | Enforces |` tables left
	// `ai/rules/`, so hooktable.go names the published document in one constant
	// and coverage_report.go reads that path instead of joining a rule stem.
	//
	// Updated 2026-08-30: the second session's edits to session_coverage.go are
	// committed now, so the digest moves to the value the tree with them in it
	// answers. Those edits rename analyseSessionCoverage to the US spelling,
	// name the go-test keyword constant, and repoint the Design reference at
	// docs/architecture/core-design.md. None of them changes what the coverage
	// report decides, which is what the digest exists to pin.
	const want = "a7af5ef3c8fcfafb36433badd6b1524c1436dbb9db8d85eded9a9dbdf552b030"
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
