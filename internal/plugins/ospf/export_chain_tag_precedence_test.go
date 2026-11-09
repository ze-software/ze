// VALIDATES: an `ospf filter export` chain whose tag modify matches
// `protocol [ static ]; tag [ 0 ]` keeps the route tag precedence at runtime:
// a route carrying its own tag keeps it, and an untagged route takes the
// configured tag.
// PREVENTS: a per-source tag that overrides a route's own tag in silence.
package ospf

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"

	// The chain names policy modify instances: the modify filter type and its
	// route evaluator register from this package's init.
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

// TestExportChainKeepsRouteTagPrecedence proves a per-source tag set by a
// modify that also matches `tag [ 0 ]` reaches an untagged route only.
//
// Method: parse a config under the current schema, deliver the ospf and policy
// roots as the engine does (each wrapped in its own name, lowered by
// ToPluginMap), compile them through parseExportChain, install the chain, and
// inject static routes shaped as the redistribute orchestrator's redistRoute
// builds them (Tag 0 and HasTag true for an untagged route). The tag-only chain
// leaves the default metric and type 2; the chain that also names a metric
// modify sets metric 50 and type 1 on every route as well.
func TestExportChainKeepsRouteTagPrecedence(t *testing.T) {
	const tagModify = "modify ospf-export-static-tag { match { protocol [ static ]; tag [ 0 ]; } set { tag 7; } } "
	const metricModify = "modify ospf-export-static { match { protocol [ static ]; } set { metric 50; metric-type type-1; } } "
	cases := []struct {
		name     string
		modifies string
		filters  string
		metric   uint32
		type2    bool
	}{
		{name: "tag only", modifies: tagModify, filters: "ospf-export-static-tag", metric: ospfredistribute.DefaultExternalMetric, type2: true},
		{name: "metric, type and tag", modifies: metricModify + tagModify, filters: "ospf-export-static ospf-export-static-tag", metric: 50, type2: false},
	}
	staticID := redistevents.RegisterProtocol("static")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installExportChain(t, "policy { "+tc.modifies+"} "+
				"ospf { router-id 10.0.0.1; redistribute static { } filter { export { filters [ "+tc.filters+" ]; final accept; } } }")

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
				require.True(t, ok, "%s: the chain accepts a static route", r.prefix)
				assert.Equal(t, r.want, ext.Tag, "%s: route tag %d", r.prefix, r.tag)
				assert.Equal(t, tc.metric, ext.Metric, "%s: metric", r.prefix)
				assert.Equal(t, tc.type2, ext.Type2, "%s: metric type", r.prefix)
			}
		})
	}
}

// installExportChain parses text, compiles the export chain it names, and
// installs it until the test ends.
func installExportChain(t *testing.T, text string) {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(text)
	require.NoError(t, err)

	var sections []configSection
	for _, root := range []string{Namespace, policy.RootName} {
		container := tree.GetContainer(root)
		require.NotNil(t, container, "the config holds %s", root)
		data, err := json.Marshal(map[string]any{root: container.ToPluginMap()})
		require.NoError(t, err)
		sections = append(sections, configSection{Root: root, Data: string(data)})
	}
	cfg, err := parseExportChain(sections)
	require.NoError(t, err)
	require.NotNil(t, cfg.chain, "the config names ospf filter export")
	cfg.install()
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
}
