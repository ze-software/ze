// VALIDATES: the functional verify tier is derived from the gating suite list,
// so a gating run that rules a suite out cannot lower a requirement's evidence.
// PREVENTS: a tier derivation that reads the recorded suite map, which is
// optional by design and would make a fail-closed derivation depend on it.

package rfc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/functional"
)

// narrowingSuiteMap is a VALID suite map that records one gating suite.
//
// Valid is the whole point. Every reader of the artifact widens to every suite
// on a map it cannot use, so an unusable map would let a map-reading derivation
// pass this test by accident. The shape is the contract in
// docs/architecture/testing/verify-freshness-scope.md: a commit, at least one
// suite, and at least one package under each suite.
const narrowingSuiteMap = `{
  "head": "0000000000000000000000000000000000000000",
  "reached": {
    "%s": ["./internal/component/ssh"]
  }
}
`

// minimalWorkflow is one workflow file, which readWorkflowSources requires
// before any carrier table can be built. It schedules nothing, so every interop
// tree resolves unrun and only the functional rows this test reads are live.
const minimalWorkflow = "name: fixture\non:\n  push:\njobs:\n  none:\n    runs-on: ubuntu-latest\n"

// TestFunctionalTierIsUnchangedBySelection is the spec's
// test_functional_tier_is_unchanged_by_selection (AC-7).
//
// A gating run no longer starts every suite: it selects the suites a change set
// can reach, in full mode as well as changed mode. The tier a `.ci` earns MUST
// NOT follow that selection, because a requirement proven only by a suite this
// run ruled out would lose its evidence and trip checkEvidenceRatchet.
//
// The tree this test builds carries a valid suite map naming ONE suite, and it
// is the only input either derivation takes. A derivation that consulted the
// map would answer that one suite's row and no other, so every other gating
// suite losing its verify row is what this test would catch.
func TestFunctionalTierIsUnchangedBySelection(t *testing.T) {
	gating := functional.GatingNames()
	if len(gating) < 2 {
		t.Fatalf("the gating list holds %d suite(s), so a map naming one narrows nothing", len(gating))
	}

	root := t.TempDir()
	writeTreeFile(t, root, ".github/workflows/fixture.yml", minimalWorkflow)
	writeTreeFile(t, root, "tmp/ze-suite-map.json", fmt.Sprintf(narrowingSuiteMap, gating[0]))

	working, err := carriers(root)
	if err != nil {
		t.Fatalf("the working carrier table: %v", err)
	}
	head, err := headCarriers(root)
	if err != nil {
		t.Fatalf("the HEAD carrier table: %v", err)
	}

	tables := []struct {
		name string
		rows []Carrier
	}{{"working", working}, {"HEAD", head}}
	for _, table := range tables {
		for _, suite := range gating {
			rel := "test/" + suite + "/fixture.ci"
			carrier, held := CarrierFor(rel, table.rows)
			if !held {
				t.Errorf("the %s carrier table classifies no carrier for %s", table.name, rel)
				continue
			}
			if carrier.Tier != tierVerify {
				t.Errorf("the %s carrier table gives %s the tier %q, so the suite map lowered it",
					table.name, rel, carrier.Tier)
			}
		}
	}
}

// writeTreeFile writes one fixture file under the test tree.
func writeTreeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(rel), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", rel, err)
	}
}
