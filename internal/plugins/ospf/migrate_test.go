package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/component/policy"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify/yang"
	_ "github.com/ze-software/ze/internal/component/policy/yang"
)

// oldRedistributeConfig is a file written before the export chain: one entry
// with all three retired leaves, one with tag 0 alone, one with neither.
const oldRedistributeConfig = `
ospf {
	router-id 10.0.0.1;
	redistribute static {
		metric 50;
		metric-type type-1;
		tag 7;
	}
	redistribute connected {
		tag 0;
		nssa-propagate true;
	}
	redistribute bgp {
	}
}
`

// TestMigrateOSPFRedistributeToExportFilter proves `ze config migrate`
// rewrites the retired `ospf redistribute` metric, metric-type and tag leaves
// into policy modify instances named on `ospf filter export`, and that the
// result loads under the current schema.
//
// VALIDATES: the current schema refuses the retired leaves naming the fix; the
// migration schema parses them; the transformation writes a modify matching
// protocol static that sets metric 50 and type-1, a modify matching protocol
// static and tag 0 that sets tag 7 (the old tag applied to an untagged route
// only), nothing for tag 0, filters in that order with final accept, and keeps
// nssa-propagate; a second pass changes nothing.
// PREVENTS: a config written before the chain that no longer loads, or one the
// migration rewrites into an export chain that drops a route or a set.
func TestMigrateOSPFRedistributeToExportFilter(t *testing.T) {
	_, err := config.NewParser(mustOSPFSchema(t, config.YANGSchema)).Parse(oldRedistributeConfig)
	require.Error(t, err, "the current schema refuses the retired leaves")
	assert.Contains(t, err.Error(), "ze config migrate", "the refusal names the fix")

	tree, err := config.NewParser(mustOSPFSchema(t, config.MigrationSchema)).Parse(oldRedistributeConfig)
	require.NoError(t, err)
	result, err := migration.Migrate(tree)
	require.NoError(t, err)
	assert.Contains(t, result.Applied, redistributeLeavesTransformationName)

	schema := mustOSPFSchema(t, config.YANGSchema)
	reparsed, err := config.NewParser(schema).Parse(config.Serialize(result.Tree, schema))
	require.NoError(t, err, "the migrated file loads under the current schema")

	export := reparsed.GetContainer(Namespace).GetContainer("filter").GetContainer("export")
	require.NotNil(t, export, "the migration wrote ospf filter export")
	assert.Equal(t, []string{"ospf-export-static", "ospf-export-static-tag"}, export.GetMultiValues("filters"))
	final, _ := export.Get("final")
	assert.Equal(t, "accept", final)

	modifies := reparsed.GetContainer(policy.RootName).GetList("modify")
	require.Len(t, modifies, 2, "tag 0 and an entry with no retired leaf write no modify")
	metricSet := modifies["ospf-export-static"]
	require.NotNil(t, metricSet)
	assert.Equal(t, []string{"static"}, metricSet.GetContainer("match").GetMultiValues("protocol"))
	assert.Empty(t, metricSet.GetContainer("match").GetMultiValues("tag"), "the metric applies to every static route")
	metric, _ := metricSet.GetContainer("set").Get("metric")
	assert.Equal(t, "50", metric)
	metricType, _ := metricSet.GetContainer("set").Get("metric-type")
	assert.Equal(t, "type-1", metricType)
	tagSet := modifies["ospf-export-static-tag"]
	require.NotNil(t, tagSet)
	assert.Equal(t, []string{"static"}, tagSet.GetContainer("match").GetMultiValues("protocol"))
	assert.Equal(t, []string{"0"}, tagSet.GetContainer("match").GetMultiValues("tag"), "the tag applies to an untagged route only")
	tag, _ := tagSet.GetContainer("set").Get("tag")
	assert.Equal(t, "7", tag)

	connected := reparsed.GetContainer(Namespace).GetList("redistribute")["connected"]
	require.NotNil(t, connected)
	propagate, _ := connected.Get("nssa-propagate")
	assert.Equal(t, "true", propagate, "nssa-propagate stays on the entry")

	again, err := migration.Migrate(reparsed)
	require.NoError(t, err)
	assert.NotContains(t, again.Applied, redistributeLeavesTransformationName, "a migrated file needs no second pass")
}

// TestMigrateOSPFRedistributeRefusesNameClash proves the migration refuses,
// rather than overwrites, a policy modify instance holding the name it writes.
func TestMigrateOSPFRedistributeRefusesNameClash(t *testing.T) {
	tree, err := config.NewParser(mustOSPFSchema(t, config.MigrationSchema)).Parse(`
policy { modify ospf-export-static { set { tag 1; } } }
ospf { router-id 10.0.0.1; redistribute static { metric 50; } }
`)
	require.NoError(t, err)
	_, err = migration.Migrate(tree)
	require.ErrorIs(t, err, errMigratedModifyExists)
	assert.Contains(t, err.Error(), "rename policy modify ospf-export-static")
}

func mustOSPFSchema(t *testing.T, build func() (*config.Schema, error)) *config.Schema {
	t.Helper()
	schema, err := build()
	require.NoError(t, err)
	return schema
}
