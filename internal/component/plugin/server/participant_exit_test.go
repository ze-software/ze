// Design: docs/architecture/api/process-protocol.md -- bus:participant-exit lifecycle emission on process exit.
// Related: dispatch.go -- cleanupProcess emits bus:participant-exit unless on-exit keep is configured.

package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: cleanupProcess emits bus:participant-exit carrying endpoint name
// and /bus/endpoint/<name> path to registered subscribers on default withdraw
// policy, and suppresses it when OnExit is plugin.OnExitKeep.
// PREVENTS: exited participants leaking per-owner state in downstream plugins
// or firing exit cleanup when on-exit keep is configured.
func TestCleanupProcessEmitsParticipantExitUnlessKeep(t *testing.T) {
	s := newSubscriptionServer()
	s.dispatcher = NewDispatcher()

	// Subscribe an engine handler via the typed handle.
	engineCh := make(chan events.ParticipantExitEvent, 4)
	unsub := events.BusParticipantExit.Subscribe(s, func(ev events.ParticipantExitEvent) {
		engineCh <- ev
	})
	defer unsub()

	// Subscribe a socket-path plugin probe via Stage 1 Receives + onReady.
	receiver, batches := newDeliveryProbe(t, "exit-watcher-socket")
	receiver.SetRegistration(&plugin.PluginRegistration{
		Name: "exit-watcher-socket",
		Receives: []rpc.MessageReceiveDecl{
			{Message: events.MessageParticipantExit},
		},
	})
	sink := &engineStartupSink{s: s, proc: receiver}
	require.NoError(t, sink.onReady(&rpc.ReadyInput{}))

	// Subscribe a DirectBridge plugin probe that also registers a structured
	// handler (proving deliverMixedBatch routes text/JSON lifecycle events to
	// DeliverEvents alongside structured handlers).
	bridgeBatches := make(chan []string, 8)
	bridgeCtx, bridgeCancel := context.WithCancel(t.Context())
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	const bridgeName = "exit-watcher-bridge"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        bridgeName,
		Description: "participant-exit bridge probe",
		RunEngine:   func(net.Conn) int { <-bridgeCtx.Done(); return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))
	bridgeProc := process.NewProcess(plugin.PluginConfig{
		Name:     bridgeName,
		Internal: true,
	})
	require.NoError(t, bridgeProc.StartWithContext(bridgeCtx))
	t.Cleanup(func() {
		bridgeCancel()
		bridgeProc.Stop()
	})
	require.NoError(t, bridgeProc.InitConns())
	bridgeProc.Conn().SetBridge(bridgeProc.Bridge())
	bridgeProc.Bridge().SetDeliverStructured(func([]any) error { return nil })
	bridgeProc.Bridge().SetDeliverEvents(func(evts []string) error {
		cp := make([]string, len(evts))
		copy(cp, evts)
		bridgeBatches <- cp
		return nil
	})
	bridgeProc.Bridge().SetReady()
	bridgeProc.StartDelivery(bridgeCtx)
	bridgeProc.SetRegistration(&plugin.PluginRegistration{
		Name: "exit-watcher-bridge",
		Receives: []rpc.MessageReceiveDecl{
			{Message: events.MessageParticipantExit},
		},
	})
	bridgeSink := &engineStartupSink{s: s, proc: bridgeProc}
	require.NoError(t, bridgeSink.onReady(&rpc.ReadyInput{}))

	// Case 1: default OnExit (withdraw) emits bus:participant-exit.
	exiting := process.NewProcess(plugin.PluginConfig{
		Name: "script-a",
	})
	s.cleanupProcess(exiting)

	batch := waitBatch(t, batches)
	require.Len(t, batch, 1)
	var gotPlugin events.ParticipantExitEvent
	require.NoError(t, json.Unmarshal([]byte(batch[0]), &gotPlugin))
	assert.Equal(t, events.MessageParticipantExit, gotPlugin.Type)
	assert.Equal(t, "script-a", gotPlugin.Endpoint)
	assert.Equal(t, "/bus/endpoint/script-a", gotPlugin.Participant)

	bridgeBatch := waitBatch(t, bridgeBatches)
	require.Len(t, bridgeBatch, 1)
	var gotBridge events.ParticipantExitEvent
	require.NoError(t, json.Unmarshal([]byte(bridgeBatch[0]), &gotBridge))
	assert.Equal(t, gotPlugin, gotBridge)

	select {
	case gotEngine := <-engineCh:
		assert.Equal(t, gotPlugin, gotEngine)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for engine bus:participant-exit event")
	}

	// Case 2: explicit OnExitWithdraw also emits bus:participant-exit.
	exitingWithdraw := process.NewProcess(plugin.PluginConfig{
		Name:   "script-b",
		OnExit: plugin.OnExitWithdraw,
	})
	s.cleanupProcess(exitingWithdraw)

	batch = waitBatch(t, batches)
	require.Len(t, batch, 1)
	var gotWithdraw events.ParticipantExitEvent
	require.NoError(t, json.Unmarshal([]byte(batch[0]), &gotWithdraw))
	assert.Equal(t, "script-b", gotWithdraw.Endpoint)
	assert.Equal(t, "/bus/endpoint/script-b", gotWithdraw.Participant)

	bridgeBatch = waitBatch(t, bridgeBatches)
	require.Len(t, bridgeBatch, 1)
	var gotBridgeWithdraw events.ParticipantExitEvent
	require.NoError(t, json.Unmarshal([]byte(bridgeBatch[0]), &gotBridgeWithdraw))
	assert.Equal(t, gotWithdraw, gotBridgeWithdraw)

	select {
	case gotEngine := <-engineCh:
		assert.Equal(t, gotWithdraw, gotEngine)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for engine bus:participant-exit event for script-b")
	}

	// Case 3: OnExitKeep suppresses bus:participant-exit completely.
	kept := process.NewProcess(plugin.PluginConfig{
		Name:   "script-keep",
		OnExit: plugin.OnExitKeep,
	})
	s.cleanupProcess(kept)

	select {
	case unexpected := <-engineCh:
		t.Fatalf("unexpected engine bus:participant-exit for on-exit keep: %+v", unexpected)
	default:
	}
	select {
	case unexpected := <-bridgeBatches:
		t.Fatalf("unexpected DirectBridge bus:participant-exit for on-exit keep: %v", unexpected)
	case unexpected := <-batches:
		t.Fatalf("unexpected socket bus:participant-exit for on-exit keep: %v", unexpected)
	case <-time.After(50 * time.Millisecond):
	}
}
