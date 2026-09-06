package domain

import (
	"testing"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
)

// rulesetBase builds the payload handleShowFirewallRuleset produces, in the
// shape it arrives in after crossing the plugin boundary as JSON: every map is
// map[string]any and every list is []any.
func rulesetBase(setName string, addresses ...string) map[string]any {
	elements := make([]any, 0, len(addresses))
	for _, addr := range addresses {
		elements = append(elements, map[string]any{"value": addr})
	}
	return map[string]any{
		"table":  "filter",
		"family": "inet",
		"chains": []any{},
		"sets": []any{
			map[string]any{
				"name":     setName,
				"type":     int(firewall.SetTypeIPv4),
				"elements": elements,
			},
		},
	}
}

// TestEnrichShowAttachesNameToAddress proves an operator reading a ruleset sees
// which DNS name supplied each address.
//
// VALIDATES: AC-8 -- each address in the group's set is displayed with the DNS
// name that supplied it.
// PREVENTS: a group holding several names rendering as a list of addresses with
// no way to tell which name put each one there.
func TestEnrichShowAttachesNameToAddress(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid", "b.invalid"}, TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	plug, _ := newTestPlugin(t, cfg, map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
		stubKey("b.invalid", mdns.TypeA): {records: []string{"192.0.2.2"}, ttl: 300, status: "NOERROR"},
	})
	for _, name := range cfg.groups[0].Names {
		_, err := plug.resolveAndRecord(nameKey{group: "cdn", name: name, family: familyV4})
		require.NoError(t, err)
	}

	v4Name, _ := setNames("cdn")
	out, err := plug.enrichShow(enrichCommand, enrichKey, "detail", rulesetBase(v4Name, "192.0.2.1", "192.0.2.2"))
	require.NoError(t, err)
	require.NotNil(t, out, "the enricher must answer for a set it supplied")

	byAddress := sourceNames(t, out)
	assert.Equal(t, "a.invalid", byAddress["192.0.2.1"])
	assert.Equal(t, "b.invalid", byAddress["192.0.2.2"])
}

// TestEnrichShowLeavesAnotherOwnersSetAlone proves the sets of copp,
// policy-routes, flowspec and firewall-irr render exactly as they did.
//
// The enricher returns the whole `sets` value because registerProxyEnrichers
// merges with maps.Copy at the top level, so it MUST copy through what it does
// not own. Dropping another owner's set would remove it from the operator's
// view of the ruleset.
func TestEnrichShowLeavesAnotherOwnersSetAlone(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	v4Name, _ := setNames("cdn")
	base := rulesetBase(v4Name, "192.0.2.1")
	ownSets, ok := base["sets"].([]any)
	require.True(t, ok)
	base["sets"] = append(ownSets, map[string]any{
		"name":     "irr_v4_AS65001",
		"type":     int(firewall.SetTypeIPv4),
		"elements": []any{map[string]any{"value": "198.51.100.1"}},
	})

	out, err := plug.enrichShow(enrichCommand, enrichKey, "detail", base)
	require.NoError(t, err)
	require.NotNil(t, out)

	sets, ok := out["sets"].([]any)
	require.True(t, ok)
	require.Len(t, sets, 2, "the other owner's set must still be there")

	irr, ok := sets[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "irr_v4_AS65001", irr["name"])
	elements, ok := irr["elements"].([]any)
	require.True(t, ok)
	element, ok := elements[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, element, keySourceName, "an address this plugin did not supply carries no name")
}

// TestEnrichShowAnswersNothingForAnotherCommand proves the enricher is scoped
// to the command it declared. A registry that delivered it elsewhere must not
// see it rewrite an unrelated payload's `sets` key.
func TestEnrichShowAnswersNothingForAnotherCommand(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)

	out, err := plug.enrichShow("show bgp summary", enrichKey, "detail", rulesetBase("domain_v4_cdn", "192.0.2.1"))
	require.NoError(t, err)
	assert.Empty(t, out, "an answer for another command adds nothing")

	out, err = plug.enrichShow(enrichCommand, "some-other-key", "detail", rulesetBase("domain_v4_cdn", "192.0.2.1"))
	require.NoError(t, err)
	assert.Empty(t, out, "an answer for another key adds nothing")
}

// TestEnrichShowAnswersNothingWithNoResolvedAddresses proves a plugin holding
// no data adds nothing, so a node whose groups have never resolved renders the
// ruleset exactly as a node without the plugin.
func TestEnrichShowAnswersNothingWithNoResolvedAddresses(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)
	out, err := plug.enrichShow(enrichCommand, enrichKey, "detail", rulesetBase("domain_v4_cdn", "192.0.2.1"))
	require.NoError(t, err)
	assert.Empty(t, out, "a plugin holding no data adds nothing")
}

// TestEnrichShowToleratesAPayloadWithNoSets proves a ruleset carrying no sets
// does not panic the enricher. show.Enrich recovers a panic and logs it, so the
// failure would be a silent column rather than a crash, which is worse.
func TestEnrichShowToleratesAPayloadWithNoSets(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	for _, base := range []map[string]any{
		{},
		{"sets": nil},
		{"sets": []any{}},
		{"sets": "not a list"},
		{"sets": []any{"not a map"}},
		{"sets": []any{map[string]any{"name": "domain_v4_cdn"}}},
		{"sets": []any{map[string]any{"name": "domain_v4_cdn", "elements": []any{"not a map"}}}},
	} {
		out, enrichErr := plug.enrichShow(enrichCommand, enrichKey, "detail", base)
		require.NoError(t, enrichErr)
		_ = out
	}
}

// TestEnrichCommandMatchesTheShowHandler pins the two spellings of the command
// name to each other. The enricher is delivered by command STRING, so a
// divergence would silently drop the column with nothing failing.
func TestEnrichCommandMatchesTheShowHandler(t *testing.T) {
	assert.Equal(t, "show firewall ruleset", enrichCommand)
	assert.Equal(t, "domain-group", enrichKey, "the key must stay kebab-case: the engine refuses any other shape")
}

// sourceNames reads the address-to-name mapping back out of an enricher answer.
func sourceNames(t *testing.T, out map[string]any) map[string]string {
	t.Helper()
	sets, ok := out["sets"].([]any)
	require.True(t, ok)

	byAddress := map[string]string{}
	for _, rawSet := range sets {
		set, ok := rawSet.(map[string]any)
		if !ok {
			continue
		}
		elements, ok := set["elements"].([]any)
		if !ok {
			continue
		}
		for _, rawElement := range elements {
			element, ok := rawElement.(map[string]any)
			if !ok {
				continue
			}
			value, _ := element["value"].(string)
			name, _ := element[keySourceName].(string)
			if value != "" && name != "" {
				byAddress[value] = name
			}
		}
	}
	return byAddress
}
