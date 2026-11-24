// Design: docs/architecture/api/process-protocol.md -- quiesce joins selected output.
// Related: rib_selected_dispatch.go -- the admitted destination owns its final receipt.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type quiesceSelectedRequest struct {
	request *rpc.Request
	input   rpc.SelectRoutesInput
	refresh *rpc.SelectRefreshInput
}

// startQuiesceRIB runs the real registration and SDK loop, then exposes its
// engine-side Process through the actual manager drain after runtime-ready has
// joined SDK activation and initialization. The fixture owns one
// engine reader and two bounded destination writers and MUST join them at stop.
func startQuiesceRIB(t *testing.T, ctx context.Context, native bool, peers [2]netip.Addr, send rpc.SelectRoutesHandler, markers ...selectedRefreshSender) (*process.ProcessManager, *RIBManager) {
	t.Helper()
	require.LessOrEqual(t, len(markers), 1)
	pluginEnd, engineEnd := net.Pipe()
	connection := ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)))
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib", Encoder: "json"})
	proc.SetConn(connection)
	proc.StartDelivery(ctx)
	proc.SetRunning(true)
	manager := process.NewProcessManager(nil)
	require.NoError(t, manager.AddProcess(proc.Name(), proc))
	runtimeCtx, stopRuntime := context.WithCancel(ctx)
	pluginConnection := pluginEnd
	var bridge *rpc.DirectBridge
	if native {
		bridge = rpc.NewDirectBridge()
		bridge.SetSelectRoutes(send)
		if len(markers) == 1 {
			bridge.SetDispatchNative(func(work context.Context, method string, raw any) (any, bool, error) {
				if method != rpc.MethodSelectRefresh {
					return nil, false, nil
				}
				input, valid := raw.(rpc.SelectRefreshInput)
				if !valid {
					return nil, true, errors.New("refresh native request has the wrong type")
				}
				if err := input.Validate(time.Now()); err != nil {
					return nil, true, err
				}
				output, err := markers[0](work, input)
				return output, true, err
			})
		}
		bridge.SetDispatchRPC(func(work context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
			if method == rpc.MethodSelectRefresh && len(markers) == 1 {
				var input rpc.SelectRefreshInput
				if err := json.Unmarshal(params, &input); err != nil {
					return nil, err
				}
				if err := input.Validate(time.Now()); err != nil {
					return nil, err
				}
				answer, err := markers[0](work, input)
				if err != nil {
					return nil, err
				}
				return json.Marshal(answer)
			}
			answer, err := quiesceRIBEngineAnswer(method)
			if err != nil {
				return nil, err
			}
			return json.Marshal(answer)
		})
		pluginConnection = rpc.NewBridgedConn(pluginEnd, bridge)
	}
	pluginDone := make(chan int, 1)
	go func() { pluginDone <- runRIBPlugin(pluginConnection) }()
	var workers sync.WaitGroup
	t.Cleanup(func() {
		stopRuntime()
		proc.Stop()
		if bridge != nil {
			bridge.CloseCallbacks()
		}
		require.NoError(t, connection.Close())
		require.NoError(t, pluginEnd.Close())
		workers.Wait()
		select {
		case <-pluginDone:
		case <-ctx.Done():
			t.Error("the real RIB did not join after its callback connection closed")
		}
		wait, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, proc.Wait(wait))
	})

	declaration := quiesceRIBStartup(t, ctx, connection, bridge)
	proc.SetRegistration(&plugin.PluginRegistration{EventSettlement: declaration.EventSettlement})
	writers := make(map[string]chan quiesceSelectedRequest, len(peers))
	for _, peer := range peers {
		queue := make(chan quiesceSelectedRequest, 1)
		writers[peer.String()] = queue
		workers.Go(func() {
			for {
				select {
				case request := <-queue:
					var output any
					var err error
					if request.refresh != nil {
						output, err = markers[0](runtimeCtx, *request.refresh)
					} else {
						output, err = send(runtimeCtx, request.input)
					}
					if err != nil {
						if runtimeCtx.Err() == nil {
							t.Errorf("selected writer: %v", err)
						}
						return
					}
					if err := connection.SendResult(runtimeCtx, request.request.ID, output); err != nil {
						if runtimeCtx.Err() == nil {
							t.Errorf("selected writer receipt: %v", err)
						}
						return
					}
				case <-runtimeCtx.Done():
					return
				}
			}
		})
	}
	workers.Go(func() {
		for {
			request, err := connection.ReadRequest(runtimeCtx)
			if err != nil {
				return // Native startup retires its mux; teardown closes either transport.
			}
			if request.Method == rpc.MethodSelectRoutes {
				var input rpc.SelectRoutesInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					t.Errorf("selected request: %v", err)
					return
				}
				queue, ok := writers[input.Destination]
				if !ok {
					t.Errorf("selected request has an unconfigured fixture destination %q", input.Destination)
					return
				}
				select {
				case queue <- quiesceSelectedRequest{request: request, input: input}:
				case <-runtimeCtx.Done():
					return
				}
				continue
			}
			if request.Method == rpc.MethodSelectRefresh && len(markers) == 1 {
				var input rpc.SelectRefreshInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					t.Errorf("refresh request: %v", err)
					return
				}
				if err := input.Validate(time.Now()); err != nil {
					t.Errorf("refresh request: %v", err)
					return
				}
				queue, configured := writers[input.Destination]
				if !configured {
					t.Errorf("refresh request has an unconfigured fixture destination %q", input.Destination)
					return
				}
				select {
				case queue <- quiesceSelectedRequest{request: request, refresh: &input}:
				case <-runtimeCtx.Done():
					return
				}
				continue
			}
			if err := quiesceRIBAnswerRequest(runtimeCtx, connection, request); err != nil {
				if runtimeCtx.Err() == nil {
					t.Errorf("RIB engine operation: %v", err)
				}
				return
			}
		}
	})
	r := activeManager.Load()
	require.NotNil(t, r)
	return manager, r
}

// quiesceRIBStartup preserves the real RIB's Stage-1 support declaration rather
// than claiming callback support on the engine's behalf.
func quiesceRIBStartup(t *testing.T, ctx context.Context, connection *ipc.PluginConn, bridge *rpc.DirectBridge) rpc.DeclareRegistrationInput {
	t.Helper()
	request, err := connection.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, rpc.MethodDeclareRegistration, request.Method)
	var declaration rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(request.Params, &declaration))
	require.NoError(t, connection.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
	require.NoError(t, connection.SendConfigure(ctx, nil, nil, nil, nil))
	request, err = connection.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", request.Method)
	require.NoError(t, connection.SendOK(ctx, request.ID))
	require.NoError(t, connection.SendShareRegistry(ctx, nil))
	request, err = connection.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, "ze-plugin-engine:ready", request.Method)
	if bridge != nil {
		connection.SetBridge(bridge)
	}
	require.NoError(t, connection.SendOK(ctx, request.ID))
	// The final OK precedes SDK bridge activation and OnStarted. Join the
	// actual runtime-ready receipt before a test can publish selected work;
	// otherwise a first batch can use the startup mux just as the SDK retires it.
	require.NoError(t, connection.SendRuntimeReadyServing(ctx, func(request *rpc.Request) error {
		return quiesceRIBAnswerRequest(ctx, connection, request)
	}))
	return declaration
}

// quiesceRIBAnswerRequest serves the same nested engine calls during startup
// initialization and runtime, preserving each operation's actual answer frame.
func quiesceRIBAnswerRequest(ctx context.Context, connection *ipc.PluginConn, request *rpc.Request) error {
	if request.Method == rpc.MethodDispatchCommand {
		// DispatchCommand reads the current record-answer grammar;
		// generic RPC ok is not a command completion receipt.
		return rpc.WriteDocumentAnswer(connection.AnswerWriter(ctx), request.ID, rpc.AnswerTail{}, nil)
	}
	answer, err := quiesceRIBEngineAnswer(request.Method)
	if err != nil {
		return err
	}
	return connection.SendResult(ctx, request.ID, answer)
}

// quiesceRIBEngineAnswer acknowledges only the RIB's existing announcements and
// session-ready command. The selected operation has its separate final writers.
func quiesceRIBEngineAnswer(method string) (any, error) {
	switch method {
	case rpc.MethodEmitEvent:
		return &rpc.EmitEventOutput{}, nil
	case rpc.MethodDispatchCommand:
		return &rpc.DispatchCommandOutput{Status: "done"}, nil
	default:
		return nil, fmt.Errorf("unexpected quiesce fixture engine operation %q", method)
	}
}

// TestQuiesceWaitsForSelectedDispatcherReceipt starts the real RIB and manager
// on both transports. A held final selected receipt must prevent successful
// quiesce while an independent destination remains useful; releasing it permits
// a fresh quiesce and preserves both exact native routes.
func TestQuiesceWaitsForSelectedDispatcherReceipt(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.181"), netip.MustParseAddr("192.0.2.182")}
			entered := make(chan struct{})
			release := make(chan struct{})
			written := make(chan rpc.SelectRoutesInput, 2)
			var releaseOnce sync.Once
			send := func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				if input.Destination == peers[0].String() {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				written <- input
				return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
			}
			manager, r := startQuiesceRIB(t, ctx, native, peers, send)
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			route := selectedDispatchRoute(88, 0x7788)
			require.NoError(t, r.selectedDispatch.publish(&route))
			require.NoError(t, r.selectedDispatch.destinationUp(peers[0], 181))
			selectedDispatchAwait(t, ctx, entered)
			require.NoError(t, r.selectedDispatch.destinationUp(peers[1], 182))
			require.NoError(t, r.selectedDispatch.barrier(ctx, peers[1]))
			healthy := selectedDispatchAwait(t, ctx, written)
			require.Equal(t, peers[1].String(), healthy.Destination)
			require.Equal(t, route.keyNLRI, healthy.Routes[0].KeyNLRI)

			// This deadline is the asserted failure boundary, not a delay: the
			// held writer cannot complete until release, so success is premature.
			blocked, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			err := manager.DrainEventDelivery(blocked)
			cancel()
			require.Error(t, err, "quiesce acknowledged while the selected receipt was held")
			require.True(t, errors.Is(err, context.DeadlineExceeded), "%v", err)

			releaseOnce.Do(func() { close(release) })
			require.NoError(t, manager.DrainEventDelivery(ctx))
			finished := selectedDispatchAwait(t, ctx, written)
			require.Equal(t, peers[0].String(), finished.Destination)
			require.Equal(t, uint64(181), finished.DestinationSession)
			require.Equal(t, route.keyNLRI, finished.Routes[0].KeyNLRI)
			require.Equal(t, uint64(0x7788), finished.Routes[0].SourceReceipt)
			require.Len(t, finished.Routes, 1)
		})
	}
}
