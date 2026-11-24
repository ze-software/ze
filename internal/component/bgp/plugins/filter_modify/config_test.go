// Design: docs/architecture/core-design.md -- route modify filter config parsing
// Related: config.go -- parseModifyDefs, the reader of policy/modify
// Related: config_tx.go -- newModifyConfigTx, the transaction a delivery runs through

package filter_modify

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// bgpSections splits a test tree written as one bgp subtree into the delivery
// the SDK hands the plugin: the bgp section first, then a policy section
// holding the tree's "policy" entry when it has one.
func bgpSections(t *testing.T, cfg map[string]any) []sdk.ConfigSection {
	t.Helper()
	bgpCfg := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if k == policy.RootName {
			continue
		}
		bgpCfg[k] = v
	}
	data, err := json.Marshal(map[string]any{configRootBGP: bgpCfg})
	require.NoError(t, err)
	sections := []sdk.ConfigSection{{Root: configRootBGP, Data: string(data)}}
	policyCfg, ok := cfg[policy.RootName].(map[string]any)
	if !ok {
		return sections
	}
	data, err = json.Marshal(map[string]any{policy.RootName: policyCfg})
	require.NoError(t, err)
	return append(sections, sdk.ConfigSection{Root: policy.RootName, Data: string(data)})
}

// attributeDefaultsTree wraps one bgp { defaults { attribute { } } } block in
// the config subtree the plugin is delivered, beside an INC-LP modifier that
// increments local-preference by 50. The plugin reads only the policy half:
// the base is the engine's (bgp/config parseAttributeBases). extraModifiers is
// merged into the policy block, so a test can add a definition the parser
// refuses.
func attributeDefaultsTree(localPreference string, extraModifiers map[string]any) map[string]any {
	return incrementTree(localPreference, 50, extraModifiers)
}

// incrementTree is attributeDefaultsTree with the INC-LP increment given.
func incrementTree(localPreference string, increment float64, extraModifiers map[string]any) map[string]any {
	modifiers := map[string]any{
		"INC-LP": map[string]any{
			"increment": map[string]any{localPreferenceAttr: increment},
		},
	}
	maps.Copy(modifiers, extraModifiers)

	return map[string]any{
		"defaults": map[string]any{
			"attribute": map[string]any{localPreferenceAttr: localPreference},
		},
		"policy": map[string]any{"modify": modifiers},
	}
}

// incLPAnswer answers the sets the running INC-LP definition gives a route.
func incLPAnswer(t *testing.T) []string {
	t.Helper()
	route := modifyRoute("INC-LP", directionImport, `{}`)
	out, err := evaluateModify(&route)
	require.NoError(t, err)
	require.Equal(t, sdk.FilterModify, out.Action)
	return setsText(out.Sets)
}

// TestModifyReloadTakesEffect covers AC-7's plugin half. A reload is one more
// delivery of the config subtree, so a changed modifier governs the routes
// that arrive after the apply and nothing else, and a rollback reinstalls the
// replaced definitions. The base half (a reload moving the absent-attribute
// base) is the engine's: the chain compile reads bgp defaults attribute on
// each load (bgp/config TestParseAttributeBases).
func TestModifyReloadTakesEffect(t *testing.T) {
	previousDefs := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previousDefs) })
	tx := newModifyConfigTx()

	require.NoError(t, tx.Configure(bgpSections(t, incrementTree("100", 50, nil))))
	require.Equal(t, []string{"local-preference increment 50"}, incLPAnswer(t))

	require.NoError(t, tx.Verify(bgpSections(t, incrementTree("100", 30, nil))))
	require.Equal(t, []string{"local-preference increment 50"}, incLPAnswer(t), "verify installed the candidate before apply")
	require.NoError(t, tx.Apply())
	require.Equal(t, []string{"local-preference increment 30"}, incLPAnswer(t), "the reload installed the new modifier")

	// And back, because a reload that only ever moves one way would pass the
	// two lines above with a definition that latched on the first delivery.
	require.NoError(t, tx.Verify(bgpSections(t, incrementTree("100", 50, nil))))
	require.NoError(t, tx.Apply())
	require.Equal(t, []string{"local-preference increment 50"}, incLPAnswer(t), "the modifier follows every delivery")

	// A rollback reinstalls the replaced definitions.
	require.NoError(t, tx.Rollback())
	require.Equal(t, []string{"local-preference increment 30"}, incLPAnswer(t), "rollback restored the replaced modifier")
}

// TestModifyRefusedConfigInstallsNothing covers the failure half of one
// delivery. A subtree the plugin cannot parse leaves the previous
// configuration running rather than half of the new one.
func TestModifyRefusedConfigInstallsNothing(t *testing.T) {
	previousDefs := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previousDefs) })
	tx := newModifyConfigTx()

	require.NoError(t, tx.Configure(bgpSections(t, incrementTree("80", 30, nil))))

	// The INC-LP increment moves too, so a delivery that installed the good
	// definition and then refused the bad one is caught below.
	refused := incrementTree("42", 50, map[string]any{
		"BAD": map[string]any{"increment": map[string]any{"no-such-attribute": float64(1)}},
	})
	require.Error(t, tx.Verify(bgpSections(t, refused)))
	require.Equal(t, []string{"local-preference increment 30"}, incLPAnswer(t), "the accepted delivery is still the running one")
}

// TestReadOptionalUint32RefusesAnIntAboveUint32 covers the coercion every
// numeric leaf of a modify definition passes through.
//
// VALIDATES: each numeric form the config tree can carry is refused above the
// uint32 range rather than truncated, and 4294967295 itself is accepted.
//
// PREVENTS: the int branch reading its lower bound alone. int is 64 bits here,
// so 4294967296 wrapped to 0 and reported true. Restore the missing upper
// bound check and the first subtest fails.
func TestReadOptionalUint32RefusesAnIntAboveUint32(t *testing.T) {
	const aboveRange = 4294967296

	refused := map[string]any{
		"int":     aboveRange,
		"int64":   int64(aboveRange),
		"uint64":  uint64(aboveRange),
		"float64": float64(aboveRange),
		"string":  "4294967296",
	}
	for name, value := range refused {
		t.Run(name, func(t *testing.T) {
			got, ok := readOptionalUint32(value)
			require.False(t, ok, "a value above the range must be refused, not truncated")
			require.Zero(t, got)
		})
	}

	// The top of the range is a value an operator can write, so the bound above
	// must not have moved it.
	top, ok := readOptionalUint32(int(4294967295))
	require.True(t, ok)
	require.Equal(t, uint32(4294967295), top)
}

// TestModifyNextHopUnionAndImportRefusal covers AC-16 on the filter-modify plugin.
//
// VALIDATES: set { next-hop } accepts explicit IPv4, explicit global IPv6, and
// the export-only rewriting modes (self, unchanged, auto); refuses link-local,
// loopback IPv6 (::1), unspecified, multicast, zoned, IPv4-mapped IPv6, and
// unparseable values with actionable errors; marks egress-only modes on
// modifyDef; and refuses export-only modes on import routes in evaluateModify
// as defense in depth while keeping explicit IPv4 and global IPv6 usable on
// both import and export.
//
// PREVENTS: IPv6 next-hop writes being rejected at config load, non-global IPv6
// forms reaching the wire, or export-only rewriting modes silently running on an
// import chain.
func TestModifyNextHopUnionAndImportRefusal(t *testing.T) {
	t.Run("accepted_values_and_filter_sets", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			value       string
			wantSet     string
			wantDelta   string
			egressOnly  bool
			allowImport bool
		}{
			{"ipv4", "192.0.2.1", `next-hop set "192.0.2.1"`, "next-hop 192.0.2.1", false, true},
			{"ipv6_global", "2001:db8::1", `next-hop set "2001:db8::1"`, "next-hop 2001:db8::1", false, true},
			{"ipv6_ula", "fd00::1", `next-hop set "fd00::1"`, "next-hop fd00::1", false, true},
			{"mode_self", nextHopModeSelf, `next-hop set "self"`, "next-hop self", true, false},
			{"mode_unchanged", nextHopModeUnchanged, `next-hop set "unchanged"`, "next-hop unchanged", true, false},
			{"mode_auto", nextHopModeAuto, `next-hop set "auto"`, "next-hop auto", true, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				instance := map[string]any{"set": map[string]any{nextHopAttr: tc.value}}
				def := parseOne(t, "NH", instance)
				wantSets(t, def.sets, tc.wantSet)
				require.Equal(t, tc.wantDelta, def.delta)
				require.Equal(t, tc.egressOnly, isEgressOnlyNextHop(tc.value))
				if tc.egressOnly {
					require.Equal(t, tc.value, def.egressOnlyNextHop)
				} else {
					require.Empty(t, def.egressOnlyNextHop)
				}

				defs := map[string]*modifyDef{"NH": def}
				exportOut, err := evaluate(t, defs, modifyRoute("NH", directionExport, `{}`))
				require.NoError(t, err)
				require.Equal(t, sdk.FilterModify, exportOut.Action)
				wantSets(t, exportOut.Sets, tc.wantSet)

				importOut, err := evaluate(t, defs, modifyRoute("NH", directionImport, `{}`))
				if tc.allowImport {
					require.NoError(t, err)
					require.Equal(t, sdk.FilterModify, importOut.Action)
					wantSets(t, importOut.Sets, tc.wantSet)
					return
				}
				require.Error(t, err)
				require.Contains(t, err.Error(), "export-only")
				require.Contains(t, err.Error(), tc.value)
			})
		}
	})

	t.Run("conditional_egress_only_mode_refused_on_import_even_when_unmatched", func(t *testing.T) {
		const condFilter = "COND-SELF"
		def := parseOne(t, condFilter, map[string]any{
			"match": map[string]any{"community": "65001:666"},
			"set":   map[string]any{nextHopAttr: nextHopModeSelf},
		})
		defs := map[string]*modifyDef{condFilter: def}

		_, err := evaluate(t, defs, modifyRoute(condFilter, directionImport, `{"community":["65001:100"]}`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "export-only")

		exportMiss, err := evaluate(t, defs, modifyRoute(condFilter, directionExport, `{"community":["65001:100"]}`))
		require.NoError(t, err)
		require.Equal(t, sdk.FilterAccept, exportMiss.Action)

		exportHit, err := evaluate(t, defs, modifyRoute(condFilter, directionExport, `{"community":["65001:666"]}`))
		require.NoError(t, err)
		require.Equal(t, sdk.FilterModify, exportHit.Action)
		wantSets(t, exportHit.Sets, `next-hop set "self"`)
	})

	t.Run("invalid_next_hops_refused", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			value       string
			wantSubstr  string
			wantLinkLoc bool
		}{
			{"ipv6_link_local", "fe80::1", "link-local", true},
			{"ipv6_loopback", "::1", "global unicast IPv6", false},
			{"ipv6_zoned_global", "2001:db8::1%eth0", "zone", false},
			{"ipv6_zoned_link_local", "fe80::1%eth0", "zone", false},
			{"ipv6_unspecified", "::", "unspecified", false},
			{"ipv4_unspecified", "0.0.0.0", "unspecified", false},
			{"ipv6_multicast", "ff02::1", "multicast", false},
			{"ipv4_multicast", "224.0.0.1", "multicast", false},
			{"ipv4_mapped_ipv6", "::ffff:192.0.2.1", "IPv4-mapped", false},
			{"unparseable", "not-an-address", "not a valid IP address or mode", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := parseModifyDefs(map[string]any{"modify": map[string]any{
					"BAD": map[string]any{"set": map[string]any{nextHopAttr: tc.value}},
				}})
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantSubstr)
				if tc.wantLinkLoc {
					require.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
				}
			})
		}
	})
}
