// Design: docs/architecture/config/yang-config-design.md -- schema loading
// Related: rfc7950_uses_deviation_test.go -- augment body cardinality

package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoaderUsesAugmentMayUseAnEmptyGrouping proves that the required body
// member need not produce a child. Method: augment with a uses of an empty
// grouping and verify the existing target leaf survives unchanged.
// RFC 7950 Section 14 counts data-def-stmt, which includes uses-stmt, rather
// than the number of effective nodes an augment produces.
func TestLoaderUsesAugmentMayUseAnEmptyGrouping(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module uses-empty-grouping {
    yang-version 1.1; namespace "urn:ze:test:uses-empty-grouping"; prefix u;
    grouping nothing {}
    grouping g { container box { leaf original { type string; } } }
    container copy { uses g { augment box { uses nothing; } } }
}`)
	require.NoError(t, err)
	box := resolved.GetEntry("uses-empty-grouping").Dir["copy"].Dir["box"]
	require.NotNil(t, box)
	assert.Len(t, box.Dir, 1)
	assert.Contains(t, box.Dir, "original")
}
