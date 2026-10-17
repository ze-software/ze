package filter_irr

import (
	"slices"
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: an UPDATE whose filter text carries more than one
// "nlri <family> <op> ..." block (announce and withdraw in one UPDATE, or
// MP_REACH beside MP_UNREACH: reactor filter_format.go emits one block each)
// is judged block by block, and a withdrawal is never filtered.
// PREVENTS: the second block's "nlri" keyword failing to parse as a prefix, so
// the whole UPDATE was rejected as parse-error, including its valid route.
func TestFilterIRRMultiBlockVerdict(t *testing.T) {
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), newFakeReadmitEngine(), time.Hour)
	plug.refreshASN(65001)

	tests := []struct {
		name   string
		update string
		action sdk.FilterAction
		delta  string
	}{
		{
			name:   "accepted announce beside a withdrawal",
			update: "origin igp nlri add 10.0.0.0/24 nlri del 192.0.2.0/24",
			action: sdk.FilterAccept,
		},
		{
			name:   "mixed announce beside a withdrawal keeps the withdrawal whole",
			update: "origin igp nlri add 10.0.0.0/24 192.0.2.0/24 nlri del 198.51.100.0/24",
			action: sdk.FilterModify,
			delta:  "nlri add 10.0.0.0/24 nlri del 198.51.100.0/24",
		},
		{
			// The engine refuses a changed block of a non-CIDR family
			// (reactor filter_nlri_delta.go, refuseNonCIDR), so a modify
			// cannot drop the rejected FlowSpec destination.
			name:   "legacy announce beside a rejected non-CIDR MP announce fails closed",
			update: "origin igp nlri add 10.0.0.0/24 mp ipv4/flow add 192.0.2.0/24",
			action: sdk.FilterReject,
		},
		{
			// The engine rewrites an MP_REACH_NLRI of a CIDR family from its
			// own block (reactor filter_nlri_delta.go), so the block is left
			// out and the MP_REACH_NLRI dropped.
			name:   "legacy announce beside a rejected CIDR MP announce is a modify",
			update: "origin igp nlri add 10.0.0.0/24 mp ipv6/unicast add 2001:db8::/32",
			action: sdk.FilterModify,
			delta:  "nlri add 10.0.0.0/24",
		},
		{
			name:   "a mixed CIDR MP announce keeps its admitted prefix",
			update: "origin igp nlri add 10.0.0.0/24 mp ipv4/multicast add 192.0.2.0/24 10.0.0.0/24",
			action: sdk.FilterModify,
			delta:  "nlri add 10.0.0.0/24 mp ipv4/multicast add 10.0.0.0/24",
		},
		{
			// A labeled (RFC 8277) section is rewritten with its label stacks
			// (reactor filter_nlri_delta.go, subsetNLRI).
			name:   "a mixed labeled MP announce keeps its admitted prefix",
			update: "origin igp mp ipv4/mpls-label add 192.0.2.0/24 10.0.0.0/24",
			action: sdk.FilterModify,
			delta:  "mp ipv4/mpls-label add 10.0.0.0/24",
		},
		{
			// A non-CIDR family renders as a block with no prefix (reactor
			// filter_format.go, appendMPBlock); left out of the answer it reads
			// as a changed block, which the engine refuses whole.
			name:   "a marker-only MP block is echoed unchanged",
			update: "origin igp nlri add 10.0.0.0/24 192.0.2.0/24 mp l2vpn/evpn add",
			action: sdk.FilterModify,
			delta:  "nlri add 10.0.0.0/24 mp l2vpn/evpn add",
		},
		{
			name:   "a mixed legacy announce beside an MP withdrawal is a modify",
			update: "origin igp nlri add 10.0.0.0/24 192.0.2.0/24 nlri del 198.51.100.0/24 mp ipv6/unicast del 2001:db8::/32",
			action: sdk.FilterModify,
			delta:  "nlri add 10.0.0.0/24 nlri del 198.51.100.0/24 mp ipv6/unicast del 2001:db8::/32",
		},
		{
			// RFC 4271 Section 9: the withdrawal SHALL remove the route from
			// the Adj-RIB-In, so rejecting the announcement keeps it.
			name:   "rejected announce beside a withdrawal keeps the withdrawal",
			update: "origin igp nlri add 192.0.2.0/24 nlri del 10.0.0.0/24",
			action: sdk.FilterModify,
			delta:  "nlri del 10.0.0.0/24",
		},
		{
			// The engine refuses a changed block of a non-CIDR family
			// (reactor filter_nlri_delta.go, refuseNonCIDR), so this one stays
			// a whole reject.
			name:   "rejected non-CIDR MP announce beside a withdrawal fails closed",
			update: "origin igp mp ipv4/flow add 192.0.2.0/24 nlri del 10.0.0.0/24",
			action: sdk.FilterReject,
		},
		{
			name:   "rejected CIDR MP announce beside a withdrawal keeps the withdrawal",
			update: "origin igp mp ipv6/unicast add 2001:db8::/32 nlri del 10.0.0.0/24",
			action: sdk.FilterModify,
			delta:  "nlri del 10.0.0.0/24",
		},
		{
			name:   "a withdrawal alone is not filtered",
			update: "nlri del 192.0.2.0/24",
			action: sdk.FilterAccept,
		},
		{
			name:   "a malformed prefix in the second block still fails closed",
			update: "nlri add 10.0.0.0/24 nlri del not-a-prefix",
			action: sdk.FilterReject,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
				Filter: "bgp-filter-irr:65001",
				Peer:   readmitTestPeer,
				Update: tt.update,
			})
			if out.Action != tt.action {
				t.Fatalf("action = %v, want %v", out.Action, tt.action)
			}
			if out.Update != tt.delta {
				t.Errorf("delta = %q, want %q", out.Update, tt.delta)
			}
		})
	}
}

// VALIDATES: configure names every IRR-filtered peer whose config does not let
// bgp-filter-irr send a ROUTE-REFRESH, at peer or group level, and no other.
// PREVENTS: re-admission failing in silence on every such peer: without the
// grant the engine refuses each refresh the filter asks for.
func TestPeersMissingRefreshGrant(t *testing.T) {
	session := func() map[string]any {
		return map[string]any{
			"asn": map[string]any{"remote": float64(65001)},
			"irr": map[string]any{"as-set": "AS-TEST"},
		}
	}
	grant := map[string]any{"process": map[string]any{
		pluginName: map[string]any{"send": []any{"refresh"}},
	}}
	otherGrant := map[string]any{"process": map[string]any{
		pluginName: map[string]any{"send": []any{"update"}},
	}}
	cfg := parseIRRConfig(map[string]any{
		"peer": map[string]any{
			"granted":  map[string]any{"session": session(), "attach": grant},
			"bare":     map[string]any{"session": session()},
			"wrongset": map[string]any{"session": session(), "attach": otherGrant},
			"unfiltered": map[string]any{"session": map[string]any{
				"asn": map[string]any{"remote": float64(65002)},
			}},
		},
		"group": map[string]any{
			"g": map[string]any{
				"attach": grant,
				"peer":   map[string]any{"member": map[string]any{"session": session()}},
			},
		},
	})
	got := peersMissingRefreshGrant(cfg)
	slices.Sort(got)
	want := []string{"bare", "wrongset"}
	if !slices.Equal(got, want) {
		t.Errorf("peers missing the refresh grant = %v, want %v", got, want)
	}
}

// VALIDATES: an ASN with no list yet still lets the withdrawals of an UPDATE
// through. RFC 4271 Section 9: "If the UPDATE message contains a non-empty
// WITHDRAWN ROUTES field, the previously advertised routes, whose destinations
// (expressed as IP prefixes) are contained in this field, SHALL be removed from
// the Adj-RIB-In."
// PREVENTS: the no-prefix-list reject dropping a withdrawal with the UPDATE, so
// the withdrawn route stayed in the Adj-RIB-In.
// METHOD: the plugin is enrolled for AS65001 and its list was never resolved.
func TestFilterIRRNoListKeepsWithdrawals(t *testing.T) {
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), newFakeReadmitEngine(), time.Hour)

	tests := []struct {
		name   string
		update string
		action sdk.FilterAction
		delta  string
	}{
		{
			name:   "a withdrawal alone is accepted",
			update: "nlri del 192.0.2.0/24",
			action: sdk.FilterAccept,
		},
		{
			name:   "an MP withdrawal alone is accepted",
			update: "mp ipv6/unicast del 2001:db8::/32",
			action: sdk.FilterAccept,
		},
		{
			name:   "a legacy announce beside a withdrawal keeps the withdrawal",
			update: "origin igp nlri add 10.0.0.0/24 nlri del 198.51.100.0/24",
			action: sdk.FilterModify,
			delta:  "nlri del 198.51.100.0/24",
		},
		{
			name:   "an announce alone is rejected",
			update: "origin igp nlri add 10.0.0.0/24",
			action: sdk.FilterReject,
		},
		{
			name:   "a non-CIDR MP announce beside a withdrawal fails closed",
			update: "origin igp mp ipv4/flow add 192.0.2.0/24 nlri del 198.51.100.0/24",
			action: sdk.FilterReject,
		},
		{
			name:   "a CIDR MP announce beside a withdrawal keeps the withdrawal",
			update: "origin igp mp ipv6/unicast add 2001:db8::/32 nlri del 198.51.100.0/24",
			action: sdk.FilterModify,
			delta:  "nlri del 198.51.100.0/24",
		},
		{
			name:   "a malformed withdrawal fails closed",
			update: "nlri del not-a-prefix",
			action: sdk.FilterReject,
		},
		{
			name:   "an UPDATE with no prefix is rejected",
			update: "origin igp",
			action: sdk.FilterReject,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
				Filter: "bgp-filter-irr:65001",
				Peer:   readmitTestPeer,
				Update: tt.update,
			})
			if out.Action != tt.action {
				t.Fatalf("action = %v, want %v", out.Action, tt.action)
			}
			if out.Update != tt.delta {
				t.Errorf("delta = %q, want %q", out.Update, tt.delta)
			}
		})
	}
}

// VALIDATES: the refresh grant is read the way the engine reads it. `send [ * ]`
// grants refresh, and the peer's own binding decides when it states a `send`
// list, because ResolveBGPTree does not merge `send` across levels
// (bgp/config/resolve.go cumulativePaths); a peer binding without one inherits
// the group's.
// PREVENTS: a peer granted by `*` warned as missing the grant, and a peer whose
// own `send` list replaces the group's grant read as granted.
func TestRefreshGrantPeerBindingDecides(t *testing.T) {
	session := func() map[string]any {
		return map[string]any{
			"asn": map[string]any{"remote": float64(65001)},
			"irr": map[string]any{"as-set": "AS-TEST"},
		}
	}
	binding := func(fields map[string]any) map[string]any {
		return map[string]any{"process": map[string]any{pluginName: fields}}
	}
	grant := binding(map[string]any{"send": []any{"refresh"}})
	cfg := parseIRRConfig(map[string]any{
		"peer": map[string]any{
			"wildcard": map[string]any{"session": session(), "attach": binding(map[string]any{"send": []any{"*"}})},
		},
		"group": map[string]any{
			"g": map[string]any{
				"attach": grant,
				"peer": map[string]any{
					"overrides": map[string]any{"session": session(), "attach": binding(map[string]any{"send": []any{"update"}})},
					"inherits":  map[string]any{"session": session(), "attach": binding(map[string]any{"receive": []any{"update"}})},
				},
			},
			"wild": map[string]any{
				"attach": binding(map[string]any{"send": []any{"*"}}),
				"peer":   map[string]any{"wildmember": map[string]any{"session": session()}},
			},
		},
	})
	got := peersMissingRefreshGrant(cfg)
	want := []string{"overrides"}
	if !slices.Equal(got, want) {
		t.Errorf("peers missing the refresh grant = %v, want %v", got, want)
	}
}

// VALIDATES: configure names every IRR-filtered peer to which ze does not
// advertise Route Refresh, reading the peer's capability and then its group's.
// PREVENTS: re-admission failing on such a peer with nothing said until a
// list changes: ze advertises Route Refresh only when the config enables it
// (reactor config_capabilities.go), and RFC 2918 Section 4 lets a speaker send
// a ROUTE-REFRESH only to a peer whose capability it received.
func TestPeersMissingRouteRefresh(t *testing.T) {
	session := func() map[string]any {
		return map[string]any{
			"asn": map[string]any{"remote": float64(65001)},
			"irr": map[string]any{"as-set": "AS-TEST"},
		}
	}
	capability := func(mode string) map[string]any {
		return map[string]any{"route-refresh": mode}
	}
	cfg := parseIRRConfig(map[string]any{
		"peer": map[string]any{
			"enabled":  map[string]any{"session": session(), "capability": capability("enable")},
			"required": map[string]any{"session": session(), "capability": capability("require")},
			"disabled": map[string]any{"session": session(), "capability": capability("disable")},
			"refused":  map[string]any{"session": session(), "capability": capability("refuse")},
			"absent":   map[string]any{"session": session()},
			"unfiltered": map[string]any{"session": map[string]any{
				"asn": map[string]any{"remote": float64(65002)},
			}},
		},
		"group": map[string]any{
			"g": map[string]any{
				"capability": capability("enable"),
				"peer": map[string]any{
					"member":   map[string]any{"session": session()},
					"optedout": map[string]any{"session": session(), "capability": capability("disable")},
				},
			},
		},
	})
	got := peersMissingRouteRefresh(cfg)
	slices.Sort(got)
	want := []string{"absent", "disabled", "optedout", "refused"}
	if !slices.Equal(got, want) {
		t.Errorf("peers without Route Refresh = %v, want %v", got, want)
	}
}
