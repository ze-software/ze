//go:build fleetperf

// Fleet many-clients perf test (spec followup-test-infra AC-8 / L98).
//
// Tier: evidence/release, NOT ze-precommit-verify (R-6). It stands up the real managed
// hub listener (TLS 1.3, self-signed cert, the managedMaxConns=128 accept cap)
// and drives >=128 concurrent clients through auth + initial config sync. Run:
//
//	go test -tags 'ze_core fleetperf' ./cmd/ze/hub/ -run TestFleetManyClientsPerf -v
//	make ze-stress-fleet-test
//
// It lives in package hub (not internal/component/managed as the spec skeleton
// guessed) because the real hub+TLS+cap harness -- startManagedServer,
// ClientConfigKey, HubConfig -- lives here; internal/component/managed only has
// a net.Pipe mock hub. Recorded as a deviation in the spec.
package hub

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFleetManyClientsPerf provisions N client configs on one hub and drives N
// concurrent clients through auth + initial config sync, recording the latency
// distribution and asserting a zero error budget.
//
// VALIDATES: AC-8 -- >=128 concurrent managed clients complete their initial
// config sync against a single hub, each receiving its own config, within a
// recorded latency/error budget.
// PREVENTS: a hub-side regression (accept-loop deadlock, per-client config
// mixup, auth serialization) that only manifests under fleet-scale concurrency.
func TestFleetManyClientsPerf(t *testing.T) {
	const clients = 128

	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "hub.zefs"), dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}

	// Provision one distinct config + secret per client.
	secrets := make(map[string]string, clients)
	wantCfg := make(map[string]string, clients)
	for i := range clients {
		name := fmt.Sprintf("edge-%03d", i)
		secret := fmt.Sprintf("edge-%03d-secret-that-is-at-least-32chars", i)
		cfg := fmt.Sprintf("bgp { peer p%d { } }\n", i)
		secrets[name] = secret
		wantCfg[name] = cfg
		if err := store.WriteFile(pluginserver.ClientConfigKey(name), []byte(cfg), 0o600); err != nil {
			t.Fatalf("write client config %s: %v", name, err)
		}
	}

	hubConfig := &zePlugin.HubConfig{
		Servers: []zePlugin.HubServerConfig{{
			Name:    "central",
			Host:    "127.0.0.1",
			Port:    0, // OS-assigned
			Clients: secrets,
		}},
	}

	srv := startManagedServer(t.Context(), store, hubConfig)
	if srv == nil {
		t.Fatal("startManagedServer returned nil")
	}
	addrs := srv.Addrs()
	if len(addrs) != 1 {
		t.Fatalf("expected 1 managed listener, got %d", len(addrs))
	}
	addr := addrs[0].String()

	var (
		wg        sync.WaitGroup
		errCount  atomic.Int64
		mismatch  atomic.Int64
		latMu     sync.Mutex
		latencies = make([]time.Duration, 0, clients)
	)

	start := time.Now()
	for i := range clients {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("edge-%03d", n)
			t0 := time.Now()
			cfg, err := fleetInitialSync(addr, name, secrets[name])
			elapsed := time.Since(t0)
			if err != nil {
				t.Errorf("client %s initial sync: %v", name, err)
				errCount.Add(1)
				return
			}
			if cfg != wantCfg[name] {
				t.Errorf("client %s got config %q, want %q", name, cfg, wantCfg[name])
				mismatch.Add(1)
				return
			}
			latMu.Lock()
			latencies = append(latencies, elapsed)
			latMu.Unlock()
		}(i)
	}
	wg.Wait()
	wallClock := time.Since(start)

	if errCount.Load() != 0 || mismatch.Load() != 0 {
		t.Fatalf("error budget exceeded: %d sync errors, %d config mismatches", errCount.Load(), mismatch.Load())
	}
	if len(latencies) != clients {
		t.Fatalf("expected %d successful syncs, got %d", clients, len(latencies))
	}

	slices.Sort(latencies)
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	maxLat := latencies[len(latencies)-1]
	t.Logf("fleet perf: %d clients, all synced in %s wall-clock; latency p50=%s p95=%s max=%s",
		clients, wallClock, p50, p95, maxLat)
}

// fleetInitialSync performs one client's dial + TLS handshake + auth + initial
// config-fetch and returns the decoded config. It returns errors (never calls
// t.Fatalf) so it is safe to run from many concurrent goroutines.
func fleetInitialSync(addr, name, token string) (string, error) {
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test-only; self-signed managed listener cert
		MinVersion:         tls.VersionTLS13,
	}}
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("dial: %w", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort close

	if err := pluginipc.SendAuth(context.Background(), conn, token, name); err != nil {
		return "", fmt.Errorf("send auth: %w", err)
	}
	line, err := pluginipc.ReadLineRaw(conn, 512)
	if err != nil {
		return "", fmt.Errorf("read auth response: %w", err)
	}
	// The two failures are reported apart. Wrapping one error value for both
	// renders `%!w(<nil>)` on the branch where parsing succeeded and the verb was
	// simply not ok, which is the branch a real auth refusal takes.
	_, verb, _, parseErr := rpc.ParseLine(line)
	if parseErr != nil {
		return "", fmt.Errorf("parse auth response %q: %w", line, parseErr)
	}
	if verb != "ok" {
		return "", fmt.Errorf("auth not ok: verb=%q", verb)
	}

	mc := rpc.NewMuxConn(rpc.NewConn(conn, conn))
	defer mc.Close() //nolint:errcheck // best-effort close

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, fleet.ConfigFetchRequest{Version: ""})
	if err != nil {
		return "", fmt.Errorf("config-fetch: %w", err)
	}
	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("unmarshal config-fetch: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		return "", fmt.Errorf("decode config: %w", err)
	}
	return string(decoded), nil
}
