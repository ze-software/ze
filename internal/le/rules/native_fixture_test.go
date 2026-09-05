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
	// Updated 2026-08-31: reportEmptyCorpus prints its warning with
	// textbuf.Buffer.StdErr() instead of fmt.Fprintln(os.Stderr, tb.String()),
	// and actions.go no longer imports fmt. The buffer now carries the trailing
	// newline Fprintln used to add, so the bytes on stderr are unchanged, which
	// is what the digest exists to pin.
	//
	// Updated 2026-09-02: coverage.go was committed unformatted, so gofmt
	// removed one alignment space on the noPointLine var and five stray blank
	// lines above rationaleProblems. Whitespace only, so the coverage report
	// decides exactly what it decided before, which is what the digest pins.
	//
	// Updated 2026-09-05: the task corpus and the spec selector read every
	// release bucket through internal/le/spec/specpath rather than plan/ alone,
	// so the router now measures the whole population.
	const want = "e4f369a93e9ea1bf8aa8393bc892f6cd224e1dca7979d64d52156f37a9ebd8c5"
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
