package filter_family

import (
	"maps"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// ipv4Flow is the address family these tests match on (RFC 8955 FlowSpec).
var ipv4Flow = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

// TestMain registers the standard families so LookupFamily("ipv4/flow") resolves.
func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	os.Exit(m.Run())
}

// TestParseFamilyFilters validates instance parsing: family + action, with the
// canonical flowspec name ipv4/flow, and rejection of malformed action/family.
//
// VALIDATES: AC-1 -- family-filter instances parse (family + action).
// PREVENTS: silently accepting an unknown action or unregistered family.
func TestParseFamilyFilters(t *testing.T) {
	cfg := map[string]any{
		"policy": map[string]any{
			"family-filter": map[string]any{
				"NoFlow": map[string]any{"family": "ipv4/flow", "action": "remove"},
				"Kill":   map[string]any{"family": "ipv4/flow", "action": "tear-down"},
			},
		},
	}
	instances, err := parseFamilyFilters(cfg)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	assert.Equal(t, actionRemove, instances["NoFlow"].action)
	assert.Equal(t, ipv4Flow, instances["NoFlow"].family)
	assert.Equal(t, actionTearDown, instances["Kill"].action)
}

// TestParseFamilyFiltersInvalid rejects an unknown action and an unknown family.
func TestParseFamilyFiltersInvalid(t *testing.T) {
	_, err := parseFamilyFilters(map[string]any{
		"policy": map[string]any{"family-filter": map[string]any{
			"Bad": map[string]any{"family": "ipv4/flow", "action": "wat"},
		}},
	})
	require.Error(t, err)

	_, err = parseFamilyFilters(map[string]any{
		"policy": map[string]any{"family-filter": map[string]any{
			"Bad": map[string]any{"family": "not-a-family", "action": "remove"},
		}},
	})
	require.Error(t, err)
}

// TestTearDownInExportRejected validates AC-7: a tear-down instance referenced in
// any export chain is a configuration error; remove in export is fine.
func TestTearDownInExportRejected(t *testing.T) {
	base := map[string]any{
		"policy": map[string]any{"family-filter": map[string]any{
			"Kill":   map[string]any{"family": "ipv4/flow", "action": "tear-down"},
			"NoFlow": map[string]any{"family": "ipv4/flow", "action": "remove"},
		}},
	}

	// tear-down in a peer export chain -> error (AC-7), tried via all ref forms.
	for _, ref := range []string{"bgp-filter-family:Kill", "family-filter:Kill", "Kill"} {
		cfg := cloneMap(base)
		cfg["peer"] = map[string]any{
			"edge": map[string]any{"filter": map[string]any{"export": []any{ref}}},
		}
		_, err := parseFamilyFilters(cfg)
		require.Error(t, err, "tear-down ref %q in export must be rejected", ref)
	}

	// remove in export chain -> OK.
	cfg := cloneMap(base)
	cfg["peer"] = map[string]any{
		"edge": map[string]any{"filter": map[string]any{"export": []any{"bgp-filter-family:NoFlow"}}},
	}
	_, err := parseFamilyFilters(cfg)
	require.NoError(t, err)

	// tear-down in an IMPORT chain -> OK (import is its only legal direction).
	cfg = cloneMap(base)
	cfg["peer"] = map[string]any{
		"edge": map[string]any{"filter": map[string]any{"import": []any{"bgp-filter-family:Kill"}}},
	}
	_, err = parseFamilyFilters(cfg)
	require.NoError(t, err)
}

// TestExportRefInstanceName checks ref-prefix normalisation used by the AC-7 guard.
func TestExportRefInstanceName(t *testing.T) {
	assert.Equal(t, "Kill", exportRefInstanceName("bgp-filter-family:Kill"))
	assert.Equal(t, "Kill", exportRefInstanceName("family-filter:Kill"))
	// test-relax: the inactive: prefix tolerance was removed with the move to
	// out-of-band per-member deactivation -- refs now arrive clean via ToMap
	// (active-only), so exportRefInstanceName never sees an inactive: prefix.
	assert.Equal(t, "Kill", exportRefInstanceName("Kill")) // bare
	assert.Empty(t, exportRefInstanceName("prefix-list:Kill"), "another plugin's filter")
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}
