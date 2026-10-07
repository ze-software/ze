// VALIDATES: a declaration may cite a registered derived page that a fresh
// checkout has not rendered yet, and still may not cite a path nothing holds.
// PREVENTS: ./le feature check refusing docs/features/rfc-status.md on every
// checkout where the gitignored ledger page was not rebuilt.

package feature

import (
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
)

func TestRepoPathAcceptsAnUnrenderedDerivedArtifact(t *testing.T) {
	artifacts := derived.All()
	if len(artifacts) == 0 {
		t.Fatal("no derived artifact is registered in this test binary, so the case proves nothing")
	}
	in := &evidence{tree: t.TempDir()}
	if problem := in.repoPath(artifacts[0].Path); problem != "" {
		t.Fatalf("derived %s refused: %s", artifacts[0].Path, problem)
	}
	if problem := in.repoPath("docs/not-derived-and-absent.md"); problem == "" {
		t.Fatal("an absent path nothing derives was accepted")
	}
}
