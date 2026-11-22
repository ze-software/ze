// Design: docs/guide/authentication.md -- the management-listener exposure rule, MCP's inputs
// Related: loader_extract.go -- MCPExposure, MCPListenConfig.Refusals
// Related: validate_semantic.go -- the `ze doctor` route to the same refusals

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// mcpRemoteHost is a documentation address, reachable from off-box by the
// loopback rule, so the exposure rule applies to it.
const mcpRemoteHost = "192.0.2.1"

// mcpTree builds an enabled environment.mcp block with one server entry.
// bind-remote is set whenever host is not loopback, because without it the
// extractor clamps every entry to 127.0.0.1. authMode and token are written
// only when non-empty; withTLS names both TLS files.
func mcpTree(host, authMode, token string, withTLS bool) *Tree {
	tree := NewTree()
	mcp := tree.GetOrCreateContainer("environment").GetOrCreateContainer("mcp")
	mcp.Set("enabled", "true")
	if host != loopbackIP {
		mcp.Set("bind-remote", "true")
	}
	if authMode != "" {
		mcp.Set("auth-mode", authMode)
	}
	if token != "" {
		mcp.Set("token", token)
	}
	if withTLS {
		tls := mcp.GetOrCreateContainer("tls")
		tls.Set("cert", "/etc/ze/mcp.pem")
		tls.Set("key", "/etc/ze/mcp.key")
	}
	srv := NewTree()
	srv.Set("ip", host)
	srv.Set("port", "8443")
	mcp.AddListEntry("server", "main", srv)
	return tree
}

// VALIDATES: the MCP inputs to the one exposure rule. A non-loopback MCP
// listener that authenticates (bearer, bearer-list, oauth, or a token with no
// mode) and names no TLS, or only half of it, is refused with the TLS fix; the
// same listener with both TLS files, and a loopback listener without TLS, pass.
// A non-loopback listener that does not authenticate is refused for both.
// PREVENTS: MCP judging remote exposure by a rule of its own that disagrees
// with gNMI's, the defect where a remote bearer MCP listener without TLS was
// refused offline but only disabled at boot.
func TestMCPListenConfigRefusalsRequireTLSOffLoopback(t *testing.T) {
	remote := []ServerEndpoint{{Host: mcpRemoteHost, Port: "8443"}}
	loopback := []ServerEndpoint{{Host: loopbackIP, Port: "8443"}}
	tlsPair := MCPTLSConfig{Cert: "/etc/ze/mcp.pem", Key: "/etc/ze/mcp.key"}
	oauth := MCPOAuthConfig{AuthorizationServer: "https://as.example/", Audience: "https://mcp.example/"}
	identities := []MCPIdentity{{Name: "alice", Token: "alice-secret"}}

	tests := []struct {
		name     string
		cfg      MCPListenConfig
		wantFixs []string
	}{
		{"bearer remote without TLS", MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer", Token: "s3cret"}, []string{MCPTLSFix}},
		{"bearer remote with cert only", MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer", Token: "s3cret", TLS: MCPTLSConfig{Cert: "/etc/ze/mcp.pem"}}, []string{MCPTLSFix}},
		{"token without mode remote without TLS", MCPListenConfig{Servers: remote, BindRemote: true, Token: "s3cret"}, []string{MCPTLSFix}},
		{"bearer-list remote without TLS", MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer-list", Identities: identities}, []string{MCPTLSFix}},
		{"oauth remote without TLS", MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "oauth", OAuth: oauth}, []string{MCPTLSFix}},
		{"none remote without TLS", MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "none", Token: "ignored"}, []string{MCPAuthFix, MCPTLSFix}},
		{"bearer remote with TLS", MCPListenConfig{Servers: remote, BindRemote: true, AuthMode: "bearer", Token: "s3cret", TLS: tlsPair}, nil},
		{"bearer loopback without TLS", MCPListenConfig{Servers: loopback, AuthMode: "bearer", Token: "s3cret"}, nil},
		{"none loopback without TLS", MCPListenConfig{Servers: loopback}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refusals := tt.cfg.Refusals()
			fixes := make([]string, 0, len(refusals))
			for _, r := range refusals {
				fixes = append(fixes, r.Fix())
				assert.Contains(t, r.Problem(), `"192.0.2.1:8443"`, "the refusal names the listener")
			}
			if len(tt.wantFixs) == 0 {
				assert.Empty(t, fixes)
				return
			}
			assert.Equal(t, tt.wantFixs, fixes)
		})
	}
}

// VALIDATES: `ze doctor`'s route (ValidateSemantics) reports a remote bearer
// MCP listener without TLS as config-mcp-invalid carrying exactly the problem,
// why and fix that MCPExposure answers, the declaration the boot guard reads,
// and reports nothing once both TLS files are named.
// PREVENTS: doctor and the boot guard wording, or deciding, the MCP exposure
// differently.
func TestValidateSemanticsReportsMCPExposureFromTheRule(t *testing.T) {
	want := MCPExposure([]string{mcpRemoteHost + ":8443"}, true, false).Refusals()
	require.Len(t, want, 1)

	var got []diagnostic.Diagnostic
	for _, d := range ValidateSemantics(mcpTree(mcpRemoteHost, "bearer", "s3cret", false)) {
		if d.Code == diagnostic.CodeConfigMCPInvalid {
			got = append(got, d)
		}
	}
	require.Len(t, got, 1, "one exposure, one finding: %+v", got)
	assert.Equal(t, want[0].Problem(), got[0].Message)
	assert.Equal(t, want[0].Why(), got[0].Help)
	assert.Equal(t, want[0].Fix(), got[0].Fix)
	assert.Equal(t, diagnostic.SeverityError, got[0].Severity)

	for _, d := range ValidateSemantics(mcpTree(mcpRemoteHost, "bearer", "s3cret", true)) {
		assert.NotEqual(t, diagnostic.CodeConfigMCPInvalid, d.Code, "TLS secures the listener: %+v", d)
	}
}
