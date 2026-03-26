//go:build live

package rpki

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// containerName is the Docker container name used by live tests.
const containerName = "ze-live-rpki-stayrtr"

// rpkiDataURL is Cloudflare's public RPKI JSON endpoint.
const rpkiDataURL = "https://rpki.cloudflare.com/rpki.json"

// stayrtrImage is the official stayrtr container image.
const stayrtrImage = "ghcr.io/bgp/stayrtr:latest"

// dockerRM removes a container by name (best-effort cleanup, errors expected
// when container does not exist).
func dockerRM(name string) {
	if err := exec.Command("docker", "rm", "-f", name).Run(); err != nil {
		// Expected when container doesn't exist (e.g., first run). Cannot log
		// because this is called outside test context (cleanup func).
	}
}

// startStayRTR starts a stayrtr container and returns the host port.
// Caller MUST call the returned cleanup function.
func startStayRTR(t *testing.T) (port int, cleanup func()) {
	t.Helper()

	// Remove any leftover container from a previous failed run.
	dockerRM(containerName)

	// Start stayrtr with a random host port mapped to 3323.
	out, err := exec.Command(
		"docker", "run", "-d",
		"--name", containerName,
		"-p", "0:3323",
		stayrtrImage,
		"-cache", rpkiDataURL,
		"-bind", ":3323",
	).CombinedOutput()
	require.NoError(t, err, "docker run failed: %s", string(out))

	cleanup = func() { dockerRM(containerName) }

	// Discover the mapped host port.
	portOut, err := exec.Command(
		"docker", "port", containerName, "3323/tcp",
	).Output()
	if err != nil {
		cleanup()
		t.Fatalf("docker port failed: %v", err)
	}

	// Parse "0.0.0.0:NNNNN" or "[::]:NNNNN". Docker may return multiple lines
	// (IPv4 + IPv6) on dual-stack hosts; take the first line only.
	portStr := strings.TrimSpace(string(portOut))
	if first, _, ok := strings.Cut(portStr, "\n"); ok {
		portStr = first
	}
	idx := strings.LastIndex(portStr, ":")
	if idx < 0 {
		cleanup()
		t.Fatalf("cannot parse docker port output: %s", portStr)
	}
	portPart := portStr[idx+1:]

	var p int
	_, err = fmt.Sscanf(portPart, "%d", &p)
	if err != nil {
		cleanup()
		t.Fatalf("cannot parse port number %q: %v", portPart, err)
	}

	return p, cleanup
}

// waitForRTR waits until a TCP connection to the given port succeeds.
func waitForRTR(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				t.Logf("probe connection close: %v", closeErr)
			}
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("stayrtr did not become reachable on port %d within %s", port, timeout)
}

// TestLiveRPKIValidation connects to a stayrtr container serving live RPKI data
// and validates known prefixes against the real-world ROA set.
//
// VALIDATES: AC-1 (RTR sync), AC-2 (NotFound), AC-3 (Valid), AC-4 (Invalid).
// PREVENTS: RTR client incompatibility with real cache servers; validation logic
// errors that only appear with real-world data scale (~300K VRPs).
func TestLiveRPKIValidation(t *testing.T) {
	// Check Docker is available.
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available, skipping live RPKI test")
	}
	out, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil {
		t.Skipf("docker not running: %s", string(out))
	}

	// Pull image if needed (may already be cached).
	t.Log("pulling stayrtr image...")
	pullOut, err := exec.Command("docker", "pull", stayrtrImage).CombinedOutput()
	if err != nil {
		t.Skipf("cannot pull stayrtr image (no internet?): %s", string(pullOut))
	}

	// Start stayrtr container.
	t.Log("starting stayrtr container...")
	port, cleanup := startStayRTR(t)
	defer cleanup()

	// Wait for stayrtr to accept TCP connections.
	// It needs time to fetch and parse rpki.json (~10-30s depending on network).
	t.Logf("waiting for stayrtr on port %d...", port)
	waitForRTR(t, port, 60*time.Second)

	// Create ROA cache and RTR session.
	cache := NewROACache()
	stopCh := make(chan struct{})
	session := NewRTRSession("127.0.0.1", uint16(port), 100, cache, stopCh)

	// Run session in background.
	done := make(chan struct{})
	go func() {
		session.Run()
		close(done)
	}()

	// Wait for VRPs to be populated.
	t.Log("waiting for RTR sync...")
	syncDeadline := time.Now().Add(90 * time.Second)
	var v4Count, v6Count int
	for time.Now().Before(syncDeadline) {
		v4Count, v6Count = cache.Count()
		if v4Count > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// Stop session after sync (we only need the cache populated).
	close(stopCh)
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("session.Run() did not exit within 30s of stop signal")
	}

	// --- AC-1: RTR session established, VRPs populated ---
	// Checked in the parent function with require (not assert in a subtest) so
	// that a sync failure aborts all downstream subtests. Without this, AC-2
	// would false-pass against an empty cache (NotFound == empty cache).
	t.Run("AC-1_VRPs_populated", func(t *testing.T) {
		t.Logf("VRP counts: IPv4=%d, IPv6=%d", v4Count, v6Count)
	})
	require.Greater(t, v4Count, 100_000,
		"expected > 100K IPv4 VRPs from live RPKI data")
	require.Greater(t, v6Count, 10_000,
		"expected > 10K IPv6 VRPs from live RPKI data")

	// --- AC-2: NotFound for uncovered prefix ---
	t.Run("AC-2_NotFound_uncovered_prefix", func(t *testing.T) {
		result := cache.Validate("82.212.0.0/16", 64496)
		assert.Equal(t, ValidationNotFound, result,
			"82.212.0.0/16 should have no ROA coverage (NotFound)")
	})

	// --- AC-3: Valid for known RPKI-valid prefixes ---
	t.Run("AC-3_Valid_Cloudflare", func(t *testing.T) {
		result := cache.Validate("1.1.1.0/24", 13335)
		assert.Equal(t, ValidationValid, result,
			"1.1.1.0/24 AS13335 (Cloudflare) should be Valid")
	})
	t.Run("AC-3_Valid_Google", func(t *testing.T) {
		result := cache.Validate("8.8.8.0/24", 15169)
		assert.Equal(t, ValidationValid, result,
			"8.8.8.0/24 AS15169 (Google) should be Valid")
	})

	// --- AC-3: Valid for known RPKI-valid IPv6 prefix ---
	t.Run("AC-3_Valid_Cloudflare_IPv6", func(t *testing.T) {
		result := cache.Validate("2606:4700::/32", 13335)
		assert.Equal(t, ValidationValid, result,
			"2606:4700::/32 AS13335 (Cloudflare IPv6) should be Valid")
	})

	// --- AC-4: Invalid for covered prefix with wrong origin ---
	t.Run("AC-4_Invalid_wrong_origin", func(t *testing.T) {
		result := cache.Validate("1.1.1.0/24", 64496)
		assert.Equal(t, ValidationInvalid, result,
			"1.1.1.0/24 AS64496 should be Invalid (covered, wrong origin)")
	})

	// --- AC-4: Invalid due to maxLength exceeded ---
	t.Run("AC-4_Invalid_max_length_exceeded", func(t *testing.T) {
		// Cloudflare's ROA for 1.1.1.0/24 has maxLength=24.
		// A /25 more-specific exceeds maxLength even with the correct origin.
		result := cache.Validate("1.1.1.0/25", 13335)
		assert.Equal(t, ValidationInvalid, result,
			"1.1.1.0/25 AS13335 should be Invalid (maxLength=24 exceeded)")
	})
}
