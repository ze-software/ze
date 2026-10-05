// Design: docs/architecture/core-design.md -- family and nexthop syntax conversion
// Related: migrate_family.go -- exabgpFamilies, the table under test

package migration

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestExaBGPFamiliesAreRegistered holds every ExaBGP family phrase to a family
// the registry knows.
//
// The goal is that a migrated config never carries a name no Ze surface
// accepts. The method is to render each entry and require a registered name
// back: family.Family.String() falls back to "afi-1/safi-133" for a pair no
// registrar composed, and that fallback is what would reach a config file.
//
// VALIDATES: each AFI and SAFI pair in exabgpFamilies names a registered
// family, and the rendered name round-trips through family.LookupFamily.
// PREVENTS: a silent numeric fallback in a migrated config, and the table
// drifting from the registrars that compose the names it renders.
func TestExaBGPFamiliesAreRegistered(t *testing.T) {
	family.RegisterTestFamilies()

	if len(exabgpFamilies) == 0 {
		t.Fatal("exabgpFamilies is empty, so the migration renames nothing")
	}

	for phrase, fam := range exabgpFamilies {
		name := fam.String()
		if strings.HasPrefix(name, "afi-") {
			t.Errorf("%q renders %q: no registrar composes that AFI and SAFI pair", phrase, name)
			continue
		}
		got, ok := family.LookupFamily(name)
		if !ok {
			t.Errorf("%q renders %q, which the registry does not answer", phrase, name)
			continue
		}
		if got != fam {
			t.Errorf("%q: %v renders %q, which names %v", phrase, fam, name, got)
		}
	}
}

// TestConvertFamilySyntaxUsesRegistryNames pins the four ExaBGP SAFI names that
// differ from Ze's to the spelling the registrars compose.
//
// VALIDATES: convertFamilySyntax answers the registry's name, so ExaBGP
// "flowspec" reaches the config as the FlowSpec registrar's "flow" rather than
// as a second spelling of it.
// PREVENTS: the rename table drifting from the registrar, which would migrate a
// working ExaBGP config into one Ze refuses.
func TestConvertFamilySyntaxUsesRegistryNames(t *testing.T) {
	family.RegisterTestFamilies()

	cases := []struct {
		phrase string
		want   family.Family
	}{
		{"ipv4 flowspec", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}},
		{"ipv6 flowspec", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}},
		{"ipv4 nlri-mpls", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}},
		{"ipv4 mcast-vpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}},
	}
	for _, tc := range cases {
		got := convertFamilySyntax(tc.phrase)
		if got != tc.want.String() {
			t.Errorf("convertFamilySyntax(%q) = %q, want %q", tc.phrase, got, tc.want.String())
		}
	}
}

// TestMigrateImplicitFamilies preserves ExaBGP's all-known-families default by
// migrating omitted, empty, and explicit family blocks independently of fixtures.
func TestMigrateImplicitFamilies(t *testing.T) {
	family.RegisterTestFamilies()
	for _, tc := range []struct {
		name  string
		block string
		all   bool
	}{
		{name: "omitted", all: true},
		{name: "empty", block: "family { }", all: true},
		{name: "explicit", block: "family { ipv4 unicast; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "neighbor 127.0.0.1 {\nlocal-as 65000;\npeer-as 65000;\n" + tc.block + `
static {
	route 10.0.0.0/24 rd 65000:1 next-hop 200.10.0.101 label 1000 split /25;
}
}`
			tree, err := ParseExaBGPConfig(input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := MigrateFromExaBGP(tree)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}
			output := SerializeTree(result.Tree)
			if !strings.Contains(output, "ipv4/mpls-vpn add rd 65000:1 label 1000 10.0.0.0/24") {
				t.Errorf("migration dropped the VPN route:\n%s", output)
			}
			if !strings.Contains(output, "split /25") {
				t.Errorf("migration dropped VPN splitting:\n%s", output)
			}
			for _, name := range family.RegisteredFamilyNames() {
				want := tc.all || name == familyIPv4Unicast
				got := strings.Contains(output, name+" { prefix { maximum 10000; } }")
				if got != want {
					t.Errorf("family %s present = %t, want %t:\n%s", name, got, want, output)
				}
			}
		})
	}
}
