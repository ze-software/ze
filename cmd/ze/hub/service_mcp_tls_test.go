//go:build ze_mcp

// Design: docs/architecture/mcp/overview.md -- the MCP listener serves HTTPS or nothing
// Related: service_mcp.go -- startMCPServer and loadMCPTLSConfig, the refusal driven here

package hub

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zemcp "github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/core/opererr"
)

// VALIDATES: an MCP listener handed only one of its two TLS files starts
// nothing, and the refusal is an operator error naming the file it has and
// the setting it lacks.
// PREVENTS: the plain-HTTP fallback. startMCPServer served TLS only when both
// paths were set and trusted config verify to refuse a half pair, so a half
// pair that reached it served plain HTTP over a config that named TLS.
func TestStartMCPServerRefusesHalfTLSPair(t *testing.T) {
	certFile, keyFile := writeMCPTLSPair(t, t.TempDir())

	tests := []struct {
		name        string
		cert        string
		key         string
		wantFile    string
		wantSetting string
	}{
		{"certificate without key", certFile, "", certFile, "environment mcp tls key"},
		{"key without certificate", "", keyFile, keyFile, "environment mcp tls cert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMCPTLSConfig(tt.cert, tt.key)
			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("loadMCPTLSConfig error = %v (%T), want an operator error", err, err)
			}
			if !strings.Contains(refusal.Problem(), tt.wantFile) {
				t.Errorf("problem %q does not name the file %s", refusal.Problem(), tt.wantFile)
			}
			if !strings.Contains(refusal.Fix(), tt.wantSetting) {
				t.Errorf("fix %q does not name the setting %q", refusal.Fix(), tt.wantSetting)
			}

			handle := startMCPServer([]string{"127.0.0.1:0"}, mockMCPDispatch(), mockMCPCommands(),
				zemcp.StreamableConfig{}, tt.cert, tt.key)
			if handle != nil {
				handle.Shutdown(t.Context()) //nolint:errcheck // test cleanup after the failed assertion
				t.Fatal("startMCPServer started over a half TLS pair")
			}
		})
	}
}

// TestStartMCPServerRejectsRemoteBearerWithoutTLS verifies that startMCPServer
// refuses to bind non-loopback addresses for bearer and bearer-list without
// TLS, while accepting loopback without TLS and non-loopback with TLS.
//
// VALIDATES: S2 startup guard mirrors MCPListenConfig.Validate even when
// addresses or tokens come from CLI flags or environment variables.
// PREVENTS: plaintext non-loopback bearer / bearer-list listeners.
func TestStartMCPServerRejectsRemoteBearerWithoutTLS(t *testing.T) {
	certFile, keyFile := writeMCPTLSPair(t, t.TempDir())

	modes := []struct {
		name string
		cfg  zemcp.StreamableConfig
	}{
		{
			name: "bearer",
			cfg:  zemcp.StreamableConfig{AuthMode: zemcp.AuthBearer, Token: "secret"},
		},
		{
			name: "inferred bearer from token",
			cfg:  zemcp.StreamableConfig{Token: "secret"},
		},
		{
			name: "bearer-list",
			cfg: zemcp.StreamableConfig{
				AuthMode:   zemcp.AuthBearerList,
				BearerList: []zemcp.BearerListEntry{{Name: "alice", Token: "alice-secret"}},
			},
		},
	}

	for _, tc := range modes {
		t.Run(tc.name+"/remote without TLS rejected", func(t *testing.T) {
			handle := startMCPServer([]string{"0.0.0.0:0"}, mockMCPDispatch(), mockMCPCommands(), tc.cfg, "", "")
			if handle != nil {
				handle.Shutdown(t.Context()) //nolint:errcheck // test cleanup after failed assertion
				t.Fatalf("startMCPServer started %s on non-loopback without TLS", tc.name)
			}
		})
		t.Run(tc.name+"/loopback without TLS allowed", func(t *testing.T) {
			handle := startMCPServer([]string{"127.0.0.1:0"}, mockMCPDispatch(), mockMCPCommands(), tc.cfg, "", "")
			require.NotNil(t, handle, "loopback %s without TLS must start", tc.name)
			t.Cleanup(func() { handle.Shutdown(t.Context()) }) //nolint:errcheck // test cleanup
		})
		t.Run(tc.name+"/remote with TLS allowed", func(t *testing.T) {
			handle := startMCPServer([]string{"0.0.0.0:0"}, mockMCPDispatch(), mockMCPCommands(), tc.cfg, certFile, keyFile)
			require.NotNil(t, handle, "remote %s with TLS must start", tc.name)
			t.Cleanup(func() { handle.Shutdown(t.Context()) }) //nolint:errcheck // test cleanup
		})
	}
}

// TestMCPServerReconfigureRejectsRemoteBearerWithoutTLS verifies that
// Reconfigure refuses to migrate a plaintext bearer or bearer-list MCP server
// off loopback, keeps its existing listener intact, still permits loopback
// rebinds, and allows non-loopback migration when TLS is enabled.
//
// VALIDATES: S2 reload/reconfigure guard prevents a config reload from moving
// a loopback plaintext bearer/bearer-list listener onto a non-loopback address.
// PREVENTS: bypassing the startup/validation TLS requirement via SIGHUP or
// commit reload.
func TestMCPServerReconfigureRejectsRemoteBearerWithoutTLS(t *testing.T) {
	certFile, keyFile := writeMCPTLSPair(t, t.TempDir())

	modes := []struct {
		name string
		cfg  zemcp.StreamableConfig
	}{
		{
			name: "bearer",
			cfg:  zemcp.StreamableConfig{AuthMode: zemcp.AuthBearer, Token: "secret"},
		},
		{
			name: "bearer-list",
			cfg: zemcp.StreamableConfig{
				AuthMode:   zemcp.AuthBearerList,
				BearerList: []zemcp.BearerListEntry{{Name: "alice", Token: "alice-secret"}},
			},
		},
	}

	for _, tc := range modes {
		t.Run(tc.name+"/plaintext refuses remote rebind", func(t *testing.T) {
			handle := startMCPServer([]string{"127.0.0.1:0"}, mockMCPDispatch(), mockMCPCommands(), tc.cfg, "", "")
			require.NotNil(t, handle)
			t.Cleanup(func() { handle.Shutdown(t.Context()) }) //nolint:errcheck // test cleanup

			initial := handle.Addresses()
			err := handle.Reconfigure(t.Context(), []string{"0.0.0.0:0"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "requires tls.cert and tls.key on non-loopback listener")
			assert.Equal(t, initial, handle.Addresses(), "refused reconfigure must keep prior bound addresses")

			// Rebinding onto another loopback address remains allowed.
			require.NoError(t, handle.Reconfigure(t.Context(), []string{"127.0.0.1:0"}))
		})

		t.Run(tc.name+"/TLS allows remote rebind", func(t *testing.T) {
			handle := startMCPServer([]string{"127.0.0.1:0"}, mockMCPDispatch(), mockMCPCommands(), tc.cfg, certFile, keyFile)
			require.NotNil(t, handle)
			t.Cleanup(func() { handle.Shutdown(t.Context()) }) //nolint:errcheck // test cleanup

			require.NoError(t, handle.Reconfigure(t.Context(), []string{"0.0.0.0:0"}))
			require.Len(t, handle.Addresses(), 1)
		})
	}
}
