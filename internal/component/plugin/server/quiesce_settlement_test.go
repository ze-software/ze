// Design: docs/architecture/api/commands.md -- the registered plugin quiescer.
// Related: quiesce.go -- runtime manager discovery and downstream settlement.
package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPluginEventQuiescerWaitsForDeclaredSettlement uses the actual registered
// server quiescer and manager, installed after registration. Its receipt cannot
// precede the declared observer's deferred work, and pure FIFO application stays
// usable while that callback waits.
func TestPluginEventQuiescerWaitsForDeclaredSettlement(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s := &Server{}
	registerPluginEventQuiescer(s)
	manager := process.NewProcessManager(nil)
	s.procManager.Store(manager)
	bridge := rpc.NewDirectBridge()
	bridge.SetReady()
	engineEnd, pluginEnd := net.Pipe()
	connection := ipc.NewPluginConn(engineEnd, engineEnd)
	connection.SetBridge(bridge)
	proc := process.NewProcess(plugin.PluginConfig{Name: "deferred-observer"})
	proc.SetConn(connection)
	proc.SetRegistration(&plugin.PluginRegistration{EventSettlement: true})
	proc.StartDelivery(ctx)
	proc.SetRunning(true)
	require.NoError(t, manager.AddProcess(proc.Name(), proc))
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case request := <-bridge.CallbackCh():
			if request.Method != rpc.MethodEventSettlement {
				request.Result <- rpc.BridgeCallbackResult{Err: context.Canceled}
				return
			}
			var input rpc.EventSettlementInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				request.Result <- rpc.BridgeCallbackResult{Err: err}
				return
			}
			close(entered)
			select {
			case <-release:
				body, err := json.Marshal(rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS, Status: rpc.StatusOK})
				request.Result <- rpc.BridgeCallbackResult{Data: body, Err: err}
			case <-ctx.Done():
				request.Result <- rpc.BridgeCallbackResult{Err: ctx.Err()}
			}
		case <-ctx.Done():
		}
	}()
	commandDone := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		proc.Stop()
		<-joined
		<-commandDone
		pluginEnd.Close() //nolint:errcheck // Test-only unused startup endpoint.
		wait, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		require.NoError(t, proc.Wait(wait))
	})
	result := make(chan *plugin.Response, 1)
	go func() {
		defer close(commandDone)
		result <- quiesceAll(ctx, s.Quiescers(), time.Second)
	}()
	select {
	case <-entered:
	case response := <-result:
		t.Fatalf("registered quiescer answered before deferred work: %+v", response)
	case <-ctx.Done():
		t.Fatal("registered quiescer never reached its current manager's observer")
	}
	select {
	case response := <-result:
		t.Fatalf("held observer completed quiesce: %+v", response)
	default:
	}
	require.NoError(t, proc.DrainEventsApplied(ctx), "settlement must not hold the delivery worker")
	close(release)
	select {
	case response := <-result:
		require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	case <-ctx.Done():
		t.Fatal("released observer did not complete quiesce")
	}
}

// TestPluginEventQuiescerSkipsStoppedProcesses preserves the manager's existing
// terminal-process treatment while live observers gain acknowledged settlement.
func TestPluginEventQuiescerSkipsStoppedProcesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	manager := process.NewProcessManager(nil)
	stopped := process.NewProcess(plugin.PluginConfig{Name: "ended-observer"})
	stopped.StartDelivery(ctx)
	stopped.Stop()
	require.NoError(t, manager.AddProcess(stopped.Name(), stopped))
	s := &Server{}
	s.procManager.Store(manager)
	registerPluginEventQuiescer(s)
	response := quiesceAll(ctx, s.Quiescers(), time.Second)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	require.NoError(t, stopped.Wait(ctx))
}
