package flowspec

import (
	"encoding/hex"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFlowSpecSyntaxRuleResetsOnMalformedNLRI judges FlowSpec NLRI sections
// through the rule the receive path looks up (nlrisyntax.Get, as
// message.validateMPNLRISyntax does).
//
// VALIDATES: RFC 8955 Section 4.2, "An NLRI value not encoded as specified
// here ... is considered malformed", answered with RFC 7606 Section 3 (j)'s
// session reset; a well-formed section of each family passes.
// PREVENTS: the 4.2 corpus NLRI (source-ipv6 ::1/128/120 packed with sixteen
// pattern octets where RFC 8956 Section 3.1 admits one) passing the receive
// path and reaching a script as the announce of `wire[ipv6/flow](49 bytes)`.
func TestFlowSpecSyntaxRuleResetsOnMalformedNLRI(t *testing.T) {
	cases := []struct {
		name    string
		fam     family.Family
		section string
		want    nlrisyntax.Action
	}{
		{
			// qa/old responses-4.2, MP_REACH_NLRI 800E36: 49 octets of NLRI.
			name: "ipv6 flow source pattern sixteen octets long", fam: IPv6FlowSpec,
			section: "300180002A020B80001500007ACA39FFFEAEA87A" +
				"028078000000000000000000000000000000" + "01" + "0381110B81650D9107DD",
			want: nlrisyntax.ActionSessionReset,
		},
		{
			// The same route with the one pattern octet RFC 8956 Section 3.1 asks.
			name: "ipv6 flow well formed", fam: IPv6FlowSpec,
			section: "210180002A020B80001500007ACA39FFFEAEA87A" + "02807801" + "0381110B81650D9107DD",
			want:    nlrisyntax.ActionWellFormed,
		},
		{
			name: "ipv4 flow well formed", fam: IPv4FlowSpec,
			section: "060220CAFFEE01",
			want:    nlrisyntax.ActionWellFormed,
		},
		{
			// Component type 0 is unknown (RFC 8955 Section 4.2: "including an
			// NLRI that contains an unknown component type").
			name: "ipv4 flow unknown component type", fam: IPv4FlowSpec,
			section: "0400200A0A",
			want:    nlrisyntax.ActionSessionReset,
		},
		{
			// The NLRI length says six octets and four follow.
			name: "ipv4 flow overruns the section", fam: IPv4FlowSpec,
			section: "060220CAFF",
			want:    nlrisyntax.ActionSessionReset,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := nlrisyntax.Get(tt.fam)
			if !ok || rule.Judge == nil {
				t.Fatalf("no NLRI syntax rule registered for %s", tt.fam)
			}
			section, err := hex.DecodeString(tt.section)
			if err != nil {
				t.Fatalf("bad hex: %v", err)
			}
			judged := rule.Judge(section, false)
			if judged.Action != tt.want {
				t.Fatalf("%s: action %d (%q), want %d", tt.name, judged.Action, judged.Reason, tt.want)
			}
			if tt.want != nlrisyntax.ActionWellFormed && judged.Reason == "" {
				t.Fatalf("%s: a malformed verdict names no reason", tt.name)
			}
		})
	}
}

// TestFlowSpecSyntaxRuleCoversEveryFamily checks that each FlowSpec family the
// plugin declares has its receive-path rule.
//
// VALIDATES: flow, flow-vpn, IPv4 and IPv6 are all judged.
// PREVENTS: one family of the four passing malformed NLRIs.
func TestFlowSpecSyntaxRuleCoversEveryFamily(t *testing.T) {
	for _, fam := range flowSpecFamilySet {
		if rule, ok := nlrisyntax.Get(fam); !ok || rule.Judge == nil {
			t.Errorf("no NLRI syntax rule registered for %s", fam)
		}
	}
}

// wellFormedFlowSpecSections are the receive-path sections the allocation test
// and the benchmark judge: one per family, and one under ADD-PATH.
var wellFormedFlowSpecSections = []struct {
	name    string
	fam     family.Family
	section string
	addPath bool
}{
	{"ipv4 flow", IPv4FlowSpec, "060220CAFFEE01" + "0501180A0000", false},
	{"ipv6 flow corpus 4.2 twin", IPv6FlowSpec,
		"210180002A020B80001500007ACA39FFFEAEA87A" + "02807801" + "0381110B81650D9107DD", false},
	{"ipv4 flow-vpn", IPv4FlowSpecVPN, "0D" + "0000FDE800000001" + "01180A0000", false},
	{"ipv4 flow add-path", IPv4FlowSpec, "00000001" + "060220CAFFEE01", true},
}

// TestFlowSpecSyntaxRuleAllocatesNothing measures the registered rule over a
// well-formed section of each family.
//
// VALIDATES: the receive path judges a well-formed FlowSpec section with zero
// allocations (ai/rules/performance.md): no split slice, no decoded component.
// PREVENTS: the rule going back to nlrisplit.Split and flowspec.ParseFlowSpec,
// which allocate for every UPDATE.
func TestFlowSpecSyntaxRuleAllocatesNothing(t *testing.T) {
	for _, tt := range wellFormedFlowSpecSections {
		rule, ok := nlrisyntax.Get(tt.fam)
		if !ok || rule.Judge == nil {
			t.Fatalf("no NLRI syntax rule registered for %s", tt.fam)
		}
		section, err := hex.DecodeString(tt.section)
		if err != nil {
			t.Fatalf("bad hex: %v", err)
		}
		if judged := rule.Judge(section, tt.addPath); judged.Action != nlrisyntax.ActionWellFormed {
			t.Fatalf("%s: action %d (%q), want well-formed", tt.name, judged.Action, judged.Reason)
		}
		allocs := testing.AllocsPerRun(100, func() {
			rule.Judge(section, tt.addPath)
		})
		if allocs != 0 {
			t.Fatalf("%s: %v allocations per judgement, want 0", tt.name, allocs)
		}
	}
}

// BenchmarkFlowSpecSyntaxRule times the registered rule over the 4.2 corpus
// twin, the well-formed IPv6 section the receive path judges.
func BenchmarkFlowSpecSyntaxRule(b *testing.B) {
	rule, ok := nlrisyntax.Get(IPv6FlowSpec)
	if !ok || rule.Judge == nil {
		b.Fatal("no NLRI syntax rule registered for ipv6/flow")
	}
	section, err := hex.DecodeString(wellFormedFlowSpecSections[1].section)
	if err != nil {
		b.Fatalf("bad hex: %v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		rule.Judge(section, false)
	}
}
