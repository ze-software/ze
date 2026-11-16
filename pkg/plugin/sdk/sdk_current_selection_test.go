package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A blocked ordinary handler must not prevent the engine from validating the
// very selected dispatch that handler is waiting to finish.
func TestCurrentSelectionBypassesBlockedOrdinaryLoop(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "wire"
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
			}
			if typed {
				connection.SetBridge(bridge)
			}
			p.OnCurrentSelection(func(ctx context.Context, in rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
				return &rpc.CurrentSelectionOutput{Destination: in.Destination, DestinationSession: in.DestinationSession,
					Cut: 500, Results: []rpc.CurrentSelectionResult{{Family: in.Keys[0].Family, KeyNLRI: in.Keys[0].KeyNLRI,
						Revision: in.Keys[0].Revision, Presence: rpc.CurrentSelectionPresent, CurrentRevision: 3}}}, nil
			})
			stop, err := p.startCurrentSelectionWorker(ctx)
			require.NoError(t, err)
			defer stop()
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
				go func() { loopDone <- p.bridgeEventLoop(ctx) }()
				go func() {
					_, err := bridge.SendCallback(ctx, "ze-plugin-callback:execute-command", json.RawMessage(`{"command":"held"}`))
					commandDone <- err
				}()
			} else {
				go func() { loopDone <- p.eventLoop(ctx) }()
				go func() {
					answer, err := engine.mux.CallAnswer(ctx, "ze-plugin-callback:execute-command", map[string]any{"command": "held"})
					if err == nil {
						for range answer.Records {
						}
						err = answer.Err()
					}
					commandDone <- err
				}()
			}
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("ordinary callback never entered")
			}
			input := rpc.CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
				Keys: []rpc.CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{24, 192, 0, 2}, Revision: 3}}}
			var out *rpc.CurrentSelectionOutput
			out, err = connection.SendCurrentSelection(ctx, input)
			require.NoError(t, err)
			require.Equal(t, uint64(500), out.Cut)
			require.Equal(t, uint64(3), out.Results[0].CurrentRevision)
			close(release)
			require.NoError(t, <-commandDone)
			cancel()
			<-loopDone
		})
	}
}

func TestCurrentSelectionDeclarationFollowsHandler(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-handler", true: "handler"}[registered], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if registered {
				p.OnCurrentSelection(func(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) { return nil, nil })
			}
			started := make(chan struct{})
			p.OnStarted(func(context.Context) error {
				close(started)
				return nil
			})
			done := make(chan error, 1)
			go func() { done <- p.Run(ctx, &Registration{CurrentSelection: !registered}) }()
			req := readEngineRequest(t, ctx, engine.mux)
			var declaration Registration
			require.NoError(t, json.Unmarshal(req.Params, &declaration))
			require.Equal(t, registered, declaration.CurrentSelection)
			require.NoError(t, engine.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
			completeStartupFromStage2(t, ctx, engine)
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("ready acknowledgment never reached OnStarted")
			}
			cancel()
			require.NoError(t, <-done)
		})
	}
}

func TestCurrentSelectionWorkerStopJoinsHandler(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	entered, left := make(chan struct{}), make(chan struct{})
	p.OnCurrentSelection(func(ctx context.Context, _ rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		close(entered)
		<-ctx.Done()
		close(left)
		return nil, ctx.Err()
	})
	stop, err := p.startCurrentSelectionWorker(ctx)
	require.NoError(t, err)
	bridge.SetReady()
	done := make(chan error, 1)
	go func() {
		_, err := bridge.CurrentSelection(ctx, rpc.CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
			Keys: []rpc.CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 3}}})
		done <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("snapshot handler never entered")
	}
	stop()
	select {
	case <-left:
	default:
		t.Fatal("stop returned before snapshot handler joined")
	}
	require.Error(t, <-done)
	_, err = bridge.CurrentSelection(ctx, rpc.CurrentSelectionInput{})
	require.Error(t, err)
}
