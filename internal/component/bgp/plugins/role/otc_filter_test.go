// Design: docs/guide/bgp-role.md -- the OTC procedures a role inserts in the peer's filter lists
// Related: otc_filter.go -- the filters, their insertions and the evaluator under test

package role

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// otcForkedRoute builds the route a forked call delivers: the filter name,
// the UPDATE body and the declared fields object.
func otcForkedRoute(filter string, payload []byte, fields string) *sdk.FilterRoute {
	route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: filter, Raw: payload, Fields: []byte(fields)})
	return &route
}

// otcIngressAnswer is the bgp-role:otc-ingress filter's answer for one UPDATE
// received from src, through the forked transport: the fields object carries
// src's session facts under the names otcFilterDecls declares. A route the
// filter keeps answers accept or modify; a leak and a malformed OTC answer
// withdraw, which the engine applies as the withdrawal of the route's
// prefixes (reactor TestWithdrawVerdictReactorRails).
func otcIngressAnswer(t *testing.T, src filterapi.PeerFilterInfo, payload []byte) *sdk.FilterUpdateOutput {
	t.Helper()
	facts := otcFieldsJSON{PeerName: src.Name, PeerGroup: src.GroupName, PeerAS: src.PeerAS, LocalAS: src.LocalAS}
	if src.Address.IsValid() {
		facts.PeerAddress = src.Address.String()
	}
	fields, err := json.Marshal(facts)
	require.NoError(t, err)
	out, err := evaluateRoleFilter(otcForkedRoute(otcIngressFilterName, payload, string(fields)))
	require.NoError(t, err)
	require.NotNil(t, out)
	return out
}

// answerOTC is the OTC value an answer sets on the route, and whether it sets
// one. The engine writes the set as the OTC attribute (policychain
// TestOwnerAttributeNumberSets), so this is the stamp the route carries on.
func answerOTC(t *testing.T, out *sdk.FilterUpdateOutput) (uint32, bool) {
	t.Helper()
	for _, set := range out.Sets {
		if set.Field != fieldOTC.String() {
			continue
		}
		require.Equal(t, rpc.FilterSetActionSet, set.Action)
		asn, err := strconv.ParseUint(string(set.Value), 10, 32)
		require.NoError(t, err, "the otc set holds an AS number")
		return uint32(asn), true
	}
	return 0, false
}

// answerMeta is the route metadata an answer writes: each text field it sets
// (src-role, src-peer-role), unquoted, keyed by field name. A field the answer
// does not set is absent from the map, as an unwritten key is absent from the
// route.
func answerMeta(t *testing.T, out *sdk.FilterUpdateOutput) map[string]any {
	t.Helper()
	meta := map[string]any{}
	for _, set := range out.Sets {
		if set.Field == fieldOTC.String() {
			continue
		}
		require.Equal(t, rpc.FilterSetActionSet, set.Action)
		text, err := strconv.Unquote(string(set.Value))
		require.NoError(t, err, "%s holds a quoted text value", set.Field)
		meta[set.Field] = text
	}
	return meta
}

// TestOTCProceduresAreRequiredRoleMembers proves the declaration half of
// AC-7: both procedures are inserted wherever role import is set, required, with
// the RFC 9234 Section 5 reason verify prints, and served by the plugin.
// Method: read the two declaration functions both registrations use.
func TestOTCProceduresAreRequiredRoleMembers(t *testing.T) {
	served := map[string]bool{}
	for _, decl := range roleFilterDecls() {
		served[rolePluginName+":"+decl.Name] = true
	}
	var required []string
	for _, ins := range roleInsertions() {
		if !ins.Required {
			continue
		}
		required = append(required, ins.Filter+" "+ins.Direction.String())
		assert.Equal(t, []string{"role", "import"}, ins.Config, "the OTC members bind on the declared role")
		assert.Empty(t, ins.Values, "role present, whatever it holds")
		if ins.Direction == rpc.FilterExport {
			assert.Equal(t, rpc.FilterScopeAnyPeer, ins.Scope, "source export policy runs on every destination")
		} else {
			assert.Equal(t, rpc.FilterScopePeer, ins.Scope, "ingress stays on role peers")
		}
		assert.True(t, served[ins.Filter], "%s is a filter the plugin declares", ins.Filter)
		assert.True(t, strings.Contains(ins.Reason,
			`"The operator MUST NOT have the ability to modify the procedures defined in this section."`),
			"the reason quotes RFC 9234 Section 5: %s", ins.Reason)
		assert.Contains(t, ins.Reason, "role")
	}
	assert.ElementsMatch(t, []string{otcIngressReference + " import", otcEgressReference + " export"}, required)
}

// TestOTCForkedIngressVerdicts proves the RFC 9234 Section 5 ingress
// procedures answer through the forked transport, the one the old in-engine
// registration silently skipped: a leak from a customer is withdrawn, a
// malformed OTC is treated as withdraw, a route from a provider without OTC
// gets OTC set to the provider's AS beside its role metadata. Method: the
// fields object and body a forked call carries, into the plugin's evaluator.
func TestOTCForkedIngressVerdicts(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider}, // the peer is our customer
		"10.0.0.2": {role: roleCustomer}, // the peer is our provider
	}, map[string]string{"cust": "10.0.0.1", "prov": "10.0.0.2"})

	nlri := []byte{24, 10, 1, 0}
	fields := func(name, addr string, as uint32) string {
		return `{"peer-name":"` + name + `","peer-address":"` + addr + `","peer-as":` + itoa(as) + `}`
	}

	leak := otcForkedRoute(otcIngressFilterName, buildTestPayload(buildTestAttrs(65009), nlri), fields("cust", "10.0.0.1", 65001))
	out, err := evaluateRoleFilter(leak)
	require.NoError(t, err)
	assert.Equal(t, sdk.FilterWithdraw, out.Action, "a route with OTC from a customer is ineligible")

	malformedAttrs := append(buildTestAttrs(0), 0xC0, otcAttrCode, 3, 0, 0, 1)
	malformed := otcForkedRoute(otcIngressFilterName, buildTestPayload(malformedAttrs, nlri), fields("prov", "10.0.0.2", 65002))
	out, err = evaluateRoleFilter(malformed)
	require.NoError(t, err)
	assert.Equal(t, sdk.FilterWithdraw, out.Action, "an OTC of length 3 is treated as withdraw")

	stamp := otcForkedRoute(otcIngressFilterName, buildTestPayload(buildTestAttrs(0), nlri), fields("prov", "10.0.0.2", 65002))
	out, err = evaluateRoleFilter(stamp)
	require.NoError(t, err)
	require.Equal(t, sdk.FilterModify, out.Action)
	assert.ElementsMatch(t, []rpc.FilterSet{
		{Field: "src-role", Action: rpc.FilterSetActionSet, Value: []byte(`"customer"`)},
		{Field: "src-peer-role", Action: rpc.FilterSetActionSet, Value: []byte(`"provider"`)},
		{Field: "otc", Action: rpc.FilterSetActionSet, Value: []byte(`65002`)},
	}, out.Sets)
}

// TestOTCForkedEgressVerdicts proves the egress procedures through the
// forked transport: a route toward a customer gets OTC set to the local AS,
// a route carrying OTC is not sent to a provider, and a route learned from a
// provider (src-role customer) is not sent to a peer.
func TestOTCForkedEgressVerdicts(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
		"10.0.0.2": {role: roleCustomer},
		"10.0.0.3": {role: rolePeer},
	}, map[string]string{"cust": "10.0.0.1", "prov": "10.0.0.2", "peer": "10.0.0.3"})

	nlri := []byte{24, 10, 1, 0}
	toCustomer := otcForkedRoute(otcEgressFilterName, buildTestPayload(buildTestAttrs(0), nlri),
		`{"peer-name":"cust","peer-address":"10.0.0.1","local-as":64512}`)
	out, err := evaluateRoleFilter(toCustomer)
	require.NoError(t, err)
	require.Equal(t, sdk.FilterModify, out.Action)
	assert.Equal(t, []rpc.FilterSet{{Field: "otc", Action: rpc.FilterSetActionSet, Value: []byte(`64512`)}}, out.Sets)

	toProvider := otcForkedRoute(otcEgressFilterName, buildTestPayload(buildTestAttrs(65009), nlri),
		`{"peer-name":"prov","peer-address":"10.0.0.2","local-as":64512}`)
	out, err = evaluateRoleFilter(toProvider)
	require.NoError(t, err)
	assert.Equal(t, sdk.FilterReject, out.Action, "a route with OTC is not propagated to a provider")

	fromProvider := otcForkedRoute(otcEgressFilterName, buildTestPayload(buildTestAttrs(0), nlri),
		`{"src-role":"customer","peer-name":"peer","peer-address":"10.0.0.3","local-as":64512,`+
			`"source-peer-name":"prov","source-peer-address":"10.0.0.2"}`)
	out, err = evaluateRoleFilter(fromProvider)
	require.NoError(t, err)
	assert.Equal(t, sdk.FilterReject, out.Action, "a provider's route is not sent to a peer")
}

// TestOTCExportSetAppliesToRolelessDestination proves the source's export set
// still decides when the destination has no role config or capability.
// Method: send the same unmarked route through the forked evaluator with an
// excluded unknown destination, an explicitly allowed unknown, and no roles.
func TestOTCExportSetAppliesToRolelessDestination(t *testing.T) {
	clearFilterState(t)
	payload := buildTestPayload(buildTestAttrs(0), []byte{24, 10, 1, 0})
	fields := `{"peer-name":"destination","peer-address":"10.0.0.2","local-as":64512,` +
		`"source-peer-name":"source","source-peer-address":"10.0.0.1"}`
	for _, tc := range []struct {
		name   string
		source *peerRoleConfig
		want   sdk.FilterAction
	}{
		{name: "excluded", source: &peerRoleConfig{role: roleProvider, resolvedExport: []string{roleCustomer}}, want: sdk.FilterReject},
		{name: "explicit-unknown", source: &peerRoleConfig{role: roleProvider, resolvedExport: []string{roleUnknown}}, want: sdk.FilterAccept},
		{name: "no-roles", want: sdk.FilterAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configs := map[string]*peerRoleConfig{}
			if tc.source != nil {
				configs["10.0.0.1"] = tc.source
			}
			setFilterState(configs, map[string]string{"source": "10.0.0.1", "destination": "10.0.0.2"})
			out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
			if err != nil {
				t.Fatal(err)
			}
			if out.Action != tc.want {
				t.Fatalf("roleless destination verdict = %v, want %v", out.Action, tc.want)
			}
			if len(out.Sets) != 0 {
				t.Fatalf("roleless destination unexpectedly modified: %v", out.Sets)
			}
		})
	}
}

// TestRoleFilterUnknownNameFails proves a call for a filter the plugin does
// not declare fails the route rather than answering accept.
func TestRoleFilterUnknownNameFails(t *testing.T) {
	_, err := evaluateRoleFilter(otcForkedRoute("no-such", nil, ""))
	require.ErrorIs(t, err, errNoSuchRoleFilter)
}

// TestRoleFiltersPassConformance proves bgp-role's one evaluator honors the
// fast-path contract (sdk.CheckFilterConformance) for every filter it serves:
// the OTC ingress and egress procedures and the upstream-asn check each answer
// the same twice, the same after the views they read are poisoned, the same
// through the forked transport as through the fast path, and the same
// concurrently as sequentially, which -race judges. Method: routes covering
// each verdict of each filter (withdraw, modify, reject, accept), with the
// role and upstream state the evaluator snapshots installed first.
func TestRoleFiltersPassConformance(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider}, // the peer is our customer
		"10.0.0.2": {role: roleCustomer}, // the peer is our provider
		"10.0.0.3": {role: rolePeer},
	}, map[string]string{"cust": "10.0.0.1", "prov": "10.0.0.2", "peer": "10.0.0.3"})
	t.Cleanup(func() { upstreamASNs.Store(nil) })
	installUpstreamSet(upstreamSet{64500})

	nlri := []byte{24, 10, 1, 0}
	plain := buildTestPayload(buildTestAttrs(0), nlri)
	marked := buildTestPayload(buildTestAttrs(65009), nlri)
	malformed := buildTestPayload(append(buildTestAttrs(0), 0xC0, otcAttrCode, 3, 0, 0, 1), nlri)
	route := func(filter string, payload []byte, fields string) sdk.ConformanceRoute {
		return sdk.ConformanceRoute{Input: rpc.FilterUpdateInput{Filter: filter, Raw: payload, Fields: []byte(fields)}}
	}
	upstream := func(peerAS uint32, fields string) sdk.ConformanceRoute {
		return sdk.ConformanceRoute{Input: rpc.FilterUpdateInput{Filter: upstreamFilterName, PeerAS: peerAS, Fields: []byte(fields)}}
	}
	routes := []sdk.ConformanceRoute{
		route(otcIngressFilterName, marked, `{"peer-name":"cust","peer-address":"10.0.0.1","peer-as":65001}`),
		route(otcIngressFilterName, malformed, `{"peer-name":"prov","peer-address":"10.0.0.2","peer-as":65002}`),
		route(otcIngressFilterName, plain, `{"peer-name":"prov","peer-address":"10.0.0.2","peer-as":65002}`),
		route(otcEgressFilterName, plain, `{"peer-name":"cust","peer-address":"10.0.0.1","local-as":64512}`),
		route(otcEgressFilterName, marked, `{"peer-name":"prov","peer-address":"10.0.0.2","local-as":64512}`),
		route(otcEgressFilterName, plain, `{"src-role":"customer","peer-name":"peer","peer-address":"10.0.0.3",`+
			`"local-as":64512,"source-peer-name":"prov","source-peer-address":"10.0.0.2"}`),
		upstream(64600, `{"as-path":[64600,64500,65000]}`),
		upstream(64600, `{"as-path":[64600,65000]}`),
	}
	if !sdk.CheckFilterConformance(t, rolePluginName, evaluateRoleFilter, routes) {
		t.Fatal("the role evaluator breaks the fast-path contract")
	}
}

func itoa(n uint32) string {
	var b [10]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			return string(b[i:])
		}
	}
}
