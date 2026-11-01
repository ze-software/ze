package filter_irr

import (
	"slices"
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: an UPDATE that announces and withdraws in several families is
// judged family by family from the prefix and withdrawn fields, a withdrawal is
// never filtered, and a rejected NLRI is answered as a prefix remove naming it
// in the shape the prefix field carried it (unicast, multicast, ADD-PATH and
// labeled unicast alike). The NLRI of a family the list cannot judge (FlowSpec,
// EVPN) passes as received.
// PREVENTS: one family's rejected prefix rejecting the UPDATE whole and its
// valid routes with it, and a withdrawal dropped with a rejected announcement.
func TestFilterIRRMultiBlockVerdict(t *testing.T) {
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), newFakeReadmitEngine(), time.Hour)
	plug.refreshASN(65001)

	type nlri = map[string][]string
	tests := []struct {
		name      string
		announced nlri
		withdrawn nlri
		action    sdk.FilterAction
		removes   []string
	}{
		{
			name:      "accepted announce beside a withdrawal",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			withdrawn: nlri{"ipv4/unicast": {"192.0.2.0/24"}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "mixed announce beside a withdrawal keeps the withdrawal whole",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24", "192.0.2.0/24"}},
			withdrawn: nlri{"ipv4/unicast": {"198.51.100.0/24"}},
			action:    sdk.FilterModify,
			removes:   []string{`"192.0.2.0/24"`},
		},
		{
			name:      "an admitted announce beside a FlowSpec rule is accepted",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}, "ipv4/flow": {`{"destination":[["192.0.2.0/24"]]}`}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "an admitted announce beside a rejected IPv6 announce is a modify",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}, "ipv6/unicast": {"2001:db8::/32"}},
			action:    sdk.FilterModify,
			removes:   []string{`"2001:db8::/32"`},
		},
		{
			name:      "a mixed multicast announce keeps its admitted prefix",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}, "ipv4/multicast": {"192.0.2.0/24", "10.0.0.0/24"}},
			action:    sdk.FilterModify,
			removes:   []string{`"192.0.2.0/24"`},
		},
		{
			// RFC 8277 Section 2.2: the remove names the NLRI with its label
			// stack, the shape the prefix field carried it.
			name: "a mixed labeled announce keeps its admitted prefix",
			announced: nlri{"ipv4/mpls-label": {
				`{"labels":[[100,1601]],"prefix":"192.0.2.0/24"}`,
				`{"labels":[[200,3201]],"prefix":"10.0.0.0/24"}`,
			}},
			action:  sdk.FilterModify,
			removes: []string{`{"labels":[[100,1601]],"prefix":"192.0.2.0/24"}`},
		},
		{
			// RFC 7911 Section 3: under ADD-PATH the remove names one path.
			name: "a mixed ADD-PATH labeled announce removes the refused path",
			announced: nlri{"ipv4/mpls-label": {
				`{"labels":[[100,1601]],"path-id":7,"prefix":"192.0.2.0/24"}`,
				`{"labels":[[100,1601]],"path-id":8,"prefix":"10.0.0.0/24"}`,
			}},
			action:  sdk.FilterModify,
			removes: []string{`{"labels":[[100,1601]],"path-id":7,"prefix":"192.0.2.0/24"}`},
		},
		{
			name: "a mixed ADD-PATH unicast announce removes the refused path",
			announced: nlri{"ipv4/unicast": {
				`{"path-id":0,"prefix":"10.0.0.0/24"}`,
				`{"path-id":1,"prefix":"192.0.2.0/24"}`,
			}},
			action:  sdk.FilterModify,
			removes: []string{`{"path-id":1,"prefix":"192.0.2.0/24"}`},
		},
		{
			name:      "an EVPN route beside a mixed announce passes unjudged",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24", "192.0.2.0/24"}, "l2vpn/evpn": {`{"route-type":"mac-ip-advertisement"}`}},
			action:    sdk.FilterModify,
			removes:   []string{`"192.0.2.0/24"`},
		},
		{
			name:      "a mixed announce beside an IPv4 and an IPv6 withdrawal is a modify",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24", "192.0.2.0/24"}},
			withdrawn: nlri{"ipv4/unicast": {"198.51.100.0/24"}, "ipv6/unicast": {"2001:db8::/32"}},
			action:    sdk.FilterModify,
			removes:   []string{`"192.0.2.0/24"`},
		},
		{
			// RFC 4271 Section 9: the withdrawal SHALL remove the route from
			// the Adj-RIB-In, so rejecting the announcement keeps it.
			name:      "rejected announce beside a withdrawal keeps the withdrawal",
			announced: nlri{"ipv4/unicast": {"192.0.2.0/24"}},
			withdrawn: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			action:    sdk.FilterModify,
			removes:   []string{`"192.0.2.0/24"`},
		},
		{
			name:      "a FlowSpec rule beside a withdrawal is accepted",
			announced: nlri{"ipv4/flow": {`{"destination":[["192.0.2.0/24"]]}`}},
			withdrawn: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "rejected IPv6 announce beside a withdrawal keeps the withdrawal",
			announced: nlri{"ipv6/unicast": {"2001:db8::/32"}},
			withdrawn: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			action:    sdk.FilterModify,
			removes:   []string{`"2001:db8::/32"`},
		},
		{
			name:      "a withdrawal alone is not filtered",
			withdrawn: nlri{"ipv4/unicast": {"192.0.2.0/24"}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "a malformed withdrawn prefix still fails closed",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			withdrawn: nlri{"ipv4/unicast": {"not-a-prefix"}},
			action:    sdk.FilterReject,
		},
		{
			name:      "a malformed announced element fails closed",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24", `{"path-id":1}`}},
			action:    sdk.FilterReject,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
				Filter: "bgp-filter-irr:65001",
				Peer:   readmitTestPeer,
				Fields: routeFields(tt.announced, tt.withdrawn),
			})
			if out.Action != tt.action {
				t.Fatalf("action = %v, want %v", out.Action, tt.action)
			}
			if got := removedValues(t, out); !slices.Equal(got, tt.removes) && (len(got) != 0 || len(tt.removes) != 0) {
				t.Errorf("removes = %v, want %v", got, tt.removes)
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
	cfg := parseIRRConfigOf(map[string]any{
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

	type nlri = map[string][]string
	tests := []struct {
		name      string
		announced nlri
		withdrawn nlri
		action    sdk.FilterAction
		removes   []string
	}{
		{
			name:      "a withdrawal alone is accepted",
			withdrawn: nlri{"ipv4/unicast": {"192.0.2.0/24"}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "an IPv6 withdrawal alone is accepted",
			withdrawn: nlri{"ipv6/unicast": {"2001:db8::/32"}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "an announce beside a withdrawal keeps the withdrawal",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			withdrawn: nlri{"ipv4/unicast": {"198.51.100.0/24"}},
			action:    sdk.FilterModify,
			removes:   []string{`"10.0.0.0/24"`},
		},
		{
			name:      "an announce alone is rejected",
			announced: nlri{"ipv4/unicast": {"10.0.0.0/24"}},
			action:    sdk.FilterReject,
		},
		{
			name:      "a FlowSpec rule beside a withdrawal is accepted",
			announced: nlri{"ipv4/flow": {`{"destination":[["192.0.2.0/24"]]}`}},
			withdrawn: nlri{"ipv4/unicast": {"198.51.100.0/24"}},
			action:    sdk.FilterAccept,
		},
		{
			name:      "an IPv6 announce beside a withdrawal keeps the withdrawal",
			announced: nlri{"ipv6/unicast": {"2001:db8::/32"}},
			withdrawn: nlri{"ipv4/unicast": {"198.51.100.0/24"}},
			action:    sdk.FilterModify,
			removes:   []string{`"2001:db8::/32"`},
		},
		{
			name:      "a malformed withdrawal fails closed",
			withdrawn: nlri{"ipv4/unicast": {"not-a-prefix"}},
			action:    sdk.FilterReject,
		},
		{
			name:   "an UPDATE with no prefix is rejected",
			action: sdk.FilterReject,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
				Filter: "bgp-filter-irr:65001",
				Peer:   readmitTestPeer,
				Fields: routeFields(tt.announced, tt.withdrawn),
			})
			if out.Action != tt.action {
				t.Fatalf("action = %v, want %v", out.Action, tt.action)
			}
			if got := removedValues(t, out); !slices.Equal(got, tt.removes) && (len(got) != 0 || len(tt.removes) != 0) {
				t.Errorf("removes = %v, want %v", got, tt.removes)
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
	cfg := parseIRRConfigOf(map[string]any{
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
// advertise Route Refresh, reading the peer's session capability and then
// its group's.
// PREVENTS: re-admission failing on such a peer with nothing said until a
// list changes: ze advertises Route Refresh only when the config enables it
// (reactor config_capabilities.go), and RFC 2918 Section 4 lets a speaker send
// a ROUTE-REFRESH only to a peer whose capability it received.
func TestPeersMissingRouteRefresh(t *testing.T) {
	// The capability container sits under session, as the YANG peer config
	// places it (configjson.GetCapability).
	session := func(mode string) map[string]any {
		s := map[string]any{
			"asn": map[string]any{"remote": float64(65001)},
			"irr": map[string]any{"as-set": "AS-TEST"},
		}
		if mode != "" {
			s["capability"] = map[string]any{"route-refresh": mode}
		}
		return s
	}
	cfg := parseIRRConfigOf(map[string]any{
		"peer": map[string]any{
			"enabled":  map[string]any{"session": session("enable")},
			"required": map[string]any{"session": session("require")},
			"disabled": map[string]any{"session": session("disable")},
			"refused":  map[string]any{"session": session("refuse")},
			"absent":   map[string]any{"session": session("")},
			"unfiltered": map[string]any{"session": map[string]any{
				"asn": map[string]any{"remote": float64(65002)},
			}},
		},
		"group": map[string]any{
			"g": map[string]any{
				"session": map[string]any{
					"capability": map[string]any{"route-refresh": "enable"},
				},
				"peer": map[string]any{
					"member":   map[string]any{"session": session("")},
					"optedout": map[string]any{"session": session("disable")},
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
