package cli

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
	err := formatDocCommand(&buf, "show bgp peer list")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "show bgp peer list")
	assert.Contains(t, out, "read-only")
}

// VALIDATES: AC-7 -- unknown command returns error.
// PREVENTS: Silent failure on typos.
func TestDocCommandUnknown(t *testing.T) {
	var buf bytes.Buffer
	err := formatDocCommand(&buf, "nonexistent command")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// VALIDATES: AC-8 -- doc --list shows all commands.
// PREVENTS: Missing commands from listing.
func TestDocList(t *testing.T) {
	var buf bytes.Buffer
	err := formatDocList(&buf)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "show bgp peer list", "should list show bgp peer list")
	assert.Contains(t, out, "request shutdown", "should list request shutdown")
	assert.Contains(t, out, "Command", "should have header")
}

// PREVENTS: Doc output missing output parameters.
func TestDocCommandWithOutputParams(t *testing.T) {
	var buf bytes.Buffer
	// `show bgp` is the CLI path of ze-bgp:overview, and the `overview` RPC in
	// ze-bgp-api.yang declares the peer-summary payload it answers with.
	err := formatDocCommand(&buf, "show bgp")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Parameters (output):", "the overview should have output parameters")
	assert.Contains(t, out, "uptime", "the overview output should list uptime")
}

// PREVENTS: Doc output not showing commands with no parameters.
func TestDocCommandNoParams(t *testing.T) {
	var buf bytes.Buffer
	err := formatDocCommand(&buf, "request shutdown")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "request shutdown")
	assert.NotContains(t, out, "Parameters", "shutdown has no YANG params")
}

// PREVENTS: Case-insensitive matching failure.
func TestDocCommandCaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	err := formatDocCommand(&buf, "SHOW BGP PEER LIST")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "show bgp peer list")
}

// PREVENTS: Empty command string treated as valid.
func TestDocCommandEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := formatDocCommand(&buf, "")
	assert.Error(t, err)
}
