package filter_prefix

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// testAttachmentBGP carries prefix and family and lets no filter set either,
// as a BGP peer chain does: a match's NLRI subset is admitted by its reads.
const testAttachmentBGP = "filter-prefix-test bgp import"

func init() {
	policy.MustRegisterAttachment("bgp", testAttachmentBGP,
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily}, nil)
}

// bgpUpdate builds an UPDATE body: withdrawn routes, path attributes, body NLRI.
func bgpUpdate(t *testing.T, withdrawn, attrs, nlri []byte) *wireu.WireUpdate {
	t.Helper()
	payload := binary.BigEndian.AppendUint16(nil, uint16(len(withdrawn)))
	payload = append(payload, withdrawn...)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs)))
	payload = append(payload, attrs...)
	payload = append(payload, nlri...)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

// mpReach is an MP_REACH_NLRI attribute (extended length) of afi/safi with
// next hop nh and nlri.
func mpReach(afi uint16, safi uint8, nh, nlri []byte) []byte {
	value := binary.BigEndian.AppendUint16(nil, afi)
	value = append(value, safi, byte(len(nh)))
	value = append(value, nh...)
	value = append(value, 0)
	value = append(value, nlri...)
	attr := []byte{0x90, 14}
	attr = binary.BigEndian.AppendUint16(attr, uint16(len(value)))
	return append(attr, value...)
}

// flowRule is one FlowSpec NLRI of components (RFC 8955 Section 4), with the
// 8-octet route distinguisher first when rd is set (SAFI 134).
func flowRule(rd bool, components ...byte) []byte {
	body := components
	if rd {
		body = append(make([]byte, 8), components...)
	}
	return append([]byte{byte(len(body))}, body...)
}

// matchChain compiles a one-member chain naming a match whose prefix-list
// condition names the protected list (10.0.0.0/8 ge 16 le 24 accept).
func matchChain(t *testing.T) *policychain.Chain {
	t.Helper()
	listCfg := map[string]any{"entry": map[string]any{
		"10.0.0.0/8": map[string]any{"ge": "16", "le": "24", "action": "accept"}}}
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "protected", Type: filterTypeName, Config: listCfg},
		policy.Instance{Name: "m", Type: policy.MatchFilterType, Config: map[string]any{filterTypeName: "protected"}},
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

// TestBGPMatchPrefixListParity proves a match naming a prefix-list judges a
// BGP UPDATE as the list's own filter judges its text: accept, reject, or
// the denied prefixes dropped (announced and withdrawn), a denied or absent
// FlowSpec destination rejecting the whole UPDATE. Method: each case is one
// UPDATE given both as wire bytes, run through a compiled chain, and as the
// filter text the reactor renders for it, run through handleFilterUpdate;
// the verdicts and the dropped prefixes must agree.
func TestBGPMatchPrefixListParity(t *testing.T) {
	family.RegisterTestFamilies()
	listCfg := map[string]any{"entry": map[string]any{
		"10.0.0.0/8": map[string]any{"ge": "16", "le": "24", "action": "accept"}}}
	entries, err := parsePrefixListEntries("protected", listCfg)
	if err != nil {
		t.Fatal(err)
	}
	old := listsByName.Load()
	t.Cleanup(func() { listsByName.Store(old) })
	lists := map[string]*prefixList{"protected": {name: "protected", entries: entries}}
	listsByName.Store(&lists)

	v4 := func(octets ...byte) []byte { return octets }
	accepted := v4(24, 10, 1, 0) // 10.1.0.0/24
	denied := v4(24, 192, 0, 2)  // 192.0.2.0/24
	nh4 := []byte{192, 0, 2, 1}  // a next hop for MP_REACH
	v6 := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	destination := func(bits byte, octets ...byte) []byte { return append([]byte{1, bits}, octets...) }
	source := []byte{2, 24, 10, 1, 0} // a source prefix component only
	cases := []struct {
		name                   string
		withdrawn, attrs, nlri []byte
		text                   string
		action                 sdk.FilterAction
		drops                  map[policychain.NLRISection][]uint16
	}{
		{"accepted prefix", nil, nil, accepted, "nlri add 10.1.0.0/24", sdk.FilterAccept, nil},
		{"denied prefix", nil, nil, denied, "nlri add 192.0.2.0/24", sdk.FilterReject, nil},
		{"mixed announce", nil, nil, append(slices.Clone(accepted), denied...),
			"nlri add 10.1.0.0/24 192.0.2.0/24", sdk.FilterModify,
			map[policychain.NLRISection][]uint16{policychain.NLRISectionAnnounce: {4}}},
		{"denied withdrawal beside accepted announce", denied, nil, accepted,
			"nlri del 192.0.2.0/24 nlri add 10.1.0.0/24", sdk.FilterModify,
			map[policychain.NLRISection][]uint16{policychain.NLRISectionWithdrawn: {0}}},
		{"denied withdrawal only", denied, nil, nil, "nlri del 192.0.2.0/24", sdk.FilterReject, nil},
		{"no prefix", nil, nil, nil, "", sdk.FilterAccept, nil},
		{"ipv6 prefix against an ipv4 list", nil, mpReach(2, 1, make([]byte, 16), v6), nil,
			"mp ipv6/unicast add 2001:db8::/32", sdk.FilterReject, nil},
		{"mixed mp-reach", nil, mpReach(1, 1, nh4, append(slices.Clone(accepted), denied...)), nil,
			"mp ipv4/unicast add 10.1.0.0/24 192.0.2.0/24", sdk.FilterModify,
			map[policychain.NLRISection][]uint16{policychain.NLRISectionMPReach: {4}}},
		{"flowspec accepted destination", nil, mpReach(1, 133, nil, flowRule(false, destination(24, 10, 1, 0)...)), nil,
			"mp ipv4/flow add 10.1.0.0/24", sdk.FilterAccept, nil},
		{"flowspec destination too long", nil, mpReach(1, 133, nil, flowRule(false, destination(25, 10, 1, 0, 0)...)), nil,
			"mp ipv4/flow add 10.1.0.0/25", sdk.FilterReject, nil},
		{"flowspec denied destination", nil, mpReach(1, 133, nil, flowRule(false, destination(24, 192, 0, 2)...)), nil,
			"mp ipv4/flow add 192.0.2.0/24", sdk.FilterReject, nil},
		{"flowspec absent destination", nil, mpReach(1, 133, nil, flowRule(false, source...)), nil,
			"mp ipv4/flow add invalid", sdk.FilterReject, nil},
		{"flowspec mixed rules", nil, mpReach(1, 133, nil, append(flowRule(false, destination(24, 10, 1, 0)...),
			flowRule(false, destination(24, 192, 0, 2)...)...)), nil,
			"mp ipv4/flow add 10.1.0.0/24 192.0.2.0/24", sdk.FilterReject, nil},
		{"flowspec vpn accepted destination", nil, mpReach(1, 134, nil, flowRule(true, destination(24, 10, 1, 0)...)), nil,
			"mp ipv4/flow-vpn add 10.1.0.0/24", sdk.FilterAccept, nil},
	}
	chain := matchChain(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := handleFilterUpdate(&sdk.FilterUpdateInput{Filter: "protected", Update: tc.text})
			if text.Action != tc.action {
				t.Fatalf("text path answered %v, want %v", text.Action, tc.action)
			}
			var view policyview.View
			view.Reset(bgpUpdate(t, tc.withdrawn, tc.attrs, tc.nlri), policyview.Session{})
			defer view.Clear()
			sets := new(policychain.RunSets)
			outcome := chain.Run(&view, sets)
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
			if want == policychain.OutcomeReject {
				return
			}
			for section := policychain.NLRISectionAnnounce; section <= policychain.NLRISectionMPUnreach; section++ {
				if got := sets.Folded().NLRIDrops(section); !slices.Equal(got, tc.drops[section]) {
					t.Fatalf("section %d drops %v, want %v", section, got, tc.drops[section])
				}
			}
		})
	}
}

// TestBGPMatchAllocatesNothing proves judging an UPDATE's prefixes at a BGP
// chain allocates nothing per run once the view and sets are warm. Method:
// testing.AllocsPerRun over a mixed UPDATE, one prefix dropped per run.
func TestBGPMatchAllocatesNothing(t *testing.T) {
	chain := matchChain(t)
	update := bgpUpdate(t, []byte{24, 192, 0, 2}, nil, []byte{24, 10, 1, 0, 24, 192, 0, 2})
	var view policyview.View
	sets := new(policychain.RunSets)
	run := func() {
		view.Reset(update, policyview.Session{})
		if outcome := chain.Run(&view, sets); outcome.Action != policychain.OutcomeAccept {
			t.Fatalf("outcome = %+v, want accept with drops", outcome)
		}
		view.Clear()
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a run allocates %.1f times, want 0", allocs)
	}
}
