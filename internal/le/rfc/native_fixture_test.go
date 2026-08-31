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
	// Re-sealed 2026-08-31, for the review-gate fixes of
	// spec-rfc-tag-claim-discrimination. Two verdicts moved, both deliberately.
	// An escape is now tied to the CLAIM it discharges and not only to a file an
	// author names, so a `no-break` record that named any declaration-only file,
	// or any interop carrier, is refused where it was accepted. And a record
	// staled by an edit NOBODY HAS COMMITTED is reported rather than refused
	// (owner decision, 2026-08-31), so a session editing a producer no longer reds
	// the gate for every other session sharing this checkout.
	//
	// Over this tree the violation count is unchanged. The 24 it reports are 2
	// pre-existing stale rfc7606 audit verdicts and 22 new-tag violations from
	// another session's three UNTRACKED files, and no record in this repository
	// carries an escape, so the tightened escape refuses nothing here.
	const want = "3514714978236e94eaba591a381d92666c1481a377337be5e93d000d58b1b42d"
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
