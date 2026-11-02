// Design: docs/guide/bgp-role.md -- the OTC procedures a role inserts in the peer's filter lists
// Related: otc_filter.go -- the filters, their insertions and the evaluator under test

package role

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// otcForkedRoute builds the route a forked call delivers: the filter name,
// the UPDATE body and the declared fields object.
func otcForkedRoute(filter string, payload []byte, fields string) *sdk.FilterRoute {
	route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: filter, Raw: payload, Fields: []byte(fields)})
	return &route
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

// TestRoleFilterUnknownNameFails proves a call for a filter the plugin does
// not declare fails the route rather than answering accept.
func TestRoleFilterUnknownNameFails(t *testing.T) {
	_, err := evaluateRoleFilter(otcForkedRoute("no-such", nil, ""))
	require.ErrorIs(t, err, errNoSuchRoleFilter)
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
