// Design: docs/architecture/config/transaction-protocol.md -- rollback completion and non-blocking decompose planning
// Related: reload_tx.go -- runTxCoordinator, completeRolledBack, operationPlannerFromTrees

package server

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReloadRolledBackCompletesWhenCapabilityRestoreFails proves that a failed
// capability restoration after an apply rollback does not short-circuit
// completeRolledBack: completion owners still receive config-complete(false),
// the restore failure is recorded for doctor, and both errors are returned.
//
// VALIDATES: runTxCoordinator runs completeRolledBack and joins all errors even
// when bridge.restoreCapabilities fails.
// PREVENTS: a capability restore failure stranding a completion owner (such as
// a paused BGP session) waiting forever for config-complete(false).
func TestReloadRolledBackCompletesWhenCapabilityRestoreFails(t *testing.T) {
	t.Parallel()

	trace := &orderRecorder{}
	plugins := []pluginDef{
		{
			name:             "caps",
			roots:            []string{"bgp"},
			configCompletion: true,
			trace:            trace,
			verifyResp:       &rpc.ConfigVerifyOutput{Status: rpc.StatusOK, Capabilities: &rpc.DeclareCapabilitiesInput{}},
		},
		{
			name:      "failing",
			roots:     []string{"bgp"},
			applyResp: &rpc.ConfigApplyOutput{Status: rpc.StatusError, Error: "journal full"},
		},
	}
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{
		"bgp": map[string]any{"router-id": "1.2.3.4"},
	}}, plugins)
	s.capInjector = plugin.NewCapabilityInjector()
	seedCapabilities(t, s, "caps", roleDeclaration("00"))

	intruder := capabilitiesFromRPC(roleDeclaration("07"))
	intruder.PluginName = "intruder"
	added := make(chan error, 1)
	failing := plugins[1].responder
	failing.mu.Lock()
	failing.beforeApplyRsp = func() {
		addErr := s.capInjector.AddPluginCapabilities(intruder)
		select {
		case added <- addErr:
		default:
		}
	}
	failing.mu.Unlock()

	err := s.ReloadConfig(context.Background(), map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}})
	require.Error(t, err)
	require.ErrorContains(t, err, "journal full")
	require.ErrorContains(t, err, "restore previous capabilities")

	select {
	case addErr := <-added:
		require.NoError(t, addErr, "intruder failed to claim the released capability")
	default:
		t.Fatal("failing apply never ran, so nothing was installed to restore")
	}

	failures := s.capabilityRestore.read()
	require.Len(t, failures, 1, "failed capability restore must be recorded")
	require.Contains(t, failures[0].Err, "restore previous capabilities")

	want := "complete:caps:false"
	require.Eventually(t, func() bool { return slices.Contains(trace.snapshot(), want) }, 5*time.Second, 10*time.Millisecond,
		"completeRolledBack did not run when capability restore failed; trace %v", trace.snapshot())
	require.Equal(t, 1, countOf(trace.snapshot(), want), "completion sent more than once: %v", trace.snapshot())
}

// decomposeTestGateway is a synchronous fake transaction.EventGateway used to
// exercise operationPlannerFromTrees with multi-participant second-pass acks
// and late/extra acks after planner exit.
type decomposeTestGateway struct {
	mu       sync.Mutex
	handlers map[string][]func([]byte)
	retained map[string][]func([]byte)
}

func newDecomposeTestGateway() *decomposeTestGateway {
	return &decomposeTestGateway{
		handlers: make(map[string][]func([]byte)),
		retained: make(map[string][]func([]byte)),
	}
}

func (g *decomposeTestGateway) EmitConfigEvent(eventType string, payload []byte) (int, error) {
	g.mu.Lock()
	hs := append([]func([]byte){}, g.handlers[eventType]...)
	g.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return len(hs), nil
}

func (g *decomposeTestGateway) SubscribeConfigEvent(eventType string, handler func([]byte)) func() {
	if handler == nil {
		return func() {}
	}
	g.mu.Lock()
	g.handlers[eventType] = append(g.handlers[eventType], handler)
	g.retained[eventType] = append(g.retained[eventType], handler)
	idx := len(g.handlers[eventType]) - 1
	g.mu.Unlock()
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		hs := g.handlers[eventType]
		if idx < len(hs) {
			g.handlers[eventType] = append(hs[:idx], hs[idx+1:]...)
		}
	}
}

func (g *decomposeTestGateway) SubscribeEvent(string, string, func(any)) func() {
	return func() {}
}

// invokeRetained simulates an in-flight event callback firing after the planner
// has returned and unsubscribed.
func (g *decomposeTestGateway) invokeRetained(eventType string, payload []byte) {
	g.mu.Lock()
	hs := append([]func([]byte){}, g.retained[eventType]...)
	g.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
}

// TestOperationPlannerNonBlockingExtraAndSecondPassAcks proves that
// operationPlannerFromTrees's event subscribers never block the emitter
// goroutine:
//  1. A single-root diff that disturbs an address triggers a second pass over
//     multiple decomposing participants (producing more acks than len(diffs)),
//     with extra duplicate acks emitted synchronously alongside real ones.
//  2. Late acks delivered to retained subscriber callbacks after the planner
//     exits early (on context cancel) return immediately instead of blocking.
//
// VALIDATES: EventOperationDecomposeOK and EventOperationDecomposeFailed
// callbacks cannot deadlock the synchronous event dispatcher.
// PREVENTS: a second-pass decomposition or late/duplicate ack wedging the
// event emitter goroutine.
func TestOperationPlannerNonBlockingExtraAndSecondPassAcks(t *testing.T) {
	t.Parallel()

	const (
		rootAddr  = "test-disturb-addr-root"
		rootBindA = "test-disturb-binder-a"
		rootBindB = "test-disturb-binder-b"
		opRemove  = transaction.ConfigOperationType("test-remove-addr")
		opRebindA = transaction.ConfigOperationType("test-rebind-a")
		opRebindB = transaction.ConfigOperationType("test-rebind-b")
	)

	gw := newDecomposeTestGateway()
	participants := []transaction.Participant{
		{
			Name:        "addr-owner",
			ConfigRoots: []string{rootAddr},
			ConfigOperations: []transaction.ConfigOperationDecl{
				{Root: rootAddr, Decompose: true, Operations: []transaction.ConfigOperationType{opRemove}},
			},
		},
		{
			Name:        "binder-a",
			ConfigRoots: []string{rootBindA},
			ConfigOperations: []transaction.ConfigOperationDecl{
				{Root: rootBindA, Decompose: true, Operations: []transaction.ConfigOperationType{opRebindA}},
			},
		},
		{
			Name:        "binder-b",
			ConfigRoots: []string{rootBindB},
			ConfigOperations: []transaction.ConfigOperationDecl{
				{Root: rootBindB, Decompose: true, Operations: []transaction.ConfigOperationType{opRebindB}},
			},
		},
	}

	removeOp := transaction.ConfigOperation{
		ID:       "rm-addr-1",
		Root:     rootAddr,
		Owner:    "addr-owner",
		Type:     opRemove,
		Verb:     transaction.VerbDestroy,
		Target:   transaction.ResourceRef{Kind: transaction.ResourceAddress, Address: "192.0.2.1/32"},
		Produces: []transaction.ResourceRef{{Kind: transaction.ResourceAddress, Address: "192.0.2.1/32"}},
	}
	rebindAOp := transaction.ConfigOperation{
		ID:     "rebind-a-1",
		Root:   rootBindA,
		Owner:  "binder-a",
		Type:   opRebindA,
		Verb:   transaction.VerbModify,
		Target: transaction.ResourceRef{Kind: transaction.ResourcePeer, Peer: "peer-a"},
	}
	rebindBOp := transaction.ConfigOperation{
		ID:     "rebind-b-1",
		Root:   rootBindB,
		Owner:  "binder-b",
		Type:   opRebindB,
		Verb:   transaction.VerbModify,
		Target: transaction.ResourceRef{Kind: transaction.ResourcePeer, Peer: "peer-b"},
	}

	emitAck := func(topic string, ack transaction.ConfigOperationDecomposeAck) {
		raw, err := json.Marshal(ack)
		require.NoError(t, err)
		_, err = gw.EmitConfigEvent(topic, raw)
		require.NoError(t, err)
	}

	gw.SubscribeConfigEvent(transaction.EventOperationDecomposeFor("addr-owner"), func(payload []byte) {
		var ev transaction.ConfigOperationDecomposeEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack := transaction.ConfigOperationDecomposeAck{
			TransactionID: ev.TransactionID,
			Plugin:        "addr-owner",
			Root:          ev.Root,
			Status:        transaction.CodeOK,
			Operations:    []transaction.ConfigOperation{removeOp},
		}
		// Emit the matching ack plus extra duplicate acks synchronously.
		emitAck(transaction.EventOperationDecomposeOK, ack)
		emitAck(transaction.EventOperationDecomposeOK, ack)
		emitAck(transaction.EventOperationDecomposeOK, ack)
	})
	gw.SubscribeConfigEvent(transaction.EventOperationDecomposeFor("binder-a"), func(payload []byte) {
		var ev transaction.ConfigOperationDecomposeEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		require.Equal(t, []string{"192.0.2.1"}, ev.DisturbedAddresses)
		ack := transaction.ConfigOperationDecomposeAck{
			TransactionID: ev.TransactionID,
			Plugin:        "binder-a",
			Root:          ev.Root,
			Status:        transaction.CodeOK,
			Operations:    []transaction.ConfigOperation{rebindAOp},
		}
		emitAck(transaction.EventOperationDecomposeOK, ack)
		emitAck(transaction.EventOperationDecomposeOK, ack)
	})
	gw.SubscribeConfigEvent(transaction.EventOperationDecomposeFor("binder-b"), func(payload []byte) {
		var ev transaction.ConfigOperationDecomposeEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		require.Equal(t, []string{"192.0.2.1"}, ev.DisturbedAddresses)
		ack := transaction.ConfigOperationDecomposeAck{
			TransactionID: ev.TransactionID,
			Plugin:        "binder-b",
			Root:          ev.Root,
			Status:        transaction.CodeOK,
			Operations:    []transaction.ConfigOperation{rebindBOp},
		}
		emitAck(transaction.EventOperationDecomposeOK, ack)
		emitAck(transaction.EventOperationDecomposeOK, ack)
	})

	planner := operationPlannerFromTrees(gw, map[string]any{}, map[string]any{}, participants)
	ops, err := planner(t.Context(), transaction.OperationPlanRequest{
		TransactionID: "tx-second-pass",
		Diffs: map[string][]transaction.DiffSection{
			rootAddr: {{Root: rootAddr, Removed: `{"address":"192.0.2.1/32"}`}},
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 3)

	// Now verify early planner exit + burst of late acks never blocks emitter.
	lateGW := newDecomposeTestGateway()
	latePlanner := operationPlannerFromTrees(lateGW, map[string]any{}, map[string]any{}, participants[:1])
	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = latePlanner(canceledCtx, transaction.OperationPlanRequest{
		TransactionID: "tx-late-acks",
		Diffs: map[string][]transaction.DiffSection{
			rootAddr: {{Root: rootAddr, Changed: `{"address":"changed"}`}},
		},
	})
	require.ErrorIs(t, err, context.Canceled)

	okRaw, err := json.Marshal(transaction.ConfigOperationDecomposeAck{
		TransactionID: "tx-late-acks", Plugin: "addr-owner", Root: rootAddr, Status: transaction.CodeOK,
	})
	require.NoError(t, err)
	failRaw, err := json.Marshal(transaction.ConfigOperationDecomposeAck{
		TransactionID: "tx-late-acks", Plugin: "addr-owner", Root: rootAddr, Status: transaction.CodeError, Error: "late",
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 32 {
			lateGW.invokeRetained(transaction.EventOperationDecomposeOK, okRaw)
			lateGW.invokeRetained(transaction.EventOperationDecomposeFailed, failRaw)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("late decompose ack callbacks blocked after planner exit")
	}
}
