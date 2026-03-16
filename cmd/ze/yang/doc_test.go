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
	err := FormatDocCommand(&buf, "bgp peer list")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "bgp peer list")
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
	assert.Contains(t, out, "bgp peer list", "should list bgp peer list")
	assert.Contains(t, out, "daemon shutdown", "should list daemon shutdown")
	assert.Contains(t, out, "Command", "should have header")
}
