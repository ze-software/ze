// Design: docs/architecture/api/process-protocol.md -- accepted-origin route-only JSON entry.
package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAcceptedOriginRegisteredEntryRefusesReplayMarker sends raw JSON through
// the actual registered op, proving mixed markers stop before either command
// handler. Useful routes retain their ID in ingress context; ordinary routes
// and both zero-ID marker forms still dispatch through the same registry.
// MUTATION: remove the OriginAdmissionID guard in updateRouteDispatchCommand;
// the mixed-marker case must then fail its refusal/zero-dispatch oracle.
func TestAcceptedOriginRegisteredEntryRefusesReplayMarker(t *testing.T) {
	family.RegisterTestFamilies()
	deadline := time.Now().Add(20 * time.Second).UnixMilli()
	for _, question := range []struct {
		name       string
		marker     string
		originID   uint64
		explicitID bool
		valid      bool
	}{
		{name: "accepted route", originID: 23, explicitID: true, valid: true},
		{name: "manual route omitted ID", valid: true},
		{name: "manual route zero ID", explicitID: true, valid: true},
		{name: "borr omitted ID", marker: "borr", valid: true},
		{name: "borr zero ID", marker: "borr", explicitID: true, valid: true},
		{name: "eorr omitted ID", marker: "eorr", valid: true},
		{name: "eorr zero ID", marker: "eorr", explicitID: true, valid: true},
		{name: "borr accepted ID", marker: "borr", originID: 23, explicitID: true},
		{name: "eorr accepted ID", marker: "eorr", originID: 23, explicitID: true},
	} {
		t.Run(question.name, func(t *testing.T) {
			values := map[string]any{"peer-selector": "192.0.2.80"}
			wantArgs := "192.0.2.80 update text origin igp add 198.18.7.0/24"
			if question.marker == "" {
				values["command"] = "update text origin igp add 198.18.7.0/24"
			} else {
				values["peer-marker"] = question.marker
				values["marker-family"] = "ipv4/unicast"
				values["destination-owner"] = uint64(17)
				values["destination-session"] = uint64(71)
				values["deadline-ms"] = deadline
				wantArgs = "192.0.2.80 " + question.marker + " ipv4/unicast"
			}
			if question.explicitID {
				values["origin-admission-id"] = question.originID
			}
			raw, err := json.Marshal(values)
			require.NoError(t, err)
			dispatcher := NewDispatcher()
			calls := 0
			handler := func(ctx *CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
				calls++
				require.Equal(t, "192.0.2.80", ctx.Peer)
				require.Equal(t, strings.Fields(wantArgs), args.Tokens())
				require.Equal(t, question.originID, plugin.OriginAdmissionID(ctx.RequestContext))
				if question.marker == "" {
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
				require.Error(t, err, "accepted-origin ID must not be discarded by a registered marker")
				require.Nil(t, output)
				require.Zero(t, calls, "mixed marker reached dispatcher")
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			result, ok := output.(*rpc.UpdateRouteOutput)
			require.True(t, ok)
			if question.marker == "" {
				require.Equal(t, uint32(1), result.Announced)
			}
		})
	}
}
