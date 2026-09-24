package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: an unsupported plugin RPC is refused through runtime dispatch.
// PREVENTS: treating an unknown operation as a successful no-op.
func TestPluginRPCUnknownMethodRefused(t *testing.T) {
	t.Parallel()
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "unknown-rpc"})
	result, err := s.dispatchPluginRPCDirect(t.Context(), proc, "ze-plugin-engine:does-not-exist", nil)
	var callErr *rpc.RPCCallError
	require.ErrorAs(t, err, &callErr)
	require.Nil(t, result)
}

// TestEngineOpJSONAndDirectMatch asserts the JSON socket path and the in-process
// Direct path answer with the same payload for a representative op, proving
// both derive from the one entry (AC-2). dispatch-command is used because it
// exercises the shared s.dispatchCommand core through both serve wrappers.
//
// The two carry that payload in different frames, and the difference is the
// transport rather than the answer. The socket writes the record sequence line
// by line (serveEngineOpJSON), and the Direct path is one marshaled value with
// no line to carry a record on, so it projects the same response instead
// (serveEngineOpDirect). What must not drift is the payload inside them.
func TestEngineOpJSONAndDirectMatch(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	d.Register("parity test", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"ok": true}}, nil
	}, "parity test")

	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	proc := process.NewProcess(plugin.PluginConfig{Name: "parity"})

	op := lookupEngineOp(rpc.MethodDispatchCommand)
	require.NotNil(t, op)

	params := []byte(`{"command":"parity test"}`)
	result, err := op.handle(s, t.Context(), proc, params)
	require.NoError(t, err)

	answer, records := result.(*recordAnswer)
	require.True(t, records, "the socket path answers a record sequence, got %T", result)
	var wire bytes.Buffer
	require.NoError(t, answer.write(&wire, 5))

	direct, err := s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodDispatchCommand, params)
	require.NoError(t, err)

	var projected rpc.DispatchCommandOutput
	require.NoError(t, json.Unmarshal(direct, &projected))
	assert.Equal(t, plugin.StatusDone, projected.Status)
	assert.JSONEq(t, `{"ok":true}`, string(projected.Data))

	// One payload, two frames: what the Direct path projects under "data" is
	// what the socket writes as the answer's one item.
	assert.Equal(t, []string{
		"#5 top doc 0: 0:",
		string(rpc.AppendAnswerItem(nil, 5, projected.Data)),
		"#5 end 1 0 0:",
	}, strings.Split(strings.TrimSuffix(wire.String(), "\n"), "\n"))
}

// VALIDATES: registered update-route transports can execute an internal route
// push under real authorization while an anonymous send is refused.
// PREVENTS: missing registration, typed wiring, or trusted caller identity
// silently disabling native route producers on an RBAC-configured engine.
func TestUpdateRouteRPCWithAuthorization(t *testing.T) {
	for _, transport := range []string{"socket", "direct", "typed"} {
		t.Run(transport, func(t *testing.T) {
			d := NewDispatcher()
			d.SetAuthorizer(authz.StoreAuthorizer{Store: authz.NewStore()})
			executed := 0
			d.Register("send bgp", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
				executed++
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "send bgp")
			s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
			s.ctx, s.cancel = context.WithCancel(context.Background())
			defer s.cancel()
			proc := process.NewProcess(plugin.PluginConfig{Name: "routepush"})

			_, err := d.Dispatch(&CommandContext{Server: s}, "send bgp p1 route")
			require.ErrorIs(t, err, ErrUnauthorized)
			require.Zero(t, executed, "an anonymous send must not execute")

			switch transport {
			case "socket":
				client, engine := net.Pipe()
				p := sdk.NewWithConn("routepush", client)
				conn := plugipc.NewPluginConn(engine, engine)
				done := make(chan error, 1)
				t.Cleanup(func() {
					_ = p.Close()
					_ = engine.Close()
					require.NoError(t, <-done)
				})
				go func() {
					request, readErr := conn.ReadRequest(s.ctx)
					if readErr == nil {
						s.dispatchPluginRPC(proc, conn, request)
					}
					done <- readErr
				}()
				ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
				defer cancel()
				_, _, err = p.UpdateRoute(ctx, "p1", "route")
			case "direct":
				_, err = s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodUpdateRoute, json.RawMessage(`{"command":"route","peer-selector":"p1"}`))
			case "typed":
				bridge := rpc.NewDirectBridge()
				for i := range engineOps {
					if engineOps[i].typedWire != nil {
						engineOps[i].typedWire(s, proc, bridge)
					}
				}
				bridge.SetReady()
				_, _, err = bridge.UpdateRouteSel(t.Context(), selector.ParseDefault("p1"), "route", nil)
			}
			require.NoError(t, err)
			require.Equal(t, 1, executed, "the authorized route push must execute exactly once")
		})
	}
}

// TestEngineOpInjectWireRouteJSONFallback exercises the inject-wire-route JSON
// codec fallback end-to-end (AC-6): dispatch the wire method through the Direct
// path, prove opInjectWireRoute unmarshals rpc.InjectWireRouteInput and forwards
// the round-tripped protocol/peer/body to the registered route injector, and that
// an unregistered injector fails closed.
//
// VALIDATES: AC-6 -- inject-wire-route has a working non-typed (JSON codec) path.
// PREVENTS: a silent regression in the InjectWireRouteInput round-trip, the
//
//	GetRouteInjector wiring, or the "no route injector registered" guard.
//
// Not parallel: mutates the process-global route injector (saved/restored).
func TestEngineOpInjectWireRouteJSONFallback(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "inject-json"})

	prev := rpc.GetRouteInjector()
	defer rpc.RegisterRouteInjector(prev)

	// Unregistered: fail closed with an explicit error, no panic.
	rpc.RegisterRouteInjector(nil)
	_, err := s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodInjectWireRoute, json.RawMessage(`{"protocol":"bmp","peer-key":"p","update-body":"AQID"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no route injector registered")

	// Registered: the injector receives the round-tripped values; base64 "AQID"
	// decodes to 0x01 0x02 0x03.
	var gotProto, gotPeer string
	var gotBody []byte
	rpc.RegisterRouteInjector(func(protocol, peerKey string, updateBody []byte) error {
		gotProto, gotPeer, gotBody = protocol, peerKey, updateBody
		return nil
	})
	res, err := s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodInjectWireRoute, json.RawMessage(`{"protocol":"bmp","peer-key":"peer-1","update-body":"AQID"}`))
	require.NoError(t, err)
	assert.Nil(t, res, "inject-wire-route returns no result payload")
	assert.Equal(t, "bmp", gotProto)
	assert.Equal(t, "peer-1", gotPeer)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, gotBody)
}

// TestEngineOpBatchValidateJSONFallback exercises the batch-validate JSON codec
// fallback end-to-end (AC-6): dispatch the wire method through the Direct path,
// prove opBatchValidate unmarshals rpc.BatchValidateInput, forwards the decisions
// to the registered batch validator, and marshals the *BatchValidateResult back;
// and that an unregistered validator fails closed.
//
// VALIDATES: AC-6 -- batch-validate has a working non-typed (JSON codec) path.
// PREVENTS: a silent regression in the BatchValidateInput/Result round-trip, the
//
//	GetBatchValidator wiring, or the "no batch validator registered" guard.
//
// Not parallel: mutates the process-global batch validator (saved/restored).
func TestEngineOpBatchValidateJSONFallback(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "batch-json"})

	prev := rpc.GetBatchValidator()
	defer rpc.RegisterBatchValidator(prev)

	// Unregistered: fail closed with an explicit error.
	rpc.RegisterBatchValidator(nil)
	_, err := s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodBatchValidate, json.RawMessage(`{"decisions":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no batch validator registered")

	// Registered: the validator receives the round-tripped decision and its
	// result marshals back through directResultResponse. ValidationDecision has
	// no JSON tags, so keys are the Go field names.
	var got []rpc.ValidationDecision
	rpc.RegisterBatchValidator(func(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
		got = decisions
		return &rpc.BatchValidateResult{Accepted: 2, Rejected: 1, Early: 0}, nil
	})
	res, err := s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodBatchValidate, json.RawMessage(`{"decisions":[{"Accept":true,"PeerAddr":"10.0.0.1","Family":"ipv4/unicast","Prefix":"10.0.0.0/24","PathID":7,"ValState":1}]}`))
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.True(t, got[0].Accept)
	assert.Equal(t, "10.0.0.1", got[0].PeerAddr)
	assert.Equal(t, uint32(7), got[0].PathID)
	assert.Equal(t, uint8(1), got[0].ValState)

	var out rpc.BatchValidateResult
	require.NoError(t, json.Unmarshal(res, &out))
	assert.Equal(t, 2, out.Accepted)
	assert.Equal(t, 1, out.Rejected)
}

// TestDispatchCommandAlwaysAnswersRecords drives both dispatch ops with a peer
// that named no protocol at Stage 3, and reads the bytes it is answered with.
//
// The peer is the one the negotiation used to send down the other path: a
// process whose capability declaration named nothing. It must now receive the
// head, the item and the terminator, on dispatch-command and on
// dispatch-command-args alike, because one answer has one encoding.
//
// VALIDATES: AC-1 -- a plugin that completes Stage 3 declaring no protocol name
// receives the record answer sequence for dispatch-command and for
// dispatch-command-args.
// PREVENTS: the negotiation branch returning, which would put two encodings of
// one answer back on the wire and let a peer read either.
// Not parallel, and neither are its subtests: the two dispatches share one
// registered target that serves exactly two calls in order, and the verdict on
// that target is read once both have run.
func TestDispatchCommandAlwaysAnswersRecords(t *testing.T) {
	d := NewDispatcher()
	done := registerExecuteCommandTarget(t, d, "request target version", 2,
		func(_ int, _ *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
			return &rpc.ExecuteCommandOutput{
				Status: plugin.StatusDone,
				Data:   json.RawMessage(`{"version":"3"}`),
			}, nil
		})

	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	// A process that has declared nothing. Stage 3 no longer writes a protocol
	// list, so this is the state every caller is in.
	caller := process.NewProcess(plugin.PluginConfig{Name: "unconditional"})

	// The record payload states its own byte count, so the record line is
	// spelled by the shipped appender rather than by a second copy of the
	// grammar here.
	wantLines := []string{
		"#7 top doc 0: 0:",
		string(rpc.AppendAnswerItem(nil, 7, json.RawMessage(`{"version":"3"}`))),
		"#7 end 1 0 0:",
	}

	cases := []struct {
		name   string
		method string
		params string
	}{
		{
			name:   "dispatch-command",
			method: rpc.MethodDispatchCommand,
			params: `{"command":"request target version"}`,
		},
		{
			name:   "dispatch-command-args",
			method: rpc.MethodDispatchCommandArgs,
			params: `{"command":"request target version","args":[]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := lookupEngineOp(tc.method)
			require.NotNil(t, op)

			result, err := op.handle(s, t.Context(), caller, json.RawMessage(tc.params))
			require.NoError(t, err)

			answer, records := result.(*recordAnswer)
			require.True(t, records, "want the record answer, got %T", result)

			var wire bytes.Buffer
			require.NoError(t, answer.write(&wire, 7))
			assert.Equal(t, wantLines, strings.Split(strings.TrimSuffix(wire.String(), "\n"), "\n"))
		})
	}
	require.NoError(t, <-done)
}

// Canceling an SDK route push must release the actual registered command,
// including the typed selector path; timing out only the caller would leave an
// old export writing after its collector or configuration was removed.
func TestUpdateRouteDirectCancellation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprintf("typed=%t", typed), func(t *testing.T) {
			s := &Server{dispatcher: NewDispatcher()}
			s.ctx, s.cancel = context.WithCancel(t.Context())
			defer s.cancel()
			entered := make(chan struct{})
			s.dispatcher.Register("send bgp", func(ctx *CommandContext, _ []string) (*plugin.Response, error) {
				close(entered)
				<-ctx.Context().Done()
				return nil, ctx.Context().Err()
			}, "wait for route operation cancellation")
			proc := process.NewProcess(plugin.PluginConfig{Name: "cancel-route"})
			bridge := rpc.NewDirectBridge()
			bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
				return s.dispatchPluginRPCDirect(ctx, proc, method, params)
			})
			if typed {
				lookupEngineOp(rpc.MethodUpdateRoute).typedWire(s, proc, bridge)
			}
			bridge.SetReady()
			client, engine := net.Pipe()
			p := sdk.NewWithConn(proc.Name(), rpc.NewBridgedConn(client, bridge))
			t.Cleanup(func() {
				require.NoError(t, p.Close())
				require.NoError(t, engine.Close())
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				if typed {
					_, _, err = p.UpdateRouteSel(ctx, selector.All(), "update cancel")
				} else {
					_, _, err = p.UpdateRoute(ctx, "*", "update cancel")
				}
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("SDK route call did not reach the command handler")
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("canceled route call left its engine operation running")
			}
		})
	}
}
