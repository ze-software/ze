// Design: docs/architecture/web-interface.md -- shared CLI session preferences.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestSessionFormatUnavailableSchemaPreservesPreference proves a missing leaf
// or an empty enumeration refuses a change and offers no format completions.
func TestSessionFormatUnavailableSchemaPreservesPreference(t *testing.T) {
	empty := config.NewSchema()
	empty.Define("environment", config.Container(config.Field("cli", config.Container(config.Field("format", config.Container(config.Field("default", config.Leaf(config.TypeString))))))))
	for _, tc := range []struct {
		name   string
		schema *config.Schema
	}{
		{name: "missing-leaf", schema: config.NewSchema()},
		{name: "empty-enumeration", schema: empty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer, handled, err := ExecuteSessionFormat(tc.schema, "set cli format json", "custom")
			require.True(t, handled)
			require.NoError(t, err)
			assert.Equal(t, "custom", answer.Format)
			assert.Equal(t, "cli format unavailable: the schema declares no output formats", answer.Output)
			assert.Empty(t, SessionFormatCompletions(tc.schema, "set cli format "))
		})
	}
}
