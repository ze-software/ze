// VALIDATES: hub-side SHA-1 configuration hash chain persistence, historical
// base lookup for diff resync, zero-write steady-state pings, reanchor and full
// replay fallback, write-observer non-looping, RejectKind alarm separation,
// and corrupt chain key doctor reporting and healing (spec-fleet-10 WP-3).
// PREVENTS: hub chain advances before confirmed ack, store fsyncs on 30s pings,
// write-observer feedback loops, duplicate alarms between config-rejected and
// config-resync-failed, and silent erasure of unreadable chain keys.

package server

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// writeCountingStore wraps storage.Storage to count raw WriteKey calls without
// altering storage semantics.
type writeCountingStore struct {
	storage.Storage
	writeKeyCalls atomic.Int64
	chainWrites   atomic.Int64
}

func (w *writeCountingStore) WriteKey(key string, data []byte) error {
	w.writeKeyCalls.Add(1)
	if _, ok := clientNameFromChainKey(key); ok {
		w.chainWrites.Add(1)
	}
	return w.Storage.WriteKey(key, data)
}

// faultyReadKeyStore wraps storage.Storage to inject per-key errors from ReadKey.
type faultyReadKeyStore struct {
	storage.Storage
	errs map[string]error
}

func (f *faultyReadKeyStore) ReadKey(key string) ([]byte, error) {
	if err, ok := f.errs[key]; ok {
		return nil, err
	}
	return f.Storage.ReadKey(key)
}

func writeClientConfigWithVersion(t *testing.T, store storage.Storage, data []byte, stamp time.Time) {
	t.Helper()
	key := ClientConfigKey(testClientName)
	guard, err := store.AcquireLock(key)
	require.NoError(t, err)
	require.NoError(t, guard.WriteVersion(key, data, stamp))
	require.NoError(t, guard.WriteFile(key, data, 0o600))
	require.NoError(t, guard.Release())
}

func startChainTestServer(t *testing.T, store storage.Storage, reg metrics.Registry) (*ManagedServer, string) {
	t.Helper()
	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Listeners: []ManagedListener{{
			Addr:    "127.0.0.1:0",
			Clients: map[string]string{testClientName: testClientSecret},
		}},
		ReadConfig: func(name string) ([]byte, error) {
			return store.ReadFile(ClientConfigKey(name))
		},
		ReadBaseVersion: NewStoreBaseVersionReader(store),
		Store:           store,
		Metrics:         reg,
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	require.Len(t, addrs, 1)
	return srv, addrs[0].String()
}

func callConfigFetch(t *testing.T, mc *rpc.MuxConn, req fleet.ConfigFetchRequest) fleet.ConfigFetchResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, req)
	require.NoError(t, err)
	var resp fleet.ConfigFetchResponse
	require.NoError(t, json.Unmarshal(raw, &resp))
	return resp
}

func callConfigAck(t *testing.T, mc *rpc.MuxConn, ack fleet.ConfigAck) fleet.ManagedReply {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigAck, ack)
	require.NoError(t, err)
	reply, err := fleet.DecodeManagedReply(raw)
	require.NoError(t, err)
	return reply
}

func callPing(t *testing.T, mc *rpc.MuxConn, chainHash string) fleet.ManagedReply {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbPing, fleet.PingRequest{ChainHash: chainHash})
	require.NoError(t, err)
	reply, err := fleet.DecodeManagedReply(raw)
	require.NoError(t, err)
	return reply
}

func makeLargeConfig(peerLine string) []byte {
	var b strings.Builder
	b.WriteString("# Managed edge configuration\n")
	b.WriteString("system {\n")
	for i := range 20 {
		b.WriteString("\t# padding comment line to ensure line diff is much smaller than full config ")
		b.WriteString(strings.Repeat("x", i+10))
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	b.WriteString("bgp {\n")
	b.WriteByte('\t')
	b.WriteString(peerLine)
	b.WriteByte('\n')
	b.WriteString("}\n")
	return []byte(b.String())
}

// TestManagedServerResolvesHistoricalBaseForDiff proves that the hub resolves
// historical base bytes A from storage.ListVersions/ReadVersion by VersionHash
// prefix and serves Mode "diff" whose ApplyDiff reconstructs byte-exact V2.
//
// VALIDATES: spec-fleet-10 A-1, AC-4.
// PREVENTS: hub falling back to full config on every update when dated version
// history exists in the store.
func TestManagedServerResolvesHistoricalBaseForDiff(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.1 { peer-as 65001; }")
	cfgV2 := makeLargeConfig("peer 192.0.2.2 { peer-as 65002; }")
	v1 := fleet.VersionHash(cfgV1)
	v2 := fleet.VersionHash(cfgV2)
	c1 := fleet.FoldFull(cfgV1)

	t1 := time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	writeClientConfigWithVersion(t, store, cfgV1, t1)

	_, addr := startChainTestServer(t, store, nil)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	// Initial fetch and confirmed ack anchor both sides on (v1, c1).
	resp1 := callConfigFetch(t, mc, fleet.ConfigFetchRequest{})
	require.Equal(t, fleet.FetchModeFull, resp1.Mode)
	require.Equal(t, v1, resp1.Version)
	require.Equal(t, c1, resp1.ChainHash)
	ackReply1 := callConfigAck(t, mc, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1})
	require.Equal(t, c1, ackReply1.ChainHash)

	// Write V2 on hub so V1 remains in dated history while active is V2.
	writeClientConfigWithVersion(t, store, cfgV2, t2)

	// Fetch from converged client (v1, c1): hub must serve Mode "diff" with Reanchor false.
	resp2 := callConfigFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1})
	require.Equal(t, fleet.FetchModeDiff, resp2.Mode)
	require.Equal(t, v1, resp2.BaseVersion)
	require.Equal(t, v2, resp2.Version)
	require.False(t, resp2.Reanchor)
	require.Empty(t, resp2.Config)
	require.NotEmpty(t, resp2.Diff)

	delta, err := base64.StdEncoding.DecodeString(resp2.Diff)
	require.NoError(t, err)
	reconstructed, err := fleet.ApplyDiff(cfgV1, delta, rpc.MaxMessageSize)
	require.NoError(t, err)
	assert.Equal(t, cfgV2, reconstructed)
	assert.Equal(t, v2, fleet.VersionHash(reconstructed))

	wantC2, err := fleet.FoldDiff(c1, v1, v2, delta)
	require.NoError(t, err)
	require.Equal(t, wantC2, resp2.ChainHash)

	// Confirmed ack advances hub chain state to (v2, wantC2).
	ackReply2 := callConfigAck(t, mc, fleet.ConfigAck{Version: v2, OK: true, ChainHash: wantC2})
	require.Equal(t, wantC2, ackReply2.ChainHash)

	rawStored, err := store.ReadKey(ManagedClientChainKey(testClientName))
	require.NoError(t, err)
	var persisted fleet.ChainState
	require.NoError(t, json.Unmarshal(rawStored, &persisted))
	assert.Equal(t, fleet.ChainState{Version: v2, Chain: wantC2}, persisted)
}

// TestManagedServerChainPersistsOnlyOnConfirmedAck proves the hub never
// persists meta/managed/client/<name>/chain on config-fetch, on a rejected ack,
// or on an ack carrying a mismatched ChainHash — only on ack.OK with the
// matching chain hash.
//
// VALIDATES: spec-fleet-10 Transformation Path step 6, AC-4, AC-5.
// PREVENTS: hub advancing its persisted chain before the client confirmed apply.
func TestManagedServerChainPersistsOnlyOnConfirmedAck(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.10 { peer-as 65010; }")
	v1 := fleet.VersionHash(cfgV1)
	c1 := fleet.FoldFull(cfgV1)
	writeClientConfigWithVersion(t, store, cfgV1, time.Date(2026, 11, 27, 8, 10, 0, 0, time.UTC))

	reg := metrics.NewPrometheusRegistry()
	_, addr := startChainTestServer(t, store, reg)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	// Fetch V1; chain key must NOT be written before confirmed ack.
	resp := callConfigFetch(t, mc, fleet.ConfigFetchRequest{})
	require.Equal(t, v1, resp.Version)
	require.Equal(t, c1, resp.ChainHash)
	_, readErr := store.ReadKey(ManagedClientChainKey(testClientName))
	require.ErrorIs(t, readErr, fs.ErrNotExist)

	// Verify refusal must NOT persist chain.
	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version:    v1,
		OK:         false,
		Error:      "invalid syntax",
		RejectKind: fleet.RejectKindVerify,
	})
	_, readErr = store.ReadKey(ManagedClientChainKey(testClientName))
	require.ErrorIs(t, readErr, fs.ErrNotExist)

	// OK ack with wrong chain hash must NOT persist chain and increments mismatch counter.
	wrongChain := strings.Repeat("a", fleet.ChainHashLen)
	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version:   v1,
		OK:        true,
		ChainHash: wrongChain,
	})
	_, readErr = store.ReadKey(ManagedClientChainKey(testClientName))
	require.ErrorIs(t, readErr, fs.ErrNotExist)
	lines := exposition(t, reg, "ze_managed_chain_mismatches_total")
	require.Contains(t, strings.Join(lines, "\n"), "ze_managed_chain_mismatches_total 1")

	// Confirmed OK ack with matching chain hash persists the key and returns c1.
	reply := callConfigAck(t, mc, fleet.ConfigAck{
		Version:   v1,
		OK:        true,
		ChainHash: c1,
	})
	require.Equal(t, c1, reply.ChainHash)

	rawStored, err := store.ReadKey(ManagedClientChainKey(testClientName))
	require.NoError(t, err)
	var persisted fleet.ChainState
	require.NoError(t, json.Unmarshal(rawStored, &persisted))
	assert.Equal(t, fleet.ChainState{Version: v1, Chain: c1}, persisted)
}

// TestPingMatchPerformsZeroStoreWrites proves matching steady-state pings carry
// ManagedReply{ChainHash} and execute zero storage WriteKey calls, while a
// mismatched ping also performs zero writes and increments the mismatch counter.
//
// VALIDATES: spec-fleet-10 A-6, AC-5, R-12.
// PREVENTS: 30-second heartbeat fsync storms at 2,000-device scale.
func TestPingMatchPerformsZeroStoreWrites(t *testing.T) {
	baseStore, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = baseStore.Close() })

	counting := &writeCountingStore{Storage: baseStore}
	cfgV1 := makeLargeConfig("peer 192.0.2.20 { peer-as 65020; }")
	v1 := fleet.VersionHash(cfgV1)
	c1 := fleet.FoldFull(cfgV1)
	writeClientConfigWithVersion(t, baseStore, cfgV1, time.Date(2026, 11, 27, 8, 20, 0, 0, time.UTC))

	reg := metrics.NewPrometheusRegistry()
	_, addr := startChainTestServer(t, counting, reg)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	// Converge client to (v1, c1).
	resp := callConfigFetch(t, mc, fleet.ConfigFetchRequest{})
	require.Equal(t, c1, resp.ChainHash)
	_ = callConfigAck(t, mc, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1})
	require.Equal(t, int64(1), counting.chainWrites.Load())

	// Reset counters and run repeated matching pings.
	counting.chainWrites.Store(0)
	counting.writeKeyCalls.Store(0)
	for range 5 {
		reply := callPing(t, mc, c1)
		require.Equal(t, c1, reply.ChainHash)
	}
	assert.Equal(t, int64(0), counting.chainWrites.Load(), "matching ping must perform zero chain key writes")
	assert.Equal(t, int64(0), counting.writeKeyCalls.Load(), "matching ping must perform zero store writes")

	// Mismatched ping returns expected chain, performs zero writes, and increments mismatch metric.
	wrongChain := strings.Repeat("f", fleet.ChainHashLen)
	mismatchReply := callPing(t, mc, wrongChain)
	require.Equal(t, c1, mismatchReply.ChainHash)
	assert.Equal(t, int64(0), counting.chainWrites.Load())
	assert.Equal(t, int64(0), counting.writeKeyCalls.Load())
	lines := exposition(t, reg, "ze_managed_chain_mismatches_total")
	require.Contains(t, strings.Join(lines, "\n"), "ze_managed_chain_mismatches_total 1")
}

// TestManagedServerReanchorAndFullFallback verifies bootstrap convergence on
// first steady-state fetch (AC-3), current reanchor when config bytes match
// with a drifted chain (AC-7), diff reanchor when historical base exists with
// a drifted chain (AC-7), full fallback when base history is missing (AC-8),
// and explicit full resync even when client version equals hub version (AC-8).
//
// VALIDATES: spec-fleet-10 AC-3, AC-7, AC-8.
// PREVENTS: broken reanchor recovery or missing full-replay fallback.
func TestManagedServerReanchorAndFullFallback(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.31 { peer-as 65031; }")
	cfgV2 := makeLargeConfig("peer 192.0.2.32 { peer-as 65032; }")
	v1 := fleet.VersionHash(cfgV1)
	v2 := fleet.VersionHash(cfgV2)
	anchorV1 := fleet.FoldFull(cfgV1)
	anchorV2 := fleet.FoldFull(cfgV2)

	t1 := time.Date(2026, 11, 27, 8, 30, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	writeClientConfigWithVersion(t, store, cfgV1, t1)
	writeClientConfigWithVersion(t, store, cfgV2, t2)

	_, addr := startChainTestServer(t, store, nil)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	// AC-3: first steady-state fetch after bootstrap with matching FoldFull(V2) converges cleanly.
	bootResp := callConfigFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   v2,
		ChainHash: anchorV2,
	})
	require.Equal(t, managedFetchStatusCurrent, bootResp.Status)
	require.False(t, bootResp.Reanchor)
	require.Equal(t, anchorV2, bootResp.ChainHash)

	// Same version with drifted chain hash returns current + Reanchor true + anchorV2.
	driftedChain := strings.Repeat("b", fleet.ChainHashLen)
	curReanchor := callConfigFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   v2,
		ChainHash: driftedChain,
		Resync:    fleet.ResyncModeDiff,
	})
	require.Equal(t, managedFetchStatusCurrent, curReanchor.Status)
	require.True(t, curReanchor.Reanchor)
	require.Equal(t, anchorV2, curReanchor.ChainHash)

	// Different version with known historical base V1 and drifted chain returns Mode "diff" + Reanchor true.
	diffReanchor := callConfigFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   v1,
		ChainHash: driftedChain,
		Resync:    fleet.ResyncModeDiff,
	})
	require.Equal(t, fleet.FetchModeDiff, diffReanchor.Mode)
	require.True(t, diffReanchor.Reanchor)
	require.Equal(t, v1, diffReanchor.BaseVersion)
	require.Equal(t, v2, diffReanchor.Version)
	delta, err := base64.StdEncoding.DecodeString(diffReanchor.Diff)
	require.NoError(t, err)
	wantReanchorChain, err := fleet.FoldDiff(anchorV1, v1, v2, delta)
	require.NoError(t, err)
	require.Equal(t, wantReanchorChain, diffReanchor.ChainHash)

	// Missing base version falls back directly to Mode "full".
	missingBaseResp := callConfigFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   "0000000000000000",
		ChainHash: driftedChain,
		Resync:    fleet.ResyncModeDiff,
	})
	require.Equal(t, fleet.FetchModeFull, missingBaseResp.Mode)
	require.Equal(t, v2, missingBaseResp.Version)
	require.Equal(t, anchorV2, missingBaseResp.ChainHash)
	gotFull, err := base64.StdEncoding.DecodeString(missingBaseResp.Config)
	require.NoError(t, err)
	assert.Equal(t, cfgV2, gotFull)

	// Explicit Resync "full" returns full bytes even when Version == V2.
	explicitFull := callConfigFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   v2,
		ChainHash: driftedChain,
		Resync:    fleet.ResyncModeFull,
	})
	require.Equal(t, fleet.FetchModeFull, explicitFull.Mode)
	require.Equal(t, v2, explicitFull.Version)
	require.Equal(t, anchorV2, explicitFull.ChainHash)
	require.NotEmpty(t, explicitFull.Config)
}

// TestHubChainKeyDoesNotTriggerWriteObserver proves that writing
// meta/managed/client/<name>/chain never matches ClientNameFromConfigKey and
// never causes ManagedServer's store write observer to trigger config-changed.
//
// VALIDATES: spec-fleet-10 R-8.
// PREVENTS: config-changed notification storms whenever hub chain state writes.
func TestHubChainKeyDoesNotTriggerWriteObserver(t *testing.T) {
	key := ManagedClientChainKey("edge-01")
	require.Equal(t, "meta/managed/client/edge-01/chain", key)
	_, matched := ClientNameFromConfigKey(key)
	require.False(t, matched, "hub chain key must never match ClientNameFromConfigKey")

	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	var mu sync.Mutex
	var matchedNames []string
	detach := store.SetWriteObserver(func(writtenKey string) {
		if name, ok := ClientNameFromConfigKey(writtenKey); ok {
			mu.Lock()
			matchedNames = append(matchedNames, name)
			mu.Unlock()
		}
	})
	t.Cleanup(detach)

	chains := newManagedChainStore(store)
	require.NoError(t, chains.put("edge-01", fleet.ChainState{
		Version: "0123456789abcdef",
		Chain:   strings.Repeat("c", fleet.ChainHashLen),
	}))

	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, matchedNames, "writing hub chain key must not trigger client config write observer")
}

// TestManagedServerRejectKindAlarmSeparation verifies that RejectKindVerify
// opens config-rejected and clears ResyncFailure, while RejectKindCommit and
// RejectKindResync populate ResyncFailure, propagate via OnChainResyncState to
// external alarm evaluators, surface in ManagedRuntimeHealth().ChainDrifted,
// and suppress config-rejected; holding acks and matching pings clear
// ResyncFailure and close the external alarm.
//
// VALIDATES: spec-fleet-10 AC-1, R-11.
// PREVENTS: duplicate alarms between config-rejected, config-diverged, and config-resync-failed.
func TestManagedServerRejectKindAlarmSeparation(t *testing.T) {
	t.Cleanup(SnapshotAlarmKinds())

	const testResyncKind = "test-resync-failed-hook"
	MustRegisterAlarmKind(AlarmKindSpec{
		Name:     testResyncKind,
		Severity: AlarmSeverityWarning,
		Scope:    AlarmScopeDevice,
		Causes:   []string{AlarmKindDeviceOffline},
		Summary:  "Test resync failed",
		Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
			var out []AlarmObservation
			for _, dev := range ctx.Devices {
				if !dev.Declared || dev.ConfigDiverged || dev.ResyncFailure == nil {
					continue
				}
				out = append(out, AlarmObservation{
					Device:     dev.Name,
					Summary:    "Test resync failed for " + dev.Name,
					Detail:     dev.ResyncFailure.FullReason,
					ObservedAt: dev.ResyncFailure.FailedAt,
				})
			}
			return out
		},
	})

	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.41 { peer-as 65041; }")
	v1 := fleet.VersionHash(cfgV1)
	c1 := fleet.FoldFull(cfgV1)
	writeClientConfigWithVersion(t, store, cfgV1, time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC))

	srv, addr := startChainTestServer(t, store, nil)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	// Fetch missing base -> full fallback, then simulate commit failure.
	_ = callConfigFetch(t, mc, fleet.ConfigFetchRequest{Version: "deadbeef00000000", Resync: fleet.ResyncModeDiff})
	clientChain := strings.Repeat("1", fleet.ChainHashLen)
	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version:    v1,
		OK:         false,
		Error:      "confirmed commit window is open",
		ChainHash:  clientChain,
		RejectKind: fleet.RejectKindCommit,
	})

	page, err := srv.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testClientName})
	require.NoError(t, err)
	_, hasRejected := findOpenAlarm(page.Alarms, AlarmKindConfigRejected, testClientName, "")
	assert.False(t, hasRejected, "commit/resync failure must not open config-rejected")
	resyncObs, hasResync := findOpenAlarm(page.Alarms, testResyncKind, testClientName, "")
	require.True(t, hasResync, "OnChainResyncState must propagate ResyncFailure to external alarm kinds")
	assert.Contains(t, resyncObs.Detail, "confirmed commit window is open")

	health, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.Len(t, health.ChainDrifted, 1)
	assert.Equal(t, testClientName, health.ChainDrifted[0].Client)
	assert.Equal(t, clientChain, health.ChainDrifted[0].ClientChain)
	assert.Equal(t, c1, health.ChainDrifted[0].ExpectedChain)
	assert.Contains(t, health.ChainDrifted[0].Reason, "not found in hub history")
	assert.Contains(t, health.ChainDrifted[0].Reason, "confirmed commit window is open")

	// Verify refusal must open config-rejected and clear ResyncFailure.
	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version:    v1,
		OK:         false,
		Error:      "yang verify failed",
		ChainHash:  clientChain,
		RejectKind: fleet.RejectKindVerify,
	})
	page, err = srv.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testClientName})
	require.NoError(t, err)
	_, hasRejected = findOpenAlarm(page.Alarms, AlarmKindConfigRejected, testClientName, "")
	assert.True(t, hasRejected, "verify refusal must open config-rejected")
	_, hasResync = findOpenAlarm(page.Alarms, testResyncKind, testClientName, "")
	assert.False(t, hasResync, "verify refusal must clear ResyncFailure")

	// Re-open with RejectKindResync, then confirm a holding ack clears it.
	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version:    v1,
		OK:         false,
		Error:      "diff apply failed",
		ChainHash:  clientChain,
		RejectKind: fleet.RejectKindResync,
	})
	page, err = srv.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testClientName})
	require.NoError(t, err)
	_, hasResync = findOpenAlarm(page.Alarms, testResyncKind, testClientName, "")
	require.True(t, hasResync)

	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version: v1,
		OK:      false,
		Holding: true,
		Error:   "holding local edits",
	})
	page, err = srv.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testClientName})
	require.NoError(t, err)
	_, hasResync = findOpenAlarm(page.Alarms, testResyncKind, testClientName, "")
	assert.False(t, hasResync, "holding ack must clear ResyncFailure")

	// Re-open with RejectKindResync, then confirm matching ping clears it.
	_ = callConfigAck(t, mc, fleet.ConfigAck{
		Version:    v1,
		OK:         false,
		Error:      "resync timeout",
		ChainHash:  clientChain,
		RejectKind: fleet.RejectKindResync,
	})
	page, err = srv.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testClientName})
	require.NoError(t, err)
	_, hasResync = findOpenAlarm(page.Alarms, testResyncKind, testClientName, "")
	require.True(t, hasResync)

	pingReply := callPing(t, mc, c1)
	require.Equal(t, c1, pingReply.ChainHash)
	page, err = srv.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testClientName})
	require.NoError(t, err)
	_, hasResync = findOpenAlarm(page.Alarms, testResyncKind, testClientName, "")
	assert.False(t, hasResync, "matching ping must clear ResyncFailure")
}

// TestManagedServerCorruptChainKeyDoctorAndHeal proves that a corrupt
// meta/managed/client/<name>/chain key fails strict decode, surfaces in
// ManagedRuntimeHealth().ChainUnreadable, and is overwritten and cleared by a
// subsequent confirmed full replay.
//
// VALIDATES: spec-fleet-10 AC-12, R-9.
// PREVENTS: corrupt hub chain keys being silently ignored or permanently stuck.
func TestManagedServerCorruptChainKeyDoctorAndHeal(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.51 { peer-as 65051; }")
	v1 := fleet.VersionHash(cfgV1)
	c1 := fleet.FoldFull(cfgV1)
	writeClientConfigWithVersion(t, store, cfgV1, time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC))

	// Seed corrupt JSON into the hub chain key before ManagedServer starts.
	require.NoError(t, store.WriteKey(ManagedClientChainKey(testClientName), []byte(`{"version":"v1","unknown-field":1}`)))

	_, addr := startChainTestServer(t, store, nil)

	health, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.Len(t, health.ChainUnreadable, 1)
	assert.Equal(t, testClientName, health.ChainUnreadable[0].Client)
	assert.Contains(t, health.ChainUnreadable[0].Err, "unknown field")

	// Confirmed full replay overwrites the corrupt key and clears ChainUnreadable.
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	resp := callConfigFetch(t, mc, fleet.ConfigFetchRequest{Resync: fleet.ResyncModeFull})
	require.Equal(t, fleet.FetchModeFull, resp.Mode)
	require.Equal(t, c1, resp.ChainHash)

	reply := callConfigAck(t, mc, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1})
	require.Equal(t, c1, reply.ChainHash)

	health, running = ManagedRuntimeHealth()
	require.True(t, running)
	assert.Empty(t, health.ChainUnreadable, "confirmed full replay must heal unreadable chain state")

	rawStored, err := store.ReadKey(ManagedClientChainKey(testClientName))
	require.NoError(t, err)
	var persisted fleet.ChainState
	require.NoError(t, json.Unmarshal(rawStored, &persisted))
	assert.Equal(t, fleet.ChainState{Version: v1, Chain: c1}, persisted)
}

// TestNewStoreBaseVersionReaderReadKeyErrors proves that NewStoreBaseVersionReader
// propagates non-fs.ErrNotExist errors from store.ReadKey instead of silently
// returning (nil, false, nil), while skipping fs.ErrNotExist entries cleanly.
//
// VALIDATES: spec-fleet-10 review ISSUE-2.
// PREVENTS: storage I/O or corruption errors on dated version keys being
// swallowed as missing history.
func TestNewStoreBaseVersionReaderReadKeyErrors(t *testing.T) {
	baseStore, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = baseStore.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.61 { peer-as 65061; }")
	cfgV2 := makeLargeConfig("peer 192.0.2.62 { peer-as 65062; }")
	v1 := fleet.VersionHash(cfgV1)
	t1 := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	writeClientConfigWithVersion(t, baseStore, cfgV1, t1)
	writeClientConfigWithVersion(t, baseStore, cfgV2, t2)

	versions, err := baseStore.ListVersions(ClientConfigKey(testClientName))
	require.NoError(t, err)
	require.Len(t, versions, 2)

	// Identify which dated entry points to V2 and which points to V1.
	var v1Path, v2Path string
	wantV1Prefix := managedChainEntryPrefix + v1
	for _, vi := range versions {
		val, readErr := baseStore.ReadKey(vi.Path)
		require.NoError(t, readErr)
		if strings.HasPrefix(string(val), wantV1Prefix) {
			v1Path = vi.Path
		} else {
			v2Path = vi.Path
		}
	}
	require.NotEmpty(t, v1Path)
	require.NotEmpty(t, v2Path)

	t.Run("non-ErrNotExist error propagates", func(t *testing.T) {
		errIO := errors.New("simulated storage I/O failure")
		faulty := &faultyReadKeyStore{
			Storage: baseStore,
			errs:    map[string]error{versions[0].Path: errIO},
		}
		reader := NewStoreBaseVersionReader(faulty)
		data, found, readErr := reader(testClientName, v1)
		require.Error(t, readErr)
		assert.ErrorIs(t, readErr, errIO)
		assert.Contains(t, readErr.Error(), versions[0].Path)
		assert.Contains(t, readErr.Error(), testClientName)
		assert.False(t, found)
		assert.Nil(t, data)
	})

	t.Run("fs.ErrNotExist skips cleanly to matching version", func(t *testing.T) {
		faulty := &faultyReadKeyStore{
			Storage: baseStore,
			errs:    map[string]error{v2Path: fs.ErrNotExist},
		}
		reader := NewStoreBaseVersionReader(faulty)
		data, found, readErr := reader(testClientName, v1)
		require.NoError(t, readErr)
		assert.True(t, found)
		assert.Equal(t, cfgV1, data)
	})

	t.Run("fs.ErrNotExist on all entries returns not found with nil error", func(t *testing.T) {
		faulty := &faultyReadKeyStore{
			Storage: baseStore,
			errs: map[string]error{
				v1Path: fs.ErrNotExist,
				v2Path: fs.ErrNotExist,
			},
		}
		reader := NewStoreBaseVersionReader(faulty)
		data, found, readErr := reader(testClientName, v1)
		require.NoError(t, readErr)
		assert.False(t, found)
		assert.Nil(t, data)
	})
}

// TestStaleSessionPingCannotMutateChainState proves that a ping from a stale or
// superseded MuxConn is refused by checkActiveSession and cannot increment the
// mismatch counter or clear an open resync failure owned by the active session.
//
// VALIDATES: spec-fleet-10 review NOTE-3.
// PREVENTS: a superseded connection corrupting mismatch metrics or clearing
// active chain-drift diagnostics via VerbPing.
func TestStaleSessionPingCannotMutateChainState(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	cfgV1 := makeLargeConfig("peer 192.0.2.71 { peer-as 65071; }")
	v1 := fleet.VersionHash(cfgV1)
	c1 := fleet.FoldFull(cfgV1)
	writeClientConfigWithVersion(t, store, cfgV1, time.Date(2026, 11, 27, 10, 10, 0, 0, time.UTC))

	reg := metrics.NewPrometheusRegistry()
	srv, addr := startChainTestServer(t, store, reg)
	activeMC := dialTestClient(t, addr, testClientName, testClientSecret)

	// Converge active session to (v1, c1), then open a resync failure on the active session.
	resp := callConfigFetch(t, activeMC, fleet.ConfigFetchRequest{})
	require.Equal(t, c1, resp.ChainHash)
	_ = callConfigAck(t, activeMC, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1})
	_ = callConfigAck(t, activeMC, fleet.ConfigAck{
		Version:    v1,
		OK:         false,
		Error:      "resync apply failed",
		ChainHash:  strings.Repeat("2", fleet.ChainHashLen),
		RejectKind: fleet.RejectKindResync,
	})

	health, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.Len(t, health.ChainDrifted, 1)

	clientPipe, serverPipe := net.Pipe()
	staleServerMC := rpc.NewMuxConn(rpc.NewConn(serverPipe, serverPipe))
	t.Cleanup(func() {
		_ = clientPipe.Close()
		_ = serverPipe.Close()
		_ = staleServerMC.Close()
	})
	pipeReader := bufio.NewReader(clientPipe)

	sendStalePing := func(id uint64, chainHash string) string {
		params, marshalErr := json.Marshal(fleet.PingRequest{ChainHash: chainHash})
		require.NoError(t, marshalErr)
		var session managedSession
		done := make(chan struct{})
		go func() {
			defer close(done)
			srv.handleRequest(testClientName, staleServerMC, &rpc.Request{
				ID:     id,
				Method: fleet.VerbPing,
				Params: params,
			}, &session)
		}()
		line, readErr := pipeReader.ReadString('\n')
		require.NoError(t, readErr)
		<-done
		return line
	}

	// Mismatched ping on stale connection must error and not increment mismatches.
	mismatchResp := sendStalePing(1, strings.Repeat("e", fleet.ChainHashLen))
	assert.Contains(t, mismatchResp, "ping session is superseded")
	lines := exposition(t, reg, "ze_managed_chain_mismatches_total")
	assert.Contains(t, strings.Join(lines, "\n"), "ze_managed_chain_mismatches_total 0")

	// Matching ping on stale connection must error and not clear active resync failure.
	matchResp := sendStalePing(2, c1)
	assert.Contains(t, matchResp, "ping session is superseded")
	health, running = ManagedRuntimeHealth()
	require.True(t, running)
	assert.Len(t, health.ChainDrifted, 1, "stale session ping must not clear active resync failure")

	// Active session matching ping clears the resync failure.
	reply := callPing(t, activeMC, c1)
	require.Equal(t, c1, reply.ChainHash)
	health, running = ManagedRuntimeHealth()
	require.True(t, running)
	assert.Empty(t, health.ChainDrifted)
}
