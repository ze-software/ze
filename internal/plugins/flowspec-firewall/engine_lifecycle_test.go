package flowspecfirewall

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Records successful backend states, including a transient loss of the rule
// while the server compensates a refused removal with Configure.
type lifecycleBackend struct {
	mu sync.Mutex
	lowerOnlyCanonicalBackend
	failNext  error
	guardDrop bool
	lostDrop  bool
}

func (b *lifecycleBackend) Apply(desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failNext != nil {
		err := b.failNext
		b.failNext = nil
		return err
	}
	if err := b.lowerOnlyCanonicalBackend.Apply(desired); err != nil {
		return err
	}
	if b.guardDrop && !selectedKernelAction[firewall.Drop](b.flushed) {
		b.lostDrop = true
	}
	return nil
}

func (b *lifecycleBackend) snapshot() []firewall.Table {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]firewall.Table(nil), b.flushed...)
}

func flowSpecEngineRequest(t *testing.T, ctx context.Context, mux *rpc.MuxConn, method string) {
	t.Helper()
	select {
	case req := <-mux.Requests():
		require.NotNil(t, req)
		require.Equal(t, method, req.Method)
		require.NoError(t, mux.SendOK(ctx, req.ID))
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", method, ctx.Err())
	}
}

func flowSpecEngineCallback(t *testing.T, ctx context.Context, mux *rpc.MuxConn, method string, input any) {
	t.Helper()
	raw, err := mux.CallRPC(ctx, method, input)
	require.NoError(t, err)
	if len(raw) != 0 {
		var result rpc.ConfigApplyOutput
		require.NoError(t, json.Unmarshal(raw, &result))
		require.NotEqual(t, rpc.StatusError, result.Status, result.Error)
	}
}

// Drives the production SDK callbacks rather than calling removeRules alone.
// Configure after a refused Bye must preserve the live rule throughout and
// must not install another selected-route subscription.
func TestFlowSpecFailedByeConfigureKeepsLiveSelection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	backend := &lifecycleBackend{}
	const backendName = "flowspec-engine-lifecycle"
	require.NoError(t, firewall.RegisterBackend(backendName, func() (firewall.Backend, error) { return backend, nil }))
	require.NoError(t, firewall.LoadBackend(backendName))
	t.Cleanup(func() {
		_ = firewall.RegisterTables("flowspec", nil)
		_ = firewall.CloseBackend()
	})
	bus, err := server.NewServer(nil, nil)
	require.NoError(t, err)
	previousBus := getEventBusRef()
	setEventBusRef(bus)
	t.Cleanup(func() { setEventBusRef(previousBus) })
	reg := newReasonRegistry()
	previousMetrics := bridgeMetricsPtr.Load()
	bindMetrics(reg)
	t.Cleanup(func() { bridgeMetricsPtr.Store(previousMetrics) })
	change := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
	unsubscribeReplay := ribevents.ReplayRequest.Subscribe(bus, func(_ *replay.Request) {
		if _, err := ribevents.FlowSpecChanged.Emit(bus, change); err != nil {
			t.Errorf("selected replay: %v", err)
		}
	})
	t.Cleanup(unsubscribeReplay)

	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan struct{})
	var exitCode int
	go func() {
		exitCode = runEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("FlowSpec engine did not exit after its transport closed")
		}
	})
	flowSpecEngineRequest(t, ctx, mux, "ze-plugin-engine:declare-registration")
	flowSpecEngineCallback(t, ctx, mux, "ze-plugin-callback:configure", rpc.ConfigureInput{})
	flowSpecEngineRequest(t, ctx, mux, "ze-plugin-engine:declare-capabilities")
	flowSpecEngineCallback(t, ctx, mux, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	flowSpecEngineRequest(t, ctx, mux, "ze-plugin-engine:ready")
	require.True(t, selectedKernelAction[firewall.Drop](backend.snapshot()))

	backend.mu.Lock()
	backend.failNext = errors.New("withdrawal rejected")
	backend.mu.Unlock()
	_, err = mux.CallRPC(ctx, "ze-plugin-callback:bye", map[string]string{"reason": "removed"})
	require.Error(t, err)
	require.True(t, selectedKernelAction[firewall.Drop](backend.snapshot()))
	backend.mu.Lock()
	backend.guardDrop = true
	backend.mu.Unlock()
	flowSpecEngineCallback(t, ctx, mux, "ze-plugin-callback:configure", rpc.ConfigureInput{})
	backend.mu.Lock()
	lostDrop := backend.lostDrop
	backend.guardDrop = false
	backend.mu.Unlock()
	require.False(t, lostDrop, "compensating Configure must never clear the retained live rule")

	change.ExtendedCommunities = []byte{0x80, 9, 0, 0, 0, 0, 0, 46}
	_, err = ribevents.FlowSpecChanged.Emit(bus, change)
	require.NoError(t, err)
	require.True(t, selectedKernelAction[firewall.SetDSCP](backend.snapshot()))
	refused := selectedFixture(t, "10.2.0.0/24", []byte{0x80, 8, 0, 1, 0, 0, 0, 1})
	_, err = ribevents.FlowSpecChanged.Emit(bus, refused)
	require.NoError(t, err)
	require.Equal(t, 1, reg.count(refusedReasonUnsupportedAct), "one selected refusal must not be delivered twice after compensation")

	flowSpecEngineCallback(t, ctx, mux, "ze-plugin-callback:bye", map[string]string{"reason": "removed"})
	select {
	case <-done:
		require.Zero(t, exitCode)
	case <-ctx.Done():
		t.Fatal("successful removal did not stop the engine")
	}
	require.False(t, selectedTablePresent(backend.snapshot()))
	_, err = ribevents.FlowSpecChanged.Emit(bus, change)
	require.NoError(t, err)
	require.False(t, selectedTablePresent(backend.snapshot()), "delivery must remain stopped after successful removal")
}
