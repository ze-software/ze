// Design: docs/guide/graceful-restart.md -- OPEN capability belongs to one physical connection.
package gr

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"sync"
	"testing"
)

// MUTATION: hasLLGR ignores caps.connectionID.
func TestLLGREgressCannotBorrowPriorConnectionCapability(t *testing.T) {
	const connection uint64 = 1<<53 + 71
	old := egressState.Load()
	setEgressState(&egressFilterState{mu: new(sync.Mutex), peerLLGRCaps: map[uint64]*llgrPeerCap{81: {connectionID: connection}, 82: {connectionID: connection + 2}}})
	t.Cleanup(func() { setEgressState(old) })
	body := ingressTestPlainUpdate()
	for _, row := range []struct {
		owner, connection uint64
		want              rpc.FilterAction
	}{
		{81, connection, rpc.FilterAccept}, {81, connection + 1, rpc.FilterWithdraw},
		{81, 0, rpc.FilterWithdraw}, {83, connection, rpc.FilterWithdraw},
		{82, connection + 2, rpc.FilterAccept},
	} {
		dest := filterapi.PeerFilterInfo{PeerOwner: row.owner, ConnectionID: row.connection, Address: netip.MustParseAddr("192.0.2.81"), PeerAS: 65001, LocalAS: 65000}
		var view policyview.View
		view.Reset(wireu.NewWireUpdate(body, 0), policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit).WithExportMetadata(map[string]any{"stale": uint8(2)}))
		fields, err := policyview.AppendFields(nil, &view, []string{"peer-owner", "connection-id", "ibgp", "stale"})
		require.NoError(t, err)
		header := rpc.FilterUpdateInput{Filter: llgrEgressFilterName}
		fast := rpc.ViewFilterRoute(&header, llgrViewRoute{view: &view, raw: body})
		framed := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: llgrEgressFilterName, Raw: body, Fields: fields})
		for _, route := range []*rpc.FilterRoute{&fast, &framed} {
			answer, err := evaluateGRFilter(route)
			require.NoError(t, err)
			require.Equal(t, row.want, answer.Action)
			require.Empty(t, answer.Sets)
		}
	}
}
