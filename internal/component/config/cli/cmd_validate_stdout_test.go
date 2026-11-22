//go:build ze_bgp

// Design: docs/guide/command-reference.md -- the text `ze config validate` prints
// Related: cmd_validate.go -- outputValidateText and writeFindingDetail, the lines asserted here
// Related: cmd_validate_summary_test.go -- captureValidateStdout, which is built under ze_bgp

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: the text `ze config validate` prints for a remote bearer MCP
// listener without TLS carries the problem, then why it matters, then the
// command that fixes it on a `fix:` line of its own, and never the token.
// PREVENTS: the text output dropping the finding's why and fix, which told the
// operator what was wrong without the command that clears it.
func TestValidateTextPrintsMcpFindingWhyAndFix(t *testing.T) {
	want := config.MCPExposure([]string{"192.0.2.1:8443"}, true, false).Refusals()
	require.Len(t, want, 1)
	path := filepath.Join(t.TempDir(), "mcp.conf")
	require.NoError(t, os.WriteFile(path, []byte(mcpBearerNoTLSConfig), 0o600))

	code, out := captureValidateStdout(t, []string{path})
	require.Equal(t, 1, code, out)
	assert.Contains(t, out, "\n  "+want[0].Problem()+"\n    "+want[0].Why()+"\n    fix: "+config.MCPTLSFix+"\n", out)
	assert.NotContains(t, out, "s3cret-token", "a validation finding never prints the configured token")
}
