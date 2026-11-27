// Design: docs/guide/chaos-testing.md -- the orchestrator options reached through le chaos run

package chaosrun

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
)

// chaosRun runs the chaos orchestrator the way `le chaos run ...` runs it:
// through Answer, the entry this package registers, so each test proves the
// command a developer types and not only the orchestrator behind it.
func chaosRun(t *testing.T, args []string) int {
	t.Helper()
	_, code := Answer(args)
	return code
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

// TestRunIfaceFaultFlags verifies valid and invalid interface fault flags.
//
// VALIDATES: --fault-iface, --fault-addr, --fault-iface-cycles, --fault-iface-interval,
// and --peer-local-addr validation at CLI entry.
// PREVENTS: Silent no-ops or runtime panics from malformed CIDR, missing interface,
// out-of-range cycles, non-positive interval, or invalid peer source IP.
func TestRunIfaceFaultFlags(t *testing.T) {
	valid := chaosRun(t, []string{
		"--config-only", "--seed", "42", "--peers", "2", "--quiet",
		"--local-addr", "10.242.0.1",
		"--peer-local-addr", "10.242.0.2",
		"--chaos-actions", "iface-link-flap,iface-addr-remove",
		"--fault-iface", "ze_cf1",
		"--fault-addr", "10.242.0.2/24",
		"--fault-iface-cycles", "2",
		"--fault-iface-interval", "100ms",
	})
	assert.Equal(t, 0, valid)

	invalidCases := []struct {
		name string
		args []string
	}{
		{"addr-without-iface", []string{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--fault-addr", "10.242.0.2/24"}},
		{"bad-cidr", []string{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--fault-iface", "veth0", "--fault-addr", "10.242.0.2"}},
		{"zero-cycles", []string{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--fault-iface", "veth0", "--fault-iface-cycles", "0"}},
		{"excess-cycles", []string{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--fault-iface", "veth0", "--fault-iface-cycles", "21"}},
		{"zero-interval", []string{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--fault-iface", "veth0", "--fault-iface-interval", "0s"}},
		{"bad-peer-local-addr", []string{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--peer-local-addr", "10.242.0.2/24"}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, 1, chaosRun(t, tc.args))
		})
	}
}

// TestRunStreamFaultFlags verifies valid and invalid in-process stream fault flags.
//
// VALIDATES: --stream-fragment-rate, --stream-blackhole-rate, and --stream-reset-rate
// bounds validation at CLI entry.
// PREVENTS: Negative or >1.0 stream fault probabilities reaching mocknet.FaultConn.
func TestRunStreamFaultFlags(t *testing.T) {
	valid := chaosRun(t, []string{
		"--config-only", "--seed", "42", "--peers", "2", "--quiet",
		"--stream-fragment-rate", "0.5",
		"--stream-blackhole-rate", "0.05",
		"--stream-reset-rate", "0.01",
	})
	assert.Equal(t, 0, valid)

	for _, args := range [][]string{
		{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--stream-fragment-rate", "-0.1"},
		{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--stream-fragment-rate", "1.1"},
		{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--stream-blackhole-rate", "-0.1"},
		{"--config-only", "--seed", "1", "--peers", "2", "--quiet", "--stream-reset-rate", "1.5"},
	} {
		assert.Equal(t, 1, chaosRun(t, args))
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
