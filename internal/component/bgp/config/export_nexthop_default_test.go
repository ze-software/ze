// Design: docs/architecture/core-design.md -- Policy Filter Chain, built-in next-hop steps and implied-filter seam
// Related: peers.go -- validateChainNames, prependDefaultFilters, appendDefaultExportFilters
// Related: filter_insertion.go -- insertForPeer, insertionImpliedBy
// Related: library_chains.go -- compileLibraryChain, isImpliedTailAuto
// Related: templated_chain.go -- templatedChain.CompileResolved
// Related: filter_reference_doctor.go -- undefinedFilterRefs

package bgpconfig

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAppendDefaultExportNextHopAuto verifies trailing next-hop:auto insertion,
// RFC 8212 preservation, suppression by explicit built-ins, retention after
// conditional modify filters, and dynamic-group CompileResolved parity (AC-17).
//
// VALIDATES: appendDefaultExportFilters appends next-hop:auto at export tail
// without marking unconfigured policies Configured(), omits it when an active
// built-in next-hop:* is present, keeps it with AutoDefault=true after a
// conditional modify or inactive built-in, and preserves it across dynamic
// group CompileResolved without creating a spurious process state binding.
// PREVENTS: RFC 8212 default-deny regression, duplicate next-hop:auto steps,
// explicit next-hop:auto misclassified as AutoDefault, or dynamic groups
// dropping trailing next-hop:auto.
func TestAppendDefaultExportNextHopAuto(t *testing.T) {
	t.Run("unconfigured_export_preserves_rfc8212", func(t *testing.T) {
		noInsertingPluginLoaded(t)
		const cfg = `
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer ebgp1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
	}
	peer ibgp1 {
		connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
		session { asn { remote 65000; } }
	}
}
`
		tree, err := config.ParseTreeWithYANG(cfg, nil)
		require.NoError(t, err)
		peers, err := PeersFromConfigTree(tree)
		require.NoError(t, err)
		require.Len(t, peers, 2)

		for _, ps := range peers {
			assert.False(t, ps.ExportPolicy.Configured(), "peer %s: trailing next-hop:auto must leave Configured() false", ps.Name)
			assert.Equal(t, refs(policychain.BuiltinNextHopAuto), ps.ExportPolicy.Filters())
			assert.Equal(t, []filterapi.ImpliedFilter{{
				Direction: directionExport,
				Position:  filterapi.ImpliedPositionTail,
				Ref:       policychain.BuiltinNextHopAuto,
				ImpliedBy: "RFC 4271 default next-hop",
			}}, ps.ExportPolicy.Implied())

			if ps.IsEBGP() {
				assert.False(t, ps.ExportPolicy.AcceptsAtEnd(true), "RFC 8212 default-deny must hold on unconfigured eBGP export")
			} else {
				assert.True(t, ps.ExportPolicy.AcceptsAtEnd(false), "unconfigured iBGP export keeps internal accept default")
			}

			chain, ok := ps.ExportPolicy.Library().(*policychain.Chain)
			require.True(t, ok, "peer %s: export chain must compile", ps.Name)
			intent, static := chain.StaticNextHop()
			assert.True(t, static, "peer %s: trailing next-hop:auto alone uses static fast path", ps.Name)
			assert.Equal(t, policychain.NextHopIntent{Mode: policychain.NextHopModeAuto, AutoDefault: true}, intent)
			assert.True(t, chain.HasAutoDefaultNextHop())

			_, owned := filterOwnerProcess(policy.Ref{Name: policychain.BuiltinNextHopAuto}, nil, nil)
			assert.False(t, owned, "built-in next-hop:* must not have an external filter owner")
		}
	})

	t.Run("explicit_builtins_suppress_default", func(t *testing.T) {
		noInsertingPluginLoaded(t)
		cases := []struct {
			name string
			ref  string
			mode policychain.NextHopMode
		}{
			{"self", policychain.BuiltinNextHopSelf, policychain.NextHopModeSelf},
			{"unchanged", policychain.BuiltinNextHopUnchanged, policychain.NextHopModeUnchanged},
			{"auto", policychain.BuiltinNextHopAuto, policychain.NextHopModeAuto},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := `
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter { export { filters [ ` + tc.ref + ` ]; final accept; } }
	}
}
`
				tree, err := config.ParseTreeWithYANG(cfg, nil)
				require.NoError(t, err)
				peers, err := PeersFromConfigTree(tree)
				require.NoError(t, err)
				require.Len(t, peers, 1)

				ep := peers[0].ExportPolicy
				assert.True(t, ep.Configured())
				assert.Equal(t, refs(tc.ref), ep.Filters(), "explicit built-in must not append a duplicate next-hop:auto")
				assert.Empty(t, ep.Implied(), "explicit built-in carries no implied tail entry")

				chain, ok := ep.Library().(*policychain.Chain)
				require.True(t, ok)
				intent, static := chain.StaticNextHop()
				assert.True(t, static)
				assert.Equal(t, policychain.NextHopIntent{Mode: tc.mode, AutoDefault: false}, intent)
				assert.False(t, chain.HasAutoDefaultNextHop(), "explicit %s must not set AutoDefault", tc.ref)
			})
		}
	})

	t.Run("inactive_self_still_appends_auto", func(t *testing.T) {
		noInsertingPluginLoaded(t)
		const cfg = `
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter { export { filters [ inactive:next-hop:self ]; final accept; } }
	}
}
`
		tree, err := config.ParseTreeWithYANG(cfg, nil)
		require.NoError(t, err)
		peers, err := PeersFromConfigTree(tree)
		require.NoError(t, err)
		require.Len(t, peers, 1)

		ep := peers[0].ExportPolicy
		assert.Equal(t, refs("inactive:next-hop:self", policychain.BuiltinNextHopAuto), ep.Filters())
		assert.Len(t, ep.Implied(), 1)
		chain, ok := ep.Library().(*policychain.Chain)
		require.True(t, ok)
		intent, static := chain.StaticNextHop()
		assert.True(t, static)
		assert.Equal(t, policychain.NextHopIntent{Mode: policychain.NextHopModeAuto, AutoDefault: true}, intent)
	})

	t.Run("conditional_modify_retains_trailing_auto", func(t *testing.T) {
		noInsertingPluginLoaded(t)
		const cfg = `
policy {
	modify COND-NH {
		match { community [ 65001:100 ]; }
		set { next-hop unchanged; }
	}
}
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter { export { filters [ COND-NH ]; final accept; } }
	}
}
`
		tree, err := config.ParseTreeWithYANG(cfg, nil)
		require.NoError(t, err)
		peers, err := PeersFromConfigTree(tree)
		require.NoError(t, err)
		require.Len(t, peers, 1)

		ep := peers[0].ExportPolicy
		assert.Equal(t, refs("bgp-filter-modify:COND-NH", policychain.BuiltinNextHopAuto), ep.Filters())
		assert.Equal(t, []filterapi.ImpliedFilter{{
			Direction: directionExport,
			Position:  filterapi.ImpliedPositionTail,
			Ref:       policychain.BuiltinNextHopAuto,
			ImpliedBy: "RFC 4271 default next-hop",
		}}, ep.Implied())

		chain, ok := ep.Library().(*policychain.Chain)
		require.True(t, ok)
		assert.True(t, chain.HasAutoDefaultNextHop())
		_, static := chain.StaticNextHop()
		assert.False(t, static, "chain with a conditional modify member is not static-only")
	})

	t.Run("dynamic_group_compile_resolved_preserves_trailing_auto", func(t *testing.T) {
		registerInserter(t, rpc.FilterInsertion{
			Filter:    "fake-inserter:late",
			Direction: rpc.FilterExport,
			Priority:  10,
			Session:   bilateralEBGP[:1],
		})
		const cfg = `
policy {
	prefix-list ALLOW-LIST-65010 {
		entry 10.0.0.0/8 { action accept; }
	}
	match ALLOW-65010 {
		prefix-list ALLOW-LIST-65010
	}
}
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group dyn-default {
		connection {
			remote { ip dynamic; connect false; range 10.0.0.0/8; }
			local { ip 10.0.0.1; accept true; }
		}
		filter { export { filters [ ALLOW-$remote_as ]; final accept; } }
	}
	group dyn-self {
		connection {
			remote { ip dynamic; connect false; range 10.1.0.0/16; }
			local { ip 10.1.0.1; accept true; }
		}
		filter { export { filters [ ALLOW-$remote_as next-hop:self ]; final accept; } }
	}
}
`
		tree, err := config.ParseTreeWithYANG(cfg, nil)
		require.NoError(t, err)
		_, groups, err := peersAndDynamicGroups(tree)
		require.NoError(t, err)
		require.Len(t, groups, 2)

		byGroup := make(map[string]filterapi.Policy, len(groups))
		for _, dg := range groups {
			byGroup[dg.GroupName] = dg.Settings.ExportPolicy
		}

		// Group with default trailing next-hop:auto + session-conditional insertion.
		dynDefault := byGroup["dyn-default"]
		require.True(t, dynDefault.Templated())
		require.Equal(t, []string{"fake-inserter:late", "ALLOW-$remote_as", policychain.BuiltinNextHopAuto}, refNames(dynDefault.Filters()))

		resolveFor := func(tmpl filterapi.Policy, remoteAS string, facts filterapi.PeerFilterInfo) filterapi.Policy {
			t.Helper()
			resolvedRefs := make([]policy.Ref, len(tmpl.Filters()))
			for i, ref := range tmpl.Filters() {
				resolvedRefs[i] = ref
				resolvedRefs[i].Name = strings.ReplaceAll(ref.Name, "$remote_as", remoteAS)
			}
			compiled, compileErr := tmpl.WithBoundFilters(resolvedRefs, tmpl.Bindings()).CompileResolved(facts)
			require.NoError(t, compileErr)
			return compiled
		}

		// eBGP dynamic member keeps both the session insertion and trailing next-hop:auto.
		ebgpCompiled := resolveFor(dynDefault, "65010", filterapi.PeerFilterInfo{LocalAS: 65000, PeerAS: 65010})
		assert.Equal(t, []string{"fake-inserter:late", "ALLOW-65010", policychain.BuiltinNextHopAuto}, refNames(ebgpCompiled.Filters()))
		ebgpChain, ok := ebgpCompiled.Library().(*policychain.Chain)
		require.True(t, ok)
		assert.True(t, ebgpChain.HasAutoDefaultNextHop())
		assert.Len(t, ebgpCompiled.Implied(), 2)

		// iBGP dynamic member drops the eBGP-only insertion and its implied entry, keeping trailing next-hop:auto.
		ibgpCompiled := resolveFor(dynDefault, "65010", filterapi.PeerFilterInfo{LocalAS: 65000, PeerAS: 65000, IsIBGP: true})
		assert.Equal(t, []string{"ALLOW-65010", policychain.BuiltinNextHopAuto}, refNames(ibgpCompiled.Filters()))
		ibgpChain, ok := ibgpCompiled.Library().(*policychain.Chain)
		require.True(t, ok)
		assert.True(t, ibgpChain.HasAutoDefaultNextHop())
		assert.Equal(t, []filterapi.ImpliedFilter{{
			Direction: directionExport,
			Position:  filterapi.ImpliedPositionTail,
			Ref:       policychain.BuiltinNextHopAuto,
			ImpliedBy: "RFC 4271 default next-hop",
		}}, ibgpCompiled.Implied())

		// Group with explicit next-hop:self has no trailing next-hop:auto after CompileResolved.
		dynSelf := byGroup["dyn-self"]
		selfCompiled := resolveFor(dynSelf, "65010", filterapi.PeerFilterInfo{LocalAS: 65000, PeerAS: 65000, IsIBGP: true})
		assert.Equal(t, []string{"ALLOW-65010", policychain.BuiltinNextHopSelf}, refNames(selfCompiled.Filters()))
		selfChain, ok := selfCompiled.Library().(*policychain.Chain)
		require.True(t, ok)
		assert.False(t, selfChain.HasAutoDefaultNextHop())
		assert.Empty(t, selfCompiled.Implied())
	})
}

// TestInactiveNextHopAutoRefused verifies that deactivating next-hop:auto
// (inline or structurally) and unknown next-hop:<other> names are refused at
// config load (AC-17, D-2).
//
// VALIDATES: inactive:next-hop:auto is refused with guidance pointing to
// next-hop:unchanged, and unknown next-hop:* spellings name the three valid
// built-ins instead of queuing as external plugins.
// PREVENTS: operators disabling the export default via inactive: or typos in
// next-hop:* silently waiting for a nonexistent external plugin.
func TestInactiveNextHopAutoRefused(t *testing.T) {
	noInsertingPluginLoaded(t)

	for _, tc := range []struct {
		name      string
		exportCfg string
		wants     []string
	}{
		{
			name:      "inline_inactive_auto",
			exportCfg: "filters [ inactive:next-hop:auto ]; final accept;",
			wants:     []string{"inactive:next-hop:auto", "next-hop:unchanged"},
		},
		{
			name:      "structural_inactive_auto",
			exportCfg: "filters [ next-hop:auto ]; inactive: filters next-hop:auto; final accept;",
			wants:     []string{"inactive:next-hop:auto", "next-hop:unchanged"},
		},
		{
			name:      "unknown_builtin",
			exportCfg: "filters [ next-hop:bogus ]; final accept;",
			wants:     []string{"next-hop:bogus", "next-hop:self", "next-hop:unchanged", "next-hop:auto"},
		},
		{
			name:      "templated_builtin",
			exportCfg: "filters [ next-hop:$remote_as ]; final accept;",
			wants:     []string{"next-hop:$remote_as", "next-hop:self", "next-hop:unchanged", "next-hop:auto"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := `
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter { export { ` + tc.exportCfg + ` } }
	}
}
`
			tree, err := config.ParseTreeWithYANG(cfg, nil)
			require.NoError(t, err)
			_, err = PeersFromConfigTree(tree)
			require.Error(t, err)
			for _, want := range tc.wants {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// TestImportRefusesEgressNextHopModes verifies that import chains refuse
// built-in next-hop:* references and policy { modify } instances setting
// egress-only next-hop modes, while permitting explicit IPv4 and global IPv6
// modify next-hop filters on import (AC-16, D-4).
//
// VALIDATES: import chains refuse next-hop:self, next-hop:unchanged,
// next-hop:auto (active or inactive) and modify filters setting self,
// unchanged, or auto (plain, modify:, or bgp-filter-modify: qualified), while
// accepting explicit IPv4 and global IPv6 modify filters on import and
// egress-only modify filters on export.
// PREVENTS: egress-only next-hop rewriting modes silently reaching ingress.
func TestImportRefusesEgressNextHopModes(t *testing.T) {
	noInsertingPluginLoaded(t)

	for _, ref := range []string{
		policychain.BuiltinNextHopSelf,
		policychain.BuiltinNextHopUnchanged,
		policychain.BuiltinNextHopAuto,
		"inactive:" + policychain.BuiltinNextHopSelf,
	} {
		t.Run("builtin_import_"+ref, func(t *testing.T) {
			cfg := `
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter { import { filters [ ` + ref + ` ]; final accept; } }
	}
}
`
			tree, err := config.ParseTreeWithYANG(cfg, nil)
			require.NoError(t, err)
			_, err = PeersFromConfigTree(tree)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "export")
		})
	}

	for _, mode := range []string{"self", "unchanged", "auto"} {
		for _, spelling := range []string{"MOD-NH", "modify:MOD-NH", "bgp-filter-modify:MOD-NH"} {
			t.Run("modify_import_"+mode+"_"+spelling, func(t *testing.T) {
				cfg := `
policy {
	modify MOD-NH {
		set { next-hop ` + mode + `; }
	}
}
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter { import { filters [ ` + spelling + ` ]; final accept; } }
	}
}
`
				tree, err := config.ParseTreeWithYANG(cfg, nil)
				require.NoError(t, err)
				_, err = PeersFromConfigTree(tree)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "MOD-NH")
				assert.Contains(t, err.Error(), mode)
				assert.Contains(t, err.Error(), "import")
			})
		}
	}

	t.Run("explicit_ipv4_and_ipv6_modify_permitted_on_import", func(t *testing.T) {
		const cfg = `
policy {
	modify STEER-V4 {
		set { next-hop 192.0.2.55; }
	}
	modify STEER-V6 {
		set { next-hop 2001:db8::55; }
	}
	modify EGRESS-SELF {
		set { next-hop self; }
	}
}
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer p1 {
		connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
		session { asn { remote 65001; } }
		filter {
			import { filters [ STEER-V4 STEER-V6 ]; final accept; }
			export { filters [ EGRESS-SELF ]; final accept; }
		}
	}
}
`
		tree, err := config.ParseTreeWithYANG(cfg, nil)
		require.NoError(t, err)
		peers, err := PeersFromConfigTree(tree)
		require.NoError(t, err)
		require.Len(t, peers, 1)
		assert.Equal(t, refs("bgp-filter-modify:STEER-V4", "bgp-filter-modify:STEER-V6"), peers[0].ImportPolicy.Filters())
		assert.Equal(t, refs("bgp-filter-modify:EGRESS-SELF", policychain.BuiltinNextHopAuto), peers[0].ExportPolicy.Filters())
	})
}

// TestImpliedFilterSeamIncludesEngineAndPluginInsertions verifies the unified
// Policy.Implied() seam across engine loop-detection (import head), plugin
// FilterInsertions (import/export head), and trailing next-hop:auto (export
// tail) (AC-20, D-6).
//
// VALIDATES: every automatic filter member inserted by BGP config loading
// publishes its direction, position, ref, and generic implied-by reason
// through filterapi.Policy.Implied().
// PREVENTS: `| implied` or peer inspection missing engine-inserted steps or
// plugin-declared FilterInsertions.
func TestImpliedFilterSeamIncludesEngineAndPluginInsertions(t *testing.T) {
	registerInserter(t,
		rpc.FilterInsertion{
			Filter:    "fake-inserter:early",
			Direction: rpc.FilterImport,
			Priority:  10,
			Config:    rrClient,
			Values:    []string{"on"},
		},
		rpc.FilterInsertion{
			Filter:    "fake-inserter:late",
			Direction: rpc.FilterExport,
			Priority:  20,
			Config:    rrClient,
			Values:    []string{"on"},
			Scope:     rpc.FilterScopeAnyPeer,
		},
		rpc.FilterInsertion{
			Filter:    "fake-inserter:early",
			Direction: rpc.FilterExport,
			Priority:  30,
			Session:   bilateralEBGP,
		},
	)

	cfg := `
policy {
	loop-detection LOOPS {}
}
` + strings.Replace(insertionConfig, "%A%", "", 1)

	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(cfg)
	require.NoError(t, err)
	peers, _, err := peersAndDynamicGroups(tree)
	require.NoError(t, err)

	byName := make(map[string]filterapi.Policy, len(peers)*2)
	for _, ps := range peers {
		byName[ps.Name+" import"] = ps.ImportPolicy
		byName[ps.Name+" export"] = ps.ExportPolicy
	}

	// Peer A (iBGP, insertion-test mode on): import has loop:LOOPS + fake-inserter:early.
	aImport := byName["A import"].Implied()
	assert.Contains(t, aImport, filterapi.ImpliedFilter{
		Direction: directionImport,
		Position:  filterapi.ImpliedPositionHead,
		Ref:       "loop:LOOPS",
		ImpliedBy: "policy loop-detection",
	})
	assert.Contains(t, aImport, filterapi.ImpliedFilter{
		Direction: directionImport,
		Position:  filterapi.ImpliedPositionHead,
		Ref:       "fake-inserter:early",
		ImpliedBy: "bgp > group G > peer A > insertion-test mode=on",
	})

	// Peer B (eBGP, no insertion-test leaf): export has any-peer plugin insertion,
	// session-conditioned plugin insertion, and trailing next-hop:auto.
	bExport := byName["B export"].Implied()
	assert.Contains(t, bExport, filterapi.ImpliedFilter{
		Direction: directionExport,
		Position:  filterapi.ImpliedPositionHead,
		Ref:       "fake-inserter:late",
		ImpliedBy: "bgp > group G > peer A > insertion-test mode=on",
	})
	assert.Contains(t, bExport, filterapi.ImpliedFilter{
		Direction: directionExport,
		Position:  filterapi.ImpliedPositionHead,
		Ref:       "fake-inserter:early",
		ImpliedBy: "session ibgp=false, rs-client=false",
	})
	assert.Contains(t, bExport, filterapi.ImpliedFilter{
		Direction: directionExport,
		Position:  filterapi.ImpliedPositionTail,
		Ref:       policychain.BuiltinNextHopAuto,
		ImpliedBy: "RFC 4271 default next-hop",
	})
}

// TestBuiltInNextHopDoctorAndCompletion verifies that bgp-filter-references
// doctor check accepts built-in next-hop:* on export only and that the
// policy-filter reference source offers built-in next-hop:* on export Tab
// completion only (AC-20).
//
// VALIDATES: undefinedFilterRefs skips valid built-in next-hop:* on export and
// flags unknown next-hop:<other> or import next-hop:*; policy-filter completion
// offers next-hop:self, next-hop:unchanged, and next-hop:auto on export only,
// excludes egress-only modify instances on import, and ValidateReferences
// matches config-load rules.
// PREVENTS: false doctor-config-reference errors on valid export built-ins, or
// editor completion hiding built-ins on export or leaking them onto import.
func TestBuiltInNextHopDoctorAndCompletion(t *testing.T) {
	t.Run("doctor_accepts_export_builtins_and_flags_invalid", func(t *testing.T) {
		tree := config.NewTree()
		bgp := tree.GetOrCreateContainer("bgp")
		peer := config.NewTree()
		peer.GetOrCreateContainer("filter").GetOrCreateContainer(directionExport).SetSlice("filters", []string{
			policychain.BuiltinNextHopSelf,
			policychain.BuiltinNextHopUnchanged,
			policychain.BuiltinNextHopAuto,
		})
		bgp.AddListEntry("peer", "192.0.2.1", peer)
		assert.Empty(t, filterReferenceDiags(tree), "valid built-in export next-hop:* refs must not trigger doctor errors")

		badExport := config.NewTree()
		badBGP := badExport.GetOrCreateContainer("bgp")
		badBGP.GetOrCreateContainer("filter").GetOrCreateContainer(directionExport).SetSlice("filters", []string{"next-hop:bogus"})
		diags := filterReferenceDiags(badExport)
		require.Len(t, diags, 1)
		assert.Equal(t, codeFilterReferenceUndefined, diags[0].Code)
		assert.Contains(t, diags[0].Message, "next-hop:bogus")

		badImport := config.NewTree()
		badImportBGP := badImport.GetOrCreateContainer("bgp")
		badImportBGP.GetOrCreateContainer("filter").GetOrCreateContainer(directionImport).SetSlice("filters", []string{policychain.BuiltinNextHopSelf})
		diags = filterReferenceDiags(badImport)
		require.Len(t, diags, 1)
		assert.Equal(t, codeFilterReferenceUndefined, diags[0].Code)
		assert.Contains(t, diags[0].Message, policychain.BuiltinNextHopSelf)
	})

	t.Run("policy_filter_completion_and_validation", func(t *testing.T) {
		resolved, err := configyang.DefaultLoader()
		require.NoError(t, err)
		v := configyang.NewValidator(resolved)

		const cfg = `
policy {
	modify STEER-V4 {
		set { next-hop 192.0.2.55; }
	}
	modify EGRESS-SELF {
		set { next-hop self; }
	}
}
bgp {
	peer p1 {
		filter {
			import { filters [ STEER-V4 ]; final accept; }
			export { filters [ next-hop:self next-hop:unchanged next-hop:auto EGRESS-SELF ]; final accept; }
		}
	}
}
`
		tree, err := config.ParseTreeWithYANG(cfg, nil)
		require.NoError(t, err)
		root := tree.ToPluginMap()

		exportNames := v.ReferenceInstances(root, []string{"bgp", "peer", "p1", "filter", directionExport, "filters"})
		for _, want := range []string{
			policychain.BuiltinNextHopSelf,
			policychain.BuiltinNextHopUnchanged,
			policychain.BuiltinNextHopAuto,
			"STEER-V4",
			"EGRESS-SELF",
		} {
			assert.True(t, slices.Contains(exportNames, want), "export completion %v must include %q", exportNames, want)
		}

		importNames := v.ReferenceInstances(root, []string{"bgp", "peer", "p1", "filter", directionImport, "filters"})
		assert.True(t, slices.Contains(importNames, "STEER-V4"), "import completion %v must include explicit-IP modify", importNames)
		for _, forbidden := range []string{
			policychain.BuiltinNextHopSelf,
			policychain.BuiltinNextHopUnchanged,
			policychain.BuiltinNextHopAuto,
			"EGRESS-SELF",
		} {
			assert.False(t, slices.Contains(importNames, forbidden), "import completion %v must exclude %q", importNames, forbidden)
		}

		refused := v.ValidateReferences(root)
		assert.Empty(t, refused, "valid export next-hop:* and import explicit-IP modify must pass ValidateReferences")

		badTree, err := config.ParseTreeWithYANG(`
bgp {
	peer p1 {
		filter {
			import { filters [ next-hop:self ]; final accept; }
		}
	}
}
`, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, v.ValidateReferences(badTree.ToPluginMap()), "import next-hop:self must fail ValidateReferences")
	})
}
