// Design: docs/architecture/plugin/rib-storage-design.md -- original FlowSpec catch-up.
package ribevents

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// Known lag never becomes a negative path verdict on either provider rail. An
// unrelated instance, a larger foreign message and malformed tail grant none.
func TestFlowSpecMinimumKeepsWholePendingOnLocalAndCommandRails(t *testing.T) {
	family.RegisterTestFamilies()
	fs := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	previous := flowSpecLookup.Load()
	defer flowSpecLookup.Store(previous)
	priorDispatch := gateDispatch.Load()
	defer gateDispatch.Store(priorDispatch)
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "command-json"}[remote], func(t *testing.T) {
			key := ValidationRoute{Peer: netip.MustParseAddr("192.0.2.7"), PeerOwner: 7, ConnectionID: 9, Family: fs}
			processed := uint64(10)
			lookups := 0
			progress := func(route ValidationRoute, min uint64) (bool, error) {
				if route.Peer != key.Peer || route.PeerOwner != 7 || route.ConnectionID != 9 {
					return false, errors.New("unknown original physical actor")
				}
				return processed >= min, nil
			}
			path := func(route ValidationRoute) (FlowSpecPath, bool) {
				lookups++
				return FlowSpecPath{MsgID: 10, SourceReceipt: 31, SourcePolicyEpoch: 5}, route == key
			}
			eligible := func(ValidationRoute, uint64) bool { return true }
			present := func(ValidationRoute) bool { return true }
			routes := func() []ValidationRoute { return []ValidationRoute{key} }
			if remote {
				flowSpecLookup.Store(remoteFlowSpec("request test original flowspec"))
				fn := GateDispatch(func(_ context.Context, command string, args ...string) (string, json.RawMessage, error) {
					require.Equal(t, "request test original flowspec", command)
					answer, err := AnswerFlowSpecFor(args[0], eligible, present, path, routes, nil, progress)
					raw, encodeErr := json.Marshal(answer)
					require.NoError(t, encodeErr)
					return gateStatusDone, raw, err
				})
				gateDispatch.Store(&fn)
			} else {
				RegisterFlowSpecLookup(eligible, present, path, routes, progress)
			}
			input := FlowSpecRequest{Queries: []FlowSpecQuery{{Operation: FlowSpecPathOf, Route: key, AfterMsgID: 11}}, Routes: true}
			answer, err := LookupFlowSpecRequest(t.Context(), input)
			require.NoError(t, err)
			require.True(t, answer.Pending)
			require.Empty(t, answer.Results)
			require.Empty(t, answer.Routes)
			require.Zero(t, lookups)
			malformed := input
			malformed.Queries = append(append([]FlowSpecQuery(nil), input.Queries...), FlowSpecQuery{Operation: FlowSpecPathOf, Route: ValidationRoute{Peer: key.Peer, PeerOwner: 8, Family: fs}, AfterMsgID: 1000})
			_, err = LookupFlowSpecRequest(t.Context(), malformed)
			require.Error(t, err, "lagged first item cannot mask malformed original tail")
			require.Zero(t, lookups)
			unknown := input
			unknown.Queries = append([]FlowSpecQuery(nil), input.Queries...)
			unknown.Queries[0].Route.ConnectionID = 99
			_, err = LookupFlowSpecRequest(t.Context(), unknown)
			require.Error(t, err)
			processed = 11
			answer, err = LookupFlowSpecRequest(t.Context(), input)
			require.NoError(t, err)
			require.False(t, answer.Pending)
			require.Len(t, answer.Results, 1)
			require.True(t, answer.Results[0].Verdict)
			require.Equal(t, uint64(31), answer.Results[0].Path.SourceReceipt)
			require.Equal(t, []ValidationRoute{key}, answer.Routes)
			canceled, cancel := context.WithCancel(t.Context())
			cancel()
			_, err = LookupFlowSpecRequest(canceled, input)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}
