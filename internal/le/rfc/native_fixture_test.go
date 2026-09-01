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
	//
	// Re-sealed 2026-09-01, for plan/spec-publish-the-rfc-requirement-ledger.md.
	// No verdict moved. The change is an EXPORT pass: the polarities, the
	// annotation kinds, the audit verdicts, the discrimination routes, the proof
	// states, the site dispositions, the cover key, the discrimination verdict,
	// the per-RFC coverage row and the markdown cell escape are renamed to their
	// exported spellings so internal/le/site can publish them, `Audit.Record`
	// answers one verdict typed, the audit vocabulary carries the sentence each
	// word means, `RequirementRows` becomes the ONE producer of a shard's six
	// cells with `RenderShards` formatting what it answers, `parseEnrolled` keeps
	// the enrolment reason, and a summary's Meta `| Title |` row becomes a parsed
	// fact. Verified by regenerating: `ai/RFC-REQUIREMENTS.md` and 189 shards
	// re-rendered byte-identical apart from `rfc/requirements/rfc4724.md`, which
	// was stale against a tag committed before this work.
	//
	// The digest covers every non-test byte of this package, so it also carries
	// the uncommitted edits other sessions hold in this shared checkout.
	//
	// Re-sealed 2026-09-01, for the two baselines the discrimination obligation
	// reads (owner decision). Two verdicts moved. A tag owes its proof where the
	// TIP COMMIT added it against HEAD^, so a tag only in somebody's working tree
	// is nobody's violation and the author meets it inside the detached verify
	// worktree. A stale record's drift is judged at the granularity the record
	// fingerprints, so an unrelated uncommitted edit elsewhere in the producer's
	// file no longer downgrades a committed drift to a report.
	//
	// Re-sealed 2026-09-01, for the escape's reach test. One verdict moved: a
	// carrier that runs the daemon is no longer exempt from reach. The exemption
	// read that a .ci or an interop scenario reaches every compiled file, which is
	// true and is why it had to go: a predicate every file satisfies ties the
	// escape to no claim, so the route shut for unit tags stayed open for the 94
	// .ci and 37 interop ones. Those two reasons are now unreachable on that
	// carrier by construction.
	const want = "d7326b6c51f67120e8906dcfdb908d6d97e9030c1a01eaec31654a7535e49e9d"
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
