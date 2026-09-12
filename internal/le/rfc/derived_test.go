// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
)

// registeredArtifact answers the registration for one path, or fails.
func registeredArtifact(t *testing.T, path string) derived.Artifact {
	t.Helper()
	for _, artifact := range derived.All() {
		if artifact.Path == path {
			return artifact
		}
	}
	t.Fatalf("%s is not registered as a derived artifact", path)
	return derived.Artifact{}
}

// TestTheRFCFamilyIsRegisteredAsDerived covers AC-7's other half.
//
// The five generated paths leave git, so each one owes a registration: a write
// to one of their inputs removes them, and a command that names one rebuilds
// them. A path that leaves git with no registration is a file nothing writes
// and nothing reports missing, which is what registeredArtifact refuses.
//
// The POPULATION is what it then asserts. A nil predicate and a nil rebuild are
// not reachable here -- derived.Register panics on either, so the process would
// not have started -- and an assertion over an unreachable state reads as
// coverage while testing nothing. What a later edit can still get wrong is
// registering one of the five against a narrower predicate: one render writes
// all five, so the five answer to one population or four are rebuilt and one
// sits there stale.
func TestTheRFCFamilyIsRegisteredAsDerived(t *testing.T) {
	for _, path := range []string{ledgerRel, shardRelDir, enrolledRel, notEnrolledRel, statusRel} {
		artifact := registeredArtifact(t, path)
		if !artifact.Feeds(".", summaryRel+"/rfc4271.md") {
			t.Errorf("%s does not answer to a summary write, and a summary is what renders it", path)
		}
		// The shard DIRECTORY is the one member whose presence does not mean it
		// is whole, so it is the one that answers Complete. A file that took a
		// question it does not need, or the directory losing the one it does,
		// each put a stat back where a partial directory reads as the answer.
		if wants := path == shardRelDir; wants != (artifact.Complete != nil) {
			t.Errorf("%s carries Complete = %v, and a directory artifact needs one where a file does not",
				path, artifact.Complete != nil)
		}
	}
}

// TestTheRebuildAnswersNothingForATreeWithNoSummaries pins the refusal the hook
// path depends on.
//
// VALIDATES: rebuildLedger answers nil, and writes nothing, for a tree that
// holds no rfc/short/.
// PREVENTS: a scratch checkout in which every shell command naming ai/ or rfc/
// is blocked by a rebuild that cannot run there.
//
// A rebuild runs over whatever root the command was pointed at, and a scratch
// checkout a test or a fixture created holds no rfc/short/. IndexUpdate REFUSES
// that tree, correctly, because an explicit run over an empty corpus would
// prune every shard as an orphan. As a REBUILD that refusal would block an
// ordinary grep in a tree the artifacts never lived in, so the rebuild answers
// nothing rather than passing the refusal on.
func TestTheRebuildAnswersNothingForATreeWithNoSummaries(t *testing.T) {
	root := t.TempDir()
	if err := rebuildLedger(root); err != nil {
		t.Fatalf("rebuildLedger over a tree with no %s: %v", summaryRel, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ledgerRel))); !os.IsNotExist(err) {
		t.Errorf("the rebuild wrote %s for a tree that declares no requirement", ledgerRel)
	}
}

// TestTheRFCArtifactsAreFedByEverySourceTheRenderReads states the predicate's
// population, one case for each walk a render performs.
//
// The render reads the summaries, the audit verdicts, the discrimination
// records, the extraction sign-offs, each RFC's own text, the workflow files
// that say which actions CI runs on a schedule, every tag carrier, and the
// production Go an `RFC requirement:` comment can sit in. A source the
// predicate misses leaves a stale artifact on disk that reads as current, which
// is the failure the byte comparison used to catch and nothing else would.
//
// Each case names the walk that reaches it, because a case with no producer
// behind it is a guess that the next reader cannot check.
func TestTheRFCArtifactsAreFedByEverySourceTheRenderReads(t *testing.T) {
	artifact := registeredArtifact(t, ledgerRel)
	feeds := map[string]string{
		"rfc/short/rfc4271.md":                           "Collect: every requirement and every Meta row",
		"rfc/audit/rfc4271.json":                         "loadAudits: the verdicts and the freshness states",
		"rfc/discrimination/rfc4271.json":                "loadDiscrimination: the recorded breaks",
		"rfc/extraction/rfc4271.json":                    "renderExtractionTable -> evaluateExtractions -> LoadExtractions",
		"rfc/full/rfc4271.txt":                           "evaluateExtractions -> Deriver.Inventory -> SourceText",
		"rfc/drafts/draft-ietf-idr-bgp-model.txt":        "the same walk, for an RFC enrolled as a draft",
		".github/workflows/nightly.yml":                  "carriers -> scheduledWorkflowActions -> readWorkflowSources",
		"internal/component/bgp/message/rfc7606_test.go": "tagCovers: a tag carrier",
		"test/plugin/open-hold-time.ci":                  "tagCovers: a tag carrier",
		"internal/component/bgp/reactor/reactor.go":      "unscannedTags: an unclaimed `RFC requirement:` comment",
		"internal/le/interoplab/ipsec/checkers.go":       "unscannedTags, over native interop Go",
		"plan/immediate/spec-ipsec-remote-access.md":     "relocationErrors: a spec a relocated-to site names",
		"plan/spec-ipsec-ipcomp.md":                      "relocationErrors, for a spec at the plan/ root",
	}
	for path, walk := range feeds {
		if !artifact.Feeds(".", path) {
			t.Errorf("%s feeds the RFC render through %s and the predicate says it does not", path, walk)
		}
	}
	// The other side. A predicate that answers true for everything invalidates
	// on every write, so the artifact is absent for the whole session and every
	// read pays a rebuild.
	for _, path := range []string{
		"docs/guide/bgp-peering.md",
		"website/blog/posts/reference-from-the-system.md",
	} {
		if artifact.Feeds(".", path) {
			t.Errorf("%s does not feed the RFC render and the predicate says it does", path)
		}
	}
}

// TestTheRFCArtifactsAreNotFedByThemselves pins that the family does not feed
// itself. A generated file that fed its own artifact would be removed by the
// write that produced it, so the rebuild would never settle.
func TestTheRFCArtifactsAreNotFedByThemselves(t *testing.T) {
	paths := []string{ledgerRel, enrolledRel, notEnrolledRel, statusRel, shardRel("rfc4271")}
	for _, artifact := range derived.All() {
		if !slices.Contains([]string{ledgerRel, shardRelDir, enrolledRel, notEnrolledRel, statusRel}, artifact.Path) {
			continue
		}
		for _, path := range paths {
			if artifact.Feeds(".", path) {
				t.Errorf("%s feeds %s, so the write that renders it invalidates it", path, artifact.Path)
			}
		}
	}
}
