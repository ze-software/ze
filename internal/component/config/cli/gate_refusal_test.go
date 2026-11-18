// Design: docs/architecture/api/commands.md -- complete native editing refusals.
package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDetachedSetMalformedSourceKeepsTheNativeRefusal compares the real root
// and daemon routes when the source cannot parse. Neither may change it or
// replace the shared failure with a second explanation or recovery command.
func TestDetachedSetMalformedSourceKeepsTheNativeRefusal(t *testing.T) {
	const malformed = "bgp {\n"
	file := writeTestConfig(t, malformed)
	code, diagnostic := captureStderr(t, func() int {
		return cmdSet([]string{file, "bgp", "router-id", "203.0.113.11"})
	})
	require.Equal(t, exitError, code)
	server := utilityServer(t)
	pair := suppliedConfigPair(t, malformed, "set bgp router-id 203.0.113.11")
	response, err := utilityCall(t, server, t.Context(), "show config supplied set"+utilityContent(pair))
	var refused *plugin.RefusalError
	require.ErrorAs(t, err, &refused)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Empty(t, response.Data)
	line, found := plugin.RefusalLine(err)
	require.True(t, found)
	require.Equal(t, strings.TrimSuffix(diagnostic, "\n"), line)
	require.Contains(t, line, "unexpected EOF")
	require.Equal(t, 1, strings.Count(line, "\nfix: "))
	unchanged, readErr := os.ReadFile(file)
	require.NoError(t, readErr)
	require.Equal(t, malformed, string(unchanged))

	validPair := suppliedConfigPair(t, validConfig, "set bgp router-id 203.0.113.11")
	accepted, err := utilityCall(t, server, t.Context(), "show config supplied set"+utilityContent(validPair))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, accepted.Status)
	require.NotEmpty(t, accepted.Data)
}
