// Design: docs/guide/graceful-restart.md -- destination-scoped LLGR.
package gr

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"sync"
	"testing"
)

// Same-address destinations carry distinct original owners on both the view
// and serialized Fields rail. Unknown/missing owners cannot borrow capability.
// MUTATION: hasLLGR accepts any recorded neighbor rather than its exact owner.
func TestLLGREgressUsesCapturedDestinationOwner(t *testing.T) {
	const capable uint64 = 1<<53 + 17
	const incapable uint64 = capable + 1
	previous := egressState.Load()
	setEgressState(&egressFilterState{mu: new(sync.Mutex), peerLLGRCaps: map[uint64]*llgrPeerCap{capable: {connectionID: 55}, 0: {connectionID: 55}}})
	t.Cleanup(func() { setEgressState(previous) })
	address := netip.MustParseAddr("192.0.2.81")
	body := ingressTestPlainUpdate()
	for _, row := range []struct {
		name  string
		owner uint64
		want  rpc.FilterAction
	}{
		{"original capable", capable, rpc.FilterAccept},
		{"same-address sibling", incapable, rpc.FilterWithdraw},
		{"unrecorded owner", incapable + 1, rpc.FilterWithdraw},
		{"missing owner", 0, rpc.FilterWithdraw},
	} {
		t.Run(row.name, func(t *testing.T) {
			dest := filterapi.PeerFilterInfo{ConnectionID: 55, PeerOwner: row.owner, Address: address, PeerAS: 65001, LocalAS: 65000}
			session := policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit).WithExportMetadata(map[string]any{"stale": uint8(2)})
			var view policyview.View
			view.Reset(wireu.NewWireUpdate(body, 0), session)
			fields, err := policyview.AppendFields(nil, &view, []string{"connection-id", "peer-owner", "peer-address", "peer-as", "local-as", "ibgp", "stale"})
			require.NoError(t, err)
			var retained struct {
				Owner uint64 `json:"peer-owner"`
			}
			require.NoError(t, json.Unmarshal(fields, &retained))
			require.Equal(t, row.owner, retained.Owner)
			header := rpc.FilterUpdateInput{Filter: llgrEgressFilterName}
			fast := rpc.ViewFilterRoute(&header, llgrViewRoute{view: &view, raw: body})
			framed := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: llgrEgressFilterName, Raw: body, Fields: fields})
			for rail, route := range map[string]*rpc.FilterRoute{"view": &fast, "serialized": &framed} {
				answer, err := evaluateGRFilter(route)
				require.NoError(t, err, rail)
				require.Equal(t, row.want, answer.Action, rail)
				require.Empty(t, answer.Sets, rail)
			}
		})
	}
	for _, bad := range []string{`{"stale":2,"peer-owner":-1}`, `{"stale":2,"peer-owner":1.5}`, `{"stale":2,"peer-owner":"9007199254741009"}`} {
		_, err := fieldsLLGRSession([]byte(bad))
		require.Error(t, err)
	}
}

// Equal AS facts cannot distinguish an RFC 7705 migration session from an
// external peer. Both transports must retain the actual classification.
// MUTATION: replace dest.IsIBGP with dest.PeerAS==dest.LocalAS.
func TestLLGREgressUsesActualMigrationSessionClassification(t *testing.T) {
	previous := egressState.Load()
	setEgressState(&egressFilterState{mu: new(sync.Mutex), peerLLGRCaps: map[uint64]*llgrPeerCap{}})
	t.Cleanup(func() { setEgressState(previous) })
	body := ingressTestPlainUpdate()
	for _, internal := range []bool{true, false} {
		dest := filterapi.PeerFilterInfo{ConnectionID: 55, PeerOwner: 81, Address: netip.MustParseAddr("192.0.2.81"), PeerAS: 65001, LocalAS: 65000, IsIBGP: internal}
		var view policyview.View
		view.Reset(wireu.NewWireUpdate(body, 0), policyview.ExportSession(dest, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit).WithExportMetadata(map[string]any{"stale": uint8(2)}))
		fields, err := policyview.AppendFields(nil, &view, []string{"connection-id", "peer-owner", "ibgp", "stale"})
		require.NoError(t, err)
		var retained struct {
			IBGP *bool `json:"ibgp"`
		}
		require.NoError(t, json.Unmarshal(fields, &retained))
		require.NotNil(t, retained.IBGP)
		require.Equal(t, internal, *retained.IBGP)
		header := rpc.FilterUpdateInput{Filter: llgrEgressFilterName}
		fast := rpc.ViewFilterRoute(&header, llgrViewRoute{view: &view, raw: body})
		framed := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: llgrEgressFilterName, Raw: body, Fields: fields})
		for _, route := range []*rpc.FilterRoute{&fast, &framed} {
			answer, err := evaluateGRFilter(route)
			require.NoError(t, err)
			if internal {
				require.Equal(t, rpc.FilterModify, answer.Action)
				require.Equal(t, []rpc.FilterSet{{Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(`"65535:65281"`)}, {Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("0")}}, answer.Sets)
			} else {
				require.Equal(t, rpc.FilterWithdraw, answer.Action)
				require.Empty(t, answer.Sets)
			}
		}
	}
	for _, fields := range []string{`{"stale":2,"peer-owner":81}`, `{"stale":2,"peer-owner":81,"ibgp":"true"}`} {
		route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: llgrEgressFilterName, Raw: body, Fields: []byte(fields)})
		answer, err := evaluateGRFilter(&route)
		require.Error(t, err)
		require.Nil(t, answer)
	}
	route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: llgrEgressFilterName, Raw: body, Fields: []byte(`{}`)})
	answer, err := evaluateGRFilter(&route)
	require.NoError(t, err)
	require.Equal(t, rpc.FilterAccept, answer.Action, "fresh route does not require destination classification")
	var noPeer policyview.View
	noPeer.Reset(wireu.NewWireUpdate(body, 0), policyview.Session{}.WithExportMetadata(map[string]any{"stale": uint8(2)}))
	header := rpc.FilterUpdateInput{Filter: llgrEgressFilterName}
	missing := rpc.ViewFilterRoute(&header, llgrViewRoute{view: &noPeer, raw: body})
	answer, err = evaluateGRFilter(&missing)
	require.Error(t, err)
	require.Nil(t, answer)
}
