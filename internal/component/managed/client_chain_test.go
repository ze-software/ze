package managed

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// chainMemoryDevice creates an in-memory storage.Storage preloaded with active
// and optional baseline configs so live ping-loop tests do not stall behind
// disk fsync contention under concurrent test load. Real on-disk persistence is
// exercised by chain_test.go and cmd/ze/ze_core_start_bootstrap_test.go.
func chainMemoryDevice(t *testing.T, active, earlier string) storage.Storage {
	t.Helper()
	store := storage.CreateMemory()
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(active), 0))
	if earlier != "" {
		require.NoError(t, StoreBaseline(store, versionOf([]byte(earlier))))
	}
	return store
}

func findChainDriftWarning() (report.Issue, bool) {
	for _, w := range report.Warnings() {
		if w.Source == doctorComponent && w.Code == pluginserver.CodeDoctorManagedChainDrift && w.Subject == chainWarningSubject {
			return w, true
		}
	}
	return report.Issue{}, false
}

// TestMissingBaseUsesFirstFullReply proves AC-8 when the hub lacks historical
// base V1: the hub returns Mode=="full" in the original fetch reply, the client
// makes exactly one fetch, commits under CommitOver, converges to FoldFull(B),
// and records zero failed diffs.
//
// VALIDATES: spec-fleet-10 AC-8 (missing hub base direct full reply, 1 fetch, 0 diff fallbacks).
// PREVENTS: a client issuing an unnecessary second fetch or incrementing the diff-fallback metric
// when the hub already fell back to full config in the first reply.
func TestMissingBaseUsesFirstFullReply(t *testing.T) {
	t.Parallel()

	v1Bytes := []byte("system {\n\thost edge-01;\n}\n")
	v2Bytes := []byte("system {\n\thost edge-01-updated;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c1 := fleet.FoldFull(v1Bytes)
	c2 := fleet.FoldFull(v2Bytes)

	store := holdDevice(t, string(v1Bytes), string(v1Bytes))
	chainAccess := NewChainAccess(store)
	require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

	reg := newTestChainMetrics()
	access := NewBaselineAccess(store)
	var fetches atomic.Int32
	var committed []byte
	var lastStatus ChainStatus

	cfg := ClientConfig{
		Name:     "edge-01",
		Baseline: access,
		Chain:    chainAccess,
		Metrics:  reg,
		OnChainState: func(st ChainStatus) {
			lastStatus = st
		},
		Handler: &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func(data []byte) error {
			committed = append([]byte(nil), data...)
			require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
			_, err := RecordActiveAsBaseline(store)
			return err
		}),
	}

	mc, seen := holdPipe(t, func(req *rpc.Request) any {
		switch req.Method {
		case fleet.VerbConfigFetch:
			fetches.Add(1)
			return fleet.ConfigFetchResponse{
				Mode:      fleet.FetchModeFull,
				Version:   v2,
				Config:    base64.StdEncoding.EncodeToString(v2Bytes),
				ChainHash: c2,
			}
		case fleet.VerbConfigAck:
			return fleet.ManagedReply{ChainHash: c2}
		default:
			return fleet.ManagedReply{}
		}
	})

	_, err := fetchAgainstBaselineWithMode(context.Background(), mc, &cfg, access, refetchOnce, fleet.ResyncModeDiff, c2, nil)
	require.NoError(t, err)

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.True(t, ack.OK)
	assert.Equal(t, v2, ack.Version)
	assert.Equal(t, c2, ack.ChainHash)
	assert.Equal(t, int32(1), fetches.Load(), "missing base must complete in one fetch")
	assert.Equal(t, v2Bytes, committed)

	persisted, present, err := chainAccess.Read()
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, fleet.ChainState{Version: v2, Chain: c2}, persisted)

	assert.Equal(t, ChainStateInSync, lastStatus.ChainState)
	assert.Equal(t, LastResyncFullReplay, lastStatus.LastResync)
	assert.Zero(t, reg.count("ze_managed_client_resyncs_total:diff,fallback"))
	assert.Equal(t, 1.0, reg.count("ze_managed_client_resyncs_total:full,applied"))
}

// TestClientPingMismatchTriggersDiffResyncThenFullFallback proves AC-6 and AC-8
// over a live connection: a ping reply with a mismatched chain-hash signals
// driftCh, triggers a diff-based resync, falls back once to full replay when
// the diff is corrupt, and converges both chains without reconnecting.
//
// VALIDATES: spec-fleet-10 AC-5, AC-6, AC-8, R-1, R-5, R-6.
// PREVENTS: ping mismatches going unnoticed, concurrent commit races, or a
// corrupt diff staging invalid bytes instead of falling back to full replay.
func TestClientPingMismatchTriggersDiffResyncThenFullFallback(t *testing.T) {
	v1Bytes := []byte("system {\n\thost edge-01;\n}\nbgp {\n\trouter-id 192.0.2.1;\n}\n")
	v2Bytes := []byte("system {\n\thost edge-01;\n}\nbgp {\n\trouter-id 192.0.2.2;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c1 := fleet.FoldFull(v1Bytes)
	c2Full := fleet.FoldFull(v2Bytes)

	addr, admitted := fetchHub(t)
	store := chainMemoryDevice(t, string(v1Bytes), string(v1Bytes))
	chainAccess := NewChainAccess(store)
	require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

	reg := newTestChainMetrics()
	access := NewBaselineAccess(store)
	cfg, _, applied := holdClient(addr, access)
	cfg.Chain = chainAccess
	cfg.Metrics = reg
	cfg.fetchTimeout = holdWait
	cfg.pingInterval = 25 * time.Millisecond
	cfg.pingTimeout = 2 * time.Second
	cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }
	cfg.CommitOver = stagedOver(access, func(data []byte) error {
		require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
		_, baseErr := RecordActiveAsBaseline(store)
		applied <- append([]byte(nil), data...)
		return baseErr
	})

	returned := runFetchConnection(t, &cfg)
	hub := nextSession(t, admitted)

	// Initial connect fetch: hub reports current with matching c1.
	initFetch := hubRequest(t, hub)
	require.Equal(t, fleet.VerbConfigFetch, initFetch.Method)
	require.NoError(t, hub.SendResult(t.Context(), initFetch.ID, fleet.ConfigFetchResponse{
		Status:    fetchStatusCurrent,
		ChainHash: c1,
	}))

	// First ping: hub advertises mismatched c2Full.
	ping1 := hubRequest(t, hub)
	require.Equal(t, fleet.VerbPing, ping1.Method)
	var p1 fleet.PingRequest
	require.NoError(t, json.Unmarshal(ping1.Params, &p1))
	assert.Equal(t, c1, p1.ChainHash)
	require.NoError(t, hub.SendResult(t.Context(), ping1.ID, fleet.ManagedReply{ChainHash: c2Full}))

	// Client signals driftCh and sends resync=="diff".
	diffFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, c2Full)
	var dfReq fleet.ConfigFetchRequest
	require.NoError(t, json.Unmarshal(diffFetch.Params, &dfReq))
	assert.Equal(t, v1, dfReq.Version)
	assert.Equal(t, c1, dfReq.ChainHash)
	assert.Equal(t, fleet.ResyncModeDiff, dfReq.Resync)

	// Hub sends a corrupt diff payload -> client must stage nothing and request resync=="full".
	require.NoError(t, hub.SendResult(t.Context(), diffFetch.ID, fleet.ConfigFetchResponse{
		Mode:        fleet.FetchModeDiff,
		BaseVersion: v1,
		Version:     v2,
		Diff:        base64.StdEncoding.EncodeToString([]byte(`[{"op":"copy","start":999,"count":1}]`)),
		ChainHash:   c2Full,
	}))

	fullFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, c2Full)
	var ffReq fleet.ConfigFetchRequest
	require.NoError(t, json.Unmarshal(fullFetch.Params, &ffReq))
	assert.Equal(t, fleet.ResyncModeFull, ffReq.Resync)
	assert.Empty(t, applied, "corrupt diff must not stage or commit anything")

	require.NoError(t, hub.SendResult(t.Context(), fullFetch.ID, fleet.ConfigFetchResponse{
		Mode:      fleet.FetchModeFull,
		Version:   v2,
		Config:    base64.StdEncoding.EncodeToString(v2Bytes),
		ChainHash: c2Full,
	}))

	ackReq := waitForMethod(t, hub, fleet.VerbConfigAck, c2Full)
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(ackReq.Params, &ack))
	assert.True(t, ack.OK)
	assert.Equal(t, v2, ack.Version)
	assert.Equal(t, c2Full, ack.ChainHash)
	require.NoError(t, hub.SendResult(t.Context(), ackReq.ID, fleet.ManagedReply{ChainHash: c2Full}))

	select {
	case got := <-applied:
		assert.Equal(t, v2Bytes, got)
	case <-time.After(holdWait):
		t.Fatal("full replay config was not committed")
	}

	// Next ping reports converged c2Full.
	ping2 := waitForMethod(t, hub, fleet.VerbPing, c2Full)
	var p2 fleet.PingRequest
	require.NoError(t, json.Unmarshal(ping2.Params, &p2))
	assert.Equal(t, c2Full, p2.ChainHash)
	require.NoError(t, hub.SendResult(t.Context(), ping2.ID, fleet.ManagedReply{ChainHash: c2Full}))

	requireStillConnected(t, returned)
	persisted, present, err := chainAccess.Read()
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, fleet.ChainState{Version: v2, Chain: c2Full}, persisted)
	assert.GreaterOrEqual(t, reg.count("ze_managed_client_chain_mismatches_total"), 1.0)
	assert.Equal(t, 1.0, reg.count("ze_managed_client_resyncs_total:diff,fallback"))
	assert.Equal(t, 1.0, reg.count("ze_managed_client_resyncs_total:full,applied"))
}

// TestClientFailedFullReplayLatchesNoLoop proves AC-11 and R-4: when both diff
// and full replay fail, the client sends config-ack naming both causes and its
// RejectKind, raises doctor-managed-chain-drift, latches the failed target so
// subsequent pings do not loop, and retries as soon as the hub advertises a
// new version/chain.
//
// VALIDATES: spec-fleet-10 AC-11 and R-4 (latch prevents 30s ping resync loops).
// PREVENTS: an unapplyable hub config causing a resync storm on every heartbeat.
func TestClientFailedFullReplayLatchesNoLoop(t *testing.T) {
	// Not parallel: inspects process-wide report warnings.
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	v1Bytes := []byte("system {\n\thost edge-01;\n}\n")
	v2BadBytes := []byte("system {\n\thost edge-01-bad;\n}\n")
	v3GoodBytes := []byte("system {\n\thost edge-01-good;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2BadBytes)
	v3 := fleet.VersionHash(v3GoodBytes)
	c1 := fleet.FoldFull(v1Bytes)
	c2 := fleet.FoldFull(v2BadBytes)
	c3 := fleet.FoldFull(v3GoodBytes)

	addr, admitted := fetchHub(t)
	store := chainMemoryDevice(t, string(v1Bytes), string(v1Bytes))
	chainAccess := NewChainAccess(store)
	require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

	access := NewBaselineAccess(store)
	var statusMu sync.Mutex
	var lastStatus ChainStatus
	cfg, _, applied := holdClient(addr, access)
	cfg.Chain = chainAccess
	cfg.fetchTimeout = holdWait
	cfg.pingInterval = 25 * time.Millisecond
	cfg.pingTimeout = 2 * time.Second
	cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }
	cfg.OnChainState = func(st ChainStatus) {
		statusMu.Lock()
		lastStatus = st
		statusMu.Unlock()
	}
	cfg.Handler = &Handler{
		Validate: func(data []byte) error {
			if bytes.Equal(data, v2BadBytes) {
				return errors.New("bad syntax near secret \"$9$doNotLeakSecretValue\"") // secret-scan: allow
			}
			return nil
		},
	}
	cfg.CommitOver = stagedOver(access, func(data []byte) error {
		require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
		_, baseErr := RecordActiveAsBaseline(store)
		applied <- append([]byte(nil), data...)
		return baseErr
	})

	returned := runFetchConnection(t, &cfg)
	hub := nextSession(t, admitted)

	// Connect fetch -> current (c1).
	initFetch := hubRequest(t, hub)
	require.Equal(t, fleet.VerbConfigFetch, initFetch.Method)
	require.NoError(t, hub.SendResult(t.Context(), initFetch.ID, fleet.ConfigFetchResponse{
		Status:    fetchStatusCurrent,
		ChainHash: c1,
	}))

	// Ping advertises c2 -> triggers diff fetch -> corrupt diff -> full replay -> Validate rejects v2BadBytes.
	ping1 := hubRequest(t, hub)
	require.Equal(t, fleet.VerbPing, ping1.Method)
	require.NoError(t, hub.SendResult(t.Context(), ping1.ID, fleet.ManagedReply{ChainHash: c2}))

	diffFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, c2)
	require.NoError(t, hub.SendResult(t.Context(), diffFetch.ID, fleet.ConfigFetchResponse{
		Mode:        fleet.FetchModeDiff,
		BaseVersion: v1,
		Version:     v2,
		Diff:        base64.StdEncoding.EncodeToString([]byte(`not-valid-json`)),
		ChainHash:   c2,
	}))

	fullFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, c2)
	require.NoError(t, hub.SendResult(t.Context(), fullFetch.ID, fleet.ConfigFetchResponse{
		Mode:      fleet.FetchModeFull,
		Version:   v2,
		Config:    base64.StdEncoding.EncodeToString(v2BadBytes),
		ChainHash: c2,
	}))

	ackReq := waitForMethod(t, hub, fleet.VerbConfigAck, c2)
	var ack fleet.ConfigAck
	require.NoError(t, json.Unmarshal(ackReq.Params, &ack))
	assert.False(t, ack.OK)
	assert.Equal(t, v2, ack.Version)
	assert.Equal(t, fleet.RejectKindVerify, ack.RejectKind)
	assert.Equal(t, c1, ack.ChainHash)
	assert.Contains(t, ack.Error, "diff failed")
	assert.Contains(t, ack.Error, "config failed validation")
	assert.NotContains(t, ack.Error, "doNotLeakSecretValue", "raw validation error must never leak into ConfigAck.Error")
	require.NoError(t, hub.SendResult(t.Context(), ackReq.ID, fleet.ManagedReply{ChainHash: c2}))

	// Drift warning must be raised without leaking raw config/validation text, and status must be mismatched.
	var driftIssue report.Issue
	require.Eventually(t, func() bool {
		var found bool
		driftIssue, found = findChainDriftWarning()
		return found
	}, holdWait, 10*time.Millisecond, "doctor-managed-chain-drift warning was not raised")
	assert.NotContains(t, driftIssue.Message, "doNotLeakSecretValue", "raw validation error must never leak into report warning")
	statusMu.Lock()
	assert.Equal(t, ChainStateMismatched, lastStatus.ChainState)
	assert.NotContains(t, lastStatus.DriftError, "doNotLeakSecretValue", "raw validation error must never leak into DriftError")
	statusMu.Unlock()

	// Next 3 pings advertising the SAME failed c2 MUST NOT trigger any config-fetch!
	for range 3 {
		req := hubRequest(t, hub)
		require.Equal(t, fleet.VerbPing, req.Method, "latched failed target must not re-trigger config-fetch")
		require.NoError(t, hub.SendResult(t.Context(), req.ID, fleet.ManagedReply{ChainHash: c2}))
	}

	// Now hub updates to v3 (c3 != c2) -> next ping immediately triggers resync and converges!
	pingRecover := hubRequest(t, hub)
	require.Equal(t, fleet.VerbPing, pingRecover.Method)
	require.NoError(t, hub.SendResult(t.Context(), pingRecover.ID, fleet.ManagedReply{ChainHash: c3}))

	recoverFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, c3)
	require.NoError(t, hub.SendResult(t.Context(), recoverFetch.ID, fleet.ConfigFetchResponse{
		Mode:      fleet.FetchModeFull,
		Version:   v3,
		Config:    base64.StdEncoding.EncodeToString(v3GoodBytes),
		ChainHash: c3,
	}))

	recoverAck := waitForMethod(t, hub, fleet.VerbConfigAck, c3)
	var rAck fleet.ConfigAck
	require.NoError(t, json.Unmarshal(recoverAck.Params, &rAck))
	assert.True(t, rAck.OK)
	assert.Equal(t, c3, rAck.ChainHash)
	require.NoError(t, hub.SendResult(t.Context(), recoverAck.ID, fleet.ManagedReply{ChainHash: c3}))

	select {
	case got := <-applied:
		assert.Equal(t, v3GoodBytes, got)
	case <-time.After(holdWait):
		t.Fatal("v3GoodBytes was not committed")
	}

	require.Eventually(t, func() bool {
		_, found := findChainDriftWarning()
		return !found
	}, holdWait, 10*time.Millisecond, "doctor-managed-chain-drift warning must clear after convergence")
	requireStillConnected(t, returned)
}

// TestChainMismatchRespectsLocalEditHold proves AC-9 and R-3: when a managed
// client has a local edit (Diverged(active, baseline)), a chain-hash mismatch
// on ping NEVER auto-resyncs or overwrites the local configuration; subsequent
// adopt (current + reanchor) and revert (DiscardVersion + full) end the hold
// and re-anchor the chain.
//
// VALIDATES: spec-fleet-10 AC-9, R-3 (fleet-6/fleet-7 hold precedence over hash chain drift).
// PREVENTS: hash-chain drift detection silently stomping a local emergency edit.
func TestChainMismatchRespectsLocalEditHold(t *testing.T) {
	v1Bytes := []byte(holdEarlierConfig)
	localBytes := []byte(holdDeviceConfig)
	hubBytes := []byte(holdHubConfig)
	c1 := fleet.FoldFull(v1Bytes)
	cHub := fleet.FoldFull(hubBytes)
	cLocal := fleet.FoldFull(localBytes)

	t.Run("ping mismatch does not auto-resync and adopt re-anchors chain", func(t *testing.T) {
		addr, admitted := fetchHub(t)
		store := chainMemoryDevice(t, holdDeviceConfig, holdEarlierConfig)
		chainAccess := NewChainAccess(store)
		require.NoError(t, chainAccess.Write(fleet.ChainState{Version: fleet.VersionHash(v1Bytes), Chain: c1}))

		access := NewBaselineAccess(store)
		cfg, _, applied := holdClient(addr, access)
		cfg.Chain = chainAccess
		cfg.fetchTimeout = holdWait
		cfg.pingInterval = 25 * time.Millisecond
		cfg.pingTimeout = 2 * time.Second
		cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }

		returned := runFetchConnection(t, &cfg)
		hub := nextSession(t, admitted)

		// Initial connect fetch -> hub offers hubBytes -> device holds.
		initFetch := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigFetch, initFetch.Method)
		require.NoError(t, hub.SendResult(t.Context(), initFetch.ID, fleet.ConfigFetchResponse{
			Version:   fleet.VersionHash(hubBytes),
			Config:    base64.StdEncoding.EncodeToString(hubBytes),
			ChainHash: cHub,
		}))
		heldReq := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigHeld, heldReq.Method)
		require.NoError(t, hub.SendResult(t.Context(), heldReq.ID, fleet.ManagedReply{ChainHash: cHub}))
		holdAck := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigAck, holdAck.Method)
		var hAck fleet.ConfigAck
		require.NoError(t, json.Unmarshal(holdAck.Params, &hAck))
		assert.True(t, hAck.Holding)
		require.NoError(t, hub.SendResult(t.Context(), holdAck.ID, fleet.ManagedReply{ChainHash: cHub}))

		// 3 pings with mismatched hubChain: client must NOT issue any config-fetch!
		for range 3 {
			req := hubRequest(t, hub)
			require.Equal(t, fleet.VerbPing, req.Method, "holding client must not auto-resync on ping mismatch")
			require.NoError(t, hub.SendResult(t.Context(), req.ID, fleet.ManagedReply{ChainHash: cHub}))
		}
		assert.Empty(t, applied, "local edit must never be overwritten while holding")

		// Operator runs adopt on hub -> hub pushes config-changed -> replies current + reanchor = FoldFull(localBytes).
		require.NoError(t, pushChanged(t, hub))
		adoptFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, cLocal)
		require.NoError(t, hub.SendResult(t.Context(), adoptFetch.ID, fleet.ConfigFetchResponse{
			Status:    fetchStatusCurrent,
			Reanchor:  true,
			ChainHash: cLocal,
		}))
		adoptAck := waitForMethod(t, hub, fleet.VerbConfigAck, cLocal)
		var aAck fleet.ConfigAck
		require.NoError(t, json.Unmarshal(adoptAck.Params, &aAck))
		assert.True(t, aAck.OK)
		assert.Equal(t, fleet.VersionHash(localBytes), aAck.Version)
		assert.Equal(t, cLocal, aAck.ChainHash)
		require.NoError(t, hub.SendResult(t.Context(), adoptAck.ID, fleet.ManagedReply{ChainHash: cLocal}))

		requireStillConnected(t, returned)
		persisted, present, err := chainAccess.Read()
		require.NoError(t, err)
		require.True(t, present)
		assert.Equal(t, fleet.ChainState{Version: fleet.VersionHash(localBytes), Chain: cLocal}, persisted)
	})

	t.Run("revert ends hold and re-anchors chain to FoldFull(hubBytes)", func(t *testing.T) {
		store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
		chainAccess := NewChainAccess(store)
		require.NoError(t, chainAccess.Write(fleet.ChainState{Version: fleet.VersionHash(v1Bytes), Chain: c1}))
		access := NewBaselineAccess(store)
		var committed []byte
		cfg := ClientConfig{
			Name:     "edge-01",
			Baseline: access,
			Chain:    chainAccess,
			Handler:  &Handler{Validate: func([]byte) error { return nil }},
			CommitOver: stagedOver(access, func(data []byte) error {
				committed = append([]byte(nil), data...)
				require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
				_, baseErr := RecordActiveAsBaseline(store)
				return baseErr
			}),
		}
		mc, seen := holdPipe(t, func(req *rpc.Request) any {
			switch req.Method {
			case fleet.VerbConfigFetch:
				return fleet.ConfigFetchResponse{
					Mode:           fleet.FetchModeFull,
					Version:        fleet.VersionHash(hubBytes),
					Config:         base64.StdEncoding.EncodeToString(hubBytes),
					DiscardVersion: fleet.VersionHash(localBytes),
					ChainHash:      cHub,
				}
			case fleet.VerbConfigAck:
				return fleet.ManagedReply{ChainHash: cHub}
			default:
				return fleet.ManagedReply{}
			}
		})

		require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))
		require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
		ack := ackOf(t, seen)
		assert.True(t, ack.OK)
		assert.Equal(t, cHub, ack.ChainHash)
		assert.Equal(t, hubBytes, committed)

		persisted, present, err := chainAccess.Read()
		require.NoError(t, err)
		require.True(t, present)
		assert.Equal(t, fleet.ChainState{Version: fleet.VersionHash(hubBytes), Chain: cHub}, persisted)
	})
}

// TestClientDiffResyncPreservesConcurrentLocalCommit proves AC-10 and A-2:
// when a local commit lands while a Mode=="diff" fetch is in flight,
// StageMachineCandidateOver (activeCheck) returns errActiveChanged, the client
// leaves meta/managed/chain-state untouched, refetches once over the local
// edit, and enters hold.
//
// VALIDATES: spec-fleet-10 AC-10 and A-2.
// PREVENTS: a racing local commit being overwritten or folded into the hub chain.
func TestClientDiffResyncPreservesConcurrentLocalCommit(t *testing.T) {
	t.Parallel()

	const baseBlock = "system {\n\thost edge-01;\n\tdomain example.net;\n}\n" +
		"interfaces {\n\tinterface eth0 {\n\t\tdescription \"uplink transit primary\";\n\t\tmtu 1500;\n\t}\n" +
		"\tinterface eth1 {\n\t\tdescription \"uplink transit backup\";\n\t\tmtu 1500;\n\t}\n}\n"
	v1Bytes := []byte(baseBlock + "bgp {\n\trouter-id 192.0.2.1;\n}\n")
	v2Bytes := []byte(baseBlock + "bgp {\n\trouter-id 192.0.2.2;\n}\n")
	localEditBytes := []byte("system {\n\thost edge-01-emergency;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c1 := fleet.FoldFull(v1Bytes)
	delta, worth, err := fleet.ComputeDiff(v1Bytes, v2Bytes)
	require.NoError(t, err)
	require.True(t, worth)
	c2, err := fleet.FoldDiff(c1, v1, v2, delta)
	require.NoError(t, err)

	store := holdDevice(t, string(v1Bytes), string(v1Bytes))
	chainAccess := NewChainAccess(store)
	require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))
	access := NewBaselineAccess(store)

	var appliedCount atomic.Int32
	var firstCommitCheck atomic.Bool
	cfg := ClientConfig{
		Name:     "edge-01",
		Baseline: access,
		Chain:    chainAccess,
		Handler:  &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: func(data []byte, check func(active []byte) error) error {
			if !firstCommitCheck.Swap(true) {
				// Simulate a local edit landing in the store while diff fetch was in flight.
				require.NoError(t, store.WriteFile(fleetStatusConfigName, localEditBytes, 0))
			}
			return stagedOver(access, func([]byte) error {
				appliedCount.Add(1)
				return nil
			})(data, check)
		},
	}

	var fetchCount atomic.Int32
	mc, seen := holdPipe(t, func(req *rpc.Request) any {
		switch req.Method {
		case fleet.VerbConfigFetch:
			if fetchCount.Add(1) == 1 {
				return fleet.ConfigFetchResponse{
					Mode:        fleet.FetchModeDiff,
					BaseVersion: v1,
					Version:     v2,
					Diff:        base64.StdEncoding.EncodeToString(delta),
					ChainHash:   c2,
				}
			}
			return fleet.ConfigFetchResponse{
				Mode:      fleet.FetchModeFull,
				Version:   v2,
				Config:    base64.StdEncoding.EncodeToString(v2Bytes),
				ChainHash: fleet.FoldFull(v2Bytes),
			}
		case fleet.VerbConfigHeld, fleet.VerbConfigAck:
			return fleet.ManagedReply{ChainHash: c2}
		default:
			return fleet.ManagedReply{}
		}
	})

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

	// First fetch -> errActiveChanged -> refetch over localEditBytes -> config-held -> holding ack.
	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	secondFetch := nextRequest(t, seen)
	assert.Equal(t, fleet.VersionHash(localEditBytes), sentVersion(t, secondFetch))
	require.Equal(t, fleet.VerbConfigHeld, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.False(t, ack.OK)
	assert.True(t, ack.Holding)
	assert.Zero(t, appliedCount.Load(), "diff must not commit over concurrent local edit")

	// Chain state must remain at {v1, c1}, untouched.
	persisted, present, err := chainAccess.Read()
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, fleet.ChainState{Version: v1, Chain: c1}, persisted)

	// Local edit must remain intact in store.
	gotActive, err := store.ReadFile(fleetStatusConfigName)
	require.NoError(t, err)
	assert.Equal(t, localEditBytes, gotActive)
}

// TestDiffWorksWithUnknownExternalPluginSection proves A-4: a configuration
// containing an unknown external-plugin section (unchecked-plugins on the hub),
// comments, and $9$ secrets diffs cleanly on the hub without YANG parsing,
// reconstructs byte-identically on the client, passes client validation, and
// advances the hash chain.
//
// VALIDATES: spec-fleet-10 A-4 and AC-4.
// PREVENTS: external-plugin sections forcing full config replay on every edit.
func TestDiffWorksWithUnknownExternalPluginSection(t *testing.T) {
	t.Parallel()

	v1Bytes := []byte(
		"# edge-01 managed config\n" +
			"system {\n\thost edge-01;\n\tsecret \"$9$opaqueSaltValue123456\";\n}\n" +
			"vendor-ext-plugin {\n\tendpoint primary;\n\tinterval 15;\n}\n" +
			"bgp {\n\trouter-id 192.0.2.10;\n}\n",
	)
	v2Bytes := []byte(
		"# edge-01 managed config\n" +
			"system {\n\thost edge-01;\n\tsecret \"$9$opaqueSaltValue123456\";\n}\n" +
			"vendor-ext-plugin {\n\tendpoint primary;\n\tinterval 30;\n}\n" +
			"bgp {\n\trouter-id 192.0.2.10;\n}\n",
	)
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c1 := fleet.FoldFull(v1Bytes)

	delta, worth, err := fleet.ComputeDiff(v1Bytes, v2Bytes)
	require.NoError(t, err)
	require.True(t, worth)
	c2, err := fleet.FoldDiff(c1, v1, v2, delta)
	require.NoError(t, err)

	store := holdDevice(t, string(v1Bytes), string(v1Bytes))
	chainAccess := NewChainAccess(store)
	require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))
	access := NewBaselineAccess(store)

	var validated []byte
	var committed []byte
	cfg := ClientConfig{
		Name:     "edge-01",
		Baseline: access,
		Chain:    chainAccess,
		Handler: &Handler{
			Validate: func(data []byte) error {
				validated = append([]byte(nil), data...)
				if !strings.Contains(string(data), "vendor-ext-plugin") || !strings.Contains(string(data), "interval 30;") {
					return errors.New("external plugin section missing or corrupted")
				}
				return nil
			},
		},
		CommitOver: stagedOver(access, func(data []byte) error {
			committed = append([]byte(nil), data...)
			require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
			_, baseErr := RecordActiveAsBaseline(store)
			return baseErr
		}),
	}

	mc, seen := holdPipe(t, func(req *rpc.Request) any {
		switch req.Method {
		case fleet.VerbConfigFetch:
			return fleet.ConfigFetchResponse{
				Mode:        fleet.FetchModeDiff,
				BaseVersion: v1,
				Version:     v2,
				Diff:        base64.StdEncoding.EncodeToString(delta),
				ChainHash:   c2,
			}
		case fleet.VerbConfigAck:
			return fleet.ManagedReply{ChainHash: c2}
		default:
			return fleet.ManagedReply{}
		}
	})

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))
	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.True(t, ack.OK)
	assert.Equal(t, c2, ack.ChainHash)
	assert.Equal(t, v2Bytes, validated, "reconstructed bytes must preserve comments, $9$ secrets, and external-plugin section byte-for-byte")
	assert.Equal(t, v2Bytes, committed)

	persisted, present, err := chainAccess.Read()
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, fleet.ChainState{Version: v2, Chain: c2}, persisted)
}

// TestSendPingRespectsTimeoutBound proves R-6: a stalled hub that never
// answers VerbPing causes sendPing to fail with context.DeadlineExceeded
// within its pingTimeout bound rather than blocking forever.
//
// VALIDATES: spec-fleet-10 R-6 and pingTimeout boundary table.
// PREVENTS: a stalled hub hanging the ping goroutine and leaking it across shutdown.
func TestSendPingRespectsTimeoutBound(t *testing.T) {
	t.Parallel()

	clientEnd, hubEnd := net.Pipe()
	t.Cleanup(func() {
		_ = clientEnd.Close()
		_ = hubEnd.Close()
	})
	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	t.Cleanup(func() { _ = mc.Close() })

	// Hub reads the ping request and deliberately never answers.
	hubRC := rpc.NewConn(hubEnd, hubEnd)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = hubRC.ReadRequest(t.Context())
	}()

	start := time.Now()
	_, err := sendPing(t.Context(), mc, fleet.FoldFull([]byte("cfg")), 80*time.Millisecond)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 2*time.Second)
	<-readDone
}

// waitForMethod reads requests from hub until wantMethod arrives (bounded by
// holdWait), answering any interleaved VerbPing requests with
// ManagedReply{ChainHash: pingReplyChain}.
func waitForMethod(t *testing.T, hub fetchSession, wantMethod, pingReplyChain string) *rpc.Request {
	t.Helper()
	timer := time.NewTimer(holdWait)
	defer timer.Stop()
	for {
		select {
		case req, ok := <-hub.Requests():
			require.True(t, ok, "the client closed the connection while waiting for %q", wantMethod)
			if req.Method == wantMethod {
				return req
			}
			if req.Method == fleet.VerbPing {
				require.NoError(t, hub.SendResult(t.Context(), req.ID, fleet.ManagedReply{ChainHash: pingReplyChain}))
				continue
			}
			t.Fatalf("unexpected RPC method %q while waiting for %q", req.Method, wantMethod)
		case <-timer.C:
			t.Fatalf("did not receive %q within %s", wantMethod, holdWait)
			return nil
		}
	}
}

// TestFailedAckDoesNotSpinDriftOrBumpMismatch proves BLOCKER-1 is fixed:
//   - A non-OK config-ack never signals driftCh and never bumps
//     ze_managed_client_chain_mismatches_total, even when the hub's ack reply
//     carries a different ChainHash.
//   - Both triggerChain (from config-changed) and reply.ChainHash (from the
//     hub's ack reply) are latched on failure, preventing tight driftCh spin
//     loops and duplicate diff fetches/commits.
//
// VALIDATES: spec-fleet-10 AC-11, R-4, and Round 1 Review BLOCKER-1.
// PREVENTS: a failed config-ack reply re-triggering driftCh in an unbounded
// loop or inflating client mismatch counters.
func TestFailedAckDoesNotSpinDriftOrBumpMismatch(t *testing.T) {
	const baseBlock = "system {\n\thost edge-01;\n\tdomain example.net;\n}\n" +
		"interfaces {\n\tinterface eth0 {\n\t\tdescription \"uplink transit primary\";\n\t\tmtu 1500;\n\t}\n" +
		"\tinterface eth1 {\n\t\tdescription \"uplink transit backup\";\n\t\tmtu 1500;\n\t}\n}\n"
	v1Bytes := []byte(baseBlock + "bgp {\n\trouter-id 192.0.2.1;\n}\n")
	v2Bytes := []byte(baseBlock + "bgp {\n\trouter-id 192.0.2.2;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c1 := fleet.FoldFull(v1Bytes)
	c2Full := fleet.FoldFull(v2Bytes)
	c2BadResp := strings.Repeat("b", fleet.ChainHashLen)
	c2Reply := strings.Repeat("c", fleet.ChainHashLen)
	c2Notify := strings.Repeat("d", fleet.ChainHashLen)

	t.Run("mismatched fetch chain-hash latches reply chain without spinning driftCh", func(t *testing.T) {
		addr, admitted := fetchHub(t)
		store := chainMemoryDevice(t, string(v1Bytes), string(v1Bytes))
		chainAccess := NewChainAccess(store)
		require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

		reg := newTestChainMetrics()
		access := NewBaselineAccess(store)
		cfg, _, _ := holdClient(addr, access)
		cfg.Chain = chainAccess
		cfg.Metrics = reg
		cfg.fetchTimeout = holdWait
		cfg.pingInterval = 25 * time.Millisecond
		cfg.pingTimeout = 2 * time.Second
		cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }

		returned := runFetchConnection(t, &cfg)
		hub := nextSession(t, admitted)

		// Initial connect fetch returns full config with mismatched resp.ChainHash,
		// and ack reply advertises a distinct hubExpected chain (c2Reply).
		initFetch := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigFetch, initFetch.Method)
		require.NoError(t, hub.SendResult(t.Context(), initFetch.ID, fleet.ConfigFetchResponse{
			Mode:      fleet.FetchModeFull,
			Version:   v2,
			Config:    base64.StdEncoding.EncodeToString(v2Bytes),
			ChainHash: c2BadResp,
		}))

		ackReq := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigAck, ackReq.Method)
		var ack fleet.ConfigAck
		require.NoError(t, json.Unmarshal(ackReq.Params, &ack))
		assert.False(t, ack.OK)
		assert.Equal(t, fleet.RejectKindResync, ack.RejectKind)
		require.NoError(t, hub.SendResult(t.Context(), ackReq.ID, fleet.ManagedReply{ChainHash: c2Reply}))

		// Subsequent pings advertising c2Reply (or c2BadResp) MUST NOT trigger any config-fetch!
		for _, advertised := range []string{c2Reply, c2BadResp, c2Reply} {
			req := hubRequest(t, hub)
			require.Equal(t, fleet.VerbPing, req.Method, "failed ack reply must not spin driftCh or trigger refetch")
			require.NoError(t, hub.SendResult(t.Context(), req.ID, fleet.ManagedReply{ChainHash: advertised}))
		}
		assert.Zero(t, reg.count("ze_managed_client_chain_mismatches_total"),
			"non-OK config-ack reply must not bump client chain mismatch counter")
		requireStillConnected(t, returned)
	})

	t.Run("diff commit failure after config-changed latches triggerChain and replyChain without double fetch", func(t *testing.T) {
		delta, worth, err := fleet.ComputeDiff(v1Bytes, v2Bytes)
		require.NoError(t, err)
		require.True(t, worth)
		c2Diff, err := fleet.FoldDiff(c1, v1, v2, delta)
		require.NoError(t, err)

		addr, admitted := fetchHub(t)
		store := chainMemoryDevice(t, string(v1Bytes), string(v1Bytes))
		chainAccess := NewChainAccess(store)
		require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

		reg := newTestChainMetrics()
		access := NewBaselineAccess(store)
		var commitCalls atomic.Int32
		cfg, _, _ := holdClient(addr, access)
		cfg.Chain = chainAccess
		cfg.Metrics = reg
		cfg.fetchTimeout = holdWait
		cfg.pingInterval = 25 * time.Millisecond
		cfg.pingTimeout = 2 * time.Second
		cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }
		cfg.CommitOver = stagedOver(access, func([]byte) error {
			commitCalls.Add(1)
			return errors.New("confirmed-commit window open")
		})

		returned := runFetchConnection(t, &cfg)
		hub := nextSession(t, admitted)

		// Connect fetch -> current c1.
		initFetch := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigFetch, initFetch.Method)
		require.NoError(t, hub.SendResult(t.Context(), initFetch.ID, fleet.ConfigFetchResponse{
			Status:    fetchStatusCurrent,
			ChainHash: c1,
		}))

		// Hub pushes config-changed carrying c2Notify; diff CommitOver fails;
		// hub ack reply advertises c2Full (different from c2Diff and c2Notify).
		ctx, cancel := context.WithTimeout(t.Context(), holdWait)
		defer cancel()
		_, pushErr := hub.CallRPC(ctx, fleet.VerbConfigChanged, fleet.ConfigChanged{
			Version:   v2,
			ChainHash: c2Notify,
		})
		require.NoError(t, pushErr)

		diffFetch := waitForMethod(t, hub, fleet.VerbConfigFetch, c1)
		require.NoError(t, hub.SendResult(t.Context(), diffFetch.ID, fleet.ConfigFetchResponse{
			Mode:        fleet.FetchModeDiff,
			BaseVersion: v1,
			Version:     v2,
			Diff:        base64.StdEncoding.EncodeToString(delta),
			ChainHash:   c2Diff,
		}))

		ackReq := waitForMethod(t, hub, fleet.VerbConfigAck, c1)
		var ack fleet.ConfigAck
		require.NoError(t, json.Unmarshal(ackReq.Params, &ack))
		assert.False(t, ack.OK)
		assert.Equal(t, fleet.RejectKindCommit, ack.RejectKind)
		require.NoError(t, hub.SendResult(t.Context(), ackReq.ID, fleet.ManagedReply{ChainHash: c2Full}))

		// Next pings advertising c2Notify, c2Diff, and c2Full must all stay latched (no double fetch/commit).
		for _, advertised := range []string{c2Notify, c2Diff, c2Full} {
			req := hubRequest(t, hub)
			require.Equal(t, fleet.VerbPing, req.Method, "latched chain %s must not trigger duplicate fetch", advertised)
			require.NoError(t, hub.SendResult(t.Context(), req.ID, fleet.ManagedReply{ChainHash: advertised}))
		}
		assert.Equal(t, int32(1), commitCalls.Load(), "failed diff commit must not double-commit")
		assert.Zero(t, reg.count("ze_managed_client_chain_mismatches_total"))
		requireStillConnected(t, returned)
	})
}

// TestLatchedFailureRetriesOnNewHubChainAndReconnect proves that a latched
// failure retries immediately when the hub advances to a brand-new chain hash
// (neither failedChain, failedTriggerChain, nor failedReplyChain) AND retries
// cleanly after a session reconnect clears the per-connection latch.
//
// VALIDATES: spec-fleet-10 AC-11 recovery paths (new hub chain + reconnect).
// PREVENTS: a client permanently stuck latched after the fault is cleared or
// across a reconnect.
func TestLatchedFailureRetriesOnNewHubChainAndReconnect(t *testing.T) {
	v1Bytes := []byte("system {\n\thost edge-01;\n}\n")
	v2Bytes := []byte("system {\n\thost edge-01-v2;\n}\n")
	v3Bytes := []byte("system {\n\thost edge-01-v3;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	v3 := fleet.VersionHash(v3Bytes)
	c1 := fleet.FoldFull(v1Bytes)
	c2 := fleet.FoldFull(v2Bytes)
	c2Reply := strings.Repeat("e", fleet.ChainHashLen)
	c3 := fleet.FoldFull(v3Bytes)

	t.Run("retries on brand-new hub chain hash", func(t *testing.T) {
		addr, admitted := fetchHub(t)
		store := chainMemoryDevice(t, string(v1Bytes), string(v1Bytes))
		chainAccess := NewChainAccess(store)
		require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

		access := NewBaselineAccess(store)
		var failV2 atomic.Bool
		failV2.Store(true)
		cfg, _, applied := holdClient(addr, access)
		cfg.Chain = chainAccess
		cfg.fetchTimeout = holdWait
		cfg.pingInterval = 25 * time.Millisecond
		cfg.pingTimeout = 2 * time.Second
		cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }
		cfg.CommitOver = stagedOver(access, func(data []byte) error {
			if failV2.Load() && bytes.Equal(data, v2Bytes) {
				return errors.New("temporary commit refusal")
			}
			require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
			_, baseErr := RecordActiveAsBaseline(store)
			applied <- append([]byte(nil), data...)
			return baseErr
		})

		returned := runFetchConnection(t, &cfg)
		hub := nextSession(t, admitted)

		// Connect fetch returns v2 -> CommitOver fails; ack reply carries c2Reply.
		initFetch := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigFetch, initFetch.Method)
		require.NoError(t, hub.SendResult(t.Context(), initFetch.ID, fleet.ConfigFetchResponse{
			Mode:      fleet.FetchModeFull,
			Version:   v2,
			Config:    base64.StdEncoding.EncodeToString(v2Bytes),
			ChainHash: c2,
		}))
		ack1 := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigAck, ack1.Method)
		require.NoError(t, hub.SendResult(t.Context(), ack1.ID, fleet.ManagedReply{ChainHash: c2Reply}))

		// Pings advertising c2 or c2Reply stay latched.
		for _, advertised := range []string{c2, c2Reply} {
			req := hubRequest(t, hub)
			require.Equal(t, fleet.VerbPing, req.Method)
			require.NoError(t, hub.SendResult(t.Context(), req.ID, fleet.ManagedReply{ChainHash: advertised}))
		}

		// Ping advertising brand-new c3 unlatches and converges.
		pingNew := hubRequest(t, hub)
		require.Equal(t, fleet.VerbPing, pingNew.Method)
		require.NoError(t, hub.SendResult(t.Context(), pingNew.ID, fleet.ManagedReply{ChainHash: c3}))

		fetch3 := waitForMethod(t, hub, fleet.VerbConfigFetch, c3)
		require.NoError(t, hub.SendResult(t.Context(), fetch3.ID, fleet.ConfigFetchResponse{
			Mode:      fleet.FetchModeFull,
			Version:   v3,
			Config:    base64.StdEncoding.EncodeToString(v3Bytes),
			ChainHash: c3,
		}))
		ack3 := waitForMethod(t, hub, fleet.VerbConfigAck, c3)
		var a3 fleet.ConfigAck
		require.NoError(t, json.Unmarshal(ack3.Params, &a3))
		assert.True(t, a3.OK)
		assert.Equal(t, c3, a3.ChainHash)
		require.NoError(t, hub.SendResult(t.Context(), ack3.ID, fleet.ManagedReply{ChainHash: c3}))

		select {
		case got := <-applied:
			assert.Equal(t, v3Bytes, got)
		case <-time.After(holdWait):
			t.Fatal("v3Bytes was not committed after new hub chain")
		}
		requireStillConnected(t, returned)
	})

	t.Run("retries same chain after reconnect clears session latch", func(t *testing.T) {
		addr, admitted := fetchHub(t)
		store := chainMemoryDevice(t, string(v1Bytes), string(v1Bytes))
		chainAccess := NewChainAccess(store)
		require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

		access := NewBaselineAccess(store)
		var failOnce atomic.Bool
		failOnce.Store(true)
		cfg, _, applied := holdClient(addr, access)
		cfg.Chain = chainAccess
		cfg.fetchTimeout = holdWait
		cfg.pingInterval = 25 * time.Millisecond
		cfg.pingTimeout = 2 * time.Second
		cfg.reportProducersFn = func() []fleet.ReportProducer { return nil }
		cfg.CommitOver = stagedOver(access, func(data []byte) error {
			if failOnce.Swap(false) {
				return errors.New("transient session-1 fault")
			}
			require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
			_, baseErr := RecordActiveAsBaseline(store)
			applied <- append([]byte(nil), data...)
			return baseErr
		})

		_ = runClient(t, cfg)

		// Session 1: v2 fails commit and latches c2 + c2Reply.
		first := nextSession(t, admitted)
		f1 := hubRequest(t, first)
		require.Equal(t, fleet.VerbConfigFetch, f1.Method)
		require.NoError(t, first.SendResult(t.Context(), f1.ID, fleet.ConfigFetchResponse{
			Mode:      fleet.FetchModeFull,
			Version:   v2,
			Config:    base64.StdEncoding.EncodeToString(v2Bytes),
			ChainHash: c2,
		}))
		a1 := hubRequest(t, first)
		require.Equal(t, fleet.VerbConfigAck, a1.Method)
		require.NoError(t, first.SendResult(t.Context(), a1.ID, fleet.ManagedReply{ChainHash: c2Reply}))

		// Drop session 1 -> client reconnects, resets session latch, retries v2 and converges.
		first.drop()

		second := nextSession(t, admitted)
		f2 := hubRequest(t, second)
		require.Equal(t, fleet.VerbConfigFetch, f2.Method)
		require.NoError(t, second.SendResult(t.Context(), f2.ID, fleet.ConfigFetchResponse{
			Mode:      fleet.FetchModeFull,
			Version:   v2,
			Config:    base64.StdEncoding.EncodeToString(v2Bytes),
			ChainHash: c2,
		}))
		a2 := waitForMethod(t, second, fleet.VerbConfigAck, c2)
		var ack2 fleet.ConfigAck
		require.NoError(t, json.Unmarshal(a2.Params, &ack2))
		assert.True(t, ack2.OK)
		assert.Equal(t, c2, ack2.ChainHash)
		require.NoError(t, second.SendResult(t.Context(), a2.ID, fleet.ManagedReply{ChainHash: c2}))

		select {
		case got := <-applied:
			assert.Equal(t, v2Bytes, got)
		case <-time.After(holdWait):
			t.Fatal("v2Bytes was not committed after reconnect cleared latch")
		}
	})
}
