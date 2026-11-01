package filter_community_match

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const testAttachmentBGP = "filter-community-match-test bgp import"

func init() {
	policy.MustRegisterAttachment("bgp", testAttachmentBGP,
		[]policy.Field{policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity}, nil)
}

// testListConfig is one community-match list in its delivered slice form,
// so the entry order is the operator's: 65000:200 rejects before 65000:100
// accepts.
func testListConfig() map[string]any {
	return map[string]any{"entry": []any{
		map[string]any{"community": "65000:200", "action": "reject"},
		map[string]any{"community": "65000:100", "action": "accept"},
		map[string]any{"community": "no-export", "action": "accept"},
		map[string]any{"community": "65000:1:2", "type": "large", "action": "accept"},
		map[string]any{"community": "target:65000:5", "type": "extended", "action": "accept"},
		map[string]any{"community": "65000:3:4", "type": "large", "action": "reject"},
	}}
}

// conditionUpdate builds an UPDATE carrying attrs and one IPv4 prefix.
func conditionUpdate(t *testing.T, attrs []byte) *wireu.WireUpdate {
	t.Helper()
	payload := binary.BigEndian.AppendUint16(nil, 0)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs))) //nolint:gosec // G115: a test attribute block
	payload = append(payload, attrs...)
	payload = append(payload, 24, 10, 1, 0)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

// optionalTransitive frames one optional transitive attribute.
func optionalTransitive(code byte, value []byte) []byte {
	return append([]byte{0xC0, code, byte(len(value))}, value...)
}

func communityChain(t *testing.T) *policychain.Chain {
	t.Helper()
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "cl", Type: filterTypeName, Config: testListConfig()},
		policy.Instance{Name: "m", Type: policy.MatchFilterType,
			Config: map[string]any{policy.FieldCommunity.String(): "cl"}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

// TestBGPMatchCommunityParity proves the community condition at a BGP chain
// gives the verdict the community-match filter gives over the same route's
// text form. Method: each case holds typed communities; the text is what
// their AppendText producers write, the wire is their RFC 1997, 4360 and
// 8092 encoding, and both paths judge the same list.
func TestBGPMatchCommunityParity(t *testing.T) {
	entries, err := parseCommunityEntries("cl", testListConfig())
	if err != nil {
		t.Fatal(err)
	}
	old := listsByName.Load()
	t.Cleanup(func() { listsByName.Store(old) })
	lists := map[string]*communityList{"cl": {name: "cl", entries: entries}}
	listsByName.Store(&lists)

	target, err := attribute.ParseSingleExtCommunity("target:65000:5")
	if err != nil {
		t.Fatal(err)
	}
	other, err := attribute.ParseSingleExtCommunity("target:65000:6")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		standard attribute.Communities
		large    attribute.LargeCommunities
		extended attribute.ExtendedCommunities
		action   sdk.FilterAction
	}{
		{"accepted community", attribute.Communities{65000<<16 | 100}, nil, nil, sdk.FilterAccept},
		{"rejecting entry", attribute.Communities{65000<<16 | 200}, nil, nil, sdk.FilterReject},
		{"first entry wins", attribute.Communities{65000<<16 | 100, 65000<<16 | 200}, nil, nil, sdk.FilterReject},
		{"well-known community", attribute.Communities{0xFFFFFF01}, nil, nil, sdk.FilterAccept},
		{"unlisted community", attribute.Communities{65000<<16 | 999}, nil, nil, sdk.FilterReject},
		{"no community", nil, nil, nil, sdk.FilterReject},
		{"large community", nil, attribute.LargeCommunities{{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}}, nil, sdk.FilterAccept},
		{"rejecting large community", nil, attribute.LargeCommunities{{GlobalAdmin: 65000, LocalData1: 3, LocalData2: 4}}, nil, sdk.FilterReject},
		{"large value as standard", attribute.Communities{65000<<16 | 1}, nil, nil, sdk.FilterReject},
		{"extended community", nil, nil, attribute.ExtendedCommunities{target}, sdk.FilterAccept},
		{"unlisted extended community", nil, nil, attribute.ExtendedCommunities{other}, sdk.FilterReject},
		{"large before standard in the list", attribute.Communities{65000<<16 | 999},
			attribute.LargeCommunities{{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}}, nil, sdk.FilterAccept},
	}
	chain := communityChain(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var text, attrs []byte
			if len(tc.standard) > 0 {
				text = tc.standard.AppendText(text)
				var value []byte
				for _, c := range tc.standard {
					value = binary.BigEndian.AppendUint32(value, uint32(c))
				}
				attrs = append(attrs, optionalTransitive(8, value)...)
			}
			if len(tc.extended) > 0 {
				text = append(tc.extended.AppendText(append(text, ' ')), ' ')
				var value []byte
				for _, e := range tc.extended {
					value = append(value, e[:]...)
				}
				attrs = append(attrs, optionalTransitive(16, value)...)
			}
			if len(tc.large) > 0 {
				text = tc.large.AppendText(append(text, ' '))
				var value []byte
				for _, l := range tc.large {
					value = binary.BigEndian.AppendUint32(value, l.GlobalAdmin)
					value = binary.BigEndian.AppendUint32(value, l.LocalData1)
					value = binary.BigEndian.AppendUint32(value, l.LocalData2)
				}
				attrs = append(attrs, optionalTransitive(32, value)...)
			}
			got := handleFilterUpdate(&sdk.FilterUpdateInput{Filter: "cl", Update: string(text) + " nlri add 10.1.0.0/24"})
			if got.Action != tc.action {
				t.Fatalf("text path answered %v, want %v (text %q)", got.Action, tc.action, text)
			}
			var view policyview.View
			view.Reset(conditionUpdate(t, attrs), policyview.Session{})
			defer view.Clear()
			outcome := chain.Run(&view, new(policychain.RunSets))
			if outcome.Failed {
				t.Fatalf("chain failed the route: %+v", outcome)
			}
			want := policychain.OutcomeAccept
			if tc.action == sdk.FilterReject {
				want = policychain.OutcomeReject
			}
			if outcome.Action != want {
				t.Fatalf("BGP chain answered %+v, text answered %v", outcome, tc.action)
			}
		})
	}
}

// TestBGPMatchCommunityMalformedFailsClosed proves a community attribute
// the view refuses (RFC 7606 Section 7.8 length) fails the member rather
// than passing as an absent community.
func TestBGPMatchCommunityMalformedFailsClosed(t *testing.T) {
	chain := communityChain(t)
	var view policyview.View
	view.Reset(conditionUpdate(t, optionalTransitive(8, []byte{0xFD, 0xE8, 0})), policyview.Session{})
	defer view.Clear()
	outcome := chain.Run(&view, new(policychain.RunSets))
	if outcome.Action == policychain.OutcomeAccept && !outcome.Failed {
		t.Fatalf("a malformed community was accepted: %+v", outcome)
	}
}

// TestBGPMatchCommunityAllocatesNothing proves the condition judges an
// UPDATE with no heap allocation once the view is warm.
func TestBGPMatchCommunityAllocatesNothing(t *testing.T) {
	chain := communityChain(t)
	value := binary.BigEndian.AppendUint32(nil, 65000<<16|999)
	value = binary.BigEndian.AppendUint32(value, 65000<<16|100)
	update := conditionUpdate(t, append(optionalTransitive(8, value),
		optionalTransitive(32, []byte{0, 0, 0xFD, 0xE8, 0, 0, 0, 9, 0, 0, 0, 9})...))
	var view policyview.View
	sets := new(policychain.RunSets)
	allocs := testing.AllocsPerRun(100, func() {
		view.Reset(update, policyview.Session{})
		if outcome := chain.Run(&view, sets); outcome.Action != policychain.OutcomeAccept {
			t.Fatalf("outcome %+v, want accept", outcome)
		}
		view.Clear()
	})
	if allocs != 0 {
		t.Fatalf("%v allocations per UPDATE, want 0", allocs)
	}
}
