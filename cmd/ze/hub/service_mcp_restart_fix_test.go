//go:build ze_mcp

// Design: docs/architecture/mcp/overview.md -- the MCP listener serves HTTPS or nothing
// Related: service_mcp.go -- loadMCPTLSConfig, whose half-pair refusal names the restart

package hub

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: the fix of the MCP half-TLS-pair refusal names the restart for this
// process: `sudo systemctl restart ze` when ze runs under ze.service, and the
// stop-and-start steps (ze signal stop, then ze start) for a ze started another way,
// with no systemctl. Neither names `ze signal restart`, which exits without starting
// a new process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, then read
// the fix of loadMCPTLSConfig over a key without a certificate and a certificate
// without a key.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves MCP down with it, or told to run systemctl for a ze systemd did
// not start.
func TestMCPTLSFixNamesTheRestartForThisProcess(t *testing.T) {
	producers := []struct {
		name  string
		cert  string
		key   string
		exact string
	}{
		{
			name:  "key without certificate",
			key:   "/etc/ze/mcp.key",
			exact: "set environment mcp tls cert to the certificate of that key, then restart ze: sudo systemctl restart ze",
		},
		{
			name:  "certificate without key",
			cert:  "/etc/ze/mcp.pem",
			exact: "set environment mcp tls key to the private key of that certificate, then restart ze: sudo systemctl restart ze",
		},
	}
	fix := func(t *testing.T, cert, key string) string {
		t.Helper()
		_, err := loadMCPTLSConfig(cert, key)
		var refusal *opererr.Error
		if !errors.As(err, &refusal) {
			t.Fatalf("loadMCPTLSConfig error = %v (%T), want an operator error", err, err)
		}
		return refusal.Fix()
	}
	for _, producer := range producers {
		t.Run(producer.name+"/ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
			if got := fix(t, producer.cert, producer.key); got != producer.exact {
				t.Fatalf("under ze.service the fix is %q, want %q", got, producer.exact)
			}
		})
		t.Run(producer.name+"/other", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
			got := fix(t, producer.cert, producer.key)
			if strings.Contains(got, "systemctl") {
				t.Errorf("outside ze.service the fix names systemctl: %q", got)
			}
			if strings.Contains(got, "ze signal restart") {
				t.Errorf("the fix names ze signal restart, which leaves ze stopped: %q", got)
			}
			for _, want := range []string{"ze signal stop", "ze start"} {
				if !strings.Contains(got, want) {
					t.Errorf("outside ze.service the fix does not name %q: %q", want, got)
				}
			}
		})
	}
}
