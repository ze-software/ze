// Design: model_keys.go -- `set cli format` reads the schema leaf, never a literal
//
// Goal: prove the session command and the config leaf answer one set of output
// formats, and that the command closes when the schema does not answer.
// Method: read the enumeration out of the loaded YANG schema, then drive
// handleSetCLIFormat with every value it declares and with one it does not.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// schemaCLIFormats reads the output formats straight out of the loaded model,
// so the assertions below compare the command against the schema rather than
// against a second copy of the list.
func schemaCLIFormats(t *testing.T) []string {
	t.Helper()

	schema, err := config.YANGSchema()
	require.NoError(t, err, "the YANG schema must load")

	node, err := schema.Lookup(cliFormatLeafPath)
	require.NoError(t, err, "the schema must declare %s", cliFormatLeafPath)

	leaf, isLeaf := node.(*config.LeafNode)
	require.True(t, isLeaf, "%s must be a leaf", cliFormatLeafPath)
	require.NotEmpty(t, leaf.Enums, "%s must carry an enumeration", cliFormatLeafPath)

	return leaf.Enums
}

// modelWithSchema builds a Model carrying the validator the daemon gives a
// session, which is where the schema reaches `set cli format`.
func modelWithSchema(t *testing.T) *Model {
	t.Helper()

	validator, err := newConfigValidator()
	require.NoError(t, err, "the config validator must build")
	return &Model{validator: validator}
}

// TestCLIFormatNamesMatchTheSchemaLeaf proves the command's set IS the leaf's
// enumeration. A renamed leaf, a moved container or a dropped enum turns this
// red instead of leaving the command silently refusing every format.
func TestCLIFormatNamesMatchTheSchemaLeaf(t *testing.T) {
	want := schemaCLIFormats(t)
	got := modelWithSchema(t).cliFormatNames()

	assert.ElementsMatch(t, want, got, "set cli format must offer exactly what %s declares", cliFormatLeafPath)
	assert.IsIncreasing(t, got, "the names are sorted, because the error text and the completions read them in order")
}

// TestCLIFormatAcceptsEverySchemaValue drives the operator's own entry point
// with each declared value, so the command and the leaf cannot disagree about
// which words an operator can type.
func TestCLIFormatAcceptsEverySchemaValue(t *testing.T) {
	model := modelWithSchema(t)
	for _, format := range schemaCLIFormats(t) {
		require.True(t, handleSetCLIFormat("set cli format "+format, model), "the command must handle %q", format)
		assert.Equal(t, format, model.cliFormat, "%q is declared by %s, so the session must take it", format, cliFormatLeafPath)
	}
}

// TestCLIFormatRefusesAnUndeclaredValue proves the schema still gates the
// command: `raw` is a pipe operator the catalog holds and the leaf does not,
// which is the nearest word an operator would try.
func TestCLIFormatRefusesAnUndeclaredValue(t *testing.T) {
	model := modelWithSchema(t)
	require.NotContains(t, model.cliFormatNames(), "raw", "the leaf must not declare raw, or this test proves nothing")

	require.True(t, handleSetCLIFormat("set cli format raw", model))
	assert.Contains(t, model.statusMessage, "invalid format")
	assert.Empty(t, model.cliFormat, "a refused value must not reach the session")
}

// TestCLIFormatClosesWithoutASchema proves the guard fails closed. The set is
// what decides whether a format is allowed, so a model that read no schema
// refuses every value rather than accepting any (ai/rules/principles.md).
func TestCLIFormatClosesWithoutASchema(t *testing.T) {
	model := &Model{}
	assert.Empty(t, model.cliFormatNames(), "no validator means no set")

	require.True(t, handleSetCLIFormat("set cli format json", model))
	assert.Contains(t, model.statusMessage, "unavailable")
	assert.Empty(t, model.cliFormat, "an unanswered schema must not let a format through")
}

// TestCLIFormatCompletionsComeFromTheSchema proves the dropdown reads the same
// set the command validates against.
func TestCLIFormatCompletionsComeFromTheSchema(t *testing.T) {
	names := modelWithSchema(t).cliFormatNames()

	completions := appendCLIFormatCompletions(nil, "set cli format ", names)
	require.Len(t, completions, len(names), "one completion for each declared format")

	for i, completion := range completions {
		assert.Equal(t, "set cli format "+names[i], completion.Text)
	}
}
