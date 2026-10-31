// VALIDATES: the export chain `ze config migrate` writes for a retired
// `ospf redistribute static { tag 7; }` keeps the old tag precedence at
// runtime: a route carrying its own tag keeps it, and an untagged route takes
// the configured tag.
// PREVENTS: a migration whose tag modify matches every route, which overrides
// a route's own tag with the configured one in silence.
package ospf

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"

	// The migration writes policy modify instances: the modify filter type
	// and its route evaluator register from this package's init.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify"
)

// externalsInjector records the AS-external fields each injected prefix
// carries, standing in for the OSPFv2 engine.
type externalsInjector struct {
	externals map[netip.Prefix]ospfredistribute.External
}

func (r *externalsInjector) InjectExternal(p netip.Prefix, _ string, ext ospfredistribute.External) error {
	r.externals[p] = ext
	return nil
}

func (r *externalsInjector) WithdrawExternal(p netip.Prefix) (bool, error) {
	_, ok := r.externals[p]
	delete(r.externals, p)
	return ok, nil
}

// TestMigratedExportChainKeepsRouteTagPrecedence proves the chain the
// migration writes for a retired per-source tag sets that tag on an untagged
// route only.
//
// Method: migrate an old config, reparse it under the current schema, deliver
// the ospf and policy roots as the engine does (each wrapped in its own name,
// lowered by ToPluginMap), compile them through parseExportChain, install the
// chain, and inject static routes shaped as the redistribute orchestrator's
// redistRoute builds them (Tag 0 and HasTag true for an untagged route). The
// tag-only entry leaves the default metric and type 2; the entry with all
// three retired leaves sets metric 50 and type 1 on every route as well.
func TestMigratedExportChainKeepsRouteTagPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		leaves string
		metric uint32
		type2  bool
	}{
		{name: "tag only", leaves: "tag 7;", metric: ospfredistribute.DefaultExternalMetric, type2: true},
		{name: "metric, type and tag", leaves: "metric 50; metric-type type-1; tag 7;", metric: 50, type2: false},
	}
	staticID := redistevents.RegisterProtocol("static")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installMigratedExportChain(t, "ospf { router-id 10.0.0.1; redistribute static { "+tc.leaves+" } }")

			inj := &externalsInjector{externals: map[netip.Prefix]ospfredistribute.External{}}
			consumer := ospfredistribute.NewConsumer(inj)
			routes := []struct {
				prefix string
				tag    uint32
				want   uint32
			}{
				{prefix: "10.1.0.0/24", tag: 4242, want: 4242},
				{prefix: "10.2.0.0/24", tag: 0xFFFFFFFF, want: 0xFFFFFFFF},
				{prefix: "10.3.0.0/24", tag: 0, want: 7},
			}
			for _, r := range routes {
				prefix := netip.MustParsePrefix(r.prefix)
				consumer.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
					Prefix: r.prefix,
					Source: "static",
					Tag:    r.tag,
					Route: ribpolicy.Route{
						Family: family.IPv4Unicast,
						Prefix: prefix,
						Source: staticID,
						Tag:    r.tag,
						HasTag: true,
					},
				})
				ext, ok := inj.externals[prefix]
				require.True(t, ok, "%s: the migrated chain accepts a static route", r.prefix)
				assert.Equal(t, r.want, ext.Tag, "%s: route tag %d", r.prefix, r.tag)
				assert.Equal(t, tc.metric, ext.Metric, "%s: metric", r.prefix)
				assert.Equal(t, tc.type2, ext.Type2, "%s: metric type", r.prefix)
			}
		})
	}
}

// installMigratedExportChain migrates old, compiles the export chain the
// result names, and installs it until the test ends.
func installMigratedExportChain(t *testing.T, old string) {
	t.Helper()
	tree, err := config.NewParser(mustOSPFSchema(t, config.MigrationSchema)).Parse(old)
	require.NoError(t, err)
	result, err := migration.Migrate(tree)
	require.NoError(t, err)
	require.Contains(t, result.Applied, redistributeLeavesTransformationName)
	schema := mustOSPFSchema(t, config.YANGSchema)
	reparsed, err := config.NewParser(schema).Parse(config.Serialize(result.Tree, schema))
	require.NoError(t, err)

	var sections []configSection
	for _, root := range []string{Namespace, policy.RootName} {
		container := reparsed.GetContainer(root)
		require.NotNil(t, container, "the migrated config holds %s", root)
		data, err := json.Marshal(map[string]any{root: container.ToPluginMap()})
		require.NoError(t, err)
		sections = append(sections, configSection{Root: root, Data: string(data)})
	}
	cfg, err := parseExportChain(sections)
	require.NoError(t, err)
	require.NotNil(t, cfg.chain, "the migration wrote ospf filter export")
	cfg.install()
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
}
