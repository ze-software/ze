// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
package rfc

import (
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
// and nothing reports missing.
func TestTheRFCFamilyIsRegisteredAsDerived(t *testing.T) {
	for _, path := range []string{ledgerRel, shardRelDir, enrolledRel, notEnrolledRel, statusRel} {
		artifact := registeredArtifact(t, path)
		if artifact.Feeds == nil || artifact.Rebuild == nil {
			t.Errorf("%s registers without a predicate or without a rebuild", path)
		}
	}
}

// TestTheRFCArtifactsAreFedByEverySourceTheRenderReads states the predicate's
// population, one case for each walk NewRenderInput performs.
//
// The render reads the summaries, the audit verdicts, the discrimination
// records, every tag carrier, and the production Go an `RFC requirement:`
// comment can sit in. A source the predicate misses leaves a stale artifact on
// disk that reads as current, which is the failure the byte comparison used to
// catch and nothing else would.
func TestTheRFCArtifactsAreFedByEverySourceTheRenderReads(t *testing.T) {
	artifact := registeredArtifact(t, ledgerRel)
	feeds := []string{
		"rfc/short/rfc4271.md",
		"rfc/audit/rfc4271.json",
		"rfc/discrimination/rfc4271.json",
		"internal/component/bgp/message/rfc7606_test.go",
		"test/plugin/open-hold-time.ci",
		"internal/component/bgp/reactor/reactor.go",
		"internal/le/interoplab/ipsec/checkers.go",
	}
	for _, path := range feeds {
		if !artifact.Feeds(".", path) {
			t.Errorf("%s feeds the RFC render and the predicate says it does not", path)
		}
	}
	// The other side. A predicate that answers true for everything invalidates
	// on every write, so the artifact is absent for the whole session and every
	// read pays a rebuild.
	for _, path := range []string{
		"docs/guide/bgp-peering.md",
		"plan/spec-derived-indexes-answer-a-query.md",
		"website/blog/posts/reference-from-the-system.md",
		"rfc/full/rfc4271.txt",
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
