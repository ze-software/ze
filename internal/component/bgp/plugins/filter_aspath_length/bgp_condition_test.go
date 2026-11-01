package filter_aspath_length

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
const testAttachmentBGP = "filter-aspath-length-test bgp import"

func init() {
	policy.MustRegisterAttachment("bgp", testAttachmentBGP, []policy.Field{policy.FieldASPath}, nil)
}

// testDefs are the as-path-length instances the parity cases judge, in the
// string form config delivers.
func testDefs() map[string]map[string]any {
	return map[string]map[string]any{
		"short":   {"max": "3"},
		"range":   {"min": "2", "max": "3"},
		"nonzero": {"min": "1"},
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

// matchChain compiles a one-member chain naming a match whose
// as-path-length condition names the instance.
func matchChain(t *testing.T, name string) *policychain.Chain {
	t.Helper()
	instances := []policy.Instance{{Name: "m", Type: policy.MatchFilterType,
		Config: map[string]any{filterTypeName: name}}}
	for defName, cfg := range testDefs() {
		instances = append(instances, policy.Instance{Name: defName, Type: filterTypeName, Config: cfg})
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

// TestBGPMatchASPathLengthParity proves a match naming an as-path-length
// instance judges a BGP UPDATE as the instance's own filter judges its text.
// Method: each case is one AS path, encoded as the UPDATE's AS_PATH and run
// through a compiled chain, and rendered by the producer of the filter text
// (ASPath.AppendText) and run through handleFilterUpdate; the two verdicts
// must agree, and each must be the one the case names.
func TestBGPMatchASPathLengthParity(t *testing.T) {
	old := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(old) })
	defs := map[string]*asPathLengthDef{}
	for name, cfg := range testDefs() {
		def, err := parseAsPathLengthDef(name, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defs[name] = def
	}
	defsByName.Store(&defs)

	seq := func(asns ...uint32) attribute.ASPathSegment {
		return attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: asns}
	}
	set := func(asns ...uint32) attribute.ASPathSegment {
		return attribute.ASPathSegment{Type: attribute.ASSet, ASNs: asns}
	}
	cases := []struct {
		name     string
		def      string
		segments []attribute.ASPathSegment // nil: the UPDATE carries no AS_PATH
		action   sdk.FilterAction
	}{
		{"one hop under max", "short", []attribute.ASPathSegment{seq(65001)}, sdk.FilterAccept},
		{"at max", "short", []attribute.ASPathSegment{seq(65001, 65002, 65003)}, sdk.FilterAccept},
		{"over max", "short", []attribute.ASPathSegment{seq(65001, 65002, 65003, 65004)}, sdk.FilterReject},
		{"as-set members each count", "short", []attribute.ASPathSegment{seq(65001), set(65002, 65003, 65004)}, sdk.FilterReject},
		{"confederation asns count", "short",
			[]attribute.ASPathSegment{{Type: attribute.ASConfedSequence, ASNs: []uint32{65010, 65011}}, seq(65001, 65002)}, sdk.FilterReject},
		{"no as-path under max", "short", nil, sdk.FilterAccept},
		{"under min", "range", []attribute.ASPathSegment{seq(65001)}, sdk.FilterReject},
		{"at min", "range", []attribute.ASPathSegment{seq(65001, 4200000000)}, sdk.FilterAccept},
		{"over range max", "range", []attribute.ASPathSegment{seq(65001, 65002), seq(65003, 65004)}, sdk.FilterReject},
		{"no as-path under min", "nonzero", nil, sdk.FilterReject},
		{"empty sequence under min", "nonzero", []attribute.ASPathSegment{}, sdk.FilterReject},
		{"one as-set member meets min", "nonzero", []attribute.ASPathSegment{set(65001)}, sdk.FilterAccept},
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
			textOut := handleFilterUpdate(&sdk.FilterUpdateInput{Filter: tc.def, Update: string(text) + " nlri add 10.1.0.0/24"})
			if textOut.Action != tc.action {
				t.Fatalf("text path answered %v for %q, want %v", textOut.Action, text, tc.action)
			}
			var view policyview.View
			view.Reset(bgpUpdate(t, attrs), policyview.Session{})
			defer view.Clear()
			outcome := matchChain(t, tc.def).Run(&view, new(policychain.RunSets))
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

// TestBGPMatchASPathLengthMalformedFails proves an AS_PATH that does not
// decode fails the member rather than counting as an empty path. Method: a
// segment whose count claims more ASNs than the attribute holds, against an
// instance an empty path passes.
func TestBGPMatchASPathLengthMalformedFails(t *testing.T) {
	var view policyview.View
	view.Reset(bgpUpdate(t, []byte{0x40, 2, 6, 2, 2, 0, 0, 0xfd, 0xe9}), policyview.Session{})
	defer view.Clear()
	outcome := matchChain(t, "short").Run(&view, new(policychain.RunSets))
	if !outcome.Failed {
		t.Fatalf("outcome = %+v, want the member failed", outcome)
	}
}

// TestBGPMatchASPathLengthAllocatesNothing proves judging an UPDATE's AS path
// length at a BGP chain allocates nothing per run once the view and sets are
// warm. Method: testing.AllocsPerRun over a two-hop path the instance passes.
func TestBGPMatchASPathLengthAllocatesNothing(t *testing.T) {
	chain := matchChain(t, "range")
	path := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{65001, 4200000000}}}}
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
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a run allocates %.1f times, want 0", allocs)
	}
}
