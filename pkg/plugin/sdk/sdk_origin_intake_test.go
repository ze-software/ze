// Design: docs/architecture/api/process-protocol.md -- mutable origin intake.
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func originIntakeTestInput() rpc.OriginIntakeInput {
	return rpc.OriginIntakeInput{Operation: rpc.OriginIntakeAnnounce, Destination: "192.0.2.1", PeerOwner: 7,
		OriginReceipt: 13, Producer: rpc.OriginProducer{Kind: rpc.OriginProducerProcess, Name: "producer"},
		Source: rpc.OriginSource{Kind: rpc.OriginSourceLocalBGP},
		Route: rpc.OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0},
			AttributesExplicit: true, Attributes: []byte{64, 1, 1, 0}, SourceContext: 17,
			SourceAddPath: true, SourcePathID: 0, NextHopPolicy: rpc.OriginNextHopSelf}}
}

func originIntakeTestReply(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
	return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: in.Route.Family,
		KeyNLRI: in.Route.KeyNLRI, Destination: in.Destination, PeerOwner: in.PeerOwner,
		Generation: in.OriginReceipt, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}
}

// The same public connection entry point takes both transports. Cancellation
// cannot return while a handler that has observed it still owns a mutation.
func TestOriginIntakeTransportsJoinCancellation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bridge := rpc.NewDirectBridge()
			connection := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				connection.SetBridge(bridge)
			}
			entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			releaseHandler := sync.OnceFunc(func() { close(release) })
			p.OnOriginIntake(func(work context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
				if input.Route.SourceContext != 17 {
					t.Error("lost original SourceContext")
				}
				if !bytes.Equal(input.Route.Attributes, []byte{64, 1, 1, 0}) {
					t.Error("lost original attributes")
				}
				if !input.Route.SourceAddPath || input.Route.SourcePathID != 0 {
					t.Error("lost legal source path zero")
				}
				close(entered)
				<-work.Done()
				close(canceled)
				<-release
				return nil, work.Err()
			})
			stop, err := p.startOriginIntakeWorker(ctx)
			require.NoError(t, err)
			defer func() { releaseHandler(); require.NoError(t, stop()) }()
			if typed {
				bridge.SetReady()
			}
			work, end := context.WithCancel(ctx)
			result := make(chan error, 1)
			go func() { _, err := connection.SendOriginIntake(work, originIntakeTestInput()); result <- err }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			end()
			select {
			case <-canceled:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err := <-result:
				t.Fatalf("unjoined mutation returned: %v", err)
			default:
			}
			releaseHandler()
			require.ErrorIs(t, <-result, context.Canceled)
		})
	}
}

func TestOriginIntakeTransportsKeepReceiptAndRefuseScopeRebinding(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, wrong := range []bool{false, true} {
			t.Run(map[bool]string{false: "framed", true: "typed"}[typed]+map[bool]string{false: "/retained", true: "/wrong-scope"}[wrong], func(t *testing.T) {
				p, engine := newTestPair(t)
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				bridge := rpc.NewDirectBridge()
				connection := ipc.NewMuxPluginConn(engine.mux)
				if typed {
					p.bridge = bridge
					connection.SetBridge(bridge)
				}
				p.OnOriginIntake(func(_ context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
					result := originIntakeTestReply(in)
					if wrong {
						result.Admissions[0].PeerOwner++
					}
					return result, nil
				})
				stop, err := p.startOriginIntakeWorker(ctx)
				require.NoError(t, err)
				defer func() { require.NoError(t, stop()) }()
				if typed {
					bridge.SetReady()
				}
				result, err := connection.SendOriginIntake(ctx, originIntakeTestInput())
				if wrong {
					require.Error(t, err)
					require.Nil(t, result)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.Retained())
				require.Zero(t, result.Announced)
			})
		}
	}
}

func TestOriginIntakeDeclarationComesFromHandler(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "present"}[registered], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if registered {
				p.OnOriginIntake(func(context.Context, rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
					return nil, errors.New("declaration only")
				})
			}
			started := make(chan struct{})
			p.OnStarted(func(context.Context) error { close(started); return nil })
			done := make(chan error, 1)
			go func() { done <- p.Run(ctx, &Registration{OriginIntake: !registered}) }()
			request := readEngineRequest(t, ctx, engine.mux)
			var declaration Registration
			require.NoError(t, json.Unmarshal(request.Params, &declaration))
			require.Equal(t, registered, declaration.OriginIntake)
			require.NoError(t, engine.mux.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
			completeStartupFromStage2(t, ctx, engine)
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			cancel()
			require.NoError(t, <-done)
		})
	}
}

func TestOriginIntakeTransportsPreserveFailureAndMissingReceipt(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, missing := range []bool{false, true} {
			t.Run(map[bool]string{false: "framed", true: "typed"}[typed]+map[bool]string{false: "/partial-failure", true: "/missing-receipt"}[missing], func(t *testing.T) {
				p, engine := newTestPair(t)
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				bridge := rpc.NewDirectBridge()
				connection := ipc.NewMuxPluginConn(engine.mux)
				if typed {
					p.bridge = bridge
					connection.SetBridge(bridge)
				}
				cause := errors.New("candidate reconciliation failed after completed output")
				p.OnOriginIntake(func(_ context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
					if missing {
						return nil, nil //nolint:nilnil // A missing successful mutation receipt must be refused.
					}
					output := originIntakeTestReply(in)
					output.Announced = 1
					output.Admissions[0].Disposition = rpc.RouteAdmissionSelected
					output.Warnings = []string{"earlier output completed"}
					return output, cause
				})
				stop, err := p.startOriginIntakeWorker(ctx)
				require.NoError(t, err)
				defer func() { require.NoError(t, stop()) }()
				if typed {
					bridge.SetReady()
				}
				output, err := connection.SendOriginIntake(ctx, originIntakeTestInput())
				require.Error(t, err)
				if missing {
					require.Nil(t, output)
					return
				}
				require.ErrorContains(t, err, cause.Error())
				require.NotNil(t, output)
				require.Equal(t, uint32(1), output.Announced)
				require.Equal(t, []string{"earlier output completed"}, output.Warnings)
				require.Equal(t, cause.Error(), output.Failure)
				require.Len(t, output.Admissions, 1)
				require.False(t, output.Retained(), "partial failure cannot be reported as accepted retention")
			})
		}
	}
}
