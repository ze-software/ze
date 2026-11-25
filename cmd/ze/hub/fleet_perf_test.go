//go:build fleetperf

// Fleet many-clients perf test (spec-fleet-1 AC-19, A-9).
//
// Tier: evidence/release, NOT ze-precommit-verify. It stands up the real managed
// hub listener (TLS 1.3, 32-slot unauthenticated handshake cap, bounded inbound
// reader, device registry) and drives 2,000 concurrent declared clients through
// auth + initial config sync while holding all 2,000 sessions open simultaneously,
// asserting that ManagedRegistrySnapshot counts 2,000 connected devices. Run:
//
//	go test -tags 'ze_core fleetperf' ./cmd/ze/hub/ -run TestFleetManyClientsPerf -v
//	./le test integration stress-fleet
package hub

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// raiseNoFileLimit raises RLIMIT_NOFILE to the process hard limit and fails
// naming both soft and hard limits if fewer than minFDs file descriptors are
// available (spec-fleet-1 A-9).
func raiseNoFileLimit(t *testing.T, minFDs uint64) {
	t.Helper()
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		t.Fatalf("get RLIMIT_NOFILE: %v", err)
	}
	if lim.Cur < lim.Max {
		lim.Cur = lim.Max
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
			t.Fatalf("raise RLIMIT_NOFILE soft limit to hard limit %d: %v", lim.Max, err)
		}
	}
	if lim.Cur < minFDs {
		t.Fatalf("RLIMIT_NOFILE too low for 2,000 concurrent TLS sessions: soft=%d hard=%d, need at least %d (raise ulimit -n)",
			lim.Cur, lim.Max, minFDs)
	}
}

// TestFleetManyClientsPerf provisions 2,000 declared client configs on one hub
// and drives all 2,000 clients concurrently through TLS 1.3 + auth + initial
// config-fetch, holding all 2,000 sessions simultaneously until
// ManagedRegistrySnapshot counts 2,000 connected devices.
//
// VALIDATES: fleet-1 AC-19 and A-9 -- 2,000 declared clients hold sessions at
// the same time, each fetched its own config, and ManagedRegistrySnapshot
// reports 2,000 connected.
// PREVENTS: a hub-side regression (handshake slot leak, accept-loop deadlock,
// per-client config mixup, or registry contention) at fleet scale.
func TestFleetManyClientsPerf(t *testing.T) {
	const clients = 2000
	// 2,000 clients need ~2,000 client sockets + ~2,000 server sockets + store FDs.
	raiseNoFileLimit(t, 4500)

	// Use the in-memory storage encoding so provisioning 2,000 configs and
	// flushing 2,000 device records measure the hub's TLS/handshake/registry
	// scaling rather than 12,000 sequential ext4 fsync barriers. Every storage
	// guard, observer, and record-writer code path runs unchanged.
	store := storage.CreateMemory()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	// Provision one distinct config + secret per client.
	provStart := time.Now()
	secrets := make(map[string]string, clients)
	wantCfg := make(map[string]string, clients)
	for i := range clients {
		name := fmt.Sprintf("edge-%04d", i)
		secret := fmt.Sprintf("edge-%04d-secret-that-is-at-least-32char", i)
		cfg := fmt.Sprintf("bgp { peer p%d { } }\n", i)
		secrets[name] = secret
		wantCfg[name] = cfg
		if err := store.WriteFile(pluginserver.ClientConfigKey(name), []byte(cfg), 0o600); err != nil {
			t.Fatalf("write client config %s: %v", name, err)
		}
	}
	t.Logf("fleet perf: provisioned %d client configs in %s", clients, time.Since(provStart))

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
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	if len(addrs) != 1 {
		t.Fatalf("expected 1 managed listener, got %d", len(addrs))
	}
	addr := addrs[0].String()

	var (
		readyWG   sync.WaitGroup
		doneWG    sync.WaitGroup
		errCount  atomic.Int64
		mismatch  atomic.Int64
		latMu     sync.Mutex
		latencies = make([]time.Duration, 0, clients)
		release   = make(chan struct{})
	)
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseAll)

	// Bound concurrent outbound dials slightly so the kernel SYN backlog isn't
	// overflowed on loopback while still heavily exercising the 32-slot handshake cap.
	dialTokens := make(chan struct{}, 128)

	start := time.Now()
	readyWG.Add(clients)
	for i := range clients {
		doneWG.Add(1)
		go func(n int) {
			defer doneWG.Done()
			name := fmt.Sprintf("edge-%04d", n)
			dialTokens <- struct{}{}
			t0 := time.Now()
			mc, cfg, syncErr := fleetInitialSync(addr, name, secrets[name])
			elapsed := time.Since(t0)
			<-dialTokens
			if syncErr != nil {
				t.Errorf("client %s initial sync: %v", name, syncErr)
				errCount.Add(1)
				readyWG.Done()
				return
			}
			defer mc.Close() //nolint:errcheck // best-effort close after barrier
			if cfg != wantCfg[name] {
				t.Errorf("client %s got config %q, want %q", name, cfg, wantCfg[name])
				mismatch.Add(1)
				readyWG.Done()
				return
			}
			latMu.Lock()
			latencies = append(latencies, elapsed)
			latMu.Unlock()
			readyWG.Done()
			<-release
		}(i)
	}
	readyWG.Wait()
	wallClock := time.Since(start)

	if errCount.Load() != 0 || mismatch.Load() != 0 {
		releaseAll()
		doneWG.Wait()
		t.Fatalf("error budget exceeded: %d sync errors, %d config mismatches", errCount.Load(), mismatch.Load())
	}
	if len(latencies) != clients {
		releaseAll()
		doneWG.Wait()
		t.Fatalf("expected %d successful syncs, got %d", clients, len(latencies))
	}

	// All 2,000 sessions are held open simultaneously: verify the live registry snapshot.
	snapStart := time.Now()
	devices, err := pluginserver.ManagedRegistrySnapshot()
	if err != nil {
		releaseAll()
		doneWG.Wait()
		t.Fatalf("ManagedRegistrySnapshot: %v", err)
	}
	t.Logf("fleet perf: ManagedRegistrySnapshot returned %d rows in %s", len(devices), time.Since(snapStart))
	connectedCount := 0
	for _, dev := range devices {
		if dev.Connected {
			connectedCount++
		}
	}
	if connectedCount != clients {
		releaseAll()
		doneWG.Wait()
		t.Fatalf("ManagedRegistrySnapshot connected=%d, want %d (total rows=%d)", connectedCount, clients, len(devices))
	}

	releaseAll()
	doneWG.Wait()

	slices.Sort(latencies)
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	maxLat := latencies[len(latencies)-1]
	t.Logf("fleet perf: %d simultaneous connected clients, all synced in %s wall-clock; latency p50=%s p95=%s max=%s",
		clients, wallClock, p50, p95, maxLat)
}

// fleetInitialSync performs one client's dial + TLS handshake + auth +
// initial config-fetch and returns the open MuxConn plus decoded config so the
// caller can hold the session open until all clients are connected.
func fleetInitialSync(addr, name, token string) (*rpc.MuxConn, string, error) {
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test-only; local managed listener cert
		MinVersion:         tls.VersionTLS13,
	}}
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer dialCancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("dial: %w", err)
	}

	authCtx, authCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer authCancel()
	if err := pluginipc.SendAuth(authCtx, conn, token, name); err != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("send auth: %w", err)
	}
	line, err := pluginipc.ReadLineRaw(conn, 512)
	if err != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("read auth response: %w", err)
	}
	_, verb, _, parseErr := rpc.ParseLine(line)
	if parseErr != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("parse auth response %q: %w", line, parseErr)
	}
	if verb != "ok" {
		_ = conn.Close()
		return nil, "", fmt.Errorf("auth not ok: verb=%q", verb)
	}

	mc := rpc.NewMuxConn(rpc.NewConn(conn, conn))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, fleet.ConfigFetchRequest{Version: ""})
	if err != nil {
		_ = mc.Close()
		return nil, "", fmt.Errorf("config-fetch: %w", err)
	}
	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		_ = mc.Close()
		return nil, "", fmt.Errorf("unmarshal config-fetch: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		_ = mc.Close()
		return nil, "", fmt.Errorf("decode config: %w", err)
	}
	return mc, string(decoded), nil
}
