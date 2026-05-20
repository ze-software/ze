package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestDropPlaintextPasswordEntries: the helper drops only entries whose
// final path segment starts with "plaintext-"; preserves order; returns
// a fresh slice (input is not mutated).
//
// VALIDATES: orphan-metadata cleanup after ApplyPasswordHashing.
// PREVENTS: commit metadata referencing a leaf the hash hook removed.
func TestDropPlaintextPasswordEntries(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "bgp local-as"},
		{Path: "system authentication user bob plaintext-password"},
		{Path: "system authentication user alice profile"},
	}
	original := make([]config.SessionEntry, len(in))
	copy(original, in)

	out := dropPlaintextPasswordEntries(in)

	assert.Len(t, out, 2)
	assert.Equal(t, "bgp local-as", out[0].Path)
	assert.Equal(t, "system authentication user alice profile", out[1].Path)

	// Input slice must not have been mutated (we returned a fresh allocation).
	for i, se := range in {
		assert.Equal(t, original[i].Path, se.Path,
			"input slice modified at index %d", i)
	}
}

// TestDropPlaintextPasswordEntriesEmpty: empty input returns empty output.
func TestDropPlaintextPasswordEntriesEmpty(t *testing.T) {
	out := dropPlaintextPasswordEntries(nil)
	assert.Empty(t, out)
}

// TestDropPlaintextPasswordEntriesNoMatches: no plaintext-* entries returns
// the same content (different backing array).
func TestDropPlaintextPasswordEntriesNoMatches(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "bgp local-as"},
		{Path: "system authentication user alice profile"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Len(t, out, 2)
	assert.Equal(t, in[0].Path, out[0].Path)
	assert.Equal(t, in[1].Path, out[1].Path)
}

// TestDropPlaintextPasswordEntriesAllMatch: all-plaintext input returns empty.
func TestDropPlaintextPasswordEntriesAllMatch(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "system authentication user bob plaintext-password"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Empty(t, out)
}

// TestCommitStampsSchemaVersion verifies that CommitSession prepends the schema
// stamp to the written config file.
func TestCommitStampsSchemaVersion(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	content := string(data)
	stamp := config.FormatSchemaStamp(config.SchemaStamp)
	assert.True(t, strings.HasPrefix(content, stamp),
		"committed config should start with schema stamp, got first line: %q",
		strings.SplitN(content, "\n", 2)[0])

	scanned := config.ScanSchemaStamp(data)
	assert.Equal(t, config.SchemaStamp, scanned,
		"ScanSchemaStamp should read back the stamped value")

	// AC-7: show config (WorkingContent) must NOT include the stamp.
	showOutput := ed.WorkingContent()
	assert.False(t, strings.HasPrefix(showOutput, "# ze-schema:"),
		"show config output should not contain schema stamp")

	// AC-9: re-commit to create a backup, then verify the backup has the stamp.
	err = ed.SetValue([]string{"bgp"}, "router-id", "8.8.8.8")
	require.NoError(t, err)
	_, err = ed.CommitSession()
	require.NoError(t, err)

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	if assert.NotEmpty(t, backups, "should have at least one backup") {
		backupData, readErr := os.ReadFile(backups[0].Path)
		require.NoError(t, readErr)
		backupStamp := config.ScanSchemaStamp(backupData)
		assert.Equal(t, config.SchemaStamp, backupStamp,
			"backup should contain schema stamp from committed config")
	}
}
