package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateDefaultOutputSet verifies that ze config migrate on hierarchical input
// produces set-format output by default (not hierarchical).
//
// VALIDATES: configMigrateWithWarnings with outputFormat="set" produces "set " lines.
// PREVENTS: Hierarchical input being echoed back as hierarchical when the default is set format.
func TestMigrateDefaultOutputSet(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")

	// Write a hierarchical config (router-id and session { asn { local } } are under bgp {}).
	hierarchical := `bgp {
    router-id 1.2.3.4
    session {
        asn {
            local 65000
        }
    }
}
`
	err := os.WriteFile(configPath, []byte(hierarchical), 0o600)
	require.NoError(t, err)

	// Migrate with default format ("set").
	output, result, _, err := configMigrateWithWarnings(configPath, "", "set")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Output should be set-format lines, not hierarchical.
	assert.True(t, strings.Contains(output, "set "),
		"default output should contain 'set ' commands, got:\n%s", output)
	assert.False(t, strings.Contains(output, "bgp {"),
		"default output should not contain hierarchical braces, got:\n%s", output)
}

// TestMigrateExplicitHierarchical verifies that --format hierarchical produces
// hierarchical output from a hierarchical input.
//
// VALIDATES: configMigrateWithWarnings with outputFormat="hierarchical" preserves braces.
// PREVENTS: --format hierarchical flag being ignored.
func TestMigrateExplicitHierarchical(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")

	hierarchical := `bgp {
    router-id 1.2.3.4
    session {
        asn {
            local 65000
        }
    }
}
`
	err := os.WriteFile(configPath, []byte(hierarchical), 0o600)
	require.NoError(t, err)

	output, _, _, err := configMigrateWithWarnings(configPath, "", "hierarchical")
	require.NoError(t, err)

	assert.True(t, strings.Contains(output, "bgp {"),
		"hierarchical output should contain braces, got:\n%s", output)
	assert.False(t, strings.Contains(output, "set "),
		"hierarchical output should not contain 'set ' commands, got:\n%s", output)
}
