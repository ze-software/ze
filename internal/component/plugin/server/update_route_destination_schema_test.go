// Design: docs/architecture/api/process-protocol.md -- bound update route schema and runtime variants.
package server

import (
	"encoding/json"
	"maps"
	"testing"
	"time"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	engineyang "github.com/ze-software/ze/internal/core/ipc/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The real loader resolves the actual embedded module and its native types
// and defaults. Musts are checked as parsed declarations, not evaluated as
// XPath: Ze has no RPC XPath evaluator. The registered-entry question below
// separately proves runtime cross-field refusal at the real JSON handler.
// MUTATION: allow an empty command in the loaded native length constraint.
// MUTATION: drop the parsed bound-positive-deadline declaration.
// MUTATION: serialize an absent SDK command node for a marker.
func TestUpdateRouteLoadedModelDeclaresVariantContract(t *testing.T) {
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-plugin-engine.yang", engineyang.ZePluginEngineYANG))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	operation := schema.GetEntry("ze-plugin-engine").Dir["update-route"]
	require.NotNil(t, operation)
	require.NotNil(t, operation.RPC)
	input := operation.RPC.Input
	require.NotNil(t, input)
	validator := configyang.NewValidator(schema)
	for _, question := range []struct {
		name      string
		good, bad []any
	}{
		{"deadline-ms", []any{"0", "1", "9223372036854775807"}, []any{"-1", "9223372036854775808"}},
		{"destination-owner", []any{"1", "18446744073709551615"}, []any{"0", "-1"}},
		{"destination-session", []any{"1", "18446744073709551615"}, []any{"0", "-1"}},
		{"peer-marker", []any{"borr", "eorr"}, []any{"unknown", "borr extra"}},
		{"marker-family", []any{"ipv4/unicast", "ipv6/unicast"}, []any{"not-a-family", "ipv4/unicast extra"}},
		{"source-policy-epoch", []any{"0", "18446744073709551615"}, []any{"-1", "18446744073709551616"}},
		{"command", []any{"update text origin igp add 198.18.7.0/24"}, []any{""}},
	} {
		t.Run(question.name, func(t *testing.T) {
			leaf := input.Dir[question.name]
			require.NotNil(t, leaf)
			require.NotNil(t, leaf.Type)
			for _, value := range question.good {
				require.NoError(t, validator.ValidateLeafType(question.name, leaf, leaf.Type, value))
			}
			for _, value := range question.bad {
				require.Error(t, validator.ValidateLeafType(question.name, leaf, leaf.Type, value))
			}
		})
	}
	deadline, held := input.Dir["deadline-ms"].SingleDefaultValue()
	require.True(t, held)
	require.Equal(t, "0", deadline)
	for name, want := range map[string][]string{
		"deadline-ms":       {"../command or ../peer-marker", "not(../destination-owner) or . > 0"},
		"destination-owner": {"../destination-session"}, "destination-session": {"../destination-owner"},
		"command":       {"not(../peer-marker)", "not(../marker-family)"},
		"peer-marker":   {"not(../command)", "../marker-family", "../destination-owner and ../destination-session", "not(../source-policy-epoch) or ../source-policy-epoch = 0"},
		"marker-family": {"../peer-marker"},
	} {
		leaf, ok := input.Dir[name].Node.(*gyang.Leaf)
		require.True(t, ok, name)
		var actual []string
		for _, must := range leaf.Must {
			actual = append(actual, must.Name)
		}
		require.ElementsMatch(t, want, actual, name)
	}
	// Actual SDK/DTO encoding must produce a valid marker shape, omitting its
	// absent command, rather than serializing an empty command node.
	raw, err := json.Marshal(rpc.UpdateRouteInput{PeerMarker: "borr", MarkerFamily: "ipv4/unicast", DestinationOwner: 17, DestinationSession: 71, DeadlineMS: 1})
	require.NoError(t, err)
	var encoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &encoded))
	require.NotContains(t, encoded, "command")
	raw, err = json.Marshal(rpc.UpdateRouteInput{Command: "update text origin igp add 198.18.7.0/24"})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &encoded))
	require.Equal(t, `"update text origin igp add 198.18.7.0/24"`, string(encoded["command"]))
}

// Every case reaches the registered update-route JSON op. This is not a
// final BGP socket, capability, grant or live authenticated-process proof.
// Missing payload/default deadline, positive bound deadline and the marker
// epoch distinction are observable before the real dispatcher runs.
// MUTATION: omit the bound deadline refusal at updateRouteDispatchCommand.
// MUTATION: omit its nonzero marker source-policy-epoch refusal.
func TestUpdateRouteRegisteredEntryEnforcesVariantContract(t *testing.T) {
	family.RegisterTestFamilies()
	deadline := time.Now().Add(time.Second * 20).UnixMilli()
	ordinary := map[string]any{"command": "update text origin igp add 198.18.7.0/24", "peer-selector": "192.0.2.80"}
	marker := map[string]any{"peer-marker": "borr", "marker-family": "ipv4/unicast", "destination-owner": uint64(17), "destination-session": uint64(71), "deadline-ms": deadline, "peer-selector": "192.0.2.80"}
	for _, question := range []struct {
		name  string
		base  map[string]any
		edit  func(map[string]any)
		valid bool
		route bool
	}{
		{"legacy ordinary", ordinary, nil, true, true},
		{"bound ordinary", ordinary, func(v map[string]any) {
			v["destination-owner"] = 17
			v["destination-session"] = 71
			v["deadline-ms"] = deadline
		}, true, true},
		{"empty command", ordinary, func(v map[string]any) { v["command"] = "" }, false, true},
		{"missing payload", map[string]any{}, nil, false, false},
		{"bound default deadline", ordinary, func(v map[string]any) { v["destination-owner"] = 17; v["destination-session"] = 71 }, false, true},
		{"bound zero deadline", ordinary, func(v map[string]any) {
			v["destination-owner"] = 17
			v["destination-session"] = 71
			v["deadline-ms"] = 0
		}, false, true},
		{"owner without session", ordinary, func(v map[string]any) { v["destination-owner"] = 17; v["deadline-ms"] = deadline }, false, true},
		{"session without owner", ordinary, func(v map[string]any) { v["destination-session"] = 71; v["deadline-ms"] = deadline }, false, true},
		{"ordinary original epoch", ordinary, func(v map[string]any) { v["source-policy-epoch"] = 4 }, true, true},
		{"bound marker", marker, nil, true, false},
		{"end marker zero epoch", marker, func(v map[string]any) { v["peer-marker"] = "eorr"; v["source-policy-epoch"] = 0 }, true, false},
		{"marker omitted deadline", marker, func(v map[string]any) { delete(v, "deadline-ms") }, false, false},
		{"marker zero deadline", marker, func(v map[string]any) { v["deadline-ms"] = 0 }, false, false},
		{"marker original epoch", marker, func(v map[string]any) { v["source-policy-epoch"] = 4 }, false, false},
		{"two payloads", marker, func(v map[string]any) { v["command"] = ordinary["command"] }, false, false},
		{"marker metadata", marker, func(v map[string]any) { v["meta"] = map[string]any{"manual": true} }, false, false},
		{"marker missing family", marker, func(v map[string]any) { delete(v, "marker-family") }, false, false},
		{"marker invalid family", marker, func(v map[string]any) { v["marker-family"] = "ipv4/unicast extra" }, false, false},
		{"marker unknown value", marker, func(v map[string]any) { v["peer-marker"] = "borr extra" }, false, false},
		{"marker missing tuple", marker, func(v map[string]any) { delete(v, "destination-owner"); delete(v, "destination-session") }, false, false},
		{"marker missing owner", marker, func(v map[string]any) { delete(v, "destination-owner") }, false, false},
		{"marker missing session", marker, func(v map[string]any) { delete(v, "destination-session") }, false, false},
		{"ordinary marker-family", ordinary, func(v map[string]any) { v["marker-family"] = "ipv4/unicast" }, false, true},
	} {
		t.Run(question.name, func(t *testing.T) {
			values := make(map[string]any, len(question.base))
			maps.Copy(values, question.base)
			if question.edit != nil {
				question.edit(values)
			}
			raw, err := json.Marshal(values)
			require.NoError(t, err)
			dispatcher := NewDispatcher()
			calls := 0
			handler := func(ctx *CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
				calls++
				require.Equal(t, "192.0.2.80", ctx.Peer)
				require.NotEmpty(t, args.Tokens())
				if question.route {
					return &plugin.Response{Status: plugin.StatusDone, Data: &plugin.RouteResult{Announced: 1}}, nil
				}
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}
			require.NoError(t, dispatcher.RegisterWithOptions("send bgp", handler, "send test route", RegisterOptions{AcceptsDestinationSession: true}))
			require.NoError(t, dispatcher.RegisterWithOptions("request peer", handler, "request test marker", RegisterOptions{AcceptsDestinationSession: true}))
			host := &Server{ctx: t.Context(), dispatcher: dispatcher, subscriptions: newSubscriptionManager()}
			proc := nativeOwnerProcess(t, host)
			op := lookupEngineOp(rpc.MethodUpdateRoute)
			require.NotNil(t, op)
			output, err := op.handle(host, t.Context(), proc, raw)
			if !question.valid {
				require.Error(t, err)
				require.Zero(t, calls)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.NotNil(t, output)
			result, ok := output.(*rpc.UpdateRouteOutput)
			require.True(t, ok)
			if question.route {
				require.Equal(t, uint32(1), result.Announced)
			}
		})
	}
}
