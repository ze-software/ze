// Design: docs/architecture/config/syntax.md -- FlowSpec match criteria and operators
// RFC: rfc/short/rfc8955.md -- numeric operator (Section 4.2.1.1), IP protocol (Section 4.2.2.3)

package flowspec

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestConfigProtocolTakesNumericOperators builds protocol and next-header from
// the config path with every numeric operator, names included, and compares
// the component octets.
//
// VALIDATES: buildFlowSpecComponents reads protocol and next-header with the
// update text grammar (parseNumericTermsText): `>=6&<=17` encodes gt|eq then
// end, AND, lt|eq; `>5` encodes gt; `[ >5 <10 ]` is two OR terms; a protocol
// name stands in for its number under any operator.
// PREVENTS: config refusing a protocol range the update text, the wire and the
// ExaBGP migration all carry, because its parser read `=` and `!=` only.
func TestConfigProtocolTakesNumericOperators(t *testing.T) {
	t.Parallel()
	// Each operator octet is e(0x80) a(0x40) len(0x30) 0 lt(0x04) gt(0x02)
	// eq(0x01), RFC 8955 Section 4.2.1.1. A protocol value is one octet, so len
	// is 0. Type 3 leads the component.
	cases := []struct {
		key   string
		value string
		want  string
	}{
		{flowspec.KwProtocol, ">=6&<=17", "030306c511"},
		{flowspec.KwProtocol, ">5", "038205"},
		{flowspec.KwProtocol, "[ >5 <10 ]", "030205840a"},
		{flowspec.KwProtocol, "!=6", "038606"},
		{flowspec.KwProtocol, "=tcp", "038106"},
		{flowspec.KwProtocol, "udp", "038111"},
		{flowspec.KwNextHeader, "[ >=tcp&<=udp ]", "030306c511"},
		{flowspec.KwNextHeader, "<17", "038411"},
	}
	for _, tc := range cases {
		fs, dropped := buildFlowSpecComponents(map[string][]string{tc.key: {tc.value}}, tc.key == flowspec.KwNextHeader)
		if len(dropped) != 0 {
			t.Errorf("%s %s: refused %v, want it built", tc.key, tc.value, dropped)
			continue
		}
		if len(fs.Components()) != 1 {
			t.Errorf("%s %s: built %d components, want 1", tc.key, tc.value, len(fs.Components()))
			continue
		}
		if got := hex.EncodeToString(fs.Components()[0].Bytes()); got != tc.want {
			t.Errorf("%s %s: component %s, want %s", tc.key, tc.value, got, tc.want)
		}
	}
}

// TestConfigProtocolRefusalNamesTheFix feeds malformed protocol values to the
// config path and asserts each is refused with the criterion and the fix.
//
// VALIDATES: a value that is neither a number nor a protocol name, one past
// 255, a dangling `&` and a leading `&` are each refused, and the refusal names
// the criterion and what to write instead.
// PREVENTS: a refusal that names only the key ("invalid or unrecognized match
// criteria: protocol"), which leaves the operator guessing which value failed.
func TestConfigProtocolRefusalNamesTheFix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value string
		want  string
	}{
		{"=bogus", "tcp"},
		{"256", "255"},
		{">6&", "<op><value>"},
		{"&>6", "drop the &"},
	}
	for _, tc := range cases {
		_, dropped := buildFlowSpecComponents(map[string][]string{flowspec.KwProtocol: {tc.value}}, false)
		if len(dropped) != 1 {
			t.Errorf("protocol %s: dropped %v, want it refused", tc.value, dropped)
			continue
		}
		if !strings.Contains(dropped[0], flowspec.KwProtocol) || !strings.Contains(dropped[0], tc.want) {
			t.Errorf("protocol %s: refusal %q does not name the criterion and %q", tc.value, dropped[0], tc.want)
		}
	}
}

// TestParseConfigRoute_FlowSpecMigratedProtocolRange feeds the config route
// parser the nlri lines `ze exabgp migrate` writes for a protocol and a
// next-header range (TestMigrateFlowProtocolRange pins those lines).
//
// VALIDATES: parseConfigRoute builds a route from `protocol [ >5 <10 ]` and
// `next-header >=6&<=17`, so a migrated ExaBGP range loads.
// PREVENTS: a migrated config ze refuses at load for a form ExaBGP accepts.
func TestParseConfigRoute_FlowSpecMigratedProtocolRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		content string
		isIPv6  bool
	}{
		{"add destination-ipv4 192.168.0.1/32 protocol [ >5 <10 ]", false},
		{"add destination-ipv6 2001:db8::1/128 next-header >=6&<=17", true},
	}
	for _, tc := range cases {
		req := registry.ConfigRouteRequest{
			Content:      strings.Fields(tc.content),
			IsIPv6:       tc.isIPv6,
			ExtCommunity: []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		}
		pr, err := parseConfigRoute(req)
		if err != nil {
			t.Errorf("%s: refused: %v", tc.content, err)
			continue
		}
		if len(pr.NLRI) == 0 {
			t.Errorf("%s: empty FlowSpec NLRI", tc.content)
		}
	}
}
