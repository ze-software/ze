//go:build live

package rpki

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLiveRTRv2DowngradeToV1 starts a stayrtr container (protocol v1 default),
// connects with our RTR client (starts at v2), and verifies the v2 -> error 4 ->
// v1 fallback path works and VRPs are synced successfully.
//
// VALIDATES: RTR v2 negotiation with v1 fallback against a real server.
// PREVENTS: Version downgrade logic broken by ASPA v2 changes.
func TestLiveRTRv2DowngradeToV1(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if out, err := exec.CommandContext(t.Context(), "docker", "info").CombinedOutput(); err != nil {
		t.Skipf("docker not running: %s", string(out))
	}

	t.Log("pulling stayrtr image...")
	if out, err := exec.CommandContext(t.Context(), "docker", "pull", stayrtrImage).CombinedOutput(); err != nil {
		t.Skipf("cannot pull stayrtr: %s", string(out))
	}

	const name = "ze-live-rpki-downgrade"
	dockerRM(name)
	defer dockerRM(name)

	out, err := exec.CommandContext(t.Context(),
		"docker", "run", "-d",
		"--name", name,
		"-p", "0:3323",
		stayrtrImage,
		"-cache", rpkiDataURL,
		"-bind", ":3323",
	).CombinedOutput()
	require.NoError(t, err, "docker run failed: %s", string(out))

	portOut, err := exec.CommandContext(t.Context(), "docker", "port", name, "3323/tcp").Output()
	require.NoError(t, err, "docker port failed")

	port := parseDockerPort(t, string(portOut))
	t.Logf("waiting for stayrtr on port %d...", port)
	waitForRTR(t, port, 60*time.Second)

	cache := newROACache()
	stopCh := make(chan struct{})
	session := newRTRSession("127.0.0.1", uint16(port), 100, "", cache, newASPACache(), stopCh) //nolint:gosec // port fits uint16
	session.retryInterval = 5 * time.Second

	done := make(chan struct{})
	go func() {
		session.Run()
		close(done)
	}()

	t.Log("waiting for v2->v1 downgrade and VRP sync...")
	require.Eventually(t, func() bool {
		v4, _ := cache.Count()
		return v4 > 0
	}, 90*time.Second, 2*time.Second, "VRPs should populate via v2->v1 downgrade")

	close(stopCh)
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("session did not exit")
	}

	assert.Equal(t, uint8(1), session.version, "session should have downgraded to v1")

	v4, v6 := cache.Count()
	assert.Greater(t, v4, 100_000, "expected >100K IPv4 VRPs")
	assert.Greater(t, v6, 10_000, "expected >10K IPv6 VRPs")
	t.Logf("v2->v1 downgrade: synced %d IPv4 + %d IPv6 VRPs", v4, v6)
}

// parseDockerPort extracts the host port from `docker port` output.
func parseDockerPort(t *testing.T, output string) int {
	t.Helper()
	portStr := strings.TrimSpace(output)
	if first, _, ok := strings.Cut(portStr, "\n"); ok {
		portStr = first
	}
	idx := strings.LastIndex(portStr, ":")
	require.Greater(t, idx, 0, "cannot parse port: %s", portStr)
	var port int
	_, err := fmt.Sscanf(portStr[idx+1:], "%d", &port)
	require.NoError(t, err)
	return port
}
