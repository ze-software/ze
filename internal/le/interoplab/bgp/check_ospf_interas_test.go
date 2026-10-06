// Design: docs/architecture/testing/interop.md -- inter-AS oracle boundary cases.
// Related: check_ospf_interas.go -- source schema and FRR flooding predicates.
package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

// Literal schemas follow Ze's databaseOpaqueWithTEDecode and FRR 10.3.1's
// show_lsa_detail. These test the oracle, not daemon interoperability.
const ospfInterASSourceJSON = `[
 {"as-opaque":[{"type":"opaque-as","link-state-id":"6.0.0.7","advertising-router":"172.30.73.2","age":12,"checksum":4660,"length":80}]},
 {"te":[{"advertising-router":"172.30.73.2","scope":"as","instance":7,"link-type":"point-to-point","remote-as":65001,"remote-asbr-ipv4":"203.0.113.9","remote-asbr-ipv6":"2001:db8::9"}]}
]`

const ospfInterASFloodedJSON = `{"routerId":"172.30.73.3","asExternalOpaqueLsa":[{"linkStateId":"6.0.0.7","advertisingRouter":"172.30.73.2","lsaAge":13,"checksum":"1234","length":80}]}`

// A valid structured link needs no incidental "inter-as" label. The header and
// decoded body must identify the same configured link on a nondefault network.
func TestOSPFInterASSourceOracle(t *testing.T) {
	local := netip.MustParseAddr("172.30.73.2")
	if _, err := ospfInterASSourceVerdict(ospfInterASSourceJSON, local); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"wrong-scope", `"scope":"as"`, `"scope":"area"`},
		{"wrong-lsa-type", `"type":"opaque-as"`, `"type":"opaque-area"`},
		{"wrong-opaque-type", `"6.0.0.7"`, `"1.0.0.7"`},
		{"wrong-instance", `"instance":7`, `"instance":8`},
		{"wrong-source", `"advertising-router":"172.30.73.2"`, `"advertising-router":"172.30.73.3"`},
		{"wrong-as", `"remote-as":65001`, `"remote-as":65002`},
		{"missing-as", `"remote-as":65001,`, ``},
		{"wrong-v4", `"203.0.113.9"`, `"203.0.113.8"`},
		{"missing-v6", `,"remote-asbr-ipv6":"2001:db8::9"`, ``},
		{"wrong-v6", `"2001:db8::9"`, `"2001:db8::8"`},
		{"withdrawn", `"age":12`, `"age":3600`},
		{"missing-age", `"age":12,`, ``},
		{"missing-checksum", `"checksum":4660,`, ``},
		{"no-body", `"length":80`, `"length":20`},
		{"forbidden-link-id", `"link-type":"point-to-point"`, `"link-type":"point-to-point","link-id":"0.0.0.0"`},
		{"split-link-fields", `"remote-as":65001,`, `"remote-as":65001},{"advertising-router":"172.30.73.2","scope":"as","instance":7,`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(ospfInterASSourceJSON, tc.old, tc.replacement, 1)
			if _, err := ospfInterASSourceVerdict(input, local); err == nil {
				t.Fatal("accepted unrelated, incomplete or withdrawn inter-AS evidence")
			}
		})
	}
	for _, input := range []string{`null`, `[]`, `[{"note":"inter-as"}]`, `{`, `[{"as-opaque":[]},{"te":[]}]`} {
		if _, err := ospfInterASSourceVerdict(input, local); err == nil {
			t.Fatalf("accepted absent or malformed source: %s", input)
		}
	}
}

// The foreign database must hold the same live LSA, not merely print Ze's ID
// in another row or its own router header.
func TestOSPFInterASFloodedOracle(t *testing.T) {
	header, err := ospfInterASSourceVerdict(ospfInterASSourceJSON, netip.MustParseAddr("172.30.73.2"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ospfInterASFloodedVerdict(ospfInterASFloodedJSON, header); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"wrong-scope", `asExternalOpaqueLsa`, `areaLocalOpaqueLsa`},
		{"wrong-id", `6.0.0.7`, `6.0.0.8`},
		{"wrong-originator", `172.30.73.2`, `172.30.73.3`},
		{"withdrawn", `"lsaAge":13`, `"lsaAge":3600`},
		{"missing-age", `"lsaAge":13,`, ``},
		{"different-body", `"checksum":"1234"`, `"checksum":"1235"`},
		{"invalid-checksum", `"checksum":"1234"`, `"checksum":"no"`},
		{"missing-checksum", `"checksum":"1234",`, ``},
		{"different-length", `"length":80`, `"length":84`},
		{"split-header", `"advertisingRouter"`, `"unused":0},{"advertisingRouter"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(ospfInterASFloodedJSON, tc.old, tc.replacement, 1)
			if err := ospfInterASFloodedVerdict(input, header); err == nil {
				t.Fatal("accepted unrelated, incomplete or withdrawn flooded evidence")
			}
		})
	}
	for _, input := range []string{`null`, `{}`, `{"routerId":"172.30.73.2"}`, `{`} {
		if err := ospfInterASFloodedVerdict(input, header); err == nil {
			t.Fatalf("accepted absent or malformed peer database: %s", input)
		}
	}
}
