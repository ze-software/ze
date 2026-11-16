// Design: docs/architecture/api/process-protocol.md -- selected-route final completion.
// Related: dispatch_selected.go -- authenticated selected output boundary.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSelectedRouteOperationRegistered proves selected output has one runtime
// operation from which socket, direct and typed dispatch can be wired.
func TestSelectedRouteOperationRegistered(t *testing.T) {
	operation := lookupEngineOp("ze-plugin-engine:select-routes")
	if operation == nil {
		t.Fatal("selected-route operation is not registered")
	}
	if operation.typedWire == nil {
		t.Fatal("selected-route operation has no typed transport")
	}
}

type selectedBoundaryReactor struct {
	*mockReactor
	owner   plugin.SelectedOwner
	calls   int
	missing bool
}

func (reactor *selectedBoundaryReactor) SelectRoutes(ctx context.Context, batch *plugin.SelectedBatch, owner plugin.SelectedOwner) (*rpc.SelectRoutesOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reactor.owner = owner
	reactor.calls++
	if reactor.missing {
		return nil, nil //nolint:nilnil // Deliberate malformed engine answer under test.
	}
	input := batch.Input()
	results := make([]rpc.SelectedRouteResult, len(input.Routes))
	for i, route := range input.Routes {
		results[i] = rpc.SelectedRouteResult{Family: route.Family, KeyNLRI: route.KeyNLRI,
			Revision: route.Revision, CurrentRevision: route.Revision, Outcome: rpc.SelectedWritten}
	}
	return &rpc.SelectRoutesOutput{Results: results}, nil
}

func selectedServerInput() rpc.SelectRoutesInput {
	return rpc.SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3,
		Routes: []rpc.SelectedRoute{{Kind: rpc.SelectedRouteReceived, Family: "ipv4/unicast",
			KeyNLRI: []byte{24, 10, 0, 0}, Revision: 7, SourceReceipt: 11, StaleLevel: rpc.SelectedFresh,
			Route: &rpc.StoredRoute{SourcePeer: "192.0.2.8", Family: "ipv4/unicast",
				AttrHex: "40010100", NextHopHex: "c0000201", NLRIHex: "180a0000",
				MsgID: 13, SourcePolicyEpoch: 17, NLRIFraming: rpc.NLRIFramingPrefixOnly}}}}
}

// Socket, direct JSON and typed dispatch use the same authenticated process and
// return the same complete selected receipt. The fake represents final completion,
// not a production writer or a proof that BGP bytes have reached a peer.
func TestSelectedRouteBoundaryTransportParity(t *testing.T) {
	reactor := &selectedBoundaryReactor{mockReactor: &mockReactor{}}
	host := &Server{ctx: t.Context(), reactor: reactor}
	proc := nativeOwnerProcess(t, host)
	bridge := proc.Bridge()
	bridge.SetReady()
	input := selectedServerInput()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range []string{"socket", "direct", "typed"} {
		t.Run(transport, func(t *testing.T) {
			var output *rpc.SelectRoutesOutput
			var err error
			switch transport {
			case "socket":
				client, engine := net.Pipe()
				p := sdk.NewWithConn(proc.Name(), client)
				conn := plugipc.NewPluginConn(engine, engine)
				done := make(chan error, 1)
				go func() {
					request, readErr := conn.ReadRequest(t.Context())
					if readErr == nil {
						host.dispatchPluginRPC(proc, conn, request)
					}
					done <- readErr
				}()
				output, err = p.SelectRoutes(t.Context(), input)
				if closeErr := p.Close(); closeErr != nil {
					t.Error(closeErr)
				}
				if closeErr := engine.Close(); closeErr != nil {
					t.Error(closeErr)
				}
				if readErr := <-done; readErr != nil {
					t.Fatal(readErr)
				}
			case "direct":
				var result json.RawMessage
				result, err = bridge.DispatchRPC(t.Context(), rpc.MethodSelectRoutes, encoded)
				if err == nil {
					err = json.Unmarshal(result, &output)
				}
			case "typed":
				output, err = bridge.SelectRoutes(t.Context(), input)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := output.Validate(input); err != nil {
				t.Fatal(err)
			}
			name, named := reactor.owner.Sender().Process()
			if !named || name != proc.Name() {
				t.Fatalf("selected operation lost process authority: %v", reactor.owner.Sender())
			}
			if reactor.owner.Done() != proc.Done() {
				t.Fatal("selected operation lost exact process incarnation")
			}
		})
	}
	if reactor.calls != 3 {
		t.Fatalf("got %d selected writer calls, want 3", reactor.calls)
	}
	reactor.missing = true
	if _, err := bridge.SelectRoutes(t.Context(), input); err == nil {
		t.Fatal("missing final receipt was accepted")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := bridge.SelectRoutes(canceled, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled typed call: %v", err)
	}
	before := reactor.calls
	invalid := input
	invalid.DestinationSession = 0
	if _, err := bridge.SelectRoutes(t.Context(), invalid); err == nil {
		t.Fatal("unrecorded destination session was admitted")
	}
	if reactor.calls != before {
		t.Fatal("invalid selected input reached writer")
	}
}

// Unsupported output remains a concrete failure until the reactor's selected
// writer exists; registration alone never claims that an output was delivered.
func TestSelectedRouteUnsupportedReactorRefuses(t *testing.T) {
	host := &Server{ctx: t.Context(), reactor: &mockReactor{}}
	proc := nativeOwnerProcess(t, host)
	if _, err := host.selectRoutes(t.Context(), proc, selectedServerInput()); err == nil {
		t.Fatal("unsupported selected output returned success")
	}
}
