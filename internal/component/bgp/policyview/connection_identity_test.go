// Design: docs/architecture/core-design.md -- read-only physical connection facts.
package policyview

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"
)

// MUTATION: truncate answerConnectionID or register a config insertion fact.
func TestPhysicalConnectionFieldsAreExactReadOnlyRuntimeFacts(t *testing.T) {
	peer := filterapi.PeerFilterInfo{PeerOwner: 17, ConnectionID: ^uint64(0)}
	var view View
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), ExportSession(peer, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit))
	data, err := AppendFields(nil, &view, []string{"peer-owner", "connection-id"})
	require.NoError(t, err)
	require.JSONEq(t, `{"peer-owner":17,"connection-id":18446744073709551615}`, string(data))
	require.Contains(t, string(data), `18446744073709551615`, "compare exact bytes as well as JSON, without float64 identity rounding")
	_, err = SessionConditionHolds([]rpc.FilterSessionFact{{Fact: "connection-id", Values: []string{"18446744073709551615"}}}, peer)
	require.Error(t, err)
	peer.ConnectionID = 0
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), ExportSession(peer, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit))
	data, err = AppendFields(nil, &view, []string{"connection-id"})
	require.NoError(t, err)
	require.Equal(t, `{}`, string(data))
}
