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
