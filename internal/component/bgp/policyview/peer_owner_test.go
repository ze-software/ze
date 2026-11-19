// Design: docs/architecture/core-design.md -- read-only runtime peer owner.
package policyview

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"strconv"
	"testing"
)

// The runtime owner crosses generic Fields exactly, while configuration-only
// facts remain absent and cannot become filter-insertion conditions.
// MUTATION: truncate answerPeerOwner's number or register it as a peer condition.
func TestRuntimePeerOwnerViewKeepsExactReadOnlyIdentity(t *testing.T) {
	const owner uint64 = 1<<53 + 17
	peer := filterapi.PeerFilterInfo{PeerOwner: owner, PeerAS: 65001, LocalAS: 65000}
	var view View
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), ExportSession(peer, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit))
	reader, err := ReaderFor(FieldPeerOwner)
	require.NoError(t, err)
	value, err := view.Read(reader)
	require.NoError(t, err)
	number, present := value.Number()
	require.True(t, present)
	require.Equal(t, owner, number)
	data, err := AppendFields(nil, &view, []string{FieldPeerOwner.String()})
	require.NoError(t, err)
	require.Equal(t, `{"peer-owner":`+strconv.FormatUint(owner, 10)+`}`, string(data))
	holds, err := SessionConditionHolds([]rpc.FilterSessionFact{{Fact: "peer-owner", Values: []string{strconv.FormatUint(owner, 10)}}}, peer)
	require.ErrorContains(t, err, "not a registered peer fact")
	require.False(t, holds)
	peer.PeerOwner = 0
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), ExportSession(peer, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit))
	data, err = AppendFields(nil, &view, []string{FieldPeerOwner.String()})
	require.NoError(t, err)
	require.Equal(t, `{}`, string(data), "configuration-only facts cannot invent runtime identity")
}
