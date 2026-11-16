package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: SSH and web session preferences read the same schema declaration.
// PREVENTS: hard-coded format choices, accepting values without a schema, or
// replacing a valid preference after an invalid request.
func TestSessionFormatUsesSchema(t *testing.T) {
	schema := config.NewSchema()
	leaf := &config.LeafNode{Type: config.TypeString, Enums: []string{"text", "custom"}}
	schema.Define("environment", config.Container(config.Field("cli", config.Container(config.Field("format", config.Container(config.Field("default", leaf)))))))
	answer, handled, err := ExecuteSessionFormat(schema, "set cli format custom", "text")
	require.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, "custom", answer.Format)
	assert.Equal(t, "cli format set to custom", answer.Output)
	answer, handled, err = ExecuteSessionFormat(schema, "set cli format json", "custom")
	require.True(t, handled)
	require.Error(t, err)
	assert.Equal(t, "custom", answer.Format)
	assert.Contains(t, err.Error(), "fix: set cli format <text|custom>")
	completions := SessionFormatCompletions(schema, "set cli format ")
	require.Len(t, completions, 2)
	assert.Equal(t, "set cli format custom", completions[0].Text)
	assert.Equal(t, "set cli format text", completions[1].Text)
	answer, handled, err = ExecuteSessionFormat(nil, "set cli format json", "custom")
	require.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, "custom", answer.Format)
	assert.Equal(t, "cli format unavailable: the schema declares no output formats", answer.Output)
	assert.Empty(t, SessionFormatCompletions(nil, "set cli format "))
}
