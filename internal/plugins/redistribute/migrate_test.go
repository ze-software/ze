package redistribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"

	_ "github.com/ze-software/ze/internal/component/config/redistribute/yang"
	_ "github.com/ze-software/ze/internal/component/policy/yang"
)

// oldImportConfig is a file written before the filter chain: one rule with both
// retired leaves, one with family alone, one with tag 0, one with neither.
const oldImportConfig = `
redistribute {
	destination ospf {
		import static {
			family [ ipv4/unicast ];
			tag 42;
		}
		import connected {
			family [ ipv6/unicast ];
		}
	}
	destination bgp {
		import static {
			tag 0;
		}
		import connected {
		}
	}
}
`

// TestMigrateImportLeavesToFilters proves `ze config migrate` rewrites the
// retired import leaves into an equivalent policy match filter, and that the
// result loads under the current schema with the same verdicts the old leaves
// gave.
//
// VALIDATES: the migration schema parses the old leaves, the registered
// transformation writes policy match NAME plus filters [ NAME ]; final accept,
// and the compiled chain accepts exactly what the old rule imported.
// PREVENTS: a config written before the chain that no longer loads, or one the
// migration rewrites into a chain importing more or less than before.
func TestMigrateImportLeavesToFilters(t *testing.T) {
	_, err := config.NewParser(mustSchema(t, config.YANGSchema)).Parse(oldImportConfig)
	require.Error(t, err, "the current schema refuses the retired leaves")
	assert.Contains(t, err.Error(), "ze config migrate", "the refusal names the fix")

	tree, err := config.NewParser(mustSchema(t, config.MigrationSchema)).Parse(oldImportConfig)
	require.NoError(t, err)
	result, err := migration.Migrate(tree)
	require.NoError(t, err)
	assert.Contains(t, result.Applied, importLeavesTransformationName)
	migrated := result.Tree

	schema := mustSchema(t, config.YANGSchema)
	reparsed, err := config.NewParser(schema).Parse(config.Serialize(migrated, schema))
	require.NoError(t, err, "the migrated file loads under the current schema")

	rule := func(dest, source string) *config.Tree {
		return reparsed.GetContainer(configRoot).GetList("destination")[dest].GetList("import")[source]
	}
	policyCfg, ok := reparsed.ToPluginMap()[policy.RootName].(map[string]any)
	require.True(t, ok, "the migration wrote the policy root")

	verdict := func(dest, source string, route ribpolicy.Route) ribpolicy.Action {
		t.Helper()
		r := rule(dest, source)
		final, hasFinal := r.Get("final")
		require.True(t, hasFinal, "%s %s carries a final", dest, source)
		require.Equal(t, "accept", final)
		chain, err := policy.CompileRouteChain(chainAttachment, r.GetMultiValues("filters"), policy.FinalAccept, policyCfg)
		require.NoError(t, err)
		return chain.Evaluate(route).Action
	}
	v4 := func(tag uint32) ribpolicy.Route {
		return ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24"), Tag: tag, HasTag: true}
	}
	v6 := ribpolicy.Route{Family: family.IPv6Unicast, Prefix: netip.MustParsePrefix("2001:db8::/32"), HasTag: true}

	assert.Equal(t, []string{"redistribute-ospf-static"}, rule("ospf", "static").GetMultiValues("filters"))
	assert.Equal(t, ribpolicy.ActionAccept, verdict("ospf", "static", v4(42)), "family and tag met")
	assert.Equal(t, ribpolicy.ActionReject, verdict("ospf", "static", v4(7)), "another tag")
	assert.Equal(t, ribpolicy.ActionReject, verdict("ospf", "static", ribpolicy.Route{Family: family.IPv6Unicast, Tag: 42, HasTag: true}), "another family")
	assert.Equal(t, ribpolicy.ActionAccept, verdict("ospf", "connected", v6), "family alone")
	assert.Equal(t, ribpolicy.ActionReject, verdict("ospf", "connected", v4(0)), "family alone, another family")
	assert.Equal(t, ribpolicy.ActionAccept, verdict("bgp", "static", v4(0)), "tag 0 keeps selecting the untagged routes")
	assert.Equal(t, ribpolicy.ActionReject, verdict("bgp", "static", v4(9)), "tag 0 rejects a tagged route")
	// A rule with neither leaf was already a current shape: the migration
	// leaves it as written, and the loader reads it as a rule with no chain,
	// which imports every route of its source.
	bare := rule("bgp", "connected")
	assert.Empty(t, bare.GetMultiValues("filters"), "a rule with neither leaf names no filter")
	_, bareFinal := bare.Get("final")
	assert.False(t, bareFinal, "a rule with neither leaf is left as written, with no final")
	for _, source := range []string{"static", "connected"} {
		require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: source, Protocol: source}))
	}
	rules, err := config.ExtractRedistributeRules(reparsed)
	require.NoError(t, err)
	require.NoError(t, compileRuleChains(rules, policyCfg))
	ev := configredist.NewEvaluator(rules)
	for _, route := range []ribpolicy.Route{v4(9), v4(0), v6} {
		_, accepted := ev.Accept(&configredist.RedistRoute{Origin: "connected", Source: "connected", Route: route}, "bgp")
		assert.True(t, accepted, "a rule with neither leaf imports every route (%s tag %d)", route.Prefix, route.Tag)
	}

	again, err := migration.Migrate(reparsed)
	require.NoError(t, err)
	assert.NotContains(t, again.Applied, importLeavesTransformationName, "a migrated file needs no second pass")
}

// TestMigrateLeavesCurrentImportsAlone proves `ze config migrate`, and the
// commit path's NeedsMigration that reads the same Detect, finds nothing to
// rewrite in an import that names no retired leaf: a bare import, which
// imports every route of its source, and one with a chain.
//
// VALIDATES: Detect answers only for the retired family and tag leaves.
// PREVENTS: a current config rewritten by every migrate, or a commit told its
// config needs migrating when it does not.
func TestMigrateLeavesCurrentImportsAlone(t *testing.T) {
	const current = `
policy { match T42 { tag [ 42 ]; } }
redistribute {
	destination bgp {
		import static {
		}
		import connected {
			filters [ T42 ];
			final reject;
		}
	}
}
`
	schema := mustSchema(t, config.YANGSchema)
	tree, err := config.NewParser(schema).Parse(current)
	require.NoError(t, err, "a bare import and a chained one are current shapes")
	assert.False(t, hasImportToMigrate(tree), "an import naming no retired leaf needs no migration")
	assert.False(t, migration.NeedsMigration(tree), "the commit path does not ask for a migration")
	before := config.Serialize(tree, schema)

	result, err := migration.Migrate(tree)
	require.NoError(t, err)
	assert.NotContains(t, result.Applied, importLeavesTransformationName)
	assert.Equal(t, before, config.Serialize(result.Tree, schema), "a current config is left as written")
}

// TestMigrateImportLeavesRefusesNameClash proves the migration refuses, rather
// than overwrites, a policy match instance already holding the name it writes.
func TestMigrateImportLeavesRefusesNameClash(t *testing.T) {
	tree, err := config.NewParser(mustSchema(t, config.MigrationSchema)).Parse(`
policy { match redistribute-ospf-static { tag [ 1 ]; } }
redistribute { destination ospf { import static { tag 42; } } }
`)
	require.NoError(t, err)
	_, err = migration.Migrate(tree)
	require.ErrorIs(t, err, errMigratedFilterExists)
	assert.Contains(t, err.Error(), "rename policy match redistribute-ospf-static")
}

func mustSchema(t *testing.T, build func() (*config.Schema, error)) *config.Schema {
	t.Helper()
	schema, err := build()
	require.NoError(t, err)
	return schema
}
