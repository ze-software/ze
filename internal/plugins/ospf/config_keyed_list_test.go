// Design: docs/architecture/ospf/ospf-4-component-config.md -- keyedList fail-closed parsing.
// Related: config.go, sr_config.go.

package ospf

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestParseOSPFEmptyAreaAccepted verifies that an empty OSPF area map (`{}`)
// is accepted both directly from JSON delivery and after hierarchical ->
// set-meta -> ToPluginMap round-tripping.
//
// VALIDATES: `area 0.0.0.0 {}` produces a valid area entry with default area
// settings so interfaces referencing area 0.0.0.0 validate cleanly.
// PREVENTS: Empty OSPF areas being dropped during set-meta commit or JSON
// config delivery ("interface references undeclared area").
func TestParseOSPFEmptyAreaAccepted(t *testing.T) {
	t.Run("json empty area object", func(t *testing.T) {
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.0":{}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"}}}}}`), nil)
		if err != nil {
			t.Fatalf("parseOSPFConfig rejected valid empty area: %v", err)
		}
		if err := validateConfig(cfg); err != nil {
			t.Fatalf("validateConfig rejected valid empty area: %v", err)
		}
		if len(cfg.Areas) != 1 {
			t.Fatalf("Areas len = %d, want 1", len(cfg.Areas))
		}
		if cfg.Areas[0].AreaID != types.BackboneArea {
			t.Fatalf("AreaID = %s, want %s", cfg.Areas[0].AreaID, types.BackboneArea)
		}
		if cfg.Areas[0].AreaType != types.AreaTypeNormal {
			t.Fatalf("AreaType = %q, want %q", cfg.Areas[0].AreaType, types.AreaTypeNormal)
		}
	})

	t.Run("hierarchical through set-meta and ToPluginMap", func(t *testing.T) {
		schema, err := config.YANGSchema()
		if err != nil {
			t.Fatalf("YANGSchema: %v", err)
		}
		src := `ospf {
	router-id 10.0.0.1
	areas {
		area 0.0.0.0 {}
	}
	interfaces {
		interface eth0 {
			area 0.0.0.0
		}
	}
}
`
		tree, err := config.NewParser(schema).Parse(src)
		if err != nil {
			t.Fatalf("Parse hierarchical: %v", err)
		}
		setMeta := config.SerializeSetWithMeta(tree, config.NewMetaTree(), schema)
		reloaded, _, err := config.NewSetParser(schema).ParseWithMeta(setMeta)
		if err != nil {
			t.Fatalf("ParseWithMeta: %v\n%s", err, setMeta)
		}
		pluginMap := reloaded.ToPluginMap()
		data, err := json.Marshal(map[string]any{Namespace: pluginMap[Namespace]})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		cfg, err := parseOSPFConfig(ospfSec(string(data)), nil)
		if err != nil {
			t.Fatalf("parseOSPFConfig after set-meta round-trip: %v\njson=%s", err, data)
		}
		if err := validateConfig(cfg); err != nil {
			t.Fatalf("validateConfig after set-meta round-trip: %v", err)
		}
		if len(cfg.Areas) != 1 || cfg.Areas[0].AreaID != types.BackboneArea {
			t.Fatalf("Areas = %+v, want [0.0.0.0]", cfg.Areas)
		}
	})
}

// TestParseOSPFKeyedListRefusesMalformedEntries verifies keyedList returns an
// error instead of silently skipping non-map entries or non-map list containers.
//
// VALIDATES: Malformed list shapes (`null`, scalar, array) in areas,
// interfaces, redistribute, key-chains, keys, ranges, virtual-links, and
// nbma-neighbors fail closed during parseOSPFConfig.
// PREVENTS: Silent fall-through dropping entries and surfacing later as
// misleading validation errors or missing runtime configuration.
func TestParseOSPFKeyedListRefusesMalformedEntries(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "area entry is null",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.0":null}}}}`,
		},
		{
			name: "area entry is string",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.0":"normal"}}}}`,
		},
		{
			name: "area list is array",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":["0.0.0.0"]}}}`,
		},
		{
			name: "interface entry is string",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.0":{}}},"interfaces":{"interface":{"eth0":"bad"}}}}`,
		},
		{
			name: "redistribute entry is bool",
			json: `{"ospf":{"router-id":"10.0.0.1","redistribute":{"static":true}}}`,
		},
		{
			name: "key-chain entry is string",
			json: `{"ospf":{"router-id":"10.0.0.1","key-chains":{"kc1":"bad"}}}`,
		},
		{
			name: "key-chain key entry is number",
			json: `{"ospf":{"router-id":"10.0.0.1","key-chains":{"kc1":{"key":{"1":123}}}}}`,
		},
		{
			name: "area range entry is string",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.0":{"ranges":{"range":{"10.0.0.0/8":"bad"}}}}}}}`,
		},
		{
			name: "virtual-link entry is string",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.1":{"virtual-link":{"2.2.2.2":"bad"}}}}}}`,
		},
		{
			name: "nbma-neighbor entry is string",
			json: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0.0.0.0":{}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"nbma","nbma-neighbor":{"10.0.0.2":"bad"}}}}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseOSPFConfig(ospfSec(tc.json), nil); err == nil {
				t.Fatalf("parseOSPFConfig accepted malformed keyed list (%s); want error", tc.name)
			}
		})
	}
}

// TestValidateSRConfigRefusesMalformedPrefixSID verifies segment-routing
// prefix-sid list parsing propagates keyedList errors rather than skipping.
//
// VALIDATES: Non-map prefix-sid entries fail validateSRConfig.
// PREVENTS: Silently dropping malformed SR prefix-SID configuration.
func TestValidateSRConfigRefusesMalformedPrefixSID(t *testing.T) {
	sections := ospfSec(`{"ospf":{"segment-routing":{"enable":true,"srgb":{"lower-bound":16000,"upper-bound":23999},"prefix-sid":{"10.0.0.1/32":"bad"}}}}`)
	if err := validateSRConfig(sections); err == nil {
		t.Fatalf("validateSRConfig accepted malformed prefix-sid entry; want error")
	}
}
