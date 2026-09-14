// Design: resolver.go -- ValidationModes is the one Go declaration of the set
//
// Goal: prove the modes the resolver acts on are the modes an operator can
// configure, so neither side can gain or lose a word without the other.
// Method: load the YANG model and compare the enumeration at
// system/dns/dnssec-validation against ValidationModes().

package dns

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"

	// The blank import registers ze-system-conf.yang, which declares the leaf
	// read below. Without it the loader answers a model with no system tree.
	_ "github.com/ze-software/ze/internal/component/config/system/yang"
)

// dnssecValidationLeaf is the operator-facing declaration of the mode set.
const dnssecValidationLeaf = "system/dns/dnssec-validation"

// TestValidationModesMatchTheYANGEnumeration holds the resolver's set to the
// model an operator writes against.
//
// The resolver spells the words in Go because dnssecDecision branches on them,
// and the model spells them because an operator configures one. That makes the
// Go side a copy, so this test is the check that compares the two
// (ai/rules/principles.md).
func TestValidationModesMatchTheYANGEnumeration(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err, "the YANG schema must load")

	node, err := schema.Lookup(dnssecValidationLeaf)
	require.NoError(t, err, "the schema must declare %s", dnssecValidationLeaf)

	leaf, isLeaf := node.(*config.LeafNode)
	require.True(t, isLeaf, "%s must be a leaf", dnssecValidationLeaf)
	require.NotEmpty(t, leaf.Enums, "%s must carry an enumeration", dnssecValidationLeaf)

	assert.ElementsMatch(t, leaf.Enums, ValidationModes(),
		"the resolver acts on exactly the modes %s declares", dnssecValidationLeaf)
}

// TestEveryValidationModeReachesABranch proves the set is not merely equal in
// size: each mode the resolver publishes is a word dnssecDecision recognizes,
// so a word added to the list without a branch cannot pass as supported.
func TestEveryValidationModeReachesABranch(t *testing.T) {
	for _, mode := range ValidationModes() {
		warn, reject := dnssecDecision(2, false, mode) // 2 is SERVFAIL, the failure signal.
		if mode == dnssecOff {
			assert.Empty(t, warn, "off neither warns nor rejects")
			assert.NoError(t, reject, "off neither warns nor rejects")
			continue
		}
		assert.True(t, warn != "" || reject != nil,
			"%q is published as a validation mode, so SERVFAIL must warn or reject", mode)
	}
}

// TestValidationModesAreOrderedWeakestFirst pins the order the CLI's own help
// text prints, so the flag description cannot start naming a stricter mode
// first without this test saying so.
func TestValidationModesAreOrderedWeakestFirst(t *testing.T) {
	assert.Equal(t, []string{dnssecOff, dnssecPermissive, dnssecStrict}, ValidationModes())
	assert.Equal(t, 0, slices.Index(ValidationModes(), dnssecOff), "off is the default and prints first")
}
