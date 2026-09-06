package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestBuildDomainGroupTablesMatchesResolvedAddresses proves the set holds
// exactly the addresses the names resolved to, in the table whose rule names
// the group.
//
// VALIDATES: AC-1 -- the nftables set for a group holds exactly the addresses
// the name resolves to.
func TestBuildDomainGroupTablesMatchesResolvedAddresses(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid"}}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	tables := buildTables(cfg, func(_ group, fam family) []string {
		if fam.isV4 {
			return []string{"192.0.2.1", "192.0.2.2"}
		}
		return []string{"2001:db8::1"}
	})

	require.Len(t, tables, 1)
	assert.Equal(t, "ze_filter", tables[0].Name)
	assert.Equal(t, firewall.FamilyInet, tables[0].Family)
	require.Len(t, tables[0].Sets, 2)

	v4 := tables[0].Sets[0]
	assert.Equal(t, "domain_v4_cdn", v4.Name)
	assert.Equal(t, firewall.SetTypeIPv4, v4.Type)
	assert.Equal(t, []firewall.SetElement{{Value: "192.0.2.1"}, {Value: "192.0.2.2"}}, v4.Elements)

	v6 := tables[0].Sets[1]
	assert.Equal(t, "domain_v6_cdn", v6.Name)
	assert.Equal(t, firewall.SetTypeIPv6, v6.Type)
	assert.Equal(t, []firewall.SetElement{{Value: "2001:db8::1"}}, v6.Elements)
}

// TestBuildSetsDeclaresBothFamiliesOrNeither proves a group answering only
// IPv4 still declares its IPv6 set, empty.
//
// The config parser emits an IPv6 twin term for every domain-group term
// (expandProvidedTermV6, internal/component/firewall/config.go). A group with
// no AAAA answer would leave that twin naming a set no owner declared, and
// dropTablesMissingAProvidedSet would hold the operator's WHOLE table back
// while the commit reported success. That is a filter silently leaving the
// kernel, which is the failure this shape exists to prevent.
func TestBuildSetsDeclaresBothFamiliesOrNeither(t *testing.T) {
	sets := buildGroupSets("cdn", []string{"192.0.2.1"}, nil)
	require.Len(t, sets, 2, "the v6 twin term needs a set even with nothing in it")
	assert.Equal(t, "domain_v6_cdn", sets[1].Name)
	assert.Empty(t, sets[1].Elements, "no address of that family belongs to this group")

	sets = buildGroupSets("cdn", nil, []string{"2001:db8::1"})
	require.Len(t, sets, 2)
	assert.Empty(t, sets[0].Elements)

	assert.Nil(t, buildGroupSets("cdn", nil, nil),
		"a group holding nothing declares nothing, so a cold cache keeps the table held back")
}

// TestBuildSetsUsesNoIntervalFlag proves a domain group holds single addresses.
// A DNS A record carries one address, and an interval set of /32 singletons
// costs the kernel more for the same membership test.
func TestBuildSetsUsesNoIntervalFlag(t *testing.T) {
	sets := buildGroupSets("cdn", []string{"192.0.2.1"}, []string{"2001:db8::1"})
	require.Len(t, sets, 2)
	for _, set := range sets {
		assert.Equal(t, firewall.SetFlags(0), set.Flags)
		for _, element := range set.Elements {
			assert.False(t, element.IntervalEnd, "a single address has no interval end")
		}
	}
}

// TestBuildTablesGroupsBySameTable proves two groups named by rules in one
// table land in one table, and groups in different tables stay apart.
func TestBuildTablesGroupsBySameTable(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		refs: []termRef{
			{Group: "a", TableName: "ze_filter"},
			{Group: "b", TableName: "ze_filter"},
			{Group: "c", TableName: "ze_edge"},
		},
	}
	tables := buildTables(cfg, func(group, family) []string { return []string{"192.0.2.1"} })

	require.Len(t, tables, 2)
	byName := map[string][]firewall.Set{}
	for i := range tables {
		byName[tables[i].Name] = tables[i].Sets
	}
	assert.Len(t, byName["ze_filter"], 4, "two groups, two families each")
	assert.Len(t, byName["ze_edge"], 2)
}

// TestBuildTablesSkipsAGroupWithNoAddresses proves a group that has never
// resolved contributes no set, so a cold cache leaves its table held back
// rather than programming an empty filter. An empty permit set blocks
// everything and an empty deny set blocks nothing; neither is what the
// operator wrote.
func TestBuildTablesSkipsAGroupWithNoAddresses(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn"}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	assert.Nil(t, buildTables(cfg, func(group, family) []string { return nil }))
}

// TestBuildTablesIgnoresAReferenceWithNoGroup proves a rule naming a group no
// config defines registers nothing. Registering a set for it would invent one,
// and the firewall's own verify already refuses that config.
func TestBuildTablesIgnoresAReferenceWithNoGroup(t *testing.T) {
	cfg := &domainConfig{refs: []termRef{{Group: "ghost", TableName: "ze_filter"}}}
	assert.Nil(t, buildTables(cfg, func(group, family) []string { return []string{"192.0.2.1"} }))
}

// TestBuildTablesDeclaresEachSetOnce proves two rules naming one group in one
// table declare the set once. nftables refuses a duplicate set name, so a
// second declaration would fail the whole table.
func TestBuildTablesDeclaresEachSetOnce(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn"}},
		refs: []termRef{
			{Group: "cdn", TableName: "ze_filter"},
			{Group: "cdn", TableName: "ze_filter"},
		},
	}
	tables := buildTables(cfg, func(group, family) []string { return []string{"192.0.2.1"} })
	require.Len(t, tables, 1)
	assert.Len(t, tables[0].Sets, 2, "one set per family, not one per rule")
}

// TestSetNamesCarryTheOwnershipPrefix proves the table name this plugin
// registers under keeps the prefix RegisterTables requires. Without it every
// registration is refused and the feature never programs anything.
func TestSetNamesCarryTheOwnershipPrefix(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn"}},
		refs:   []termRef{{Group: "cdn", TableName: tableNamePrefix + "filter"}},
	}
	tables := buildTables(cfg, func(group, family) []string { return []string{"192.0.2.1"} })
	require.Len(t, tables, 1)
	assert.NoError(t, firewall.RegisterTables(tableOwner, tables),
		"RegisterTables refuses a table name without the ownership prefix")
	assert.NoError(t, firewall.RegisterTables(tableOwner, nil))
}

// TestSetNamesComeFromTheFirewallParser proves this plugin builds the set
// names the firewall's config parser matches against.
//
// One declaration rather than two spellings: firewall.DomainGroupSetNames is
// the single source, and domainSetMatch in the parser calls the same function.
// A divergence would leave every rule naming a set no owner supplies, so
// dropTablesMissingAProvidedSet would hold back each table carrying one while
// the commit still reported success.
func TestSetNamesComeFromTheFirewallParser(t *testing.T) {
	v4, v6 := setNames("cdn")
	wantV4, wantV6 := firewall.DomainGroupSetNames("cdn")
	assert.Equal(t, wantV4, v4)
	assert.Equal(t, wantV6, v6)
	assert.NotEqual(t, v4, v6, "the two families must be different sets")
}
