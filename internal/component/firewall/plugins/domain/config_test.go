package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// section wraps a config tree in the shape OnConfigure delivers it.
func section(t *testing.T, tree map[string]any) []sdk.ConfigSection {
	t.Helper()
	data, err := json.Marshal(tree)
	require.NoError(t, err)
	return []sdk.ConfigSection{{Root: configRoot, Data: string(data)}}
}

// TestParseDomainConfigReadsGroupsAndNames proves a committed domain group
// reaches the plugin's own config type.
func TestParseDomainConfigReadsGroupsAndNames(t *testing.T) {
	cfg := parseDomainConfig(section(t, map[string]any{
		configRoot: map[string]any{
			"domain-group": map[string]any{
				"cdn": map[string]any{
					"domain-names": []any{"b.invalid", "a.invalid"},
					"ttl-floor":    "300",
				},
			},
		},
	}))

	require.Len(t, cfg.groups, 1)
	assert.Equal(t, "cdn", cfg.groups[0].Name)
	assert.Equal(t, []string{"a.invalid", "b.invalid"}, cfg.groups[0].Names,
		"names are normalized so a reordered config does not read as a change")
	assert.Equal(t, uint32(300), cfg.groups[0].TTLFloor)
}

// TestParseTTLFloorReadsEveryDeliveredShape proves the leaf is read whichever
// JSON shape it arrives in.
//
// Every delivered config value arrives as a JSON string on some paths and as a
// number on others (ai/rules/config.md), so a coercion that asserted
// v.(float64) alone would silently take the default for every commit made
// through the other path.
func TestParseTTLFloorReadsEveryDeliveredShape(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  uint32
	}{
		{name: "string", value: "300", want: 300},
		{name: "number", value: float64(300), want: 300},
		{name: "absent", value: nil, want: ttlFloorDefault},
		{name: "unreadable", value: []any{1}, want: ttlFloorDefault},
		{name: "not a number", value: "soon", want: ttlFloorDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseTTLFloor(tc.value))
		})
	}
}

// TestParseTTLFloorBoundaries proves the range the YANG leaf declares is also
// enforced by the parser.
//
// BOUNDARY: ttl-floor, units seconds, range 1..86400.
//
// The YANG range refuses 0 and 86401 at commit, so a value outside it can only
// reach the parser through a path that bypassed the schema. Answering with the
// default rather than the out-of-range value is what keeps a zero from
// reaching refreshInterval, where it would mean "refresh immediately, forever".
func TestParseTTLFloorBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  uint32
	}{
		{name: "invalid below: zero", value: "0", want: ttlFloorDefault},
		{name: "invalid below: negative", value: float64(-1), want: ttlFloorDefault},
		{name: "first valid", value: "1", want: 1},
		{name: "last valid", value: "86400", want: 86400},
		{name: "invalid above", value: "86401", want: ttlFloorMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseTTLFloor(tc.value))
		})
	}
}

// TestParseDomainNamesReadsBothLeafListShapes proves a leaf-list holding one
// value is read as one name rather than dropped. A single-value leaf-list can
// arrive as the bare value on some paths.
func TestParseDomainNamesReadsBothLeafListShapes(t *testing.T) {
	assert.Equal(t, []string{"a.invalid"}, parseDomainNames([]any{"a.invalid"}))
	assert.Equal(t, []string{"a.invalid"}, parseDomainNames("a.invalid"))
	assert.Empty(t, parseDomainNames(nil))
	assert.Empty(t, parseDomainNames(""))
	assert.Empty(t, parseDomainNames([]any{"", nil}))
}

// TestExtractRefsFindsRulesNamingAGroup proves a rule's source-domain-group and
// destination-domain-group leaves are found, with the table the rule sits in.
//
// The table matters: ApplyAll merges by table NAME, so a set registered under
// the wrong table leaves the rule naming a set no owner supplied, and
// dropTablesMissingAProvidedSet holds the operator's whole table back.
func TestExtractRefsFindsRulesNamingAGroup(t *testing.T) {
	root := map[string]any{
		configRoot: map[string]any{
			"table": map[string]any{
				"filter": map[string]any{
					"chain": map[string]any{
						"input": map[string]any{
							"term": map[string]any{
								"permit-cdn": map[string]any{
									"from": map[string]any{"source-domain-group": "cdn"},
								},
								"deny-bad": map[string]any{
									"from": map[string]any{"destination-domain-group": "bad"},
								},
							},
						},
					},
				},
			},
		},
	}

	refs := extractRefsFromConfig(root)
	require.Len(t, refs, 2)
	assert.Equal(t, termRef{Group: "bad", TableName: "ze_filter"}, refs[0])
	assert.Equal(t, termRef{Group: "cdn", TableName: "ze_filter"}, refs[1])
}

// TestExtractRefsDeduplicatesWithinATable proves two rules naming one group in
// one table produce one reference. Two would register the same set twice, and
// nftables refuses a duplicate set name.
func TestExtractRefsDeduplicatesWithinATable(t *testing.T) {
	root := map[string]any{
		configRoot: map[string]any{
			"table": map[string]any{
				"filter": map[string]any{
					"chain": map[string]any{
						"input": map[string]any{
							"term": map[string]any{
								"a": map[string]any{"from": map[string]any{"source-domain-group": "cdn"}},
								"b": map[string]any{"from": map[string]any{"source-domain-group": "cdn"}},
							},
						},
					},
				},
			},
		},
	}
	assert.Len(t, extractRefsFromConfig(root), 1)
}

// TestReferencedGroupsIsWhatVerifyRefusesOn proves a group defined but never
// named by a rule is not in the referenced set. It enforces nothing, so
// refusing a commit for it would refuse a config that harms nobody.
func TestReferencedGroupsIsWhatVerifyRefusesOn(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "used"}, {Name: "unused"}},
		refs:   []termRef{{Group: "used", TableName: "ze_filter"}},
	}
	assert.Equal(t, []string{"used"}, cfg.referencedGroups())
	assert.Equal(t, []string{"unused", "used"}, cfg.groupNames())
}

// TestConfigAccessorsAreNilSafe proves the accessors answer on a nil config.
// Verify runs before the first configure on a fresh start, so a nil config is
// a state the plugin genuinely reaches.
func TestConfigAccessorsAreNilSafe(t *testing.T) {
	var cfg *domainConfig
	_, ok := cfg.groupByName("cdn")
	assert.False(t, ok)
	assert.Empty(t, cfg.groupNames())
	assert.Empty(t, cfg.referencedGroups())
}
