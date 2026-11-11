// Design: docs/guide/operations.md -- verified remote SSH connections.

package client

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
)

func TestCLIHostKeyRefusalKeepsProducerRepair(t *testing.T) {
	cause := errors.New("knownhosts: key mismatch")
	refusal := opererr.Wrap(cause, "The SSH host key changed for router.example:2222", "Verify the offered fingerprint with the device operator", "ssh-keygen -F '[router.example]:2222' -f '/trusted/known_hosts'")
	wrapped := fmt.Errorf("connect to router.example:2222: %w", refusal)
	got := daemonUnreachable("router.example", "2222", wrapped)
	if got != wrapped {
		t.Fatalf("connection mapper replaced an actionable refusal: %v", got)
	}
	var typed *opererr.Error
	if !errors.As(got, &typed) || typed != refusal || !errors.Is(got, cause) {
		t.Fatalf("connection mapper lost the producer or its cause: %v", got)
	}
	if strings.Count(got.Error(), "fix: ") != 1 || !strings.HasSuffix(got.Error(), "fix: "+refusal.Fix()) {
		t.Fatalf("connection mapper replaced the final trust repair: %v", got)
	}
}

func TestCLIHostKeyRefusalReachesRunUnchanged(t *testing.T) {
	root := t.TempDir()
	trustPath := filepath.Join(root, "missing trust", "known_hosts")
	for key, value := range map[string]string{
		"ze.config.dir":      filepath.Join(root, "missing-store"),
		"ze.ssh.known.hosts": trustPath,
		"ze.ssh.insecure":    "false",
		"ze.ssh.password":    "explicit-test-password",
	} {
		t.Setenv(key, value)
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	var code int
	stderr := captureOutput(t, true, func() {
		code = Run([]string{"--remote", "router.example:2222", "--user", "operator", "-c", "show version"})
	})
	if code != 1 {
		t.Fatalf("untrusted remote Run exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "SSH host-key trust file could not be loaded: "+trustPath) || !strings.Contains(stderr, "StrictHostKeyChecking=ask") {
		t.Fatalf("Run lost the trust producer's reason or repair: %s", stderr)
	}
	if strings.Count(stderr, "fix: ") != 1 || strings.Contains(stderr, "ze status") || strings.Contains(stderr, "ze start") {
		t.Fatalf("Run replaced host trust repair with daemon startup guidance: %s", stderr)
	}
}
