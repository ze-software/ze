// Design: docs/architecture/plugin/rib-storage-design.md -- family name agreement
// Related: rib_nlri.go -- parseFamily and formatFamily, the two halves under test
// Related: rib_formatfamily_test.go -- the rendering half of the same agreement

package rib

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/family"
)

// TestParseFamilyMatchesRegistry holds parseFamily to the family registry.
//
// The goal is that one family has one name. The method is to read the registry
// itself rather than a list written here, so a family registered after this
// test was written is covered by it and a renamed one breaks it.
//
// VALIDATES: every name family.RegisteredFamilyNames() answers parses back to
// the Family it names, and round-trips through formatFamily unchanged.
// PREVENTS: the hardcoded AFI and SAFI switch that stood in parseFamily coming
// back. It knew six SAFIs, so every plugin-registered family was unreachable
// through the RIB commands, and it spelled the FlowSpec SAFI "flowspec" where
// the registrar spells it "flow".
func TestParseFamilyMatchesRegistry(t *testing.T) {
	family.RegisterTestFamilies()

	names := family.RegisteredFamilyNames()
	if len(names) < 15 {
		t.Fatalf("registry answered %d families, too few to be a real registry", len(names))
	}

	for _, name := range names {
		fam, ok := parseFamily(name)
		if !ok {
			t.Errorf("parseFamily(%q) refused a registered family name", name)
			continue
		}
		if got := formatFamily(fam); got != name {
			t.Errorf("parseFamily/formatFamily round trip: %q became %q", name, got)
		}
	}
}

// TestParseFamilyFlowSpecSpelling reads both spellings at their sources.
//
// The registrar is the only place the FlowSpec family name is declared, and the
// RIB is the surface that reads it back. This test names neither spelling as a
// literal: it takes each name from the value family.MustRegister returned in
// internal/component/bgp/plugins/nlri/flowspec/types.go, and requires the RIB
// to parse that name back to that same value.
//
// VALIDATES: AC-2. The RIB and the family registry answer one spelling.
// PREVENTS: the two drifting again. The RIB read "flowspec" while the
// registrar composed "flow", so the name an operator types was refused and a
// name no family carries was accepted.
func TestParseFamilyFlowSpecSpelling(t *testing.T) {
	registered := []family.Family{
		flowspec.IPv4FlowSpec,
		flowspec.IPv6FlowSpec,
		flowspec.IPv4FlowSpecVPN,
		flowspec.IPv6FlowSpecVPN,
	}

	for _, want := range registered {
		name := want.String()
		got, ok := parseFamily(name)
		if !ok {
			t.Errorf("parseFamily(%q) refused the name its registrar composed", name)
			continue
		}
		if got != want {
			t.Errorf("parseFamily(%q) = %v, want %v", name, got, want)
		}
	}

	// "flowspec" is the spelling the deleted switch carried. No registrar
	// composes it, so the RIB must not accept it.
	if _, ok := parseFamily("ipv4/flowspec"); ok {
		t.Error(`parseFamily("ipv4/flowspec") accepted a name no family carries`)
	}
}
