// Design: docs/guide/operations.md -- the real CLI proof's decisions.

package fixture

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestDeploymentRemoteTrustEnvironmentRemovesInheritedSpellings(t *testing.T) {
	base := []string{"PATH=/runner/bin", "HOME=/operator", "ZE_SSH_PASSWORD=old", "ze.ssh.password=dotted", "Ze-SsH-InSeCuRe=true", "ZE_CONFIG_DIR=/operator/store", "ze.plugin.mode=declare", "ZE_TEST_SSH_SKIP_AUTH=true"}
	original := slices.Clone(base)
	got := remoteTrustEnvironment(base, map[string]string{"HOME": "/private/home", "ZE_SSH_PASSWORD": "proof-password", "ZE_SSH_KNOWN_HOSTS": "/private/trusted hosts", "ZE_SSH_INSECURE": "false"})
	for key, want := range map[string]string{"HOME": "/private/home", "ze.ssh.password": "proof-password", "ze.ssh.known.hosts": "/private/trusted hosts", "ze.ssh.insecure": "false", "ze.config.dir": "", "ze.plugin.mode": "", "ze.test.ssh.skip.auth": "", "PATH": "/runner/bin"} {
		if value := env.LookupIn(got, key); value != want {
			t.Fatalf("private child env %s = %q, want %q", key, value, want)
		}
	}
	if !slices.Equal(base, original) {
		t.Fatal("fixture changed the parent environment")
	}
}

func TestDeploymentRemoteTrustVersionVerdictRequiresSuccessfulDaemonJSON(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result fixture10ProcessResult
		accept bool
	}{
		{"version", fixture10ProcessResult{code: 0, stdout: `{"version":"Ze proof"}`}, true},
		{"failed command", fixture10ProcessResult{code: 1, stdout: `{"version":"Ze proof"}`}, false},
		{"empty answer", fixture10ProcessResult{code: 0}, false},
		{"non-JSON answer", fixture10ProcessResult{code: 0, stdout: "OK"}, false},
		{"missing version", fixture10ProcessResult{code: 0, stdout: `{"status":"done"}`}, false},
		{"empty version", fixture10ProcessResult{code: 0, stdout: `{"version":""}`}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := remoteTrustVersionVerdict(tc.result); (err == nil) != tc.accept {
				t.Fatalf("verdict err=%v, want accept=%v", err, tc.accept)
			}
		})
	}
}

func TestDeploymentRemoteTrustRefusalVerdictRequiresTheSpecificRefusal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result fixture10ProcessResult
		accept bool
	}{
		{"refusal", fixture10ProcessResult{code: 1, stderr: "SSH host key changed for device"}, true},
		{"accepted", fixture10ProcessResult{code: 0, stderr: "SSH host key changed for device"}, false},
		{"different failure", fixture10ProcessResult{code: 1, stderr: "cannot load credentials"}, false},
		{"crash", fixture10ProcessResult{code: 2, stderr: "SSH host key changed for device"}, false},
		{"answered despite refusal", fixture10ProcessResult{code: 1, stdout: `{"version":"Ze proof"}`, stderr: "SSH host key changed for device"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := remoteTrustRefusalVerdict(tc.result, "SSH host key changed"); (err == nil) != tc.accept {
				t.Fatalf("verdict err=%v, want accept=%v", err, tc.accept)
			}
		})
	}
}
