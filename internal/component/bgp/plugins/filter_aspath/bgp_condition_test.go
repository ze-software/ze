package filter_aspath

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

// testAttachmentBGP carries as-path and lets no filter set it, as a BGP peer
// chain carries it.
const testAttachmentBGP = "filter-aspath-test bgp import"

func init() {
	policy.MustRegisterAttachment("bgp", testAttachmentBGP, []policy.Field{policy.FieldASPath}, nil)
}

// testLists are the as-path-lists the parity cases judge: "transit" rejects
// a path through 64512 and accepts one learned from 65001; "empty" accepts
// only a path with no ASN, and the four-octet ASN 4200000000 alone.
func testLists() map[string]map[string]any {
	entry := func(regex, action string) map[string]any {
		return map[string]any{"regex": regex, "action": action}
	}
	return map[string]map[string]any{
		"transit": {"entry": []any{entry("(^| )64512( |$)", "reject"), entry("^65001( |$)", "accept")}},
		"empty":   {"entry": []any{entry("^$", "accept"), entry("^4200000000$", "accept")}},
	}
}

// asPathAttr encodes path as a four-octet AS_PATH attribute (RFC 4271
// Section 4.3, RFC 6793): per segment its type, its ASN count, then each ASN
// in four octets.
func asPathAttr(path *attribute.ASPath) []byte {
	var value []byte
	for _, segment := range path.Segments {
		value = append(value, byte(segment.Type), byte(len(segment.ASNs)))
		for _, number := range segment.ASNs {
			value = binary.BigEndian.AppendUint32(value, number)
		}
	}
	return append([]byte{0x40, 2, byte(len(value))}, value...)
}

// bgpUpdate builds an UPDATE announcing 10.1.0.0/24 with attrs, received on
// a four-octet session.
func bgpUpdate(t *testing.T, attrs []byte) *wireu.WireUpdate {
	t.Helper()
	payload := []byte{0, 0}
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs)))
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

// matchChain compiles a one-member chain naming a match whose as-path
// condition names the list.
func matchChain(t *testing.T, list string) *policychain.Chain {
	t.Helper()
	instances := []policy.Instance{{Name: "m", Type: policy.MatchFilterType,
		Config: map[string]any{policy.FieldASPath.String(): list}}}
	for name, cfg := range testLists() {
		instances = append(instances, policy.Instance{Name: name, Type: filterTypeName, Config: cfg})
	}
	reg, err := policy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

// TestBGPMatchASPathParity proves a match naming an as-path-list judges a BGP
// UPDATE as the list's own filter judges its text. Method: each case is one
// AS path, encoded as the UPDATE's AS_PATH and run through a compiled chain,
// and rendered by the producer of the filter text (ASPath.AppendText) and
// run through handleFilterUpdate; the two verdicts must agree, and each must
// be the one the case names.
func TestBGPMatchASPathParity(t *testing.T) {
	old := listsByName.Load()
	t.Cleanup(func() { listsByName.Store(old) })
	lists := map[string]*aspathList{}
	for name, cfg := range testLists() {
		entries, err := parseAsPathEntries(name, cfg)
		if err != nil {
			t.Fatal(err)
		}
		lists[name] = &aspathList{name: name, entries: entries}
	}
	listsByName.Store(&lists)

	seq := func(asns ...uint32) attribute.ASPathSegment {
		return attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: asns}
	}
	set := func(asns ...uint32) attribute.ASPathSegment {
		return attribute.ASPathSegment{Type: attribute.ASSet, ASNs: asns}
	}
	cases := []struct {
		name     string
		list     string
		segments []attribute.ASPathSegment // nil: the UPDATE carries no AS_PATH
		action   sdk.FilterAction
	}{
		{"learned from 65001", "transit", []attribute.ASPathSegment{seq(65001, 65002)}, sdk.FilterAccept},
		{"through 64512", "transit", []attribute.ASPathSegment{seq(65001, 64512)}, sdk.FilterReject},
		{"64512 first", "transit", []attribute.ASPathSegment{seq(64512, 65001)}, sdk.FilterReject},
		{"no entry matches", "transit", []attribute.ASPathSegment{seq(65002)}, sdk.FilterReject},
		{"no as-path against transit", "transit", nil, sdk.FilterReject},
		{"as-set read flat", "transit", []attribute.ASPathSegment{set(65001, 65003)}, sdk.FilterAccept},
		{"64512 in a trailing as-set", "transit", []attribute.ASPathSegment{seq(65001), set(64512)}, sdk.FilterReject},
		{"confederation sequence read flat", "transit",
			[]attribute.ASPathSegment{{Type: attribute.ASConfedSequence, ASNs: []uint32{65001}}, seq(65009)}, sdk.FilterAccept},
		{"no as-path is the empty text", "empty", nil, sdk.FilterAccept},
		{"empty sequence is the empty text", "empty", []attribute.ASPathSegment{}, sdk.FilterAccept},
		{"four-octet asn", "empty", []attribute.ASPathSegment{seq(4200000000)}, sdk.FilterAccept},
		{"two hops against empty", "empty", []attribute.ASPathSegment{seq(4200000000, 65001)}, sdk.FilterReject},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var attrs []byte
			var text []byte
			if tc.segments != nil {
				path := &attribute.ASPath{Segments: tc.segments}
				attrs = asPathAttr(path)
				text = path.AppendText(nil)
			}
			textOut := handleFilterUpdate(&sdk.FilterUpdateInput{Filter: tc.list, Update: string(text) + " nlri add 10.1.0.0/24"})
			if textOut.Action != tc.action {
				t.Fatalf("text path answered %v for %q, want %v", textOut.Action, text, tc.action)
			}
			var view policyview.View
			view.Reset(bgpUpdate(t, attrs), policyview.Session{})
			defer view.Clear()
			outcome := matchChain(t, tc.list).Run(&view, new(policychain.RunSets))
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

// TestBGPMatchASPathMalformedFails proves an AS_PATH that does not decode
// fails the member rather than reading as an empty path. Method: a segment
// whose count claims more ASNs than the attribute holds.
func TestBGPMatchASPathMalformedFails(t *testing.T) {
	var view policyview.View
	view.Reset(bgpUpdate(t, []byte{0x40, 2, 6, 2, 2, 0, 0, 0xfd, 0xe9}), policyview.Session{})
	defer view.Clear()
	outcome := matchChain(t, "empty").Run(&view, new(policychain.RunSets))
	if !outcome.Failed {
		t.Fatalf("outcome = %+v, want the member failed", outcome)
	}
}

// TestBGPMatchASPathAllocatesNothing proves judging an UPDATE's AS path at a
// BGP chain allocates nothing per run once the view and sets are warm: the
// text is built into the view's own buffer and the regex reads its bytes.
// Method: testing.AllocsPerRun over a three-hop path the list accepts; under
// -race the regexp matcher pool's random drops add regexPoolRaceOverhead.
func TestBGPMatchASPathAllocatesNothing(t *testing.T) {
	chain := matchChain(t, "transit")
	path := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002, 4200000000}}}}
	update := bgpUpdate(t, asPathAttr(path))
	if _, err := update.Attrs(); err != nil { // the reactor indexed it at ingress
		t.Fatal(err)
	}
	var view policyview.View
	sets := new(policychain.RunSets)
	run := func() {
		view.Reset(update, policyview.Session{})
		if outcome := chain.Run(&view, sets); outcome.Action != policychain.OutcomeAccept {
			t.Fatalf("outcome = %+v, want accept", outcome)
		}
		view.Clear()
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs > regexPoolRaceOverhead {
		t.Fatalf("a run allocates %.1f times, want at most %d", allocs, regexPoolRaceOverhead)
	}
}
