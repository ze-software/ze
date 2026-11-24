// Design: docs/architecture/api/process-protocol.md -- selected marker transport parity.
package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type selectedRefreshReactor struct {
	*mockReactor
	owner   plugin.SelectedOwner
	calls   int
	missing bool
}

func (reactor *selectedRefreshReactor) SelectRefresh(ctx context.Context, request *plugin.SelectedRefresh, owner plugin.SelectedOwner) (*rpc.SelectRefreshOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reactor.owner = owner
	reactor.calls++
	if reactor.missing {
		return nil, nil //nolint:nilnil // Deliberately malformed final receipt under test.
	}
	input := request.Input()
	return &rpc.SelectRefreshOutput{Destination: input.Destination, PeerOwner: input.PeerOwner,
		DestinationSession: input.DestinationSession, Family: input.Family, Action: input.Action,
		Outcome: rpc.SelectedRefreshWritten}, nil
}

// Real process wiring binds one exact authenticated owner on native, direct
// JSON and socket RPC paths; the fake models final completion only.
func TestSelectedRefreshBoundaryTransports(t *testing.T) {
	family.RegisterTestFamilies()
	reactor := &selectedRefreshReactor{mockReactor: &mockReactor{}}
	host := &Server{ctx: t.Context(), reactor: reactor}
	proc := nativeOwnerProcess(t, host)
	bridge := proc.Bridge()
	bridge.SetReady()
	input := rpc.SelectRefreshInput{Destination: "192.0.2.7", PeerOwner: 17,
		DestinationSession: 71, Family: "ipv4/unicast", Action: rpc.SelectedRefreshBegin,
		DeadlineMS: time.Now().Add(5 * time.Second).UnixMilli()}
	if err := host.UpdateDeliveryGraphWithEndpoints(graphNS, []DeliveryPeer{{Addr: input.Destination, Name: "dst", Path: "/" + graphNS + "/entry/dst"}}, []BusParticipantInput{{
		Path:         plugin.EndpointPath(proc.Name()),
		EndpointName: proc.Name(),
		Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
	}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range []string{"socket", "direct", "native"} {
		t.Run(transport, func(t *testing.T) {
			var output *rpc.SelectRefreshOutput
			var err error
			switch transport {
			case "socket":
				client, engine := net.Pipe()
				p := sdk.NewWithConn(proc.Name(), client)
				connection := plugipc.NewPluginConn(engine, engine)
				done := make(chan error, 1)
				go func() {
					request, readErr := connection.ReadRequest(t.Context())
					if readErr == nil {
						host.dispatchPluginRPC(proc, connection, request)
					}
					done <- readErr
				}()
				output, err = p.SelectRefresh(t.Context(), input)
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
				result, err = bridge.DispatchRPC(t.Context(), rpc.MethodSelectRefresh, encoded)
				if err == nil {
					err = json.Unmarshal(result, &output)
				}
			case "native":
				var result any
				var handled bool
				result, handled, err = bridge.DispatchNative(t.Context(), rpc.MethodSelectRefresh, input)
				if !handled {
					t.Fatal("selected refresh was not natively handled")
				}
				if err == nil {
					var valid bool
					output, valid = result.(*rpc.SelectRefreshOutput)
					if !valid {
						t.Fatalf("native result has wrong type %T", result)
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := output.Validate(input); err != nil {
				t.Fatal(err)
			}
			if reactor.owner.Done() != proc.Done() {
				t.Fatal("captured another process incarnation")
			}
			name, named := reactor.owner.Sender().Process()
			if !named {
				t.Fatal("lost authenticated process")
			}
			if name != proc.Name() {
				t.Fatal("captured another process name")
			}
		})
	}
	if reactor.calls != 3 {
		t.Fatalf("got %d evaluator calls, want 3", reactor.calls)
	}
	before := reactor.calls
	if _, _, err := bridge.DispatchNative(t.Context(), rpc.MethodSelectRefresh, (*rpc.SelectRefreshInput)(nil)); err == nil {
		t.Fatal("typed nil raw request reached the evaluator")
	}
	if reactor.calls != before {
		t.Fatal("invalid native input changed the evaluator")
	}
	reactor.missing = true
	if _, _, err := bridge.DispatchNative(t.Context(), rpc.MethodSelectRefresh, input); err == nil {
		t.Fatal("missing final receipt was accepted")
	}
}
