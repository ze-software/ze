// VALIDATES: fib-dummy plugin registration, single-source command and send
// declarations, config parsing, metrics series registration, and RFC 4724
// Graceful Restart Forwarding State bit behavior (AC-1, AC-9, AC-10, AC-11,
// AC-12, AC-13, AC-14).
// PREVENTS: fib-dummy claiming a data plane, drifting between registry and
// Stage 1 declarations, claiming forwarding state preservation, or misstating
// held routes as programmed in its command help.
package fibdummy

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type recordingMetricsRegistry struct {
	metrics.NopRegistry
	gauges   []string
	counters []string
}

func (r *recordingMetricsRegistry) Gauge(name, help string) metrics.Gauge {
	r.gauges = append(r.gauges, name)
	return r.NopRegistry.Gauge(name, help)
}

func (r *recordingMetricsRegistry) Counter(name, help string) metrics.Counter {
	r.counters = append(r.counters, name)
	return r.NopRegistry.Counter(name, help)
}

func TestFIBDummyDeclaresNoDataPlane(t *testing.T) {
	reg := registry.Lookup(pluginName)
	require.NotNil(t, reg, "fib-dummy must be registered by init()")

	assert.True(t, reg.ProgramsFIB, "fib-dummy must declare ProgramsFIB so Ze adds no writer beside it")
	assert.True(t, reg.NoDataPlaneChoice, "fib-dummy must mark NoDataPlaneChoice so the verdict refuses it beside another FIB plugin")
	assert.Empty(t, reg.DataPlane, "fib-dummy reaches no data plane and must leave DataPlane empty")
	assert.Equal(t, []string{configRoot}, reg.ConfigRoots)
	assert.Equal(t, []string{"rib"}, reg.Dependencies)
	assert.Nil(t, reg.PreservesForwardingState, "fib-dummy keeps no forwarding state across restart")

	wantSends := []rpc.MessageSendDecl{
		{Message: sysribevents.Namespace + ":" + sysribevents.EventReplayRequest},
		{Message: mplsfib.Namespace + ":" + mplsfib.EventReplayRequest},
	}
	assert.Equal(t, wantSends, reg.Sends)
	assert.Equal(t, reg.Sends, engineRegistration().Sends, "registry and Stage 1 Sends must share one source")

	wantReceives := []rpc.MessageReceiveDecl{
		{Message: sysribevents.Namespace + ":" + sysribevents.EventBestChange},
	}
	assert.Equal(t, wantReceives, reg.Receives)
	assert.Equal(t, reg.Receives, engineRegistration().Receives, "registry and Stage 1 Receives must share one source")
}

func TestCommandDeclsOneSource(t *testing.T) {
	reg := registry.Lookup(pluginName)
	require.NotNil(t, reg)

	decls := commandDecls()
	assert.Equal(t, decls, reg.Commands, "registry.Registration.Commands must come from commandDecls()")
	assert.Equal(t, decls, engineRegistration().Commands, "sdk.Registration.Commands must come from commandDecls()")
	require.Len(t, decls, 2)

	names := make([]string, 0, len(decls))
	for _, d := range decls {
		names = append(names, d.Name)
		lower := strings.ToLower(d.ShortHelp)
		assert.Contains(t, lower, "holds", "command %q ShortHelp must say it holds routes/counters", d.Name)
		assert.Contains(t, lower, "programs nowhere", "command %q ShortHelp must say it programs nowhere", d.Name)
	}
	assert.True(t, slices.Contains(names, cmdShowRoutes))
	assert.True(t, slices.Contains(names, cmdShowCounters))
}

func TestForwardingStateNotPreservedWithDummy(t *testing.T) {
	tree := map[string]any{
		"fib": map[string]any{
			"dummy": map[string]any{},
		},
	}
	assert.False(t, registry.ForwardingStatePreserved(tree))
}

func TestParseFIBDummyConfig(t *testing.T) {
	tests := []struct {
		name     string
		sections []sdk.ConfigSection
		wantErr  bool
	}{
		{
			name:     "empty container present",
			sections: []sdk.ConfigSection{{Root: configRoot, Data: `{"fib":{"dummy":{}}}`}},
		},
		{
			name:     "empty object removal",
			sections: []sdk.ConfigSection{{Root: configRoot, Data: `{}`}},
		},
		{
			name:     "unrelated root ignored",
			sections: []sdk.ConfigSection{{Root: "bgp", Data: `not-json`}},
		},
		{
			name:     "malformed JSON refused",
			sections: []sdk.ConfigSection{{Root: configRoot, Data: `{bad`}},
			wantErr:  true,
		},
		{
			name:     "missing fib/dummy object refused",
			sections: []sdk.ConfigSection{{Root: configRoot, Data: `{"other":{}}`}},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseFIBDummyConfig(tt.sections)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSetMetricsRegistryRegistersAllSeries(t *testing.T) {
	prev := metricsPtr.Load()
	t.Cleanup(func() { metricsPtr.Store(prev) })

	rec := &recordingMetricsRegistry{}
	SetMetricsRegistry(rec)

	m := metricsPtr.Load()
	require.NotNil(t, m)
	assert.NotNil(t, m.routesHeld)
	assert.NotNil(t, m.routeInstalls)
	assert.NotNil(t, m.routeReplaces)
	assert.NotNil(t, m.routeRemovals)
	assert.NotNil(t, m.labelEntriesRefused)

	assert.Equal(t, []string{"ze_fibdummy_routes_held"}, rec.gauges)
	assert.Equal(t, []string{
		"ze_fibdummy_route_installs_total",
		"ze_fibdummy_route_replaces_total",
		"ze_fibdummy_route_removals_total",
		"ze_fibdummy_label_entries_refused_total",
	}, rec.counters)
}
