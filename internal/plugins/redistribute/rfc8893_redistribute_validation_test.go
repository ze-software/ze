// RFC 8893 Section 3 conformance test for redistribution policy: proves the
// same rpki-validation-state match primitive works end-to-end inside the
// redistribute orchestrator's real rule compiler and handleBatch dispatcher
// for both BGP and locally originated non-BGP (static) sources.
//
// Related: redistribute.go -- handleBatch, redistRoute, applyBatchOriginAS
// Related: register.go -- redistribute import attachment carrying FieldRPKIValidationState

package redistribute

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// RFC requirement: RFC8893-3-1 positive -- redistribution import policy uses
// the exact same rpki-validation-state match primitive as BGP ingress and
// egress, accepting Valid and NotFound routes and rejecting Invalid routes
// through policy.CompileRouteChain("redistribute import", ...) and handleBatch
// for both BGP per-entry OriginAS and non-BGP (static) routes using the
// speaker's published BGP local AS.
func TestRFC8893RedistributePolicyFiltersByRPKIValidationState(t *testing.T) {
	resetState(t)

	ribpolicy.SetOriginValidator(func(prefix netip.Prefix, originAS uint32) ribpolicy.ValidationState {
		switch prefix {
		case netip.MustParsePrefix("198.51.100.0/24"):
			if originAS == 65001 {
				return ribpolicy.ValidationValid
			}
			return ribpolicy.ValidationInvalid
		case netip.MustParsePrefix("192.0.2.0/24"):
			if originAS == 65000 {
				return ribpolicy.ValidationValid
			}
			return ribpolicy.ValidationInvalid
		case netip.MustParsePrefix("198.18.0.0/24"):
			if originAS == 64496 {
				return ribpolicy.ValidationValid
			}
			return ribpolicy.ValidationInvalid
		}
		return ribpolicy.ValidationNotFound
	})
	ribpolicy.SetLocalOriginAS(65000)
	t.Cleanup(func() {
		ribpolicy.SetOriginValidator(nil)
		ribpolicy.SetLocalOriginAS(0)
	})

	policyCfg := map[string]any{
		policy.MatchFilterType: map[string]any{
			"drop-rpki-invalid": map[string]any{
				"rpki-validation-state": []any{"invalid"},
				"action":                "reject",
			},
		},
	}
	chain, err := policy.CompileRouteChain(chainAttachment, []string{"drop-rpki-invalid"}, policy.FinalAccept, policyCfg)
	require.NoError(t, err)

	staticID := redistevents.RegisterProtocol("static")
	bgpSrcID := redistevents.RegisterProtocol("fakeredist")
	bgpDestID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "static", Protocol: "static"}))
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Destination: "bgp", Source: "fakeredist", Final: "accept", Chain: chain},
		{Destination: "bgp", Source: "static", Final: "accept", Chain: chain},
	}))
	consumer := registerBGPConsumer(t)

	// 1. BGP-like source carrying per-entry OriginAS: Valid & NotFound injected, Invalid dropped.
	handleBatch(context.Background(), skipIDs(bgpDestID), &redistevents.RouteChangeBatch{
		Protocol: bgpSrcID,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("198.51.100.0/24"), OriginAS: 65001},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("198.51.100.0/24"), OriginAS: 65099},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("203.0.113.0/24"), OriginAS: 65099},
		},
	})

	// 2. Non-BGP static source carrying zero OriginAS/OriginASN: validated using local BGP AS 65000.
	handleBatch(context.Background(), skipIDs(bgpDestID), &redistevents.RouteChangeBatch{
		Protocol: staticID,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("192.0.2.0/24")},  // Valid for local AS 65000
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("198.18.0.0/24")}, // Invalid for local AS 65000 (VRP is 64496)
		},
	})

	injected := consumer.snapshotInjected()
	got := make([]string, 0, len(injected))
	for _, inj := range injected {
		got = append(got, inj.entry.Prefix)
	}
	require.Equal(t, []string{"198.51.100.0/24", "203.0.113.0/24", "192.0.2.0/24"}, got,
		"handleBatch must inject only Valid and NotFound routes and reject Invalid BGP and static routes")
}

// RFC requirement: RFC8893-3-1 negative -- when no RPKI validator is active,
// a redistribution policy requiring rpki-validation-state [ valid ] rejects
// routes inside handleBatch rather than falsely matching Valid.
func TestRFC8893RedistributeInactiveValidatorRefusesValidMatch(t *testing.T) {
	resetState(t)
	ribpolicy.SetOriginValidator(nil)
	ribpolicy.SetLocalOriginAS(65000)
	t.Cleanup(func() { ribpolicy.SetLocalOriginAS(0) })

	policyCfg := map[string]any{
		policy.MatchFilterType: map[string]any{
			"only-valid": map[string]any{
				"rpki-validation-state": []any{"valid"},
				"action":                "accept",
			},
		},
	}
	chain, err := policy.CompileRouteChain(chainAttachment, []string{"only-valid"}, policy.FinalAccept, policyCfg)
	require.NoError(t, err)

	staticID := redistevents.RegisterProtocol("static")
	bgpDestID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "static", Protocol: "static"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Destination: "bgp", Source: "static", Final: "accept", Chain: chain},
	}))
	consumer := registerBGPConsumer(t)

	handleBatch(context.Background(), skipIDs(bgpDestID), &redistevents.RouteChangeBatch{
		Protocol: staticID,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("192.0.2.0/24")},
		},
	})

	require.Empty(t, consumer.snapshotInjected(),
		"without an active validator, handleBatch must reject a route under rpki-validation-state [ valid ] action accept")
}
