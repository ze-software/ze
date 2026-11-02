// Design: docs/guide/graceful-restart.md -- the LLGR procedures this plugin inserts in the peers' filter lists
// Related: gr_filter.go -- the filters, their insertions and the evaluator under test

package gr

import (
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	gryang "github.com/ze-software/ze/internal/component/bgp/plugins/gr/yang"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// llgrViewRoute is a fast-path route: the engine's view and the body.
type llgrViewRoute struct {
	view *policyview.View
	raw  []byte
}

func (r llgrViewRoute) Fields() ([]byte, error) { return nil, nil }
func (r llgrViewRoute) Raw() []byte             { return r.raw }
func (r llgrViewRoute) Engine() any             { return r.view }

// llgrCase is one route an LLGR filter judges, given once and delivered on
// both transports: the session the view answers from, and the fields
// object a forked call carries for the same session.
type llgrCase struct {
	name    string
	filter  string
	payload []byte
	session policyview.Session
	fields  string
	want    rpc.FilterAction
	sets    []rpc.FilterSet
}

// judgeBoth runs tc through the evaluator on the fast path and forked, and
// asserts both answer the verdict the case names.
func judgeBoth(t *testing.T, tc llgrCase) {
	t.Helper()
	var view policyview.View
	view.Reset(wireu.NewWireUpdate(tc.payload, 0), tc.session)
	header := rpc.FilterUpdateInput{Filter: tc.filter}
	fast := rpc.ViewFilterRoute(&header, llgrViewRoute{view: &view, raw: tc.payload})
	forked := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: tc.filter, Raw: tc.payload, Fields: []byte(tc.fields)})
	for transport, route := range map[string]*sdk.FilterRoute{"fast path": &fast, "forked": &forked} {
		out, err := evaluateGRFilter(route)
		require.NoError(t, err, "%s: %s", tc.name, transport)
		assert.Equal(t, tc.want, out.Action, "%s: %s", tc.name, transport)
		assert.Equal(t, tc.sets, out.Sets, "%s: %s", tc.name, transport)
	}
}

// TestLLGRProceduresAreRequiredInsertions proves the declaration half of
// AC-13 (spec-filter-wire-6): the ingress step is inserted on the peer whose
// long-lived-stale-time makes Ze advertise LLGR, the egress step on every
// peer's export list while one peer sets it (scope any-peer: keyed on the
// route's source), both required with the RFC 9494 Section 4.3 reason, both
// served by the plugin, and only the egress step opts into the stale
// readvertise rail. Method: read the declaration functions both
// registrations use.
func TestLLGRProceduresAreRequiredInsertions(t *testing.T) {
	decls := map[string]sdk.FilterDecl{}
	for _, decl := range grFilterDecls() {
		decls[grPluginName+":"+decl.Name] = decl
	}
	ingress, egress := grPluginName+":"+llgrIngressFilterName, grPluginName+":"+llgrEgressFilterName
	require.Len(t, grInsertions(), 2)
	for _, ins := range grInsertions() {
		_, served := decls[ins.Filter]
		assert.True(t, served, "%s is a filter the plugin declares", ins.Filter)
		assert.True(t, ins.Required, "%s is required", ins.Filter)
		assert.Equal(t, []string{"session", "capability", "graceful-restart", "long-lived-stale-time"}, ins.Config)
		assert.Empty(t, ins.Values, "the leaf present, whatever it holds")
		assert.True(t, strings.Contains(ins.Reason, `"that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform"`),
			"the reason quotes RFC 9494 Section 4.3: %s", ins.Reason)
		switch ins.Filter {
		case ingress:
			assert.Equal(t, rpc.FilterImport, ins.Direction)
			assert.Equal(t, rpc.FilterScopePeer, ins.Scope, "the ingress step binds on the receiving peer")
		case egress:
			assert.Equal(t, rpc.FilterExport, ins.Direction)
			assert.Equal(t, rpc.FilterScopeAnyPeer, ins.Scope, "the egress step is keyed on the source")
		default:
			t.Errorf("unexpected insertion %s", ins.Filter)
		}
	}
	assert.False(t, decls[ingress].Readvertise, "the ingress step is not on the readvertise rail")
	assert.True(t, decls[egress].Readvertise, "the egress step governs the routes the readvertise rail sends")
}

// TestLLGRIngressVerdicts proves the RFC 9494 Section 4.3 ingress step on
// both transports: a route with LLGR_STALE from a session Ze sent LLGR on
// gets stale level 2; without the capability sent, or without the
// community, it is accepted unmarked. Method: one case list, delivered as
// the engine's view and as a forked fields object.
func TestLLGRIngressVerdicts(t *testing.T) {
	llgrPeer := filterapi.PeerFilterInfo{Address: ingressTestPeer, LLGRAdvertised: true}
	plainPeer := filterapi.PeerFilterInfo{Address: ingressTestPeer}
	stale2 := []rpc.FilterSet{{Field: "stale", Action: rpc.FilterSetActionSet, Value: json.RawMessage("2")}}
	for _, tc := range []llgrCase{
		{"LLGR_STALE from an LLGR session", llgrIngressFilterName, ingressTestUpdate(ingressLLGRStale),
			policyview.ImportSession(llgrPeer), `{"llgr-advertised":true}`, rpc.FilterModify, stale2},
		{"LLGR_STALE, LLGR not sent", llgrIngressFilterName, ingressTestUpdate(ingressLLGRStale),
			policyview.ImportSession(plainPeer), `{"llgr-advertised":false}`, rpc.FilterAccept, nil},
		{"LLGR session, other community", llgrIngressFilterName, ingressTestUpdate(ingressNoExport),
			policyview.ImportSession(llgrPeer), `{"llgr-advertised":true}`, rpc.FilterAccept, nil},
	} {
		judgeBoth(t, tc)
	}
}

// TestLLGREgressVerdicts proves the RFC 9494 Section 4.3 and 4.6 egress step
// on both transports, with the plugin's own capability state: a long-lived
// stale route is withdrawn toward an eBGP destination that did not send
// LLGR, sent with NO_EXPORT and LOCAL_PREF 0 toward an iBGP one, and sent
// unchanged toward one that did; a GR-stale (level 1) or fresh route is
// accepted; a rebuilt copy carries no level and is judged by its source's
// llgr-advertised and its LLGR_STALE community. Method: one case list,
// delivered as the engine's view and as a forked fields object.
func TestLLGREgressVerdicts(t *testing.T) {
	setEgressState(newTestEgressState(map[string]*llgrPeerCap{"10.0.0.9": {}}))
	t.Cleanup(func() { setEgressState(nil) })

	ebgp := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65001, LocalAS: 65000}
	ibgp := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.3"), PeerAS: 65000, LocalAS: 65000}
	llgrDest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.9"), PeerAS: 65001, LocalAS: 65000}
	source := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), LLGRAdvertised: true}
	silent := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	stale := func(dest filterapi.PeerFilterInfo, level uint8) policyview.Session {
		return policyview.ExportSession(dest, source, true).WithExportMetadata(map[string]any{"stale": level})
	}
	rebuilt := func(dest, src filterapi.PeerFilterInfo) policyview.Session {
		return policyview.ExportSession(dest, src, true).WithExportMetadata(map[string]any{filterapi.MetaKeyRebuilt: true})
	}
	fields := func(dest filterapi.PeerFilterInfo, extra string) string {
		return `{"peer-address":"` + dest.Address.String() + `","peer-as":` + strconv.FormatUint(uint64(dest.PeerAS), 10) +
			`,"local-as":` + strconv.FormatUint(uint64(dest.LocalAS), 10) + extra + `}`
	}
	depreference := []rpc.FilterSet{
		{Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(`"65535:65281"`)},
		{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("0")},
	}
	plain, marked := ingressTestPlainUpdate(), ingressTestUpdate(ingressLLGRStale)
	for _, tc := range []llgrCase{
		{"stale toward eBGP non-LLGR", llgrEgressFilterName, plain, stale(ebgp, 2),
			fields(ebgp, `,"stale":2`), rpc.FilterWithdraw, nil},
		{"stale toward iBGP non-LLGR", llgrEgressFilterName, plain, stale(ibgp, 2),
			fields(ibgp, `,"stale":2`), rpc.FilterModify, depreference},
		{"stale toward an LLGR peer", llgrEgressFilterName, plain, stale(llgrDest, 2),
			fields(llgrDest, `,"stale":2`), rpc.FilterAccept, nil},
		{"GR-stale level 1", llgrEgressFilterName, plain, stale(ebgp, 1),
			fields(ebgp, `,"stale":1`), rpc.FilterAccept, nil},
		{"fresh live route", llgrEgressFilterName, marked, policyview.ExportSession(ebgp, source, true),
			fields(ebgp, ""), rpc.FilterAccept, nil},
		{"rebuilt LLGR_STALE from an LLGR source", llgrEgressFilterName, marked, rebuilt(ebgp, source),
			fields(ebgp, `,"source-llgr-advertised":true`), rpc.FilterWithdraw, nil},
		{"rebuilt LLGR_STALE, source did not send LLGR", llgrEgressFilterName, marked, rebuilt(ebgp, silent),
			fields(ebgp, `,"source-llgr-advertised":false`), rpc.FilterAccept, nil},
		{"rebuilt without LLGR_STALE", llgrEgressFilterName, plain, rebuilt(ebgp, source),
			fields(ebgp, `,"source-llgr-advertised":true`), rpc.FilterAccept, nil},
	} {
		judgeBoth(t, tc)
	}
}

// TestLLGRFiltersPassConformance proves the evaluator honors the fast-path
// contract (sdk.CheckFilterConformance) over the forked transport: the same
// answer twice, concurrent calls equal to sequential ones under -race, and
// an unknown filter name refused rather than accepted.
func TestLLGRFiltersPassConformance(t *testing.T) {
	setEgressState(newTestEgressState(map[string]*llgrPeerCap{}))
	t.Cleanup(func() { setEgressState(nil) })
	route := func(filter string, payload []byte, fields string) sdk.ConformanceRoute {
		return sdk.ConformanceRoute{Input: rpc.FilterUpdateInput{Filter: filter, Raw: payload, Fields: []byte(fields)}}
	}
	routes := []sdk.ConformanceRoute{
		route(llgrIngressFilterName, ingressTestUpdate(ingressLLGRStale), `{"llgr-advertised":true}`),
		route(llgrEgressFilterName, ingressTestPlainUpdate(), `{"peer-address":"10.0.0.2","peer-as":65001,"local-as":65000,"stale":2}`),
		route(llgrEgressFilterName, ingressTestPlainUpdate(), `{"peer-address":"10.0.0.3","peer-as":65000,"local-as":65000,"stale":2}`),
	}
	if !sdk.CheckFilterConformance(t, grPluginName, evaluateGRFilter, routes) {
		t.Fatal("the LLGR evaluator breaks the fast-path contract")
	}
	unknown := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "nope"})
	_, err := evaluateGRFilter(&unknown)
	assert.ErrorIs(t, err, errNoSuchGRFilter)
}

// TestLLGRInsertionPathsAreGRConfig proves the condition both insertions name
// is a compiled node bgp-gr's own module adds (an augment under bgp's session
// capability), so Stage 1 accepts them and the members land only where
// graceful-restart config is set. Method: the judge the daemon runs at
// Stage 1 and config load, over gr's YANG and declarations.
func TestLLGRInsertionPathsAreGRConfig(t *testing.T) {
	require.NoError(t, config.CheckFilterInsertionPaths(grPluginName, gryang.ZeGracefulRestartYANG, grInsertions()))
}
