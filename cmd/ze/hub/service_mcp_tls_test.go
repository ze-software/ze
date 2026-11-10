//go:build ze_mcp

// Design: docs/architecture/mcp/overview.md -- the MCP listener serves HTTPS or nothing
// Related: service_mcp.go -- startMCPServer and loadMCPTLSConfig, the refusal driven here

package hub

import (
	"errors"
	"strings"
	"testing"

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
