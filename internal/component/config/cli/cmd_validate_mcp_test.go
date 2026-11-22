// Design: docs/guide/authentication.md -- the management-listener exposure rule, MCP's inputs
// Related: cmd_validate.go -- the MCP refusals `ze config validate` reports

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// mcpBearerNoTLSConfig authenticates MCP with a bearer token on a routable
// address but names no TLS: the daemon refuses to boot on it, because the
// token would cross the network in clear text.
const mcpBearerNoTLSConfig = `environment {
	mcp {
		enabled true
		bind-remote true
		auth-mode bearer
		token s3cret-token
		server main {
			ip 192.0.2.1
			port 8443
		}
	}
}`

// mcpBearerTLSConfig is the same listener with both TLS files named.
const mcpBearerTLSConfig = `environment {
	mcp {
		enabled true
		bind-remote true
		auth-mode bearer
		token s3cret-token
		tls {
			cert /etc/ze/mcp.pem
			key /etc/ze/mcp.key
		}
		server main {
			ip 192.0.2.1
			port 8443
		}
	}
}`

// TestValidateFlagsMcpBearerWithoutTLS drives the TLS half of the MCP rule
// from the `ze config validate` entry point.
//
// VALIDATES: `ze config validate` refuses a remote bearer MCP listener without
// TLS with exactly the problem, why and fix of config.MCPExposure, the
// declaration the boot guard and `ze doctor` read; the same listener with both
// TLS files passes the exposure check.
// PREVENTS: validate, doctor and boot wording or deciding the MCP exposure
// differently.
func TestValidateFlagsMcpBearerWithoutTLS(t *testing.T) {
	want := config.MCPExposure([]string{"192.0.2.1:8443"}, true, false).Refusals()
	require.Len(t, want, 1)

	result := runValidation(mcpBearerNoTLSConfig, "test.conf")
	assert.False(t, result.Valid, "a remote bearer MCP listener without TLS must make the config invalid")
	var got []diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "config-mcp-invalid" {
			got = append(got, result.Diagnostics[i])
		}
	}
	require.Len(t, got, 1, "one exposure, one finding: %+v", result.Diagnostics)
	assert.Equal(t, want[0].Problem(), got[0].Message)
	assert.Equal(t, want[0].Why(), got[0].Help)
	assert.Equal(t, config.MCPTLSFix, got[0].Fix)
	assert.Equal(t, diagnostic.SeverityError, got[0].Severity)

	assert.NotContains(t, validationCodes(t, mcpBearerTLSConfig), "config-mcp-invalid",
		"a bearer token with both TLS files secures a remote MCP listener")
}
