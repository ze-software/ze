// Design: docs/architecture/core-design.md -- BGP implied-filter seam tests (AC-46, S43).
// Related: implied_filters.go -- collectBGPImpliedFilters and ImpliedFiltersFromPeers.
// Related: filter_insertion.go -- anyPeerHolds and insertForPeer provenance.
// Related: peers.go -- prependDefaultFilters and appendDefaultExportFilters.

package bgpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/role"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	_ "github.com/ze-software/ze/internal/component/bgp/reactor/filter"
	_ "github.com/ze-software/ze/internal/component/bgp/reactor/filter/yang"
	"github.com/ze-software/ze/internal/component/bgp/reactor/loopdetect"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

const (
	testPositionHead       = "head"
	testPositionTail       = "tail"
	testRoleImportTrigger  = "bgp > peer edge1 > role import"
	testOTCIngressRef      = "bgp-role:otc-ingress"
	testOTCEgressRef       = "bgp-role:otc-egress"
	testBuiltinLoopReason  = "built-in loop-detection step"
	testDefaultNextHopBy   = "RFC 4271 default next-hop"
	testPeerPathEdge1      = "/bgp/peer/edge1"
	testPeerPathLab        = "/bgp/peer/lab"
	testPeerPathT1         = "/bgp/peer/t1"
	testPeerPathT2         = "/bgp/peer/t2"
	testRolePluginName     = "bgp-role"
	testRFC9234ReasonToken = "RFC 9234"
)

const s43FixtureTemplate = `
%POLICY%
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	peer edge1 {
		connection { remote { ip 192.0.2.1; } local { ip auto; } }
		session { asn { remote 65001; } }
		%EDGE1_ROLE%
	}
	peer lab {
		connection { remote { ip 192.0.2.2; } local { ip auto; } }
		session { asn { remote 65002; } }
		%LAB_FILTER%
	}
	peer t1 {
		connection { remote { ip 192.0.2.3; } local { ip auto; } }
		session { asn { remote 65003; } }
		filter { export { filters [ next-hop:auto ]; final accept; } }
	}
	peer t2 {
		connection { remote { ip 192.0.2.4; } local { ip auto; } }
		session { asn { remote 65004; } }
	}
}
`

func buildS43Tree(t *testing.T, policyBlock, edge1Role, labFilter string) *config.Tree {
	t.Helper()
	cfg := strings.NewReplacer(
		"%POLICY%", policyBlock,
		"%EDGE1_ROLE%", edge1Role,
		"%LAB_FILTER%", labFilter,
	).Replace(s43FixtureTemplate)
	tree, err := config.ParseTreeWithYANG(cfg, nil)
	require.NoError(t, err)
	return tree
}

func useOnlyRolePlugin(t *testing.T) {
	t.Helper()
	registry.SetRuntimeLoadedPlugins(func() []string { return []string{testRolePluginName} })
	t.Cleanup(func() { registry.SetRuntimeLoadedPlugins(nil) })
}

func impliedByTargetAndDir(members []config.ImpliedFilterMember, targetPath, direction string) []config.ImpliedFilterMember {
	var out []config.ImpliedFilterMember
	for _, m := range members {
		if m.TargetPath == targetPath {
			if m.Direction == direction {
				out = append(out, m)
			}
		}
	}
	return out
}

// TestImpliedShowsInsertedFilterMembers verifies S43 across built-in import
// loop-detection, per-peer and any-peer bgp-role FilterInsertions, and
// trailing export next-hop:auto suppression.
//
// VALIDATES: collectBGPImpliedFilters and config.BuildImpliedReport surface
// loop-detection at every import head, bgp-role:otc-ingress on edge1 import,
// any-peer bgp-role:otc-egress on all four peers' export heads naming edge1's
// role import line with Required=true and its RFC 9234 reason, and trailing
// next-hop:auto only on peers without a written next-hop built-in; removing
// edge1's role removes both bgp-role members from every peer while keeping
// loop-detection and trailing next-hop:auto.
// PREVENTS: missing built-in loop-detection or next-hop:auto in `| implied`,
// duplicate implied copies of written next-hop steps, or any-peer insertions
// losing the placing peer's config path and required reason.
func TestImpliedShowsInsertedFilterMembers(t *testing.T) {
	useOnlyRolePlugin(t)

	labExport := "filter { export { filters [ next-hop:unchanged ]; final accept; } }"
	tree := buildS43Tree(t, "", "role { import customer; }", labExport)

	members, err := collectBGPImpliedFilters(tree)
	require.NoError(t, err)

	report, err := config.BuildImpliedReport(tree, config.ImpliedOptions{})
	require.NoError(t, err)
	assert.ElementsMatch(t, members, report.Filters, "BuildImpliedReport must include every member from the registered bgp provider")

	allPeers := []string{testPeerPathEdge1, testPeerPathLab, testPeerPathT1, testPeerPathT2}
	for _, path := range allPeers {
		imp := impliedByTargetAndDir(members, path, directionImport)
		require.NotEmpty(t, imp, "%s import must include built-in loop-detection", path)
		assert.Equal(t, config.ImpliedFilterMember{
			TargetPath: path,
			Direction:  directionImport,
			Position:   testPositionHead,
			Ref:        loopdetect.FilterType,
			ImpliedBy:  testBuiltinLoopReason,
		}, imp[0], "%s import must list built-in loop-detection first", path)

		if path == testPeerPathEdge1 {
			require.Len(t, imp, 2, "edge1 import must list loop-detection then bgp-role:otc-ingress")
			assert.Equal(t, testPositionHead, imp[1].Position)
			assert.Equal(t, testOTCIngressRef, imp[1].Ref)
			assert.Equal(t, testRoleImportTrigger, imp[1].ImpliedBy)
			assert.True(t, imp[1].Required, "bgp-role:otc-ingress must be marked required")
			assert.Contains(t, imp[1].Reason, testRFC9234ReasonToken)
		} else {
			require.Len(t, imp, 1, "%s import must carry only built-in loop-detection", path)
		}

		exp := impliedByTargetAndDir(members, path, directionExport)
		require.NotEmpty(t, exp, "%s export must include any-peer bgp-role:otc-egress", path)
		assert.Equal(t, testPositionHead, exp[0].Position)
		assert.Equal(t, testOTCEgressRef, exp[0].Ref)
		assert.Equal(t, testRoleImportTrigger, exp[0].ImpliedBy, "%s export otc-egress must name edge1's role import path", path)
		assert.True(t, exp[0].Required, "%s export otc-egress must be marked required", path)
		assert.Contains(t, exp[0].Reason, testRFC9234ReasonToken)

		switch path {
		case testPeerPathEdge1, testPeerPathT2:
			require.Len(t, exp, 2, "%s export must list otc-egress at head and next-hop:auto at tail", path)
			assert.Equal(t, config.ImpliedFilterMember{
				TargetPath: path,
				Direction:  directionExport,
				Position:   testPositionTail,
				Ref:        policychain.BuiltinNextHopAuto,
				ImpliedBy:  testDefaultNextHopBy,
			}, exp[1])
		case testPeerPathLab, testPeerPathT1:
			require.Len(t, exp, 1, "%s has a written next-hop built-in and must not get an implied next-hop:auto", path)
		}
	}

	t.Run("removing_role_removes_role_members_on_all_peers", func(t *testing.T) {
		noRoleTree := buildS43Tree(t, "", "", labExport)
		noRoleMembers, collectErr := collectBGPImpliedFilters(noRoleTree)
		require.NoError(t, collectErr)

		for _, m := range noRoleMembers {
			assert.NotEqual(t, testOTCIngressRef, m.Ref, "otc-ingress must disappear once role is removed")
			assert.NotEqual(t, testOTCEgressRef, m.Ref, "otc-egress must disappear from all peers once role is removed")
		}
		for _, path := range allPeers {
			imp := impliedByTargetAndDir(noRoleMembers, path, directionImport)
			require.Len(t, imp, 1, "%s import must keep built-in loop-detection", path)
			assert.Equal(t, loopdetect.FilterType, imp[0].Ref)
		}
		assert.Len(t, impliedByTargetAndDir(noRoleMembers, testPeerPathEdge1, directionExport), 1)
		assert.Equal(t, policychain.BuiltinNextHopAuto, impliedByTargetAndDir(noRoleMembers, testPeerPathEdge1, directionExport)[0].Ref)
		assert.Empty(t, impliedByTargetAndDir(noRoleMembers, testPeerPathLab, directionExport))
		assert.Empty(t, impliedByTargetAndDir(noRoleMembers, testPeerPathT1, directionExport))
		assert.Len(t, impliedByTargetAndDir(noRoleMembers, testPeerPathT2, directionExport), 1)
		assert.Equal(t, policychain.BuiltinNextHopAuto, impliedByTargetAndDir(noRoleMembers, testPeerPathT2, directionExport)[0].Ref)
	})

	t.Run("nil_and_bgp_absent_trees_return_nil", func(t *testing.T) {
		gotNil, nilErr := collectBGPImpliedFilters(nil)
		require.NoError(t, nilErr)
		assert.Nil(t, gotNil)

		emptyTree := config.NewTree()
		gotEmpty, emptyErr := collectBGPImpliedFilters(emptyTree)
		require.NoError(t, emptyErr)
		assert.Nil(t, gotEmpty)
	})
}

// TestImpliedViewMatchesCompiledFilters verifies AC-46 by proving that for
// every peer in the S43 fixture, combining head implied members, written
// filters, and tail implied members reproduces the exact compiled filter chain
// in execution order.
//
// VALIDATES: head-implied + written + tail-implied equals the compiled import
// and export chains in order for built-in loop-detection, explicit policy
// loop-detection (both inherited and written), plugin insertions, and dynamic
// group templates, and ImpliedFiltersFromPeers matches collectBGPImpliedFilters.
// PREVENTS: implied view order drifting from compiled reactor chain order or
// duplicating explicitly written loop-detection / next-hop steps.
func TestImpliedViewMatchesCompiledFilters(t *testing.T) {
	useOnlyRolePlugin(t)

	combineChain := func(implied []config.ImpliedFilterMember, written []string) []string {
		var head, tail []string
		for _, m := range implied {
			switch m.Position {
			case testPositionHead:
				head = append(head, m.Ref)
			case testPositionTail:
				tail = append(tail, m.Ref)
			}
		}
		out := make([]string, 0, len(head)+len(written)+len(tail))
		out = append(out, head...)
		out = append(out, written...)
		out = append(out, tail...)
		return out
	}

	t.Run("builtin_loop_detection", func(t *testing.T) {
		labExport := "filter { export { filters [ next-hop:unchanged ]; final accept; } }"
		tree := buildS43Tree(t, "", "role { import customer; }", labExport)

		members, err := collectBGPImpliedFilters(tree)
		require.NoError(t, err)

		peers, groups, err := buildPeerPopulations(tree.Clone())
		require.NoError(t, err)
		assert.Equal(t, members, ImpliedFiltersFromPeers(peers, groups),
			"ImpliedFiltersFromPeers must match collectBGPImpliedFilters identically")

		writtenExport := map[string][]string{
			"edge1": nil,
			"lab":   {policychain.BuiltinNextHopUnchanged},
			"t1":    {policychain.BuiltinNextHopAuto},
			"t2":    nil,
		}
		for _, ps := range peers {
			path := impliedTargetPathForPeer(ps)
			expCombined := combineChain(impliedByTargetAndDir(members, path, directionExport), writtenExport[ps.Name])
			assert.Equal(t, refNames(ps.ExportPolicy.Filters()), expCombined,
				"peer %s: implied export view must match compiled ExportPolicy.Filters() in order", ps.Name)

			// Without an explicit policy { loop-detection } stanza, the reactor
			// executes built-in loop-detection at FilterStageProtocol before
			// ps.ImportPolicy.Filters().
			impCombined := combineChain(impliedByTargetAndDir(members, path, directionImport), nil)
			wantImport := append([]string{loopdetect.FilterType}, refNames(ps.ImportPolicy.Filters())...)
			assert.Equal(t, wantImport, impCombined,
				"peer %s: implied import view must match protocol loop-detection + ImportPolicy.Filters() in order", ps.Name)
		}
	})

	t.Run("explicit_policy_loop_detection", func(t *testing.T) {
		policyBlock := "policy { loop-detection LOOPS {} }"
		labFilter := `filter {
			import { filters [ LOOPS ]; final accept; }
			export { filters [ next-hop:unchanged ]; final accept; }
		}`
		tree := buildS43Tree(t, policyBlock, "role { import customer; }", labFilter)

		members, err := collectBGPImpliedFilters(tree)
		require.NoError(t, err)

		peers, _, err := buildPeerPopulations(tree.Clone())
		require.NoError(t, err)

		writtenImport := map[string][]string{
			"edge1": nil,
			"lab":   {"loop:LOOPS"},
			"t1":    nil,
			"t2":    nil,
		}
		for _, ps := range peers {
			path := impliedTargetPathForPeer(ps)
			impImplied := impliedByTargetAndDir(members, path, directionImport)
			if ps.Name == "lab" {
				assert.Empty(t, impImplied, "peer lab explicitly wrote LOOPS and must have no implied loop-detection copy")
			}
			impCombined := combineChain(impImplied, writtenImport[ps.Name])
			assert.Equal(t, refNames(ps.ImportPolicy.Filters()), impCombined,
				"peer %s: implied import view + written must equal compiled ImportPolicy.Filters() in order", ps.Name)
		}
	})

	t.Run("dynamic_group_template", func(t *testing.T) {
		const dynCfg = `
bgp {
	router-id 192.0.2.254;
	session { asn { local 65000; } }
	group edges {
		connection {
			remote { ip dynamic; connect false; range 198.51.100.0/24; }
			local { ip 198.51.100.1; accept true; }
		}
		role { import customer; }
	}
	peer t2 {
		connection { remote { ip 192.0.2.4; } local { ip auto; } }
		session { asn { remote 65004; } }
	}
}
`
		tree, err := config.ParseTreeWithYANG(dynCfg, nil)
		require.NoError(t, err)

		members, err := collectBGPImpliedFilters(tree)
		require.NoError(t, err)

		peers, groups, err := buildPeerPopulations(tree.Clone())
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, members, ImpliedFiltersFromPeers(peers, groups))

		const groupPath = "/bgp/group/edges"
		const groupTrigger = "bgp > group edges > role import"
		groupImport := impliedByTargetAndDir(members, groupPath, directionImport)
		require.Len(t, groupImport, 2)
		assert.Equal(t, loopdetect.FilterType, groupImport[0].Ref)
		assert.Equal(t, testOTCIngressRef, groupImport[1].Ref)
		assert.Equal(t, groupTrigger, groupImport[1].ImpliedBy)
		assert.True(t, groupImport[1].Required)

		t2Export := impliedByTargetAndDir(members, testPeerPathT2, directionExport)
		require.Len(t, t2Export, 2)
		assert.Equal(t, testOTCEgressRef, t2Export[0].Ref)
		assert.Equal(t, groupTrigger, t2Export[0].ImpliedBy)
		assert.Equal(t, policychain.BuiltinNextHopAuto, t2Export[1].Ref)

		var t2 *reactor.PeerSettings
		for _, ps := range peers {
			if ps.Name == "t2" {
				t2 = ps
			}
		}
		require.NotNil(t, t2)
		assert.Equal(t, refNames(t2.ExportPolicy.Filters()), combineChain(t2Export, nil))
	})
}
