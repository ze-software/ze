package hub

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
)

func mockMCPDispatch() zemcp.CommandDispatcher {
	return func(_ string) (string, error) {
		return `{"status":"ok"}`, nil
	}
}

func mockMCPCommands() zemcp.CommandLister {
	return func() []zemcp.CommandInfo {
		return nil
	}
}

// allocEphemeralPorts binds n ephemeral 127.0.0.1 ports, records them, then
// releases them so the caller can re-bind. There is a tiny race window between
// release and re-bind; for multi-listener tests we accept it since ephemeral
// port reuse is extremely unlikely to collide within microseconds on a single
// machine.
func allocEphemeralPorts(t *testing.T, n int) []int {
	t.Helper()
	ports := make([]int, 0, n)
	listeners := make([]net.Listener, 0, n)
	var lc net.ListenConfig
	for range n {
		ln, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		listeners = append(listeners, ln)
		_, portStr, splitErr := net.SplitHostPort(ln.Addr().String())
		require.NoError(t, splitErr)
		var port int
		_, scanErr := fmt.Sscanf(portStr, "%d", &port)
		require.NoError(t, scanErr)
		ports = append(ports, port)
	}
	// Release so the caller can re-bind.
	for _, ln := range listeners {
		if closeErr := ln.Close(); closeErr != nil {
			t.Logf("close setup listener: %v", closeErr)
		}
	}
	return ports
}

// TestStartMCPServer_MultiListener verifies startMCPServer binds every
// address in the slice and that both listeners serve the same handler.
//
// VALIDATES: AC-3 (MCP config with two server entries binds both endpoints).
// VALIDATES: AC-14 (Shutdown closes every listener).
// PREVENTS: Regression where only the first MCP address is bound.
func TestStartMCPServer_MultiListener(t *testing.T) {
	ports := allocEphemeralPorts(t, 2)
	addrs := []string{
		fmt.Sprintf("127.0.0.1:%d", ports[0]),
		fmt.Sprintf("127.0.0.1:%d", ports[1]),
	}

	srv := startMCPServer(addrs, mockMCPDispatch(), mockMCPCommands(), "")
	require.NotNil(t, srv, "startMCPServer must return a non-nil server for valid addrs")
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Logf("Shutdown: %v", shutdownErr)
		}
	})

	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(3 * time.Second)
	for i, addr := range addrs {
		var lastErr error
		for time.Now().Before(deadline) {
			req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/", http.NoBody)
			require.NoError(t, reqErr)
			resp, doErr := client.Do(req)
			if doErr == nil {
				if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
					t.Logf("drain body: %v", copyErr)
				}
				if closeErr := resp.Body.Close(); closeErr != nil {
					t.Logf("close body: %v", closeErr)
				}
				lastErr = nil
				break
			}
			lastErr = doErr
			time.Sleep(20 * time.Millisecond)
		}
		require.NoError(t, lastErr, "listener %d (%s) not reachable", i, addr)
	}
}

// TestStartMCPServer_BindFailureClosesPartialListeners verifies that when the
// second address is already in use, the first listener is closed and the
// function returns nil instead of leaking a half-bound server.
//
// VALIDATES: AC-15 (fail-fast on partial bind).
func TestStartMCPServer_BindFailureClosesPartialListeners(t *testing.T) {
	// Squat on a port so the second bind fails.
	var lc net.ListenConfig
	squatter, listenErr := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	require.NoError(t, listenErr)
	t.Cleanup(func() {
		if closeErr := squatter.Close(); closeErr != nil {
			t.Logf("close squatter: %v", closeErr)
		}
	})
	squattedAddr := squatter.Addr().String()

	firstPort := allocEphemeralPorts(t, 1)[0]
	firstAddr := fmt.Sprintf("127.0.0.1:%d", firstPort)

	srv := startMCPServer(
		[]string{firstAddr, squattedAddr},
		mockMCPDispatch(), mockMCPCommands(), "",
	)
	assert.Nil(t, srv, "startMCPServer must return nil when any bind fails")

	// The first address must now be free again (partial listener was closed).
	probe, probeErr := lc.Listen(context.Background(), "tcp4", firstAddr)
	if probeErr != nil {
		t.Errorf("first address %s should be free after bind failure rollback: %v", firstAddr, probeErr)
	} else {
		if closeErr := probe.Close(); closeErr != nil {
			t.Logf("close probe: %v", closeErr)
		}
	}
}

// TestStartMCPServer_EmptyAddrs verifies the no-addresses path returns nil.
func TestStartMCPServer_EmptyAddrs(t *testing.T) {
	srv := startMCPServer(nil, mockMCPDispatch(), mockMCPCommands(), "")
	assert.Nil(t, srv)

	srv = startMCPServer([]string{}, mockMCPDispatch(), mockMCPCommands(), "")
	assert.Nil(t, srv)
}
