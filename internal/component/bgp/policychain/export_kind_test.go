// Design: docs/architecture/core-design.md -- route-aware export disposition.
package policychain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// TestUnconfiguredExportChainUsesRouteKind proves automatic chain members do
// not admit an ordinary learned route, while explicit output and imports keep
// their defaults. Source facts deliberately disagree with the explicit kind.
func TestUnconfiguredExportChainUsesRouteKind(t *testing.T) {
	reg, err := policy.NewRegistry(policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 7)})
	require.NoError(t, err)
	chain, err := CompileUnconfigured(testAttachment, reg, []Member{{Name: "lp"}}, Bases{})
	require.NoError(t, err)
	wire := update(t, localPref(100))
	for _, test := range []struct {
		name                  string
		internal, sourceKnown bool
		kind                  filterapi.ExportKind
		want                  OutcomeAction
	}{
		{"internal learned without source facts", true, false, filterapi.ExportOrdinaryLearned, OutcomeReject},
		{"internal learned with source facts", true, true, filterapi.ExportOrdinaryLearned, OutcomeReject},
		{"internal explicit without source facts", true, false, filterapi.ExportExplicit, OutcomeAccept},
		{"internal explicit with source facts", true, true, filterapi.ExportExplicit, OutcomeAccept},
		{"external learned", false, true, filterapi.ExportOrdinaryLearned, OutcomeReject},
		{"external explicit", false, false, filterapi.ExportExplicit, OutcomeReject},
		{"internal unknown", true, false, filterapi.ExportUnknown, OutcomeReject},
		{"internal invalid", true, true, filterapi.ExportKind(255), OutcomeReject},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer := filterapi.PeerFilterInfo{IsIBGP: test.internal}
			session := policyview.ExportSession(peer, peer, test.sourceKnown, test.kind)
			var view policyview.View
			var sets RunSets
			view.Reset(wire, session)
			defer view.Clear()
			result := chain.Run(&view, &sets)
			require.Equal(t, test.want, result.Action)
			require.False(t, result.Failed, "the absent-policy default is a decision, not a filter failure")
			require.Equal(t, test.want == OutcomeReject, sets.Folded().Empty(), "rejected output must retain no filter sets")
		})
	}
}

// TestConfiguredExportChainRefusesUnknownKind proves even an accepting policy
// cannot turn absent engine provenance into export authority.
func TestConfiguredExportChainRefusesUnknownKind(t *testing.T) {
	reg, err := policy.NewRegistry()
	require.NoError(t, err)
	chain, err := Compile(testAttachment, reg, nil, policy.FinalAccept, Bases{})
	require.NoError(t, err)
	for _, kind := range []filterapi.ExportKind{filterapi.ExportUnknown, filterapi.ExportKind(255)} {
		peer := filterapi.PeerFilterInfo{IsIBGP: true}
		var view policyview.View
		var sets RunSets
		view.Reset(update(t, localPref(100)), policyview.ExportSession(peer, peer, true, kind))
		result := chain.Run(&view, &sets)
		view.Clear()
		require.Equal(t, OutcomeReject, result.Action, "unknown kind %d", kind)
		require.False(t, result.Failed)
	}
}

// TestConfiguredExportChainKeepsItsFinal proves either authenticated kind
// follows the explicit final on both internal and external sessions.
func TestConfiguredExportChainKeepsItsFinal(t *testing.T) {
	reg, err := policy.NewRegistry()
	require.NoError(t, err)
	for _, final := range []policy.Final{policy.FinalAccept, policy.FinalReject} {
		chain, err := Compile(testAttachment, reg, nil, final, Bases{})
		require.NoError(t, err)
		for _, internal := range []bool{false, true} {
			for _, kind := range []filterapi.ExportKind{filterapi.ExportOrdinaryLearned, filterapi.ExportExplicit} {
				peer := filterapi.PeerFilterInfo{IsIBGP: internal}
				var view policyview.View
				var sets RunSets
				view.Reset(update(t, localPref(100)), policyview.ExportSession(peer, peer, true, kind))
				result := chain.Run(&view, &sets)
				view.Clear()
				want := OutcomeReject
				if final == policy.FinalAccept {
					want = OutcomeAccept
				}
				require.Equal(t, want, result.Action, "internal=%v kind=%d final=%v", internal, kind, final)
			}
		}
	}
}
