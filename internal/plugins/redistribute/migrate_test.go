package redistribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
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
	assert.Empty(t, rule("bgp", "connected").GetMultiValues("filters"), "a rule with neither leaf names no filter")
	assert.Equal(t, ribpolicy.ActionAccept, verdict("bgp", "connected", v4(9)), "a rule with neither leaf imports every route")

	again, err := migration.Migrate(reparsed)
	require.NoError(t, err)
	assert.NotContains(t, again.Applied, importLeavesTransformationName, "a migrated file needs no second pass")
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
