// Design: docs/guide/graceful-restart.md -- Long-Lived Graceful Restart (RFC 9494)
// RFC: rfc/short/rfc9494.md -- Section 5, affirmative configuration per AFI/SAFI
// Overview: gr_llgr.go -- parseLLGRCapValue, extractLLGRCapabilities
// Related: gr_capability.go -- parseGRCapValue, refuseUncarriedGRFamilies
// Related: rfc4724_gr_capability_test.go -- deliverBGPSection, grConfig

package gr

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// llgrPayloadForConfig returns the code-71 hex payload declared for peer1 through
// the full production config chain (LoadConfig -> ToPluginMap -> ExtractConfigSubtree
// -> refuseUncarriedGRFamilies -> extractLLGRCapabilities).
func llgrPayloadForConfig(t *testing.T, text string) (string, bool) {
	t.Helper()
	section := deliverBGPSection(t, text)
	require.NoError(t, refuseUncarriedGRFamilies(section), "the configuration under test is accepted")
	caps := extractLLGRCapabilities(section)
	if len(caps) == 0 {
		return "", false
	}
	require.Len(t, caps, 1, "one peer configures long-lived, so one code-71 declaration is owed")
	require.Equal(t, uint8(llgrCapCode), caps[0].Code, "the declaration is the LLGR capability")
	return caps[0].Payload, true
}

// TestRFC9494_PerFamilyLLGRConfigPositive proves affirmative per-AFI/SAFI LLGR
// configuration through the full production config chain: each configured family
// appears in code 71 with its own 24-bit Long-Lived Stale Time, including the
// boundary values 0 and 16777215, while unconfigured session families are omitted.
//
// VALIDATES: RFC 9494 Section 5: "They MUST require affirmative configuration
// per AFI/SAFI in order to enable them."
// PREVENTS: a peer-wide LLGR timer stamping unconfigured families into code 71
// or losing distinct per-family stale times.
//
// RFC requirement: RFC9494-5-2 positive -- through LoadConfig -> ToPluginMap ->
// extractLLGRCapabilities (internal/component/bgp/plugins/gr/gr_llgr.go), a dual-family
// peer configuring graceful-restart long-lived family ipv4/unicast (stale-time 3600)
// declares code 71 with only the ipv4/unicast 7-byte tuple, and a peer configuring
// ipv4/unicast (0) and ipv6/unicast (16777215) declares both tuples with their own LLST.
func TestRFC9494_PerFamilyLLGRConfigPositive(t *testing.T) {
	t.Run("one_family_of_two", func(t *testing.T) {
		cfg := grConfig(twoFamilies,
			"\t\t\t\t\tshort-lived { restart-time 120; }\n"+
				"\t\t\t\t\tlong-lived {\n\t\t\t\t\t\tfamily {\n\t\t\t\t\t\t\tipv4/unicast 3600\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n")

		payload, ok := llgrPayloadForConfig(t, cfg)
		require.True(t, ok, "code 71 must be declared for ipv4/unicast")

		data, err := hex.DecodeString(payload)
		require.NoError(t, err)
		decoded, err := decodeLLGR(data)
		require.NoError(t, err)
		require.Len(t, decoded.Families, 1, "only ipv4/unicast was configured for LLGR; ipv6/unicast must be excluded")
		assert.Equal(t, uint16(family.AFIIPv4), decoded.Families[0].AFI)
		assert.Equal(t, uint8(family.SAFIUnicast), decoded.Families[0].SAFI)
		assert.Equal(t, uint32(3600), decoded.Families[0].LLST)
	})

	t.Run("distinct_stale_times_and_boundaries", func(t *testing.T) {
		cfg := grConfig(twoFamilies,
			"\t\t\t\t\tshort-lived { restart-time 120; }\n"+
				"\t\t\t\t\tlong-lived {\n\t\t\t\t\t\tfamily {\n"+
				"\t\t\t\t\t\t\tipv4/unicast 0\n"+
				"\t\t\t\t\t\t\tipv6/unicast 16777215\n"+
				"\t\t\t\t\t\t}\n\t\t\t\t\t}\n")

		payload, ok := llgrPayloadForConfig(t, cfg)
		require.True(t, ok, "code 71 must be declared for both configured families")

		// 0001 01 80 000000 (ipv4/unicast, LLST=0) + 0002 01 80 ffffff (ipv6/unicast, LLST=16777215)
		assert.Equal(t, "0001018000000000020180ffffff", payload)
	})
}

// TestRFC9494_PerFamilyLLGRConfigNegative proves that families without an
// affirmative graceful-restart long-lived family entry are never placed in
// code 71 and never enter LLGR on session loss, even when the peer carries
// multiple families and configures LLGR for another family or writes an empty
// long-lived container.
//
// VALIDATES: RFC 9494 Section 5: "Implementations MUST NOT enable these
// procedures by default. They MUST require affirmative configuration per
// AFI/SAFI in order to enable them."
// PREVENTS: an unconfigured family inheriting LLGR from the peer or from a
// sibling family's LLGR entry.
//
// RFC requirement: RFC9494-5-2 negative -- through LoadConfig -> ToPluginMap ->
// extractLLGRCapabilities (internal/component/bgp/plugins/gr/gr_llgr.go), neither
// a short-lived-only config nor an empty long-lived container declares code 71,
// and when ipv4/unicast alone is configured on a dual-family peer, ipv6/unicast is
// absent from code 71 and purged on session loss even if the remote peer sent LLGR
// for both families.
func TestRFC9494_PerFamilyLLGRConfigNegative(t *testing.T) {
	t.Run("short_lived_only_no_code_71", func(t *testing.T) {
		cfg := grConfig(twoFamilies, "\t\t\t\t\tshort-lived { restart-time 120; }\n")
		_, ok := llgrPayloadForConfig(t, cfg)
		assert.False(t, ok, "short-lived alone must not advertise code 71")
	})

	t.Run("empty_long_lived_no_code_71", func(t *testing.T) {
		cfg := grConfig(twoFamilies, "\t\t\t\t\tlong-lived { }\n")
		_, ok := llgrPayloadForConfig(t, cfg)
		assert.False(t, ok, "empty long-lived container without family entries must not advertise code 71")
	})

	t.Run("unconfigured_family_excluded_from_code_71_and_helper_llgr", func(t *testing.T) {
		cfg := grConfig(twoFamilies,
			"\t\t\t\t\tshort-lived { restart-time 0; }\n"+
				"\t\t\t\t\tlong-lived {\n\t\t\t\t\t\tfamily {\n\t\t\t\t\t\t\tipv4/unicast 3600\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n")

		payload, ok := llgrPayloadForConfig(t, cfg)
		require.True(t, ok)
		raw, err := hex.DecodeString(payload)
		require.NoError(t, err)
		require.Len(t, raw, 7, "code 71 must hold only the one configured family")

		var sentTuple [7]byte
		copy(sentTuple[:], raw)

		gp, rec := newRecordedGRPlugin()
		t.Cleanup(func() { gp.state.removePeer(grFixtureSource(t, testPeer, 701, 41).grPeer) })
		rfc9494Opens(t, gp, true, [][7]byte{sentTuple}, [][7]byte{llgrTupleIPv4, llgrTupleIPv6})

		sent := rec.all()
		assert.Contains(t, sent, "request bgp rib source attach-community owner 701 connection 41 family ipv4/unicast community ffff0006")
		assert.NotContains(t, sent, "request bgp rib source attach-community owner 701 connection 41 family ipv6/unicast community ffff0006",
			"unconfigured ipv6/unicast must not enter LLGR")
		assert.Contains(t, sent, "request bgp rib source purge-stale owner 701 connection 41 family ipv6/unicast",
			"unconfigured ipv6/unicast must be purged at the end of the GR phase")
	})
}

// TestLLGRSkipGRPhaseOmitsShortLived proves RFC 9494 Section 4.1: configuring
// long-lived inside graceful-restart without short-lived sends code 64 with
// Restart Time 0 and zero families, and code 71 with only the configured family.
//
// VALIDATES: code 64 is always sent alongside code 71 and skips the GR phase when
// short-lived is omitted.
// PREVENTS: omitting short-lived from dropping code 64 or stamping session families
// into code 64.
func TestLLGRSkipGRPhaseOmitsShortLived(t *testing.T) {
	cfg := grConfig(twoFamilies,
		"\t\t\t\t\tlong-lived {\n\t\t\t\t\t\tfamily {\n\t\t\t\t\t\t\tipv4/unicast 3600\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n")

	assert.Equal(t, "0000", grPayloadForConfig(t, cfg),
		"omitting short-lived sends code 64 with Restart Time 0 and no family tuples")
	llgrPayload, ok := llgrPayloadForConfig(t, cfg)
	require.True(t, ok)
	assert.Equal(t, "00010180000e10", llgrPayload,
		"code 71 carries only ipv4/unicast with stale-time 3600")
}

// TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime checks both offline YANG
// bounds (stale-time > 16777215, negative, missing, deleted legacy leaves) and
// the plugin verifier refusal for an uncarried long-lived family.
//
// VALIDATES: YANG validation bounds, mandatory stale-time, no legacy fallback,
// and InProcessConfigVerifier parity with OnConfigVerify/Stage 2.
// PREVENTS: out-of-range stale times, legacy leaves remaining accepted, or
// uncarried LLGR families passing offline validation.
func TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime(t *testing.T) {
	t.Run("uncarried_long_lived_family_refused_by_in_process_verifier", func(t *testing.T) {
		section := deliverBGPSection(t, grConfig([]string{"ipv4/unicast"},
			"\t\t\t\t\tlong-lived {\n\t\t\t\t\t\tfamily {\n\t\t\t\t\t\t\tipv6/unicast 3600\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n"))

		err := refuseUncarriedGRFamilies(section)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ipv6/unicast")
		assert.Contains(t, err.Error(), "ipv4/unicast")

		reg := registry.Lookup(grPluginName)
		require.NotNil(t, reg)
		require.NotNil(t, reg.InProcessConfigVerifier, "bgp-gr must register InProcessConfigVerifier so ze config validate matches boot")
		err = reg.InProcessConfigVerifier([]sdk.ConfigSection{{Root: configRootBGP, Data: section}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ipv6/unicast")
	})

	wrapPeer := func(grBody string) string {
		return "bgp {\n\trouter-id 192.0.2.1\n\tsession {\n\t\tasn { local 65000; }\n\t}\n" +
			"\tpeer peer1 {\n\t\tsession {\n\t\t\tfamily { ipv4/unicast { } }\n" +
			"\t\t\tcapability {\n\t\t\t\tgraceful-restart {\n" + grBody + "\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n"
	}

	invalidConfigs := []struct {
		name string
		body string
	}{
		{"stale_time_above_24bit_max", "\t\t\t\t\tlong-lived { family { ipv4/unicast 16777216 } }\n"},
		{"stale_time_negative", "\t\t\t\t\tlong-lived { family { ipv4/unicast -1 } }\n"},
		{"stale_time_missing_mandatory", "\t\t\t\t\tlong-lived { family { ipv4/unicast { } } }\n"},
		{"legacy_long_lived_stale_time_removed", "\t\t\t\t\tlong-lived-stale-time 3600\n"},
		{"legacy_top_level_restart_time_removed", "\t\t\t\t\trestart-time 120\n"},
	}

	for _, tc := range invalidConfigs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := zeconfig.LoadConfig(wrapPeer(tc.body), "test.conf", nil)
			require.Error(t, err, "config %s must be refused", tc.name)
		})
	}
}

// TestLLGRPeerContainerReplacesGroupLLGR proves that a peer's graceful-restart
// container replaces its group's container for both code 64 and code 71: a peer
// that defines short-lived alone does not inherit its group's long-lived entries.
//
// VALIDATES: extractLLGRCapabilities uses grCapabilityFor, matching extractGRCapabilities.
// PREVENTS: a peer overriding graceful-restart accidentally inheriting group LLGR.
func TestLLGRPeerContainerReplacesGroupLLGR(t *testing.T) {
	text := grGroupConfig(
		[]string{"ipv4/unicast"}, nil,
		"\t\t\t\tshort-lived { restart-time 120; }\n\t\t\t\tlong-lived { family { ipv4/unicast 7200 } }\n",
		"\t\t\t\t\tshort-lived { restart-time 60; }\n")

	assert.Equal(t, "003c00010100", grPayloadForConfig(t, text), "peer advertises its own short-lived 60s")
	_, hasLLGR := llgrPayloadForConfig(t, text)
	assert.False(t, hasLLGR, "peer's graceful-restart container replaces the group's, so group LLGR does not leak")
}

// TestLLGRDefensiveValidationAndTupleCap exercises refusePeerGRFamilies and
// parseLLGRCapValue directly for malformed inputs and the 36-tuple RFC 5492
// capability length cap (36 * 7 = 252 <= 255 octets).
//
// VALIDATES: negative, non-integer, and > 16777215 stale times are refused by
// refusePeerGRFamilies and safely handled by parseLLGRCapValue; configuring
// more than 36 families caps code 71 at 36 tuples (252 bytes).
// PREVENTS: malformed Stage-2 JSON bypassing YANG validation from producing
// corrupt code 71 bytes or overflowing the 1-octet RFC 5492 capability length.
func TestLLGRDefensiveValidationAndTupleCap(t *testing.T) {
	capWith := func(stale any) map[string]any {
		return map[string]any{
			grConfigName: map[string]any{
				grLongLivedNode: map[string]any{
					grFamilyNode: map[string]any{
						"ipv4/unicast": map[string]any{grStaleTimeLeaf: stale},
					},
				},
			},
		}
	}

	require.Error(t, refusePeerGRFamilies(capWith("not-a-number"), "10.0.0.1", []string{"ipv4/unicast"}))
	require.Error(t, refusePeerGRFamilies(capWith(float64(-1)), "10.0.0.1", []string{"ipv4/unicast"}))
	require.Error(t, refusePeerGRFamilies(capWith(float64(llgrStaleTimeMax+1)), "10.0.0.1", []string{"ipv4/unicast"}))
	require.NoError(t, refusePeerGRFamilies(capWith(float64(llgrStaleTimeMax)), "10.0.0.1", []string{"ipv4/unicast"}))
	require.NoError(t, refusePeerGRFamilies(capWith("3600"), "10.0.0.1", []string{"ipv4/unicast"}))

	assert.Empty(t, parseLLGRCapValue(capWith("not-a-number"), "10.0.0.1"), "non-integer stale-time is skipped")
	assert.Empty(t, parseLLGRCapValue(capWith(float64(-5)), "10.0.0.1"), "negative stale-time is skipped")
	assert.Equal(t, "00010180ffffff", parseLLGRCapValue(capWith(float64(llgrStaleTimeMax+99)), "10.0.0.1"), "out-of-range stale-time clamps to 24-bit max")

	registerRIBLessFamily(t)
	withRIBLess := map[string]any{
		grConfigName: map[string]any{
			grLongLivedNode: map[string]any{
				grFamilyNode: map[string]any{
					"ipv4/unicast": map[string]any{grStaleTimeLeaf: float64(60)},
					ribLessFamily:  map[string]any{grStaleTimeLeaf: float64(60)},
				},
			},
		},
	}
	assert.Equal(t, "0001018000003c", parseLLGRCapValue(withRIBLess, "10.0.0.1"),
		"family the RIB cannot store must be omitted from code 71")

	assert.LessOrEqual(t, llgrFamilyMax*7, 255, "llgrFamilyMax must fit in one RFC 5492 capability length octet")
	assert.Greater(t, (llgrFamilyMax+1)*7, 255, "llgrFamilyMax must be the tight RFC 5492 tuple cap")

	many := make(map[string]any)
	for _, name := range family.RegisteredFamilyNames() {
		many[name] = map[string]any{grStaleTimeLeaf: float64(60)}
	}
	payload := parseLLGRCapValue(map[string]any{
		grConfigName: map[string]any{
			grLongLivedNode: map[string]any{
				grFamilyNode: many,
			},
		},
	}, "10.0.0.1")
	raw, err := hex.DecodeString(payload)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(raw), llgrFamilyMax*7, "code 71 must never exceed 36 tuples (252 bytes <= 255)")
	assert.NotEmpty(t, raw)
}
