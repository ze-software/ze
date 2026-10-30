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
	instances, err := parseFamilyFiltersOf(cfg)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	assert.Equal(t, actionRemove, instances["NoFlow"].action)
	assert.Equal(t, ipv4Flow, instances["NoFlow"].family)
	assert.Equal(t, actionTearDown, instances["Kill"].action)
}

// TestParseFamilyFiltersInvalid rejects an unknown action and an unknown family.
func TestParseFamilyFiltersInvalid(t *testing.T) {
	_, err := parseFamilyFiltersOf(map[string]any{
		"policy": map[string]any{"family-filter": map[string]any{
			"Bad": map[string]any{"family": "ipv4/flow", "action": "wat"},
		}},
	})
	require.Error(t, err)

	_, err = parseFamilyFiltersOf(map[string]any{
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
			"edge": map[string]any{"filter": map[string]any{"export": map[string]any{"filters": []any{ref}, "final": "accept"}}},
		}
		_, err := parseFamilyFiltersOf(cfg)
		require.Error(t, err, "tear-down ref %q in export must be rejected", ref)
	}

	// remove in export chain -> OK.
	cfg := cloneMap(base)
	cfg["peer"] = map[string]any{
		"edge": map[string]any{"filter": map[string]any{"export": map[string]any{"filters": []any{"bgp-filter-family:NoFlow"}, "final": "accept"}}},
	}
	_, err := parseFamilyFiltersOf(cfg)
	require.NoError(t, err)

	// tear-down in an IMPORT chain -> OK (import is its only legal direction).
	cfg = cloneMap(base)
	cfg["peer"] = map[string]any{
		"edge": map[string]any{"filter": map[string]any{"import": map[string]any{"filters": []any{"bgp-filter-family:Kill"}, "final": "accept"}}},
	}
	_, err = parseFamilyFiltersOf(cfg)
	require.NoError(t, err)
}

// TestExportRefInstanceName checks ref-prefix normalisation used by the AC-7 guard.
func TestExportRefInstanceName(t *testing.T) {
	assert.Equal(t, "Kill", exportRefInstanceName("bgp-filter-family:Kill"))
	assert.Equal(t, "Kill", exportRefInstanceName("family-filter:Kill"))
	// the inactive: prefix tolerance was removed with the move to
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

// TestExportPolicyFinalDoesNotHideReferences keeps the export-only restriction
// effective at global/group levels and distinguishes a final-only empty policy.
func TestExportPolicyFinalDoesNotHideReferences(t *testing.T) {
	instances := map[string]*familyFilter{"Kill": {action: actionTearDown}}
	for _, final := range []string{"accept", "reject"} {
		empty := map[string]any{"filter": map[string]any{
			"export": map[string]any{"final": final},
		}}
		if err := validateNoTearDownInExport(empty, instances); err != nil {
			t.Fatalf("final-only export policy refused: %v", err)
		}
		chained := map[string]any{"filter": map[string]any{
			"export": map[string]any{"filters": []string{"Kill"}, "final": final},
		}}
		group := cloneMap(chained)
		group["peer"] = map[string]any{"edge": map[string]any{}}
		for _, cfg := range []map[string]any{chained, {"group": map[string]any{"g": group}}} {
			if err := validateNoTearDownInExport(cfg, instances); err == nil {
				t.Fatalf("final %s hid a tear-down reference", final)
			}
		}
	}
}

// TestDynamicGroupExportValidation parses a peerless listen-range group so the
// template visit, rather than an incidental static peer, must enforce the
// import-only restriction. Legal import and remove policies remain accepted.
func TestDynamicGroupExportValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		direction string
		action    string
		refused   bool
	}{
		{name: "tear-down export", direction: "export", action: actionTearDown, refused: true},
		{name: "tear-down import", direction: "import", action: actionTearDown},
		{name: "remove export", direction: "export", action: actionRemove},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFamilyFiltersOf(map[string]any{
				"policy": map[string]any{"family-filter": map[string]any{
					"guard": map[string]any{"family": "ipv4/flow", "action": tc.action},
				}},
				"group": map[string]any{"ix": map[string]any{
					"connection": map[string]any{"remote": map[string]any{
						"ip": "dynamic", "range": []any{"192.0.2.0/24"},
					}},
					"filter": map[string]any{tc.direction: map[string]any{
						"filters": []any{"bgp-filter-family:guard"}, "final": "accept",
					}},
				}},
			})
			if (err != nil) != tc.refused {
				t.Fatalf("parse error = %v, want refusal %v", err, tc.refused)
			}
		})
	}
}

// parseFamilyFiltersOf splits a test config written as one map into the two
// roots parseFamilyFilters reads: its "policy" entry is the policy root, and
// the rest is the bgp root whose peers and groups carry the export chains.
func parseFamilyFiltersOf(cfg map[string]any) (map[string]*familyFilter, error) {
	policyCfg, _ := cfg["policy"].(map[string]any)
	bgpCfg := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if k == "policy" {
			continue
		}
		bgpCfg[k] = v
	}
	return parseFamilyFilters(policyCfg, bgpCfg)
}
