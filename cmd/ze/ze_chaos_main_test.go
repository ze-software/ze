//go:build ze_chaos

package main

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// chaosRun runs the chaos orchestrator the way `ze chaos ...` runs it: through
// the root handler internal/chaos/orchestrator registers, which ze_chaos_run.go
// imports for its init().
//
// It used to call orchestrator.CLIRun directly. That function is `cLIRun` now,
// unexported, so this file stopped compiling and NOTHING said so: no lint pass
// and no test run had ever selected the `ze_chaos` build
// (plan/journal/gate-excludes-part-of-its-population.md). Going through the
// registry is also the stronger assertion, because it proves the registration
// that `ze chaos` depends on is present in this build.
func chaosRun(t *testing.T, args []string) int {
	t.Helper()
	handler := registry.LookupRoot("chaos")
	require.NotNil(t, handler, "the ze_chaos build registers no `chaos` root handler")
	return handler(nil, args)
}

// TestConfigOnly verifies that --config-only writes config to stdout and exits 0.
//
// VALIDATES: AC-1 -- --config-only --seed 42 --peers 3 exits 0 with valid config.
// PREVENTS: --config-only silently broken after refactor.
func TestConfigOnly(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	code := chaosRun(t, []string{"--config-only", "--seed", "42", "--peers", "3", "--quiet"})

	os.Stdout = old
	require.NoError(t, w.Close())

	data, readErr := io.ReadAll(r)
	require.NoError(t, readErr)
	require.NoError(t, r.Close())

	assert.Equal(t, 0, code)
	assert.Greater(t, len(data), 0, "config output should not be empty")
	assert.Contains(t, string(data), "peer chaos-peer-")
}

// TestConfigOnlyFile verifies that --config-only --config-out writes to a file.
//
// VALIDATES: AC-2 -- --config-only --config-out f.conf writes config to file.
// PREVENTS: File output path broken or permissions wrong.
func TestConfigOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.conf")

	code := chaosRun(t, []string{"--config-only", "--seed", "42", "--peers", "3", "--config-out", path, "--quiet"})

	assert.Equal(t, 0, code)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Greater(t, len(data), 0)
	assert.Contains(t, string(data), "peer chaos-peer-")
}

// TestConfigOnlyDeterministic verifies that the same seed produces identical config.
//
// VALIDATES: AC-3 (partial) -- deterministic config generation.
// PREVENTS: Nondeterminism from map iteration, random, or timestamps.
func TestConfigOnlyDeterministic(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.conf")
	path2 := filepath.Join(dir, "b.conf")

	code1 := chaosRun(t, []string{"--config-only", "--seed", "12345", "--peers", "4", "--config-out", path1, "--quiet"})
	code2 := chaosRun(t, []string{"--config-only", "--seed", "12345", "--peers", "4", "--config-out", path2, "--quiet"})

	assert.Equal(t, 0, code1)
	assert.Equal(t, 0, code2)

	data1, err := os.ReadFile(path1)
	require.NoError(t, err)
	data2, err := os.ReadFile(path2)
	require.NoError(t, err)

	assert.Equal(t, data1, data2, "same seed must produce byte-identical config")
}

// TestConfigOnlyNoNetwork verifies that --config-only does not open TCP connections.
//
// VALIDATES: Critical Review #7 -- --config-only must not start peers or Ze.
// PREVENTS: Side effects in config-only mode.
func TestConfigOnlyNoNetwork(t *testing.T) {
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	connected := make(chan struct{}, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		if closeErr := conn.Close(); closeErr != nil {
			return
		}
		connected <- struct{}{}
	}()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	code := chaosRun(t, []string{
		"--config-only", "--seed", "42", "--peers", "2",
		"--port", itoa(tcpAddr.Port),
		"--quiet",
	})

	assert.Equal(t, 0, code)

	select {
	case <-connected:
		t.Fatal("--config-only should not connect to the port")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestConfigOnlyPipeExclusive verifies --config-only and --pipe are mutually exclusive.
//
// VALIDATES: flag validation logic.
// PREVENTS: ambiguous mode selection.
func TestConfigOnlyPipeExclusive(t *testing.T) {
	code := chaosRun(t, []string{"--config-only", "--pipe", "--seed", "42", "--peers", "2"})
	assert.Equal(t, 1, code)
}

// TestConfigOnlyInProcessExclusive verifies --config-only and --in-process are mutually exclusive.
//
// VALIDATES: flag validation logic.
// PREVENTS: ambiguous mode selection.
func TestConfigOnlyInProcessExclusive(t *testing.T) {
	code := chaosRun(t, []string{"--config-only", "--in-process", "--seed", "42", "--peers", "2"})
	assert.Equal(t, 1, code)
}

// TestRunInvalidPeers verifies that invalid --peers is rejected.
//
// VALIDATES: boundary: peers must be 1-50.
// PREVENTS: panic from zero or negative peer count.
func TestRunInvalidPeers(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"zero", []string{"--config-only", "--seed", "1", "--peers", "0", "--quiet"}},
		{"over-max", []string{"--config-only", "--seed", "1", "--peers", "51", "--quiet"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := chaosRun(t, tt.args)
			assert.Equal(t, 1, code)
		})
	}
}

// TestRunValidPeerBoundaries verifies that boundary-valid --peers values are accepted.
//
// VALIDATES: boundary: peers=1 (minimum) and peers=50 (maximum) both succeed.
// PREVENTS: off-by-one in peer count validation.
func TestRunValidPeerBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		peers string
	}{
		{"min", "1"},
		{"max", "50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := chaosRun(t, []string{"--config-only", "--seed", "1", "--peers", tt.peers, "--quiet"})
			assert.Equal(t, 0, code)
		})
	}
}

// TestRunInvalidChaosRate verifies that invalid --chaos-rate is rejected.
//
// VALIDATES: boundary: chaos-rate must be 0.0-1.0.
// PREVENTS: out-of-range probability causing undefined behavior.
func TestRunInvalidChaosRate(t *testing.T) {
	tests := []struct {
		name string
		rate string
	}{
		{"negative", "-0.1"},
		{"over-one", "1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := chaosRun(t, []string{"--config-only", "--seed", "1", "--peers", "2", "--chaos-rate", tt.rate, "--quiet"})
			assert.Equal(t, 1, code)
		})
	}
}

// TestRunInvalidRouteRate verifies that invalid --route-rate is rejected.
//
// VALIDATES: boundary: route-rate must be 0.0-1.0.
// PREVENTS: out-of-range probability causing undefined behavior.
func TestRunInvalidRouteRate(t *testing.T) {
	tests := []struct {
		name string
		rate string
	}{
		{"negative", "-0.1"},
		{"over-one", "1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := chaosRun(t, []string{"--config-only", "--seed", "1", "--peers", "2", "--route-rate", tt.rate, "--quiet"})
			assert.Equal(t, 1, code)
		})
	}
}

// TestRunInvalidPort verifies that invalid --port is rejected.
//
// VALIDATES: boundary: port must be 0 (auto) or 1024-65535.
// PREVENTS: binding to privileged ports.
func TestRunInvalidPort(t *testing.T) {
	code := chaosRun(t, []string{"--config-only", "--seed", "1", "--peers", "2", "--port", "80", "--quiet"})
	assert.Equal(t, 1, code)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
