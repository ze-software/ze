// VALIDATES: fleet-3 Phase 2 -- ManagedServer audit lifecycle, live producer
// wiring (device.connect, config.fetch, config.push, config.ack, config.reject
// for ordinary and holding acks without leaking raw peer errors or secrets,
// config.held without config bytes, VerbFleetAuditReceipts with session-bound
// identity and idempotent replay, device.disconnect), reload retention ceiling
// validation and pruning via PrepareManagedListenersWithHistoryAlarmsAndAudit
// and ApplyManagedListeners, ManagedAuditQuery, and ManagedRuntimeHealth().Audit.
// PREVENTS: missing audit events on managed sessions, leaking config text or
// peer error strings into audit entries, race loss of final device.disconnect
// on shutdown, unpruned retention reductions, and unreported corrupt history.

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

func startAuditWiringTestServer(
	t *testing.T,
	store storage.Storage,
	clk *sim.FakeClock,
	auditCfg plugin.HubAuditConfig,
) (*ManagedServer, string) {
	t.Helper()
	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Store:     store,
		Clock:     clk,
		Audit:     auditCfg,
		Listeners: []ManagedListener{{
			Addr: "127.0.0.1:0",
			Clients: map[string]string{
				testClientName:  testClientSecret,
				testClient2Name: testClient2Secret,
			},
		}},
		ReadConfig: func(name string) ([]byte, error) {
			switch name {
			case testClientName:
				return []byte(testClientConfig), nil
			case testClient2Name:
				return []byte(testClient2Config), nil
			default:
				return nil, ErrClientConfigNotFound
			}
		},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	require.Len(t, addrs, 1)
	return srv, addrs[0].String()
}

func auditEntriesOfType(entries []FleetAuditEntry, eventType string) []FleetAuditEntry {
	var out []FleetAuditEntry
	for i := range entries {
		if entries[i].Type == eventType {
			out = append(out, entries[i])
		}
	}
	return out
}

// VALIDATES: AC-1..AC-6, AC-14 -- real TLS managed session emits connect,
// non-current fetch (and skips current fetch), push, ack, ordinary and holding
// reject (without raw peer errors), held (without config bytes), receipts RPC
// (session-bound & idempotent), and disconnect.
func TestManagedServerAuditWiringEndToEnd(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 6, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()
	srv, addr := startAuditWiringTestServer(t, store, clk, plugin.HubAuditConfig{MaxEntries: 500})

	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	waitForConnected(t, srv, testClientName, true)

	// 1. Config-fetch (new config) emits config.fetch.
	clk.Add(time.Second)
	resp := fetch(t, mc, "")
	wantVersion := fleet.VersionHash([]byte(testClientConfig))
	require.Equal(t, wantVersion, resp.Version)

	// 2. Config-fetch (current) MUST NOT emit config.fetch (AC-3).
	clk.Add(time.Second)
	currentResp := fetch(t, mc, wantVersion)
	require.Equal(t, managedFetchStatusCurrent, currentResp.Status)

	// 3. Config-changed push emits config.push once the client replies OK.
	clk.Add(time.Second)
	srv.NotifyConfigChanged(testClientName)
	select {
	case req, ok := <-mc.Requests():
		require.True(t, ok)
		require.Equal(t, fleet.VerbConfigChanged, req.Method)
		require.NoError(t, mc.SendOK(t.Context(), req.ID))
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for config-changed push")
	}

	// 4. Positive config-ack emits config.ack.
	clk.Add(time.Second)
	callCtx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	_, err := mc.CallRPC(callCtx, fleet.VerbConfigAck, fleet.ConfigAck{
		Version: wantVersion,
		OK:      true,
	})
	cancel()
	require.NoError(t, err)

	// 5. Ordinary negative config-ack with secret-carrying peer error emits
	// config.reject with safe static code/summary and ZERO raw peer error leakage.
	clk.Add(time.Second)
	const secretCanary = "SUPER-SECRET-PEER-PASSWORD-987654"
	callCtx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	_, err = mc.CallRPC(callCtx, fleet.VerbConfigAck, fleet.ConfigAck{
		Version:    wantVersion,
		OK:         false,
		RejectKind: fleet.RejectKindVerify,
		Error:      "parse error near secret " + secretCanary,
	})
	cancel()
	require.NoError(t, err)

	// 6. config-held emits config.held with version + edited-at and ZERO config bytes.
	clk.Add(time.Second)
	const heldSecretCanary = "HELD-CONFIG-SECRET-CANARY-4321"
	heldText := "# " + heldSecretCanary + "\n\n"
	heldVersion := fleet.VersionHash([]byte(heldText))
	editedAt := t0.Add(-5 * time.Minute)
	callCtx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	_, err = mc.CallRPC(callCtx, fleet.VerbConfigHeld, fleet.ConfigHeld{
		Version:  heldVersion,
		Config:   base64.StdEncoding.EncodeToString([]byte(heldText)),
		EditedAt: editedAt,
	})
	cancel()
	require.NoError(t, err)

	// Fetch reporting heldVersion so session.fetched matches held record, then
	// send holding config-ack (holdShownFromCopy path) AND non-copy holding ack.
	clk.Add(time.Second)
	_ = fetch(t, mc, heldVersion)
	callCtx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	_, err = mc.CallRPC(callCtx, fleet.VerbConfigAck, fleet.ConfigAck{
		Version: wantVersion,
		OK:      false,
		Holding: true,
		Error:   "holding with " + secretCanary,
	})
	cancel()
	require.NoError(t, err)

	// Non-copy holding ack (different fetched version).
	clk.Add(time.Second)
	_ = fetch(t, mc, "sha256:other-local-version")
	callCtx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	_, err = mc.CallRPC(callCtx, fleet.VerbConfigAck, fleet.ConfigAck{
		Version: wantVersion,
		OK:      false,
		Holding: true,
		Error:   "holding without copy " + secretCanary,
	})
	cancel()
	require.NoError(t, err)

	// 7. VerbFleetAuditReceipts: strict decode refuses unknown fields; valid batch
	// binds device to authenticated session edge-01 and deduplicates on retry.
	clk.Add(time.Second)
	callCtx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	_, err = mc.CallRPC(callCtx, fleet.VerbFleetAuditReceipts, map[string]any{
		"epoch":         "ep-1",
		"unknown-field": true,
	})
	cancel()
	require.Error(t, err, "unknown field in receipt batch must be rejected")

	receiptBatch := fleet.AuditReceiptBatch{
		Epoch: "ep-edge-01",
		Entries: []fleet.AuditReceiptEntry{
			{
				Seq:     1,
				Action:  fleet.FleetAuditReceiptActionDisable,
				Actor:   "admin-local",
				Surface: "cli",
				EventAt: t0.Add(-10 * time.Minute),
				Outcome: "success",
			},
		},
	}
	for range 2 {
		callCtx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
		rawAck, rpcErr := mc.CallRPC(callCtx, fleet.VerbFleetAuditReceipts, receiptBatch)
		cancel()
		require.NoError(t, rpcErr)
		var ack fleet.AuditReceiptAck
		require.NoError(t, json.Unmarshal(rawAck, &ack))
		assert.Equal(t, "ep-edge-01", ack.Epoch)
		assert.Equal(t, uint64(1), ack.Committed)
	}

	// 8. Close client connection and wait for disconnect audit entry.
	clk.Add(time.Second)
	require.NoError(t, mc.Close())
	waitForConnected(t, srv, testClientName, false)
	require.NoError(t, srv.Audit().Flush(t.Context()))

	entries, err := ManagedAuditQuery(allowAllFleetScope(), FleetAuditQuery{Device: testClientName})
	require.NoError(t, err)

	// Verify NO secret canary ever entered any serialized audit entry.
	rawAll, err := json.Marshal(entries)
	require.NoError(t, err)
	assert.NotContains(t, string(rawAll), secretCanary)
	assert.NotContains(t, string(rawAll), heldSecretCanary)

	// Verify each event class.
	connects := auditEntriesOfType(entries, FleetAuditDeviceConnect)
	require.Len(t, connects, 1)
	assert.Equal(t, fleetAuditActorHub, connects[0].Actor)
	assert.Equal(t, fleetAuditSurfaceSession, connects[0].Surface)
	assert.Equal(t, "127.0.0.1", connects[0].Metadata["source-address"])

	disconnects := auditEntriesOfType(entries, FleetAuditDeviceDisconnect)
	require.Len(t, disconnects, 1)

	// 3 non-current fetches (initial "", heldVersion, "sha256:other-local-version"),
	// and ZERO for the "current" fetch.
	fetches := auditEntriesOfType(entries, FleetAuditConfigFetch)
	require.Len(t, fetches, 3)
	for _, f := range fetches {
		assert.Equal(t, wantVersion, f.Version)
		assert.Equal(t, fleet.FetchModeFull, f.Metadata["mode"])
	}

	pushes := auditEntriesOfType(entries, FleetAuditConfigPush)
	require.Len(t, pushes, 1)
	assert.Equal(t, wantVersion, pushes[0].Version)

	acks := auditEntriesOfType(entries, FleetAuditConfigAck)
	require.Len(t, acks, 1)
	assert.Equal(t, wantVersion, acks[0].Version)
	assert.Equal(t, "success", acks[0].Outcome)

	helds := auditEntriesOfType(entries, FleetAuditConfigHeld)
	require.Len(t, helds, 1)
	assert.Equal(t, heldVersion, helds[0].Version)
	assert.Equal(t, editedAt.UTC().Format(time.RFC3339Nano), helds[0].Metadata["edited-at"])

	rejects := auditEntriesOfType(entries, FleetAuditConfigReject)
	require.Len(t, rejects, 3)
	// Newest first: two holding rejects, then the ordinary verify reject.
	assert.Equal(t, fleetAuditOutcomeHolding, rejects[0].Outcome)
	assert.Equal(t, "config-holding", rejects[0].DiagnosticCode)
	assert.Equal(t, "client holds local configuration", rejects[0].Summary)

	assert.Equal(t, fleetAuditOutcomeHolding, rejects[1].Outcome)
	assert.Equal(t, "config-holding", rejects[1].DiagnosticCode)

	assert.Equal(t, fleetAuditOutcomeRejected, rejects[2].Outcome)
	assert.Equal(t, "config-reject-verify", rejects[2].DiagnosticCode)
	assert.Equal(t, fleet.RejectKindVerify, rejects[2].Metadata["reject-kind"])

	disables := auditEntriesOfType(entries, FleetAuditFleetDisable)
	require.Len(t, disables, 1, "idempotent receipt retry must record exactly one entry")
	assert.Equal(t, "device:edge-01", disables[0].Actor)
	assert.Equal(t, "admin-local", disables[0].Metadata["device-actor"])
}

// VALIDATES: AC-11, AC-17 -- PrepareManagedListenersWithHistoryAlarmsAndAudit
// rejects invalid audit max-entries, ApplyManagedListeners prunes retained rows
// when max-entries decreases, and ManagedRuntimeHealth().Audit exposes live
// audit health including corrupt startup keys.
func TestManagedServerAuditReloadRetentionAndDoctorHealth(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 7, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()

	// Seed a corrupt manifest so startup recovery marks Unreadable while
	// allowing subsequent healthy writes.
	require.NoError(t, store.WriteKey(zefs.KeyFleetAuditManifest.Pattern, []byte("{corrupt-json")))

	srv, _ := startAuditWiringTestServer(t, store, clk, plugin.HubAuditConfig{MaxEntries: 300})

	healthRec, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.NotNil(t, healthRec.Audit)
	assert.True(t, healthRec.Audit.Unreadable)
	assert.Equal(t, uint64(1), healthRec.Audit.UnreadableCount)
	assert.Equal(t, 300, healthRec.Audit.MaxEntries)

	// Append 150 entries.
	for range 150 {
		require.True(t, srv.RecordAudit(FleetAuditEntry{
			Type:   FleetAuditConfigPush,
			Device: testClientName,
			Actor:  fleetAuditActorHub,
		}))
	}
	require.NoError(t, srv.Audit().Flush(t.Context()))

	entries, err := ManagedAuditQuery(allowAllFleetScope(), FleetAuditQuery{})
	require.NoError(t, err)
	require.Len(t, entries, 150)

	listens := []ManagedListener{{
		Addr: "127.0.0.1:0",
		Clients: map[string]string{
			testClientName: testClientSecret,
		},
	}}

	// Invalid audit retention (< 100) is rejected by prepare without mutating live store.
	_, err = srv.PrepareManagedListenersWithHistoryAlarmsAndAudit(
		listens, "", plugin.DefaultHubHistoryConfig(), plugin.DefaultHubAlarmsConfig(),
		plugin.HubAuditConfig{MaxEntries: 50},
	)
	require.Error(t, err)

	// Valid reduction to 100 prunes oldest 50 entries on apply.
	plan, err := srv.PrepareManagedListenersWithHistoryAlarmsAndAudit(
		listens, "", plugin.DefaultHubHistoryConfig(), plugin.DefaultHubAlarmsConfig(),
		plugin.HubAuditConfig{MaxEntries: 100},
	)
	require.NoError(t, err)
	srv.ApplyManagedListeners(plan)

	pruned, err := ManagedAuditQuery(allowAllFleetScope(), FleetAuditQuery{})
	require.NoError(t, err)
	require.Len(t, pruned, 100)
	assert.Equal(t, uint64(150), pruned[0].Seq, "newest entry retained")
	assert.Equal(t, uint64(51), pruned[len(pruned)-1].Seq, "oldest 50 pruned")

	healthRec, running = ManagedRuntimeHealth()
	require.True(t, running)
	require.NotNil(t, healthRec.Audit)
	assert.Equal(t, 100, healthRec.Audit.MaxEntries)
	assert.Equal(t, 100, healthRec.Audit.RetainedEntries)
}

// VALIDATES: fleet-audit-receipts rejects oversized peer payloads before JSON
// decoding so an authenticated peer cannot force unbounded allocation.
// PREVENTS: pre-decode memory exhaustion via oversized receipt batches.
func TestManagedServerAuditReceiptsRejectsOversizedPayload(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 7, 30, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()
	srv, addr := startAuditWiringTestServer(t, store, clk, plugin.HubAuditConfig{MaxEntries: 200})

	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	waitForConnected(t, srv, testClientName, true)
	defer mc.Close() //nolint:errcheck // test cleanup

	oversized := map[string]string{
		"epoch":   "ep-oversize",
		"padding": strings.Repeat("a", fleet.FleetAuditReceiptBatchBytesMax+1024),
	}
	callCtx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	_, err := mc.CallRPC(callCtx, fleet.VerbFleetAuditReceipts, oversized)
	cancel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}
