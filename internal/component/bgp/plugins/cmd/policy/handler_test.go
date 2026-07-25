// VALIDATES: the policy command's pure argument/selector helpers —
// isPolicyTestKeyword, parsePolicyTestArgs (direction/filter/hex/source-asn4 with
// every error branch), filterPeersByPolicySelector (wildcard/IP/name/asN), and
// toFilterRefs (inactive: and type:name canonical parsing).
// PREVENTS: a mis-parsed policy-test invocation, a selector matching the wrong
// peers, or a filter reference losing its inactive marker.

package policy

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestIsPolicyTestKeyword(t *testing.T) {
	for _, kw := range []string{"import", "export", "filter", "update", "source-asn4", "IMPORT", "Export"} {
		if !isPolicyTestKeyword(kw) {
			t.Errorf("isPolicyTestKeyword(%q) = false, want true", kw)
		}
	}
	for _, kw := range []string{"", "peer1", "10.0.0.1", "as65001", "importer"} {
		if isPolicyTestKeyword(kw) {
			t.Errorf("isPolicyTestKeyword(%q) = true, want false", kw)
		}
	}
}

func TestParsePolicyTestArgs(t *testing.T) {
	dir, filter, hexp, asn4, err := parsePolicyTestArgs([]string{"import", "update", "DEADBEEF"})
	if err != nil || dir != "import" || filter != "" || hexp != "DEADBEEF" || !asn4 {
		t.Errorf("happy path = (%q,%q,%q,%v,%v), want (import,,DEADBEEF,true,nil)", dir, filter, hexp, asn4, err)
	}

	_, filter, hexp, _, err = parsePolicyTestArgs([]string{"export", "filter", "myf", "update", "hex", "CAFE"})
	if err != nil || filter != "myf" || hexp != "CAFE" {
		t.Errorf("filter+update-hex = (%q,%q,%v), want (myf,CAFE,nil)", filter, hexp, err)
	}

	_, _, _, asn4, err = parsePolicyTestArgs([]string{"import", "update", "AA", "source-asn4", "false"})
	if err != nil || asn4 {
		t.Errorf("source-asn4 false = (%v,%v), want (false,nil)", asn4, err)
	}

	for _, tc := range []struct {
		name string
		args []string
		want error
	}{
		{"missing direction", []string{"update", "AA"}, errMissingDirection},
		{"missing update hex", []string{"import"}, errMissingUpdateHex},
		{"missing filter name", []string{"import", "filter"}, errMissingFilterName},
		{"missing asn4 value", []string{"import", "update", "AA", "source-asn4"}, errMissingASN4Value},
		{"invalid asn4 value", []string{"import", "update", "AA", "source-asn4", "maybe"}, errInvalidASN4Value},
	} {
		if _, _, _, _, err := parsePolicyTestArgs(tc.args); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}

	// An unexpected token yields a descriptive (non-sentinel) error.
	if _, _, _, _, err := parsePolicyTestArgs([]string{"import", "bogus"}); err == nil || !strings.Contains(err.Error(), "unexpected token") {
		t.Errorf("unexpected token err = %v, want contains 'unexpected token'", err)
	}
}

func TestFilterPeersByPolicySelector(t *testing.T) {
	peers := []plugin.PeerInfo{
		{Address: netip.MustParseAddr("10.0.0.1"), Name: "alpha", PeerAS: 65001},
		{Address: netip.MustParseAddr("10.0.0.2"), Name: "beta", PeerAS: 65002},
		{Address: netip.MustParseAddr("10.0.0.3"), Name: "gamma", PeerAS: 65001},
	}

	if got := filterPeersByPolicySelector(peers, "*"); len(got) != 3 {
		t.Errorf("wildcard returned %d peers, want 3", len(got))
	}
	if got := filterPeersByPolicySelector(peers, "10.0.0.2"); len(got) != 1 || got[0].Name != "beta" {
		t.Errorf("IP selector = %v, want [beta]", got)
	}
	if got := filterPeersByPolicySelector(peers, "gamma"); len(got) != 1 || got[0].Name != "gamma" {
		t.Errorf("name selector = %v, want [gamma]", got)
	}
	if got := filterPeersByPolicySelector(peers, "as65001"); len(got) != 2 {
		t.Errorf("asN selector matched %d peers, want 2", len(got))
	}
	if got := filterPeersByPolicySelector(peers, "nope"); got != nil {
		t.Errorf("unknown selector = %v, want nil", got)
	}
}

func TestToFilterRefs(t *testing.T) {
	if got := toFilterRefs(nil); got != nil {
		t.Errorf("empty = %v, want nil", got)
	}
	refs := toFilterRefs([]string{"community:in-filter", "inactive:aspath:out-filter"})
	if len(refs) != 2 {
		t.Fatalf("len = %d, want 2", len(refs))
	}
	// "type:name" keeps the name after the colon; canonical is preserved.
	if refs[0].Name != "in-filter" || refs[0].Canonical != "community:in-filter" {
		t.Errorf("ref0 = %+v, want {in-filter, community:in-filter}", refs[0])
	}
	// "inactive:type:name" keeps the inactive marker in the display name.
	if refs[1].Name != "inactive:out-filter" || refs[1].Canonical != "inactive:aspath:out-filter" {
		t.Errorf("ref1 = %+v, want {inactive:out-filter, inactive:aspath:out-filter}", refs[1])
	}
}
