package textparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: All alias forms resolve to correct canonical keyword.
// PREVENTS: Alias typos or missing entries breaking command parsing.
func TestResolveAlias(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		canonical string
	}{
		// next-hop aliases.
		{"next short", ShortNext, KWNextHop},
		{"next-hop long", KWNextHop, KWNextHop},
		{"nhop legacy", "nhop", KWNextHop},

		// local-preference aliases.
		{"pref short", ShortPref, KWLocalPreference},
		{"local-preference long", KWLocalPreference, KWLocalPreference},

		// as-path aliases.
		{"path short", ShortPath, KWASPath},
		{"as-path long", KWASPath, KWASPath},

		// community aliases (x-com pattern).
		{"s-com short", ShortSCom, KWCommunity},
		{"community long", KWCommunity, KWCommunity},
		{"short-community consistency", "short-community", KWCommunity},

		// large-community aliases.
		{"l-com short", ShortLCom, KWLargeCommunity},
		{"large-community long", KWLargeCommunity, KWLargeCommunity},

		// extended-community aliases.
		{"x-com primary", ShortXCom, KWExtendedCommunity},
		{"e-com accepted", "e-com", KWExtendedCommunity},
		{"extended-community long", KWExtendedCommunity, KWExtendedCommunity},

		// path-information aliases.
		{"info short", ShortInfo, KWPathInformation},
		{"path-information long", KWPathInformation, KWPathInformation},

		// route-distinguisher aliases.
		{"rd short", ShortRD, KWRD},
		{"route-distinguisher long", "route-distinguisher", KWRD},

		// No alias — returns unchanged.
		{"origin passthrough", "origin", "origin"},
		{"med passthrough", "med", "med"},
		{"label passthrough", "label", "label"},
		{"nlri passthrough", "nlri", "nlri"},
		{"unknown passthrough", "foobar", "foobar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.canonical, ResolveAlias(tt.input))
		})
	}
}

// VALIDATES: ShortForm returns correct API-compact form for each canonical keyword.
// PREVENTS: Event formatter emitting wrong alias.
func TestShortForm(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		short     string
	}{
		{"next-hop", KWNextHop, ShortNext},
		{"local-preference", KWLocalPreference, ShortPref},
		{"as-path", KWASPath, ShortPath},
		{"community", KWCommunity, ShortSCom},
		{"large-community", KWLargeCommunity, ShortLCom},
		{"extended-community", KWExtendedCommunity, ShortXCom},
		{"path-information", KWPathInformation, ShortInfo},
		{"rd", KWRD, ShortRD},
		// No short form — returns canonical unchanged.
		{"origin unchanged", KWOrigin, KWOrigin},
		{"med unchanged", KWMED, KWMED},
		{"label unchanged", KWLabel, KWLabel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.short, ShortForm(tt.canonical))
		})
	}
}

// VALIDATES: IsAttributeKeyword correctly classifies attribute vs non-attribute keywords.
// PREVENTS: Boundary detection misclassifying keywords.
func TestIsAttributeKeyword(t *testing.T) {
	// Attribute keywords.
	for _, kw := range []string{
		KWOrigin, KWASPath, KWMED, KWLocalPreference,
		KWAtomicAggregate, KWAggregator, KWOriginatorID,
		KWClusterList, KWCommunity, KWLargeCommunity,
		KWExtendedCommunity, KWNextHop,
	} {
		assert.True(t, IsAttributeKeyword(kw), "%s should be attribute keyword", kw)
	}

	// Non-attribute keywords.
	for _, kw := range []string{
		KWNLRI, KWRD, KWLabel, KWWatchdog, KWAttr,
		KWAdd, KWDel, KWEOR, KWSelf,
	} {
		assert.False(t, IsAttributeKeyword(kw), "%s should NOT be attribute keyword", kw)
	}
}

// VALIDATES: IsTopLevelKeyword includes all attribute keywords plus nlri.
// PREVENTS: NLRI collection not stopping at the right boundary.
func TestIsTopLevelKeyword(t *testing.T) {
	// All attribute keywords are top-level.
	for _, kw := range []string{
		KWOrigin, KWASPath, KWMED, KWLocalPreference,
		KWAtomicAggregate, KWAggregator, KWOriginatorID,
		KWClusterList, KWCommunity, KWLargeCommunity,
		KWExtendedCommunity, KWNextHop,
	} {
		assert.True(t, IsTopLevelKeyword(kw), "%s should be top-level keyword", kw)
	}

	// nlri is top-level but not attribute.
	assert.True(t, IsTopLevelKeyword(KWNLRI))
	assert.False(t, IsAttributeKeyword(KWNLRI))

	// Non-top-level.
	for _, kw := range []string{
		KWRD, KWLabel, KWWatchdog, KWAdd, KWDel,
	} {
		assert.False(t, IsTopLevelKeyword(kw), "%s should NOT be top-level keyword", kw)
	}
}

// VALIDATES: Alias resolution followed by ShortForm roundtrips correctly.
// PREVENTS: Resolve then format producing unexpected results.
func TestAliasRoundtrip(t *testing.T) {
	// All alias forms should resolve then short-form to the same API output.
	tests := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{"next-hop forms", []string{ShortNext, KWNextHop, "nhop"}, ShortNext},
		{"local-preference forms", []string{ShortPref, KWLocalPreference}, ShortPref},
		{"as-path forms", []string{ShortPath, KWASPath}, ShortPath},
		{"community forms", []string{ShortSCom, KWCommunity, "short-community"}, ShortSCom},
		{"large-community forms", []string{ShortLCom, KWLargeCommunity}, ShortLCom},
		{"extended-community forms", []string{ShortXCom, "e-com", KWExtendedCommunity}, ShortXCom},
		{"path-information forms", []string{ShortInfo, KWPathInformation}, ShortInfo},
		{"rd forms", []string{ShortRD, "route-distinguisher"}, ShortRD},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, input := range tt.inputs {
				canonical := ResolveAlias(input)
				short := ShortForm(canonical)
				assert.Equal(t, tt.expected, short, "input %q -> canonical %q -> short %q", input, canonical, short)
			}
		})
	}
}

// VALIDATES: NLRITypeKeywords contains all expected NLRI type keywords.
// PREVENTS: Event parser missing NLRI boundary keywords.
func TestNLRITypeKeywords(t *testing.T) {
	expected := []string{
		"prefix", "rd", "reachability",
		"node", "link", "srv6-sid",
		"ethernet-ad", "mac-ip", "multicast",
		"ethernet-segment", "ip-prefix",
		"flow", "flow-vpn",
	}

	for _, kw := range expected {
		assert.True(t, NLRITypeKeywords[kw], "%s should be NLRI type keyword", kw)
	}

	assert.False(t, NLRITypeKeywords["origin"], "origin should not be NLRI type keyword")
	assert.False(t, NLRITypeKeywords["nlri"], "nlri should not be NLRI type keyword")
}
