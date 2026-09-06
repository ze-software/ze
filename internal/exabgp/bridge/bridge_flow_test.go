// Design: bridge_flow.go — the translator under test
// Related: test/exabgp-compat/etc/run/api-flow.run, api-flow-merge.run, api-broken-flow.run —
// the three ported ExaBGP scripts whose every flow line is a case below

package bridge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertFlowRouteCompatScripts drives every `flow route` line the three
// ported ExaBGP compatibility scripts send, in both the braced and the flat
// form, and asserts the whole ze command each one answers.
//
// VALIDATES: the braced `match`/`then`/`scope` grammar and the flat keyword
// stream reach the same ze update-text command.
// PREVENTS: api-flow, api-flow-merge and api-broken-flow being refused by a
// translator that reads only the flat family form.
func TestConvertFlowRouteCompatScripts(t *testing.T) {
	tests := []struct {
		name string
		verb string
		body string
		want string
	}{
		{
			name: "api-flow announce with four match components",
			verb: flowVerbAdd,
			body: "flow route { match { source 0.0.0.0/32; destination 0.0.0.0/32; destination-port =3128; protocol tcp; } then { rate-limit 0; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:0] " +
				"nlri ipv4/flow add source-ipv4 0.0.0.0/32 destination-ipv4 0.0.0.0/32 destination-port =3128 protocol tcp",
		},
		{
			name: "api-flow announce with a non-zero rate",
			verb: flowVerbAdd,
			body: "flow route { match { source 255.255.255.255/32; destination 255.255.255.255/32; } then { rate-limit 65535; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:65535] " +
				"nlri ipv4/flow add source-ipv4 255.255.255.255/32 destination-ipv4 255.255.255.255/32",
		},
		{
			name: "api-flow withdraw carries the NLRI and no attribute",
			verb: flowVerbDel,
			body: "flow route { match { source 0.0.0.0/32; destination 0.0.0.0/32; destination-port =3128; protocol tcp; } }",
			want: "send bgp 10.0.0.1 update text " +
				"nlri ipv4/flow del source-ipv4 0.0.0.0/32 destination-ipv4 0.0.0.0/32 destination-port =3128 protocol tcp",
		},
		{
			name: "api-broken-flow announce",
			verb: flowVerbAdd,
			body: "flow route { match { source 170.170.170.170/32; destination 170.170.170.170/32; } then { rate-limit 1; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:1] " +
				"nlri ipv4/flow add source-ipv4 170.170.170.170/32 destination-ipv4 170.170.170.170/32",
		},
		{
			name: "api-flow-merge scope holding three interface sets, closing bracket glued",
			verb: flowVerbAdd,
			body: "flow route { match { source 10.10.10.10/32; } scope { interface-set [ transitive:input-output:1234:10 transitive:input:1234:10 transitive:output:0:0]; } then { discard; } }",
			want: "send bgp 10.0.0.1 update text extended-community " +
				"[interface-set:transitive:input-output:1234:10 interface-set:transitive:input:1234:10 interface-set:transitive:output:0:0 rate-limit:0] " +
				"nlri ipv4/flow add source-ipv4 10.10.10.10/32",
		},
		{
			name: "api-flow-merge scope holding one interface set",
			verb: flowVerbAdd,
			body: "flow route { match { source 6.6.6.6/32; } scope { interface-set [ non-transitive:input:3405770241:1 ]; } then { discard; } }",
			want: "send bgp 10.0.0.1 update text extended-community " +
				"[interface-set:non-transitive:input:3405770241:1 rate-limit:0] " +
				"nlri ipv4/flow add source-ipv4 6.6.6.6/32",
		},
		{
			name: "api-flow-merge then block written before the scope block",
			verb: flowVerbAdd,
			body: "flow route { match { source 8.8.8.8/32; } then { discard; } scope { interface-set [ non-transitive:input:3405770241:1 transitive:output:254:254 ]; } }",
			want: "send bgp 10.0.0.1 update text extended-community " +
				"[interface-set:non-transitive:input:3405770241:1 interface-set:transitive:output:254:254 rate-limit:0] " +
				"nlri ipv4/flow add source-ipv4 8.8.8.8/32",
		},
		{
			name: "api-flow-merge flat form, interface set before the action",
			verb: flowVerbAdd,
			body: "flow route destination 133.130.1.19/32 interface-set [ transitive:input:1234:10 ] discard",
			want: "send bgp 10.0.0.1 update text extended-community " +
				"[interface-set:transitive:input:1234:10 rate-limit:0] " +
				"nlri ipv4/flow add destination-ipv4 133.130.1.19/32",
		},
		{
			name: "api-flow-merge flat form, action before the interface set",
			verb: flowVerbAdd,
			body: "flow route destination 133.130.1.219/32 discard interface-set [ transitive:input:1234:10]",
			want: "send bgp 10.0.0.1 update text extended-community " +
				"[interface-set:transitive:input:1234:10 rate-limit:0] " +
				"nlri ipv4/flow add destination-ipv4 133.130.1.219/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := ConvertFlowRoute("10.0.0.1", "ipv4/flow", tt.verb, tt.body)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, tt.want, got.Text)
		})
	}
}

// TestConvertFlowRouteGrammar drives the parts of the ExaBGP flow grammar the
// compat scripts do not reach.
//
// VALIDATES: the family the body moves, the actions that carry a next-hop, the
// unit words, the community attributes and the optional route name.
// PREVENTS: a route landing in the wrong family, and an action reaching ze
// under ExaBGP's spelling rather than ze's.
func TestConvertFlowRouteGrammar(t *testing.T) {
	tests := []struct {
		name   string
		family string
		body   string
		want   string
	}{
		{
			name:   "an rd makes the route a VPN route",
			family: "ipv4/flow",
			body:   "flow route { rd 65535:65536; match { source 10.0.0.1/32; } then { discard; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:0] " +
				"nlri ipv4/flow-vpn add rd 65535:65536 source-ipv4 10.0.0.1/32",
		},
		{
			name:   "an IPv6 prefix moves the family and the component spelling",
			family: "ipv4/flow",
			body:   "flow route { match { destination 2001:db8::/32; next-header =tcp; } then { rate-limit 9600; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:9600] " +
				"nlri ipv6/flow add destination-ipv6 2001:db8::/32 next-header tcp",
		},
		{
			name:   "a packet rate keeps its unit and a byte rate drops it",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; } then { rate-limit 1000 packets; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:1000:packets] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "an explicit byte unit is dropped, which is how ze spells it",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; } then { rate-limit 9600 bytes; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:9600] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "accept carries no extended community at all",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; } then { accept; } }",
			want:   "send bgp 10.0.0.1 update text nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "a redirect naming a route target becomes the redirect community",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; } then { redirect 30740:12345; } }",
			want: "send bgp 10.0.0.1 update text extended-community [redirect:30740:12345] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "a redirect naming an address sets the next-hop instead",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; } then { redirect 1.2.3.4; } }",
			want: "send bgp 10.0.0.1 update text nhop 1.2.3.4 extended-community [redirect-to-nexthop-draft] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "copy sets the next-hop and the copy action",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; } then { copy 1.2.3.4; } }",
			want: "send bgp 10.0.0.1 update text nhop 1.2.3.4 extended-community [copy-to-nexthop] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "the communities and the ExaBGP packet-rate alias",
			family: "ipv4/flow",
			body:   "flow route { next-hop 1.2.3.4; match { source 10.0.0.1/32; } then { community [ 65000:1 65000:2 ]; large-community 65000:1:2; extended-community [ rate-limit-packets:1000 ]; } }",
			want: "send bgp 10.0.0.1 update text nhop 1.2.3.4 community [65000:1 65000:2] large-community [65000:1:2] " +
				"extended-community [rate-limit:1000:packets] nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "a named route drops the name",
			family: "ipv4/flow",
			body:   "flow route give-me-a-name { match { source 10.0.0.1/32; } then { discard; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:0] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32",
		},
		{
			name:   "a match component holding a bracketed value list loses its brackets",
			family: "ipv4/flow",
			body:   "flow route { match { source 10.0.0.1/32; protocol [ tcp udp ]; } then { discard; } }",
			want: "send bgp 10.0.0.1 update text extended-community [rate-limit:0] " +
				"nlri ipv4/flow add source-ipv4 10.0.0.1/32 protocol tcp udp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := ConvertFlowRoute("10.0.0.1", tt.family, flowVerbAdd, tt.body)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, tt.want, got.Text)
		})
	}
}

// TestConvertFlowRouteRefusesAndNames drives every refusal.
//
// VALIDATES: a body the translator cannot read produces an error naming the
// token that stopped it, and never a command.
// PREVENTS: a dropped match component installing a wider filter than the
// operator wrote, and a dropped action installing a filter with no action at
// all (ai/rules/principles.md).
func TestConvertFlowRouteRefusesAndNames(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		names string
	}{
		{
			name:  "an unknown match keyword",
			body:  "flow route { match { sauce 1.2.3.4/32; } then { discard; } }",
			names: "sauce",
		},
		{
			name:  "an action ze has no update-text spelling for",
			body:  "flow route { match { source 1.2.3.4/32; } then { mark 8; } }",
			names: "mark",
		},
		{
			name:  "a traffic action ze has no update-text spelling for",
			body:  "flow route { match { source 1.2.3.4/32; } then { action sample-terminal; } }",
			names: "action",
		},
		{
			name:  "the IETF redirect ze has no update-text spelling for",
			body:  "flow route { match { source 1.2.3.4/32; } then { redirect-to-nexthop-ietf; } }",
			names: "redirect-to-nexthop-ietf",
		},
		{
			name:  "a keyword whose value is missing at the end of its block",
			body:  "flow route { match { source; } then { discard; } }",
			names: "source",
		},
		{
			name:  "a value list that never closes",
			body:  "flow route { match { source 1.2.3.4/32; protocol [ tcp udp ; } then { discard; } }",
			names: "'['",
		},
		{
			name:  "a block word that opens no block",
			body:  "flow route { match source 1.2.3.4/32; }",
			names: "match",
		},
		{
			name:  "a route stating no match component",
			body:  "flow route { then { discard; } }",
			names: "match component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := ConvertFlowRoute("10.0.0.1", "ipv4/flow", flowVerbAdd, tt.body)
			require.Error(t, err)
			assert.True(t, ok, "a malformed flow route is still a flow route, so the bridge owns it")
			assert.Empty(t, got.Text)
			assert.Contains(t, err.Error(), tt.names)
		})
	}
}

// TestConvertFlowRouteReportsAnotherForm verifies the bool answers the question
// it is named for.
//
// VALIDATES: a body that is not a flow route is handed back untouched, with no
// error, so the caller can try the form it really names.
// PREVENTS: the flow translator claiming a unicast route and refusing it.
func TestConvertFlowRouteReportsAnotherForm(t *testing.T) {
	for _, body := range []string{
		"route 10.0.0.0/24 next-hop 1.2.3.4",
		"ipv4 flow source-ipv4 10.0.1.0/24 protocol =tcp",
		"flow",
		"",
	} {
		got, ok, err := ConvertFlowRoute("10.0.0.1", "ipv4/flow", flowVerbAdd, body)
		require.NoError(t, err, body)
		assert.False(t, ok, body)
		assert.Empty(t, got.Text, body)
	}
}

// TestConvertFlowRouteRefusesACallerMistake verifies the two arguments the
// caller supplies are checked rather than trusted.
//
// VALIDATES: a verb that is neither add nor del, and a family that is not a
// flowspec family, are refused by name.
// PREVENTS: a command that reads as a flow route and encodes as another family.
func TestConvertFlowRouteRefusesACallerMistake(t *testing.T) {
	body := "flow route { match { source 1.2.3.4/32; } then { discard; } }"

	_, ok, err := ConvertFlowRoute("10.0.0.1", "ipv4/flow", "announce", body)
	require.Error(t, err)
	assert.True(t, ok)
	assert.Contains(t, err.Error(), "announce")

	_, ok, err = ConvertFlowRoute("10.0.0.1", "ipv4/unicast", flowVerbAdd, body)
	require.Error(t, err)
	assert.True(t, ok)
	assert.Contains(t, err.Error(), "ipv4/unicast")
}

// TestConvertFlowRouteEveryPeerSelector verifies the selector is written
// through untouched, so a line naming no neighbor reaches every peer.
//
// VALIDATES: the wildcard selector reaches the command.
// PREVENTS: a flow route addressed to a peer named nowhere in the line.
func TestConvertFlowRouteEveryPeerSelector(t *testing.T) {
	got, ok, err := ConvertFlowRoute(bridgeEveryPeer, "ipv4/flow", flowVerbAdd,
		"flow route { match { source 1.2.3.4/32; } then { discard; } }")
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(got.Text, "send bgp * update text "), got.Text)
}
