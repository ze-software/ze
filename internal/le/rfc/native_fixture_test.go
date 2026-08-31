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
	// The digest changes on EVERY non-test byte of this package, so each edit owes
	// a re-seal. What the re-seal owes back is one line saying whether any VERDICT
	// moved, which is the only thing this test cannot see for itself. The change
	// itself is in its commit message, and repeating it here made this comment a
	// changelog nobody reads.
	//
	// Re-sealed 2026-08-31, for spec-rfc-tag-claim-discrimination. Three verdicts
	// moved, all on the ESCAPE, and all deliberately: it is tied to the claim it
	// discharges rather than to any file an author names, its producer must be
	// code the tagged unit reaches, and a producer key naming a function its file
	// does not declare is refused. A record staled by an edit nobody has committed
	// is reported rather than refused (owner decision, 2026-08-31).
	//
	// Over this tree no verdict moved: no record in it carries an escape, and the
	// violations `./le rfc check` reports are other sessions' corpus and tag work.
	//
	// Re-sealed 2026-08-31 for the feature-out-of-scope exclusion kind. No verdict
	// moved: the change adds one entry to the closed exclusion vocabulary that
	// ParseExtractionArtifact accepts, and rfc/audit/ records no exclusion kind.
	const want = "c2106d359d80efe61f833f1f74d6158000c6e8c448beaaddcbcbc8b088a48a2d"
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
