// VALIDATES: fleet-3 Phase 2 -- device-side offline fleet mode-change audit
// receipt outbox, recoverable write intent (crash before and after mode write),
// bounded 1,024-entry overflow accounting and device doctor reporting, batched
// session drain with lost-ack replay deduplication, backward compatibility with
// hubs returning "unknown method", and divergence adopt/revert audit emission.
// PREVENTS: fabricated mode-change receipts on crashed/failed writes, premature
// outbox trimming on lost or mismatched acks, duplicate hub audit entries on
// retry, unbounded outbox growth, and silent audit loss on divergence actions.

package managed

import (
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
	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func readOutboxForTest(t *testing.T, store storage.Storage) (auditReceiptOutbox, bool) {
	t.Helper()
	raw, err := store.ReadKey(zefs.KeyManagedAuditReceipts.Pattern)
	if err != nil {
		return auditReceiptOutbox{}, false
	}
	outbox, decErr := decodeAuditReceiptOutbox(raw)
	require.NoError(t, decErr)
	return outbox, true
}

func startReceiptRPCBridge(
	t *testing.T,
	handler func(req *rpc.Request, mc *rpc.MuxConn),
) *rpc.MuxConn {
	t.Helper()
	clientEnd, hubEnd := net.Pipe()
	clientMC := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	hubMC := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for req := range hubMC.Requests() {
			handler(req, hubMC)
		}
	}()
	t.Cleanup(func() {
		_ = clientMC.Close()
		_ = hubMC.Close()
		_ = clientEnd.Close()
		_ = hubEnd.Close()
		<-done
	})
	return clientMC
}

// VALIDATES: recoverable write intent discards an incomplete transition on
// recovery (crash before flag write) without fabricating any receipt, and
// commits a completed transition (crash after flag write) into the outbox.
func TestAuditReceiptsRecoverableIntentCrashBeforeAndAfterFlagWrite(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 1, 0, 0, 0, time.UTC)

	t.Run("crash before flag write discards intent with no receipt", func(t *testing.T) {
		store := storage.CreateMemory()
		require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))

		intent := auditModeIntent{
			Action:        fleet.FleetAuditReceiptActionDisable,
			Actor:         "alice",
			Surface:       "ssh",
			EventAt:       t0,
			OldManaged:    true,
			TargetManaged: false,
		}
		raw, err := json.Marshal(intent)
		require.NoError(t, err)
		require.NoError(t, store.WriteKey(zefs.KeyManagedAuditIntent.Pattern, raw))

		require.NoError(t, recoverAuditReceiptIntent(store))

		_, err = store.ReadKey(zefs.KeyManagedAuditIntent.Pattern)
		require.Error(t, err, "intent key must be removed after recovery")
		_, exists := readOutboxForTest(t, store)
		require.False(t, exists, "no receipt may be fabricated when mode did not change")
	})

	t.Run("crash after flag write recovers receipt into outbox", func(t *testing.T) {
		store := storage.CreateMemory()
		require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("false\n")))

		intent := auditModeIntent{
			Action:        fleet.FleetAuditReceiptActionDisable,
			Actor:         "alice",
			Surface:       "ssh",
			EventAt:       t0,
			OldManaged:    true,
			TargetManaged: false,
		}
		raw, err := json.Marshal(intent)
		require.NoError(t, err)
		require.NoError(t, store.WriteKey(zefs.KeyManagedAuditIntent.Pattern, raw))

		require.NoError(t, recoverAuditReceiptIntent(store))

		_, err = store.ReadKey(zefs.KeyManagedAuditIntent.Pattern)
		require.Error(t, err, "intent key must be removed after recovery")
		outbox, exists := readOutboxForTest(t, store)
		require.True(t, exists)
		require.NotEmpty(t, outbox.Epoch)
		require.Equal(t, uint64(2), outbox.NextSeq)
		require.Len(t, outbox.Entries, 1)
		assert.Equal(t, uint64(1), outbox.Entries[0].Seq)
		assert.Equal(t, fleet.FleetAuditReceiptActionDisable, outbox.Entries[0].Action)
		assert.Equal(t, "alice", outbox.Entries[0].Actor)
		assert.Equal(t, "ssh", outbox.Entries[0].Surface)
		assert.Equal(t, t0, outbox.Entries[0].EventAt)
	})

	t.Run("failed flag write removes intent and writes no receipt", func(t *testing.T) {
		store := storage.CreateMemory()
		require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))

		boom := errors.New("simulated store error")
		err := executeManagedModeChange(store, false, "bob", "cli", t0, func() error {
			return boom
		})
		require.ErrorIs(t, err, boom)

		_, readErr := store.ReadKey(zefs.KeyManagedAuditIntent.Pattern)
		require.Error(t, readErr, "intent must be cleaned up on failed writeFlag")
		_, exists := readOutboxForTest(t, store)
		require.False(t, exists)
	})
}

// VALIDATES: AC-14 -- offline disable then enable records distinct receipts,
// lost acknowledgement triggers replay that deduplicates on the hub without
// premature device trimming, and successful ack trims the outbox cleanly.
func TestAuditReceiptsOfflineDisableEnableLostAckDedup(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 2, 0, 0, 0, time.UTC)
	deviceStore := storage.CreateMemory()
	require.NoError(t, deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))

	// Offline disable followed by offline enable.
	require.NoError(t, executeManagedModeChange(deviceStore, false, "op-alice", "cli", t0, func() error {
		return deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("false\n"))
	}))
	require.NoError(t, executeManagedModeChange(deviceStore, true, "op-bob", "ssh", t0.Add(time.Minute), func() error {
		return deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n"))
	}))

	// Repeated no-op enable must not emit a duplicate receipt.
	require.NoError(t, executeManagedModeChange(deviceStore, true, "op-bob", "ssh", t0.Add(2*time.Minute), func() error {
		return deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n"))
	}))

	outboxBefore, exists := readOutboxForTest(t, deviceStore)
	require.True(t, exists)
	require.Len(t, outboxBefore.Entries, 2)

	// Start a real hub ManagedAuditStore.
	hubStore := storage.CreateMemory()
	hubAudit, err := pluginserver.NewManagedAuditStore(pluginserver.ManagedAuditStoreConfig{
		Store:      hubStore,
		MaxEntries: 500,
		Now:        func() time.Time { return t0.Add(5 * time.Minute) },
	})
	require.NoError(t, err)
	hubAudit.Start(t.Context())
	t.Cleanup(hubAudit.Stop)

	// Session 1: hub commits the batch durably, then drops the reply (lost ack).
	var calls atomic.Int32
	mc1 := startReceiptRPCBridge(t, func(req *rpc.Request, hubMC *rpc.MuxConn) {
		require.Equal(t, fleet.VerbFleetAuditReceipts, req.Method)
		var batch fleet.AuditReceiptBatch
		require.NoError(t, json.Unmarshal(req.Params, &batch))
		_, ingestErr := hubAudit.IngestReceipts("edge-01", batch, t0.Add(5*time.Minute))
		require.NoError(t, ingestErr)
		calls.Add(1)
		_ = hubMC.SendError(t.Context(), req.ID, "simulated lost ack transport error")
	})

	drainErr := drainAuditReceipts(t.Context(), mc1, deviceStore, 2*time.Second)
	require.Error(t, drainErr)
	require.Equal(t, int32(1), calls.Load())

	// Outbox must NOT have trimmed entries because the ack was lost.
	outboxMid, exists := readOutboxForTest(t, deviceStore)
	require.True(t, exists)
	require.Len(t, outboxMid.Entries, 2)
	require.Equal(t, uint64(0), outboxMid.AckedSeq)

	// Session 2: replay same batch; hub deduplicates and returns ack.
	mc2 := startReceiptRPCBridge(t, func(req *rpc.Request, hubMC *rpc.MuxConn) {
		require.Equal(t, fleet.VerbFleetAuditReceipts, req.Method)
		var batch fleet.AuditReceiptBatch
		require.NoError(t, json.Unmarshal(req.Params, &batch))
		ack, ingestErr := hubAudit.IngestReceipts("edge-01", batch, t0.Add(6*time.Minute))
		require.NoError(t, ingestErr)
		require.NoError(t, hubMC.SendResult(t.Context(), req.ID, ack))
	})

	require.NoError(t, drainAuditReceipts(t.Context(), mc2, deviceStore, 2*time.Second))

	// Outbox is now trimmed and AckedSeq advanced to 2.
	outboxAfter, exists := readOutboxForTest(t, deviceStore)
	require.True(t, exists)
	require.Empty(t, outboxAfter.Entries)
	require.Equal(t, uint64(2), outboxAfter.AckedSeq)

	// Hub recorded exactly two logical audit rows (zero duplicates across replay).
	rows := hubAudit.Query(pluginserver.ResolveFleetDeviceScope(&pluginserver.CommandContext{}), pluginserver.FleetAuditQuery{})
	require.Len(t, rows, 2)
	// Newest first: enable (seq 2), then disable (seq 1).
	assert.Equal(t, pluginserver.FleetAuditFleetEnable, rows[0].Type)
	assert.Equal(t, "edge-01", rows[0].Device)
	assert.Equal(t, "device:edge-01", rows[0].Actor)
	assert.Equal(t, "op-bob", rows[0].Metadata["device-actor"])
	assert.Equal(t, "ssh", rows[0].Metadata["device-surface"])
	assert.Equal(t, t0.Add(time.Minute), rows[0].EventTime)

	assert.Equal(t, pluginserver.FleetAuditFleetDisable, rows[1].Type)
	assert.Equal(t, "edge-01", rows[1].Device)
	assert.Equal(t, "device:edge-01", rows[1].Actor)
	assert.Equal(t, "op-alice", rows[1].Metadata["device-actor"])
	assert.Equal(t, "cli", rows[1].Metadata["device-surface"])
	assert.Equal(t, t0, rows[1].EventTime)
}

// VALIDATES: outbox overflow beyond 1,024 entries tracks Lost/GapFrom/GapTo,
// surfaces CodeDoctorManagedAuditReceiptGap in device doctor, and drains the
// gap plus all 1,024 retained entries in bounded <=32-entry batches to the hub.
func TestAuditReceiptsOverflowGapDoctorAndHubReport(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 3, 0, 0, 0, time.UTC)
	deviceStore := storage.CreateMemory()
	require.NoError(t, deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))

	// Record 1,030 transitions (6 overflow evictions: seq 1..6 lost, 7..1030 retained).
	const totalTransitions = fleet.FleetAuditReceiptsOutboxMax + 6
	target := false
	for i := range totalTransitions {
		ts := t0.Add(time.Duration(i) * time.Second)
		nextVal := "false\n"
		if target {
			nextVal = "true\n"
		}
		require.NoError(t, executeManagedModeChange(deviceStore, target, "alice", "cli", ts, func() error {
			return deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(nextVal))
		}))
		target = !target
	}

	lost, gapFrom, gapTo, lastOverflowAt, unreadableErr := auditReceiptOutboxHealth(deviceStore)
	require.NoError(t, unreadableErr)
	assert.Equal(t, uint64(6), lost)
	assert.Equal(t, uint64(1), gapFrom)
	assert.Equal(t, uint64(6), gapTo)
	assert.False(t, lastOverflowAt.IsZero())

	// Device doctor check reports the overflow gap.
	diags := checkManagedAuditReceipts(diagnostic.DoctorCheckContext{Store: deviceStore})
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorManagedAuditReceiptGap, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "6")
	assert.Contains(t, diags[0].Message, "1..6")

	// Drain to hub ManagedAuditStore; verify batch size cap and gap reporting.
	hubAudit, err := pluginserver.NewManagedAuditStore(pluginserver.ManagedAuditStoreConfig{
		Store:      storage.CreateMemory(),
		MaxEntries: 2000,
		Now:        func() time.Time { return t0.Add(time.Hour) },
	})
	require.NoError(t, err)
	hubAudit.Start(t.Context())
	t.Cleanup(hubAudit.Stop)

	var batchCount atomic.Int32
	mc := startReceiptRPCBridge(t, func(req *rpc.Request, hubMC *rpc.MuxConn) {
		var batch fleet.AuditReceiptBatch
		require.NoError(t, json.Unmarshal(req.Params, &batch))
		require.LessOrEqual(t, len(batch.Entries), fleet.FleetAuditReceiptsBatchMax)
		batchCount.Add(1)
		ack, ingestErr := hubAudit.IngestReceipts("edge-overflow", batch, t0.Add(time.Hour))
		require.NoError(t, ingestErr)
		require.NoError(t, hubMC.SendResult(t.Context(), req.ID, ack))
	})

	require.NoError(t, drainAuditReceipts(t.Context(), mc, deviceStore, 5*time.Second))
	assert.Equal(t, int32(fleet.FleetAuditReceiptsOutboxMax/fleet.FleetAuditReceiptsBatchMax), batchCount.Load())

	outboxAfter, exists := readOutboxForTest(t, deviceStore)
	require.True(t, exists)
	assert.Empty(t, outboxAfter.Entries)
	assert.Equal(t, uint64(totalTransitions), outboxAfter.AckedSeq)
	assert.Equal(t, uint64(6), outboxAfter.Lost, "historical overflow evidence stays visible to device doctor")

	health := hubAudit.Health()
	assert.Equal(t, uint64(1), health.ReceiptGaps)
	assert.Equal(t, "edge-overflow", health.LastReceiptGapDevice)
	assert.Equal(t, fleet.FleetAuditReceiptsOutboxMax+1, health.RetainedEntries, "1 gap entry + 1024 retained receipts")
}

// VALIDATES: an older hub answering fleet-audit-receipts with "unknown method"
// returns nil from DrainAuditReceipts and preserves the local outbox so config
// synchronization continues unaffected.
func TestAuditReceiptsUnknownMethodBackwardCompatible(t *testing.T) {
	t0 := time.Date(2026, 11, 28, 4, 0, 0, 0, time.UTC)
	deviceStore := storage.CreateMemory()
	require.NoError(t, deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("true\n")))

	require.NoError(t, executeManagedModeChange(deviceStore, false, "alice", "cli", t0, func() error {
		return deviceStore.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte("false\n"))
	}))

	mc := startReceiptRPCBridge(t, func(req *rpc.Request, hubMC *rpc.MuxConn) {
		_ = hubMC.SendError(t.Context(), req.ID, "unknown method: "+req.Method)
	})

	require.NoError(t, drainAuditReceipts(t.Context(), mc, deviceStore, 2*time.Second))

	outbox, exists := readOutboxForTest(t, deviceStore)
	require.True(t, exists)
	require.Len(t, outbox.Entries, 1, "outbox must stay intact when older hub does not support the verb")
	assert.Equal(t, uint64(0), outbox.AckedSeq)
}

// VALIDATES: decodeAuditReceiptAck refuses oversized ack payloads before JSON
// decoding so a misbehaving peer cannot trigger unbounded allocation.
// PREVENTS: pre-decode memory exhaustion on oversized receipt acknowledgements.
func TestAuditReceiptAckRejectsOversizedPayload(t *testing.T) {
	oversized := json.RawMessage(`{"epoch":"ep","committed":1,"pad":"` + strings.Repeat("x", fleet.FleetAuditReceiptAckBytesMax) + `"}`)
	_, err := decodeAuditReceiptAck(oversized)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

// VALIDATES: AC-15 -- handleDivergenceAdopt and handleDivergenceRevert emit
// structured hub audit rows with authenticated actor, surface, device, winner,
// and chosen version, while refused adopt/revert emits zero audit rows.
func TestDivergenceAdoptAndRevertEmitAudit(t *testing.T) {
	store := storage.CreateMemory()
	hubConfig := []byte("system { hostname edge-hub; }\n")
	heldConfig := []byte("system { hostname edge-local; }\n")
	hubHash := fleet.VersionHash(hubConfig)
	heldHash := fleet.VersionHash(heldConfig)

	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(divergenceClient), hubConfig, 0))

	writeHeld := func() {
		raw, err := json.Marshal(struct {
			Version    string    `json:"version"`
			Config     []byte    `json:"config"`
			ReceivedAt time.Time `json:"received-at"`
		}{
			Version:    heldHash,
			Config:     heldConfig,
			ReceivedAt: time.Date(2026, 11, 28, 5, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		require.NoError(t, store.WriteKey(pluginserver.HeldRecordKey(divergenceClient), raw))
	}

	var mu sync.Mutex
	var recorded []pluginserver.FleetAuditEntry
	InstallDivergenceTarget(&DivergenceTarget{
		Store:               store,
		ServesClient:        servesDivergenceClient,
		NotifyConfigChanged: func(string) {},
		RecordAudit: func(e pluginserver.FleetAuditEntry) bool {
			mu.Lock()
			recorded = append(recorded, e)
			mu.Unlock()
			return true
		},
	})
	t.Cleanup(func() { InstallDivergenceTarget(nil) })

	cmdCtx := &pluginserver.CommandContext{Username: "netops", Surface: "ssh"}

	// Refused revert (wrong version) must emit NO audit entry.
	writeHeld()
	resp, err := handleDivergenceRevert(cmdCtx, revertArgsOf(t, divergenceClient, "sha256:0000", hubHash))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Empty(t, recorded, "refused revert must not emit an audit entry")

	// Accepted revert emits divergence.revert with winner=hub.
	resp, err = handleDivergenceRevert(cmdCtx, revertArgsOf(t, divergenceClient, heldHash, hubHash))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	require.Len(t, recorded, 1)
	assert.Equal(t, pluginserver.FleetAuditDivergenceRevert, recorded[0].Type)
	assert.Equal(t, divergenceClient, recorded[0].Device)
	assert.Equal(t, "netops", recorded[0].Actor)
	assert.Equal(t, "ssh", recorded[0].Surface)
	assert.Equal(t, "success", recorded[0].Outcome)
	assert.Equal(t, hubHash, recorded[0].Version)
	assert.Equal(t, heldHash, recorded[0].PreviousVersion)
	assert.Equal(t, "hub", recorded[0].Metadata["winner"])

	// Refused adopt (wrong version) must emit NO audit entry.
	resp, err = handleDivergenceAdopt(cmdCtx, adoptArgsOf(t, divergenceClient, "sha256:bad"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Len(t, recorded, 1, "refused adopt must not emit an audit entry")

	// Accepted adopt emits divergence.adopt with winner=device and stamp.
	resp, err = handleDivergenceAdopt(cmdCtx, adoptArgsOf(t, divergenceClient, heldHash))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	require.Len(t, recorded, 2)
	assert.Equal(t, pluginserver.FleetAuditDivergenceAdopt, recorded[1].Type)
	assert.Equal(t, divergenceClient, recorded[1].Device)
	assert.Equal(t, "netops", recorded[1].Actor)
	assert.Equal(t, "ssh", recorded[1].Surface)
	assert.Equal(t, "success", recorded[1].Outcome)
	assert.Equal(t, heldHash, recorded[1].Version)
	assert.Equal(t, "device", recorded[1].Metadata["winner"])
	assert.NotEmpty(t, recorded[1].Metadata["stamp"])
}
