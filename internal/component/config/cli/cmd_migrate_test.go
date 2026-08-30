package cli

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
// VALIDATES: configMigrateWithWarnings with outputForm="set" produces "set " lines.
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

// TestMigrateExplicitHierarchical verifies that `format hierarchical` produces
// hierarchical output from a hierarchical input.
//
// VALIDATES: configMigrateWithWarnings with outputForm="hierarchical" preserves braces.
// PREVENTS: the hierarchical form being ignored.
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

// TestMigrateFormKeyword verifies the output form is grammar an operator types,
// with the keyword before its value, and that a form the command cannot write
// is refused by name.
//
// VALIDATES: `format set` and `format hierarchical` select the two forms, and
// the words after them are still the config file.
// PREVENTS: the deleted --format flag coming back as a silently ignored
// positional, or an unknown form being written as the default.
func TestMigrateFormKeyword(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		args     []string
		wantForm string
		wantRest []string
		wantErr  bool
	}{
		{name: "default", args: []string{"router.conf"}, wantForm: formSet, wantRest: []string{"router.conf"}},
		{name: "set", args: []string{"format", "set", "router.conf"}, wantForm: formSet, wantRest: []string{"router.conf"}},
		{
			name: "hierarchical", args: []string{"format", "hierarchical", "router.conf"},
			wantForm: formHierarchical, wantRest: []string{"router.conf"},
		},
		{name: "no value", args: []string{"format"}, wantErr: true},
		{name: "unknown form", args: []string{"format", "yaml", "router.conf"}, wantErr: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			form, rest, err := parseOutputForm(testCase.args)
			if testCase.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "format",
					"the error must name the keyword the operator typed")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.wantForm, form)
			assert.Equal(t, testCase.wantRest, rest)
		})
	}
}

// TestMigrateWritesTheFormTheKeywordNames verifies the keyword reaches the
// serializer, through the command an operator actually runs.
//
// VALIDATES: `ze config migrate format hierarchical <file>` writes braces.
// PREVENTS: parseOutputForm answering a form cmdMigrate then drops.
func TestMigrateWritesTheFormTheKeywordNames(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath,
		[]byte("set bgp session asn local 65000\n"), 0o600))

	for _, testCase := range []struct {
		form   []string
		want   string
		reject string
	}{
		{form: nil, want: "set ", reject: "bgp {"},
		{form: []string{"format", "set"}, want: "set ", reject: "bgp {"},
		{form: []string{"format", "hierarchical"}, want: "bgp {", reject: "set "},
	} {
		outputPath := filepath.Join(dir, "out.conf")
		args := append(append([]string{"-o", outputPath}, testCase.form...), configPath)
		require.Equal(t, exitOK, cmdMigrate(args))

		written, err := os.ReadFile(outputPath) //nolint:gosec // the test owns this path
		require.NoError(t, err)
		assert.Contains(t, string(written), testCase.want)
		assert.NotContains(t, string(written), testCase.reject)
	}
}
