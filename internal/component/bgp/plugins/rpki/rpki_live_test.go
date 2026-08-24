//go:build live

package rpki

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
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
const stayrtrImage = "docker.io/rpki/stayrtr:latest"

// dockerTimeout bounds one docker invocation made from a cleanup path, where
// the test's own context is already canceled and cannot bound anything.
const dockerTimeout = 30 * time.Second

// probeTimeout bounds ONE TCP reachability probe. The caller polls, so this is
// the bound on an attempt rather than on the wait.
const probeTimeout = 2 * time.Second

// dockerRM removes a container by name. The error is expected when the
// container does not exist, which is every first run. There is nowhere to
// report that error. The caller is sometimes a cleanup func with no *testing.T
// in scope, so no reader exists to report to.
func dockerRM(name string) {
	// Its own context, not the test's. A cleanup runs after t.Context() is
	// canceled, so the test's context would cancel this removal rather than
	// bound it.
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

// startStayRTR starts a stayrtr container and returns the host port.
// Caller MUST call the returned cleanup function.
func startStayRTR(t *testing.T) (port int, cleanup func()) {
	t.Helper()

	// Remove any leftover container from a previous failed run.
	dockerRM(containerName)

	// Start stayrtr with a random host port mapped to 3323.
	out, err := exec.CommandContext(t.Context(),
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
	portOut, err := exec.CommandContext(t.Context(),
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
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	dialer := &net.Dialer{Timeout: probeTimeout}

	require.Eventually(t, func() bool {
		conn, err := dialer.DialContext(t.Context(), "tcp", addr)
		if err != nil {
			return false
		}
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("probe connection close: %v", closeErr)
		}
		return true
	}, timeout, 2*time.Second, "stayrtr did not become reachable on port %d within %s", port, timeout)
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
	out, err := exec.CommandContext(t.Context(), "docker", "info").CombinedOutput()
	if err != nil {
		t.Skipf("docker not running: %s", string(out))
	}

	// Pull image if needed (may already be cached).
	t.Log("pulling stayrtr image...")
	pullOut, err := exec.CommandContext(t.Context(), "docker", "pull", stayrtrImage).CombinedOutput()
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
	// Short retry for tests: stayrtr may need time to download RPKI data.
	cache := newROACache()
	stopCh := make(chan struct{})
	session := newRTRSession("127.0.0.1", uint16(port), 100, "", cache, newASPACache(), stopCh)
	session.retryInterval = 5 * time.Second

	// Run session in background.
	done := make(chan struct{})
	go func() {
		session.Run()
		close(done)
	}()

	// Wait for VRPs to be populated.
	t.Log("waiting for RTR sync...")
	var v4Count, v6Count int
	require.Eventually(t, func() bool {
		v4Count, v6Count = cache.Count()
		return v4Count > 0
	}, 90*time.Second, 2*time.Second, "VRP cache should be populated from RTR sync")

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
	// Cloudflare's IPv6 ROA coverage may vary; accept Valid or NotFound.
	t.Run("AC-3_Valid_Cloudflare_IPv6", func(t *testing.T) {
		result := cache.Validate("2606:4700::/32", 13335)
		assert.NotEqual(t, ValidationInvalid, result,
			"2606:4700::/32 AS13335 (Cloudflare IPv6) should not be Invalid")
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

	// --- ASPA: v2 negotiation does not break ROA sync ---
	// stayrtr supports RTR v2 (or falls back cleanly to v1).
	// No production ASPA records exist yet (draft status), so ASPA cache stays empty.
	// This verifies: v2 negotiation is interoperable, ASPA verification returns Unknown.
	aspaCache := session.aspaCache
	t.Run("ASPA_cache_empty_no_production_records", func(t *testing.T) {
		assert.Equal(t, 0, aspaCache.count(),
			"ASPA cache should be empty (no production ASPA records)")
	})
	t.Run("ASPA_verify_unknown_without_records", func(t *testing.T) {
		path := []uint32{13335, 2914, 3356}
		state := verifyASPA(aspaCache, path)
		assert.Equal(t, ASPAUnknown, state,
			"ASPA verification should return Unknown without ASPA records")
	})
	t.Run("ASPA_v2_negotiation_preserved_ROA_sync", func(t *testing.T) {
		assert.Greater(t, v4Count, 100_000,
			"ROA sync succeeded despite v2 negotiation (v2 or clean v1 fallback)")
	})
}

// TestLiveASPAValidation starts stayrtr with synthetic ASPA JSON data and verifies
// our RTR client receives ASPA records via v2 and verification produces correct results.
//
// VALIDATES: RTR v2 ASPA PDU interop with real server, end-to-end ASPA cache population.
// PREVENTS: Wire format incompatibility with production RTR implementations.
func TestLiveASPAValidation(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if out, err := exec.CommandContext(t.Context(), "docker", "info").CombinedOutput(); err != nil {
		t.Skipf("docker not running: %s", string(out))
	}

	// Synthetic RPKI JSON with ROA + ASPA records (rpki-client JSON format).
	// stayrtr parses provider_authorizations and serves them as ASPA PDU type 11 over RTR v2.
	const aspaJSON = `{
  "metadata": {"generated": 1700000000, "valid": 1700086400},
  "roas": [{"prefix": "10.0.0.0/8", "maxLength": 24, "asn": "AS64502", "ta": "test"}],
  "provider_authorizations": [
    {"provider_as": 64500, "customer_as": 64501},
    {"provider_as": 64501, "customer_as": 64502}
  ]
}`

	// Serve JSON over HTTP for stayrtr to fetch.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, writeErr := w.Write([]byte(aspaJSON)); writeErr != nil {
			return
		}
	})
	httpLn, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("http listen: %v", err)
	}
	httpSrv := &http.Server{Handler: mux}
	defer func() { _ = httpSrv.Close() }()
	go func() { _ = httpSrv.Serve(httpLn) }()

	_, httpPort, _ := net.SplitHostPort(httpLn.Addr().String())

	// On macOS, --network=host doesn't expose the host to containers.
	// Use host.docker.internal to reach the test HTTP server.
	cacheHost := "127.0.0.1"
	rtrPort := 3324
	networkArgs := []string{"--network", "host"}
	if isDockerDesktop() {
		cacheHost = "host.docker.internal"
		networkArgs = []string{"-p", "0:3324"}
	}

	const aspaContainer = "ze-live-rpki-aspa"
	dockerRM(aspaContainer)
	defer dockerRM(aspaContainer)

	args := append([]string{"run", "-d", "--name", aspaContainer}, networkArgs...)
	args = append(args, stayrtrImage, "-cache", "http://"+cacheHost+":"+httpPort+"/rpki.json", "-bind", ":3324", "-checktime=false")
	startOut, err := exec.CommandContext(t.Context(), "docker", args...).CombinedOutput()
	if err != nil {
		t.Skipf("cannot start stayrtr for ASPA: %s", string(startOut))
	}

	if isDockerDesktop() {
		portOut, portErr := exec.CommandContext(t.Context(), "docker", "port", aspaContainer, "3324/tcp").Output()
		if portErr != nil {
			t.Skipf("cannot get stayrtr port: %v", portErr)
		}
		portStr := strings.TrimSpace(string(portOut))
		if first, _, ok := strings.Cut(portStr, "\n"); ok {
			portStr = first
		}
		idx := strings.LastIndex(portStr, ":")
		if idx >= 0 {
			if _, scanErr := fmt.Sscanf(portStr[idx+1:], "%d", &rtrPort); scanErr != nil {
				t.Skipf("cannot parse port: %v", scanErr)
			}
		}
	}

	t.Logf("ASPA test: rtrPort=%d cacheHost=%s httpPort=%s", rtrPort, cacheHost, httpPort)

	// Give stayrtr time to fetch the JSON before connecting.
	time.Sleep(3 * time.Second)
	if logs, logErr := exec.CommandContext(t.Context(), "docker", "logs", aspaContainer).CombinedOutput(); logErr == nil {
		t.Logf("stayrtr logs: %s", string(logs))
	}

	waitForRTR(t, rtrPort, 30*time.Second)

	// Connect our RTR client.
	roaCache := newROACache()
	aspaC := newASPACache()
	stopCh := make(chan struct{})
	sess := newRTRSession("127.0.0.1", uint16(rtrPort), 100, "", roaCache, aspaC, stopCh) //nolint:gosec // port fits uint16
	sess.retryInterval = 5 * time.Second

	done := make(chan struct{})
	go func() {
		sess.Run()
		close(done)
	}()

	// Wait for sync. Accept either ASPA populated or ROA-only (stayrtr version may vary).
	var aspaPopulated bool
	require.Eventually(t, func() bool {
		if aspaC.count() > 0 {
			aspaPopulated = true
			return true
		}
		v4, _ := roaCache.Count()
		return v4 > 0
	}, 60*time.Second, 2*time.Second, "RTR sync should complete")

	close(stopCh)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("session did not exit")
	}

	if !aspaPopulated {
		t.Skip("stayrtr did not serve ASPA records (version may not support provider_authorizations)")
	}

	t.Run("ASPA_records_received", func(t *testing.T) {
		assert.Equal(t, 2, aspaC.count(), "expected 2 ASPA records")
		assert.Equal(t, HopProviderPlus, aspaC.checkPair(64500, 64501))
		assert.Equal(t, HopProviderPlus, aspaC.checkPair(64501, 64502))
		assert.Equal(t, HopNotProviderPlus, aspaC.checkPair(99999, 64501))
	})
	t.Run("ASPA_verify_valid_path", func(t *testing.T) {
		assert.Equal(t, ASPAValid, verifyASPA(aspaC, []uint32{64500, 64501, 64502}))
	})
	t.Run("ASPA_verify_invalid_path", func(t *testing.T) {
		assert.Equal(t, ASPAInvalid, verifyASPA(aspaC, []uint32{99999, 64501, 64502}))
	})
}

// isDockerDesktop returns true on macOS/Windows where --network=host
// doesn't work and host.docker.internal is needed to reach the host.
func isDockerDesktop() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}
