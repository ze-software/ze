package exabgp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: ExaBGP CLI dispatch (Run), flag parsing, input validation.
// PREVENTS: Broken subcommand routing, panic on empty args, invalid flag acceptance.

// TestRun_NoArgs verifies exit 1 when no arguments provided.
func TestRun_NoArgs(t *testing.T) {
	code := Run(nil)
	assert.Equal(t, exitError, code)
}

// TestRun_EmptyArgs verifies exit 1 for empty slice.
func TestRun_EmptyArgs(t *testing.T) {
	code := Run([]string{})
	assert.Equal(t, exitError, code)
}

// TestRun_HelpFlag verifies help returns exit 0.
func TestRun_HelpFlag(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"help", "help"},
		{"dash_h", "-h"},
		{"double_dash_help", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := Run([]string{tt.arg})
			assert.Equal(t, exitOK, code)
		})
	}
}

// TestRun_UnknownSubcommand verifies exit 1 for unknown subcommand.
func TestRun_UnknownSubcommand(t *testing.T) {
	code := Run([]string{"nonexistent"})
	assert.Equal(t, exitError, code)
}

// TestCmdPlugin_NoArgs verifies exit 1 when plugin command has no arguments.
func TestCmdPlugin_NoArgs(t *testing.T) {
	code := cmdPlugin(nil)
	assert.Equal(t, exitError, code)
}

// TestCmdPlugin_InvalidAddPath verifies exit 1 for invalid --add-path mode.
func TestCmdPlugin_InvalidAddPath(t *testing.T) {
	code := cmdPlugin([]string{"--add-path", "invalid", "echo"})
	assert.Equal(t, exitError, code)
}

// TestCmdPlugin_InvalidFamily verifies exit 1 for unsupported address family.
func TestCmdPlugin_InvalidFamily(t *testing.T) {
	code := cmdPlugin([]string{"--family", "bad/family", "echo"})
	assert.Equal(t, exitError, code)
}

// TestCmdMigrate_NoArgs verifies exit 1 when migrate has no config file.
func TestCmdMigrate_NoArgs(t *testing.T) {
	code := cmdMigrate(nil)
	assert.Equal(t, exitError, code)
}

// TestCmdMigrate_MissingFile verifies exit 1 for non-existent config file.
func TestCmdMigrate_MissingFile(t *testing.T) {
	code := cmdMigrate([]string{"/nonexistent/path/config.conf"})
	assert.Equal(t, exitError, code)
}

// TestCmdMigrate_InvalidConfig verifies exit 1 for unparseable config content.
func TestCmdMigrate_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.conf")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid"), 0o644))

	code := cmdMigrate([]string{path})
	assert.Equal(t, exitError, code)
}

// TestFamilyList_String verifies the String method for repeated flag.
func TestFamilyList_String(t *testing.T) {
	f := familyList{"ipv4/unicast", "ipv6/unicast"}
	assert.Equal(t, "ipv4/unicast,ipv6/unicast", f.String())
}

// TestFamilyList_Set verifies the Set method appends values.
func TestFamilyList_Set(t *testing.T) {
	var f familyList
	require.NoError(t, f.Set("ipv4/unicast"))
	require.NoError(t, f.Set("ipv6/unicast"))
	assert.Equal(t, familyList{"ipv4/unicast", "ipv6/unicast"}, f)
}
