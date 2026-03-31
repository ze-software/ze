package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCmdArchive_NoArgs verifies error when no arguments are provided.
//
// VALIDATES: Missing arguments produces usage error.
// PREVENTS: Panic or confusing error on missing argument.
func TestCmdArchive_NoArgs(t *testing.T) {
	code := cmdArchive([]string{})
	assert.Equal(t, exitError, code)
}

// TestCmdArchive_MissingConfigFile verifies error when config file argument is missing.
//
// VALIDATES: Only archive name without config file produces usage error.
// PREVENTS: Panic when only one argument given.
func TestCmdArchive_MissingConfigFile(t *testing.T) {
	code := cmdArchive([]string{"backup"})
	assert.Equal(t, exitError, code)
}

// TestCmdArchive_FileNotFound verifies error when config file doesn't exist.
//
// VALIDATES: Non-existent config file produces file error.
// PREVENTS: Panic on missing file.
func TestCmdArchive_FileNotFound(t *testing.T) {
	code := cmdArchive([]string{"backup", "/nonexistent/config.conf"})
	assert.Equal(t, exitError, code)
}

// TestCmdArchive_NoArchiveBlocks verifies error when config has no archive blocks.
//
// VALIDATES: Config without system.archive blocks produces error.
// PREVENTS: Silent no-op when no archives are configured.
func TestCmdArchive_NoArchiveBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("bgp {\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000;\n\t\t}\n\t}\n}\n"), 0o600))

	code := cmdArchive([]string{"backup", configPath})
	assert.Equal(t, exitError, code)
}

// TestCmdArchive_NameNotFound verifies error when named archive block doesn't exist.
//
// VALIDATES: Non-existent archive name produces error with available names.
// PREVENTS: Silent failure or panic on wrong archive name.
func TestCmdArchive_NameNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	content := "system {\n\tarchive local {\n\t\tlocation file:///backups;\n\t}\n}\nbgp {\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000;\n\t\t}\n\t}\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	code := cmdArchive([]string{"nonexistent", configPath})
	assert.Equal(t, exitError, code)
}

// TestCmdArchive_FileLocation verifies archive to file:// location.
//
// VALIDATES: Named archive block with file:// location creates archive copy.
// PREVENTS: Archive not being triggered from CLI.
func TestCmdArchive_FileLocation(t *testing.T) {
	destDir := t.TempDir()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	content := "system {\n\thost router1;\n\tarchive local {\n\t\tlocation file://" + destDir + ";\n\t}\n}\nbgp {\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000;\n\t\t}\n\t}\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	code := cmdArchive([]string{"local", configPath})
	assert.Equal(t, exitOK, code)

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "archive file should be created")
	assert.Contains(t, entries[0].Name(), "test-router1-")
}

// TestCmdArchive_HTTPLocation verifies archive to http:// location.
//
// VALIDATES: Named archive with http:// sends POST to server.
// PREVENTS: HTTP upload not being triggered from CLI.
func TestCmdArchive_HTTPLocation(t *testing.T) {
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	content := "system {\n\tarchive remote {\n\t\tlocation " + server.URL + ";\n\t}\n}\nbgp {\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000;\n\t\t}\n\t}\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	code := cmdArchive([]string{"remote", configPath})
	assert.Equal(t, exitOK, code)
	assert.True(t, received, "HTTP server should have received POST")
}
