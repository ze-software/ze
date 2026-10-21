// Tests for the FlowSpec redirect and copy actions of an ExaBGP flow route
// `then` block, migrated per the ExaBGP 6.0 grammar
// (exabgp/configuration/grammar/types/flow.py).

package migration

import (
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
)

// migratedExtCommunity matches the extended-community leaf a migrated update carries.
var migratedExtCommunity = regexp.MustCompile(`extended-community \[([^\]]*)\]`)

// migrateFlowThen migrates one IPv4 flow route whose then block holds action,
// and returns the serialized ze config.
func migrateFlowThen(t *testing.T, action string) (string, error) {
	t.Helper()
	input := `
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	flow {
		route one {
			match {
				destination 192.168.0.1/32;
			}
			then {
				` + action + `;
			}
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse %q: %v", action, err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		return "", err
	}
	return SerializeTree(result.Tree), nil
}

// TestMigrateFlowRedirectForms pins, for every ExaBGP 6.0 redirect and copy
// keyword, the community octets ze sends and whether the route carries a next
// hop. ExaBGP 6.0 made `redirect <ip>` and `copy <ip>` the IETF redirect-to-IP
// community (draft-ietf-idr-flowspec-redirect-ip-16 Section 2: the address in
// the community, the C bit for a copy) and moved the older Simpson form
// (draft-simpson-idr-flowspec-redirect-02: 0x0800 plus the address in the
// next hop) to the `-simpson` spellings.
//
// VALIDATES: each keyword migrates to text ze parses into the octets the 6.0
// pin sends, with a next hop only for the Simpson forms.
// PREVENTS: `redirect <ip>` sending the Simpson form a 6.0 peer no longer
// expects, and `copy <ip>` losing its address to the next hop.
func TestMigrateFlowRedirectForms(t *testing.T) {
	cases := []struct {
		action  string
		octets  string // 8-octet communities, hex
		octets6 string // 20-octet IPv6-address-specific communities, hex
		nextHop string // empty: the route carries no next hop
	}{
		{"redirect 1.2.3.4", "010c010203040000", "", ""},
		{"redirect 2001:db8::1", "", "000c20010db8000000000000000000000001" + "0000", ""},
		{"redirect 65500:12345", "8008ffdc00003039", "", ""},
		{"redirect-simpson 1.2.3.4", "0800000000000000", "", "1.2.3.4"},
		{"copy 1.2.3.4", "010c010203040001", "", ""},
		{"copy 2001:db8::1", "", "000c20010db8000000000000000000000001" + "0001", ""},
		{"copy-simpson 1.2.3.4", "0800000000000001", "", "1.2.3.4"},
		{"redirect-to-nexthop", "0800000000000000", "", ""},
		{"redirect-to-nexthop 1.2.3.4", "010c010203040000", "", ""},
		{"redirect-to-nexthop-ietf 1.2.3.4", "010c010203040000", "", ""},
		{"redirect-to-nexthop-ietf 2001:db8::1", "", "000c20010db8000000000000000000000001" + "0000", ""},
		{"redirect-to-nexthop-simpson", "0800000000000000", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			output, err := migrateFlowThen(t, tc.action)
			if err != nil {
				t.Fatalf("migrate %q: %v", tc.action, err)
			}

			match := migratedExtCommunity.FindStringSubmatch(output)
			if match == nil {
				t.Fatalf("migrated %q carries no extended community:\n%s", tc.action, output)
			}
			extComm, err := bgpconfig.ParseExtendedCommunity(match[1])
			if err != nil {
				t.Fatalf("ze refuses %q, migrated from %q: %v", match[1], tc.action, err)
			}
			if got := hex.EncodeToString(extComm.Bytes); got != tc.octets {
				t.Errorf("%q: 8-octet communities = %s, want %s (ze text %q)", tc.action, got, tc.octets, match[1])
			}
			if got := hex.EncodeToString(extComm.IPv6Bytes); got != tc.octets6 {
				t.Errorf("%q: IPv6 communities = %s, want %s (ze text %q)", tc.action, got, tc.octets6, match[1])
			}

			hasNextHop := strings.Contains(output, "next-hop ")
			if tc.nextHop == "" {
				if hasNextHop {
					t.Errorf("%q: the IETF form carries its address in the community, yet the route has a next hop:\n%s", tc.action, output)
				}
				return
			}
			if !strings.Contains(output, "next-hop "+tc.nextHop) {
				t.Errorf("%q: the Simpson form needs next hop %s:\n%s", tc.action, tc.nextHop, output)
			}
		})
	}
}

// TestMigrateFlowRedirectRefusals proves a redirect or copy keyword that needs
// an address and was given something else stops the migration.
//
// VALIDATES: `redirect-simpson`, `copy`, `copy-simpson` and
// `redirect-to-nexthop-ietf` refuse a value that is not an address, and
// `redirect-to-nexthop` refuses a value that is neither empty nor an address.
// PREVENTS: a route announced with a next hop or community the operator did
// not write.
func TestMigrateFlowRedirectRefusals(t *testing.T) {
	for _, action := range []string{
		"redirect-simpson 65500:12345",
		"copy 65500:12345",
		"copy-simpson 65500:12345",
		"redirect-to-nexthop-ietf 65500:12345",
		"redirect-to-nexthop 65500:12345",
	} {
		t.Run(action, func(t *testing.T) {
			output, err := migrateFlowThen(t, action)
			if err == nil {
				t.Fatalf("migrate %q succeeded, want a refusal:\n%s", action, output)
			}
			if !strings.Contains(err.Error(), "65500:12345") {
				t.Errorf("migrate %q: the refusal does not name the value: %v", action, err)
			}
		})
	}
}
