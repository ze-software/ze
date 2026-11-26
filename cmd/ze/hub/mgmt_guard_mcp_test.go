// Design: ai/rules/evidence.md -- boot-time management-listener exposure guard, MCP
// Related: mgmt_guard.go -- mcpMgmtListener, filterMgmtListener

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// mcpGuardTLS names both TLS files of an environment.mcp block.
var mcpGuardTLS = zeconfig.MCPTLSConfig{Cert: "/etc/ze/mcp.pem", Key: "/etc/ze/mcp.key"}

// VALIDATES: the boot guard refuses to start a remote MCP listener that
// authenticates but serves no TLS, whether the token comes from the config
// block or from the environment with no block, in every mode that
// authenticates (bearer, bearer-list, a blank auth-mode whose token infers
// bearer), on a routable or a wildcard address, and accepts the same listener
// with both TLS files or on loopback. The declaration is the one
// zeconfig.MCPExposure answers, so the breach carries its TLS fix.
// PREVENTS: an exposed remote bearer MCP listener binding without TLS, or
// silently turning off without a doctor-service-not-started record and fix.
func TestCheckMgmtListenersRefusesRemoteMCPWithoutTLS(t *testing.T) {
	remote := []string{"192.0.2.1:8443"}
	loopback := []string{"127.0.0.1:8443"}
	bearer := zeconfig.MCPListenConfig{AuthMode: "bearer", Token: "s3cret"}
	bearerTLS := zeconfig.MCPListenConfig{AuthMode: "bearer", Token: "s3cret", TLS: mcpGuardTLS}
	certOnly := zeconfig.MCPListenConfig{AuthMode: "bearer", Token: "s3cret", TLS: zeconfig.MCPTLSConfig{Cert: "/etc/ze/mcp.pem"}}
	wildcard := []string{"0.0.0.0:0"}
	bearerList := zeconfig.MCPListenConfig{
		AuthMode:   "bearer-list",
		Identities: []zeconfig.MCPIdentity{{Name: "alice", Token: "alice-secret"}},
	}
	inferredBearer := zeconfig.MCPListenConfig{Token: "s3cret"}

	tests := []struct {
		name        string
		decl        mgmtListener
		wantRefused bool
		wantAllowed []string
		wantFix     string
	}{
		{"remote bearer without TLS refused", mcpMgmtListener(remote, true, bearer, "s3cret"), true, nil, zeconfig.MCPTLSFix},
		{"remote bearer with cert only refused", mcpMgmtListener(remote, true, certOnly, "s3cret"), true, nil, zeconfig.MCPTLSFix},
		{"remote env token with no block refused", mcpMgmtListener(remote, false, zeconfig.MCPListenConfig{}, "s3cret"), true, nil, zeconfig.MCPTLSFix},
		{"wildcard bearer-list without TLS refused", mcpMgmtListener(wildcard, true, bearerList, ""), true, nil, zeconfig.MCPTLSFix},
		{"wildcard blank auth-mode with token without TLS refused", mcpMgmtListener(wildcard, true, inferredBearer, "s3cret"), true, nil, zeconfig.MCPTLSFix},
		{"wildcard bearer without TLS refused", mcpMgmtListener(wildcard, true, bearer, "s3cret"), true, nil, zeconfig.MCPTLSFix},
		{"remote bearer with TLS accepted", mcpMgmtListener(remote, true, bearerTLS, "s3cret"), false, remote, ""},
		{"loopback bearer without TLS accepted", mcpMgmtListener(loopback, true, bearer, "s3cret"), false, loopback, ""},
		{"loopback env token with no block accepted", mcpMgmtListener(loopback, false, zeconfig.MCPListenConfig{}, "s3cret"), false, loopback, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, down := filterMgmtListener(tt.decl)
			assert.Equal(t, tt.wantRefused, len(down) > 0)
			assert.Equal(t, tt.wantAllowed, allowed)
			var fixes []string
			for _, d := range down {
				fixes = append(fixes, d.problem.Fix())
			}
			if !tt.wantRefused {
				assert.Empty(t, fixes)
				return
			}
			assert.Equal(t, []string{tt.wantFix}, fixes, "an authenticated listener lacks TLS alone")
		})
	}
}

// VALIDATES: the MCP guard declaration is zeconfig.MCPExposure verbatim: the
// service name, the addresses, both fixes and the TLS requirement come from
// the declaration that `ze config validate` and `ze doctor` read.
// PREVENTS: the boot guard carrying a hand-written MCP remedy or TLS flag that
// drifts from the offline checks.
func TestMcpMgmtListenerIsTheMCPExposure(t *testing.T) {
	addrs := []string{"192.0.2.1:8443"}
	decl := mcpMgmtListener(addrs, true, zeconfig.MCPListenConfig{AuthMode: "bearer", Token: "s3cret"}, "s3cret")
	assert.Equal(t, zeconfig.MCPExposure(addrs, true, false), decl.exposure())

	// An explicit auth-mode none ignores a token, as the server does.
	none := mcpMgmtListener(addrs, true, zeconfig.MCPListenConfig{AuthMode: "none", Token: "s3cret"}, "s3cret")
	assert.Equal(t, zeconfig.MCPExposure(addrs, false, false), none.exposure())
}
