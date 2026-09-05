// VALIDATES: the feature-tag answer a run publishes really scopes the
// Staticcheck matrix, and the rows the scope leaves are still dealt whole
// across the matrix stages this run's population carries.
// PREVENTS: a published answer no consumer acts on, and a cut proven only over
// the unscoped 38 rows while a scoped run's three rows fell between the pieces.
//
// The matrix package proves the cut over its own derivation. This file proves
// the join: the producer is this package, the consumer is the matrix, and
// neither package's own tests can see both ends.

package verifyengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/staticcheckfeaturematrix"
)

// matrixArea is the le command whose stages judge the matrix. staticcheckStages
// spells it, and this test reads the population rather than a second list.
const matrixArea = "staticcheck-feature-matrix"

func TestTheAnswerARunPublishesScopesTheMatrixAndIsDealtWhole(t *testing.T) {
	root := writeVerifyScopeFixture(t)
	commitVerifyScopeFixture(t, root)
	gated := filepath.Join(root, "ssh", "ssh.go")
	if err := os.WriteFile(gated, []byte("package ssh\n\n// Listen is the gated feature.\nfunc Listen() int { return 9 }\n"), 0o600); err != nil {
		t.Fatalf("edit the gated fixture file: %v", err)
	}

	_, seen := runWatchingScope(t, root)

	// The fixture manifest declares two features, so the whole matrix is four
	// rows: all_features, core_only, and one omission row per tag.
	whole, _, err := staticcheckfeaturematrix.DeriveScoped(root, "")
	if err != nil {
		t.Fatalf("derive the whole matrix: %v", err)
	}
	if len(whole) != 4 {
		t.Fatalf("the fixture's whole matrix is %d rows, want 4", len(whole))
	}

	scoped, _, err := staticcheckfeaturematrix.DeriveScoped(root, seen[0].tagsPath)
	if err != nil {
		t.Fatalf("derive the matrix the run's answer scopes: %v", err)
	}
	if len(scoped) != 3 {
		t.Fatalf("the published answer scoped the matrix to %d rows, want 3", len(scoped))
	}
	for _, row := range scoped {
		if row.Name == "without_ze_bgp" {
			t.Errorf("an ssh-only change kept %q, which its tags cannot move", row.Name)
		}
	}

	// The count is this population's, read from the stages that run the matrix,
	// so a change to either side is caught rather than assumed.
	pieces := 0
	for _, current := range StagesForMode(Mode) {
		if current.Identity.Command == matrixArea {
			pieces++
		}
	}
	if pieces < 2 {
		t.Fatalf("the verify population runs %d pieces of the matrix, want the cut it declares", pieces)
	}

	judged := make(map[string]int, len(scoped))
	for index := 1; index <= pieces; index++ {
		piece, err := scoped.Part(index, pieces)
		if err != nil {
			t.Fatalf("part %d of %d: %v", index, pieces, err)
		}
		for _, row := range piece {
			judged[row.Name]++
		}
	}
	for _, row := range scoped {
		if judged[row.Name] != 1 {
			t.Errorf("scoped row %q is judged by %d of the %d pieces, want 1", row.Name, judged[row.Name], pieces)
		}
	}
}
