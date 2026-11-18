// Design: docs/architecture/api/process-protocol.md -- independent accepted-record callback lifetime.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// admissionLookupJoin cancels and waits for the owned test goroutine to close
// done. Its channel has at most one result, so the loop is bounded by that one
// result and closure. A fresh deadline also bounds cleanup after a test fails.
func admissionLookupJoin[Result any](t *testing.T, cancel context.CancelFunc, done <-chan Result) {
	t.Helper()
	cancel()
	limit, finish := context.WithTimeout(context.Background(), 5*time.Second)
	defer finish()
	for {
		select {
		case _, open := <-done:
			if !open {
				return
			}
		case <-limit.Done():
			t.Error("owned lookup fixture goroutine did not join")
			return
		}
	}
}

func admissionLookupInput() rpc.OriginAdmissionLookupInput {
	return rpc.OriginAdmissionLookupInput{AdmissionID: 91, Family: "ipv4/unicast", Prefix: "198.51.100.0/24",
		Destination: "192.0.2.7", PeerOwner: 17, Operation: rpc.OriginIntakeAnnounce}
}

func admissionLookupOutput(input rpc.OriginAdmissionLookupInput) *rpc.OriginAdmissionLookupOutput {
	return &rpc.OriginAdmissionLookupOutput{AdmissionID: input.AdmissionID, Family: input.Family, Prefix: input.Prefix,
		Destination: input.Destination, PeerOwner: input.PeerOwner, Operation: input.Operation, State: rpc.OriginAdmissionCurrent,
		Snapshot: rpc.OriginAdmissionSnapshot{Protocol: "static", SourceInstance: 3, SourceMetric: 7,
			HasDistanceOverride: true, NextHop: "192.0.2.9", OriginASN: 64496, Community: []uint32{64512<<16 | 9},
			Tag: 42, Metric: 303, HasMetric: true, PeerScope: rpc.OriginAdmissionAllPeers}}
}

// A real blocked ordinary command cannot prevent the callback that its engine
// update needs. Both transports use the same original PluginConn and snapshot.
func TestOriginAdmissionLookupBypassesBlockedOrdinaryLoop(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "framed"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bridge := rpc.NewDirectBridge()
			connection := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				connection.SetBridge(bridge)
			}
			p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
				return admissionLookupOutput(input), nil
			})
			stop, err := p.startOriginAdmissionLookupWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()
			entered, release := make(chan struct{}), make(chan struct{})
			p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return rpc.StatusOK, nil, nil
			})
			loopDone, commandDone := make(chan error, 1), make(chan error, 1)
			if typed {
				p.activateBridge()
				go func() { defer close(loopDone); loopDone <- p.bridgeEventLoop(ctx) }()
				go func() {
					defer close(commandDone)
					_, err := bridge.SendCallback(ctx, "ze-plugin-callback:execute-command", json.RawMessage(`{"command":"held"}`))
					commandDone <- err
				}()
			} else {
				go func() { defer close(loopDone); loopDone <- p.eventLoop(ctx) }()
				go func() {
					defer close(commandDone)
					answer, err := engine.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command", map[string]any{"command": "held"})
					if err == nil {
						for range answer.Records {
						}
						err = answer.Err()
					}
					commandDone <- err
				}()
			}
			t.Cleanup(func() { admissionLookupJoin(t, cancel, commandDone); admissionLookupJoin(t, cancel, loopDone) })
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("ordinary callback never entered")
			}
			input := admissionLookupInput()
			output, err := connection.SendOriginAdmissionLookup(ctx, input)
			require.NoError(t, err)
			require.Equal(t, admissionLookupOutput(input), output)
			close(release)
			require.NoError(t, <-commandDone)
			cancel()
			<-loopDone
		})
	}
}

// A held real origin mutation can await a source snapshot without blocking the
// separate lookup lane on either typed or framed transport.
func TestOriginAdmissionLookupRunsDuringOriginMutation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "framed"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bridge := rpc.NewDirectBridge()
			connection := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				connection.SetBridge(bridge)
			}
			p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
				return admissionLookupOutput(input), nil
			})
			entered, release := make(chan struct{}), make(chan struct{})
			p.OnOriginIntake(func(ctx context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: input.Route.Family, KeyNLRI: input.Route.KeyNLRI,
					Destination: input.Destination, PeerOwner: input.PeerOwner, Generation: input.OriginReceipt,
					Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}, nil
			})
			stopMutation, err := p.startOriginIntakeWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stopMutation()) }()
			stopLookup, err := p.startOriginAdmissionLookupWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stopLookup()) }()
			if typed {
				bridge.SetReady()
			}
			mutation := rpc.OriginIntakeInput{Operation: rpc.OriginIntakeAnnounce, Destination: "192.0.2.7", PeerOwner: 17, OriginReceipt: 3,
				Producer: rpc.OriginProducer{Kind: rpc.OriginProducerOperator}, Source: rpc.OriginSource{Kind: rpc.OriginSourceLocalBGP},
				Route: rpc.OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 51, 100}, NextHopPolicy: rpc.OriginNextHopSelf}}
			type mutationResult struct {
				output *rpc.UpdateRouteOutput
				err    error
			}
			done := make(chan mutationResult, 1)
			go func() {
				defer close(done)
				output, err := connection.SendOriginIntake(ctx, mutation)
				done <- mutationResult{output: output, err: err}
			}()
			t.Cleanup(func() { admissionLookupJoin(t, cancel, done) })
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("origin mutation never entered")
			}
			input := admissionLookupInput()
			output, err := connection.SendOriginAdmissionLookup(ctx, input)
			require.NoError(t, err)
			require.Equal(t, admissionLookupOutput(input), output)
			close(release)
			completed := <-done
			require.NoError(t, completed.err)
			require.Len(t, completed.output.Admissions, 1)
		})
	}
}

// Stage 1 derives optional support from the actual handler, even when a caller
// supplied the opposite registration bit. Real startup reaches OnStarted.
func TestOriginAdmissionLookupDeclarationFollowsHandler(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "registered"}[registered], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if registered {
				p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
					return admissionLookupOutput(input), nil
				})
			}
			entered := make(chan struct{})
			p.OnStarted(func(context.Context) error { close(entered); return nil })
			done := make(chan error, 1)
			go func() { defer close(done); done <- p.Run(ctx, &Registration{OriginAdmissionLookup: !registered}) }()
			t.Cleanup(func() { admissionLookupJoin(t, cancel, done) })
			request := readEngineRequest(t, ctx, engine.mux)
			var declaration Registration
			require.NoError(t, json.Unmarshal(request.Params, &declaration))
			require.Equal(t, registered, declaration.OriginAdmissionLookup)
			require.NoError(t, engine.mux.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
			completeStartupFromStage2(t, ctx, engine)
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("startup never reached OnStarted")
			}
			cancel()
			require.NoError(t, <-done)
		})
	}
}

// Stop must wait for the canceled read handler, refuse later lookups and leave
// no queued caller dependent on an abandoned response channel.
func TestOriginAdmissionLookupStopJoinsHandler(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	entered, left := make(chan struct{}), make(chan struct{})
	p.OnOriginAdmissionLookup(func(ctx context.Context, _ rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		close(entered)
		<-ctx.Done()
		close(left)
		return nil, ctx.Err()
	})
	stop, err := p.startOriginAdmissionLookupWorker(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, stop()) }()
	bridge.SetReady()
	done := make(chan error, 1)
	go func() {
		defer close(done)
		_, err := bridge.OriginAdmissionLookup(ctx, admissionLookupInput())
		done <- err
	}()
	t.Cleanup(func() { admissionLookupJoin(t, cancel, done) })
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("lookup never entered")
	}
	require.NoError(t, stop())
	select {
	case <-left:
	default:
		t.Fatal("stop returned before handler joined")
	}
	require.Error(t, <-done)
	_, err = bridge.OriginAdmissionLookup(ctx, admissionLookupInput())
	require.Error(t, err)
}

// Wire saturation refuses before blocking the sole reader, while typed queue
// refusal does not consume an extra slot or silently drop another request.
func TestOriginAdmissionLookupQueueIsBounded(t *testing.T) {
	worker := &originAdmissionLookupWorker{requests: make(chan originAdmissionLookupJob, originAdmissionLookupQueueDepth), done: make(chan struct{})}
	request := &rpc.Request{Method: rpc.MethodOriginAdmissionLookup}
	for range originAdmissionLookupQueueDepth {
		require.NoError(t, worker.admitWire(request))
	}
	require.ErrorIs(t, worker.admitWire(request), errOriginAdmissionLookupFull)
	require.NotNil(t, worker.failure.Load())
	require.ErrorIs(t, *worker.failure.Load(), errOriginAdmissionLookupFull)
	input := admissionLookupInput()
	input.DeadlineMS = time.Now().Add(time.Second).UnixMilli()
	_, err := worker.lookup(t.Context(), input)
	require.ErrorIs(t, err, errOriginAdmissionLookupFull)
	require.Len(t, worker.requests, originAdmissionLookupQueueDepth)
	worker.stopAdmission()
	require.ErrorIs(t, worker.admitWire(request), errOriginAdmissionLookupStopped)
}

// A stale or missing provider record remains an explicit error over both rails;
// neither transport invents an empty successful accepted snapshot.
func TestOriginAdmissionLookupProviderError(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "framed"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bridge := rpc.NewDirectBridge()
			connection := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				connection.SetBridge(bridge)
			}
			p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
				if input.AdmissionID != 91 {
					return nil, errors.New("original accepted record is stale or missing")
				}
				return admissionLookupOutput(input), nil
			})
			stop, err := p.startOriginAdmissionLookupWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()
			if typed {
				bridge.SetReady()
			}
			input := admissionLookupInput()
			output, err := connection.SendOriginAdmissionLookup(ctx, input)
			require.NoError(t, err)
			require.Equal(t, admissionLookupOutput(input), output)
			input.AdmissionID++
			output, err = connection.SendOriginAdmissionLookup(ctx, input)
			require.ErrorContains(t, err, "stale or missing")
			require.Nil(t, output)
		})
	}
}

// A published but unavailable typed slot cannot fall back to a still-useful
// framed callback on the same original PluginConn.
func TestOriginAdmissionLookupBridgeCutoverNeverFallsBack(t *testing.T) {
	p, engine := newTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	connection := ipc.NewMuxPluginConn(engine.mux)
	p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		return admissionLookupOutput(input), nil
	})
	stop, err := p.startOriginAdmissionLookupWorker(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, stop()) }()
	input := admissionLookupInput()
	output, err := connection.SendOriginAdmissionLookup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, admissionLookupOutput(input), output)
	connection.SetBridge(rpc.NewDirectBridge())
	output, err = connection.SendOriginAdmissionLookup(ctx, input)
	require.ErrorIs(t, err, rpc.ErrBridgeClosed)
	require.Nil(t, output)
	output, err = engine.mux.OriginAdmissionLookup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, admissionLookupOutput(input), output)
}

// A saturated real framed ingress fails Run and joins the active canceled
// handler. It must not stall the only reader behind the full lookup queue.
func TestOriginAdmissionLookupWireQueueFullFailsRun(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	defer pluginEnd.Close() //nolint:errcheck // Test transport cleanup.
	defer engineEnd.Close() //nolint:errcheck // Test transport cleanup.
	p := NewWithConn("admission-full", pluginEnd)
	wire := rpc.NewConn(engineEnd, engineEnd)
	engine := &engineSide{mux: rpc.NewMuxConn(wire)}
	defer engine.mux.Close() //nolint:errcheck // Test transport cleanup.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	entered := make(chan struct{})
	p.OnOriginAdmissionLookup(func(ctx context.Context, _ rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := make(chan error, 1)
	go func() { defer close(done); done <- p.Run(ctx, &Registration{}) }()
	t.Cleanup(func() { admissionLookupJoin(t, cancel, done) })
	completeStartup(t, ctx, engine)
	input := admissionLookupInput()
	input.DeadlineMS = time.Now().Add(4 * time.Second).UnixMilli()
	body, err := json.Marshal(input)
	require.NoError(t, err)
	require.NoError(t, wire.WriteRawFrame(append(rpc.AppendRequest(nil, 100, rpc.MethodOriginAdmissionLookup, body), '\n')))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("lookup never entered")
	}
	for i := range originAdmissionLookupQueueDepth + 1 {
		err := wire.WriteRawFrame(append(rpc.AppendRequest(nil, uint64(101+i), rpc.MethodOriginAdmissionLookup, body), '\n'))
		if err != nil {
			require.Equal(t, originAdmissionLookupQueueDepth, i, "connection closed before queue filled")
		}
	}
	select {
	case err := <-done:
		require.ErrorIs(t, err, errOriginAdmissionLookupFull)
	case <-ctx.Done():
		t.Fatal("Run did not join failed lookup ingress")
	}
}

// Malformed wire input gets a bounded refusal and leaves the same original
// connection useful for a later valid accepted-record lookup.
func TestOriginAdmissionLookupMalformedWireKeepsValidRead(t *testing.T) {
	p, engine := newTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		return admissionLookupOutput(input), nil
	})
	stop, err := p.startOriginAdmissionLookupWorker(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, stop()) }()
	_, err = engine.mux.CallRPC(ctx, rpc.MethodOriginAdmissionLookup, map[string]any{"admission-id": 91})
	require.ErrorContains(t, err, "requires")
	input := admissionLookupInput()
	output, err := engine.mux.OriginAdmissionLookup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, admissionLookupOutput(input), output)
}

// A real absolute read deadline cancels and joins the active handler without
// relying on a wall-clock duration assertion.
func TestOriginAdmissionLookupWorkerHonorsDeadline(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	left := make(chan struct{})
	p.OnOriginAdmissionLookup(func(ctx context.Context, _ rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		<-ctx.Done()
		close(left)
		return nil, ctx.Err()
	})
	stop, err := p.startOriginAdmissionLookupWorker(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, stop()) }()
	bridge.SetReady()
	input := admissionLookupInput()
	input.DeadlineMS = time.Now().Add(100 * time.Millisecond).UnixMilli()
	_, err = bridge.OriginAdmissionLookup(ctx, input)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	select {
	case <-left:
	case <-ctx.Done():
		t.Fatal("deadline did not end read handler")
	}
}

// A defect in a registered handler ends the callback connection and remains
// visible when the owning SDK joins this worker.
func TestOriginAdmissionLookupWorkerPanicFailsConnection(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.OnOriginAdmissionLookup(func(context.Context, rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		panic("BUG: test accepted-record handler defect")
	})
	stop, err := p.startOriginAdmissionLookupWorker(ctx)
	require.NoError(t, err)
	defer func() { require.ErrorContains(t, stop(), "handler defect") }()
	bridge.SetReady()
	_, err = bridge.OriginAdmissionLookup(ctx, admissionLookupInput())
	require.ErrorContains(t, err, "handler defect")
	require.ErrorContains(t, stop(), "handler defect")
	require.False(t, bridge.Ready())
}
