package yang

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-7 -- doc for a specific command.
// PREVENTS: Missing command documentation.
func TestDocCommand(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocCommand(&buf, "peer list")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "peer list")
	assert.Contains(t, out, "read-only")
	assert.Contains(t, out, "Parameters (input):", "should show YANG parameter details")
}

// VALIDATES: AC-7 -- unknown command returns error.
// PREVENTS: Silent failure on typos.
func TestDocCommandUnknown(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocCommand(&buf, "nonexistent command")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// VALIDATES: AC-8 -- doc --list shows all commands.
// PREVENTS: Missing commands from listing.
func TestDocList(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocList(&buf)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "peer list", "should list peer list")
	assert.Contains(t, out, "daemon shutdown", "should list daemon shutdown")
	assert.Contains(t, out, "Command", "should have header")
}

// PREVENTS: Doc output missing output parameters.
func TestDocCommandWithOutputParams(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocCommand(&buf, "summary")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Parameters (output):", "summary should have output parameters")
	assert.Contains(t, out, "uptime", "summary output should list uptime")
}

// PREVENTS: Doc output not showing commands with no parameters.
func TestDocCommandNoParams(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocCommand(&buf, "daemon shutdown")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "daemon shutdown")
	assert.NotContains(t, out, "Parameters", "shutdown has no YANG params")
}

// PREVENTS: Case-insensitive matching failure.
func TestDocCommandCaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocCommand(&buf, "PEER LIST")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "peer list")
}

// PREVENTS: Empty command string treated as valid.
func TestDocCommandEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatDocCommand(&buf, "")
	assert.Error(t, err)
}
