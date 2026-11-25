// VALIDATES: spec-connected-static-reach-the-locrib -- a config with main-table
// static routes and no `fib { ... }` block loads the FIB plugin that programs
// the data plane `interface { backend }` selects (owner decision, 2026-09-06).
// PREVENTS: the static half of that spec leaving a route in the system RIB that
// nothing writes, and an auto-load overriding the backend an operator chose.

package server

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// autoLoadNames runs the config-path auto-load for the given paths and data
// plane and returns the plugin names it would start. A resolution refusal fails
// the test: every caller expects a set that resolves.
func autoLoadNames(t *testing.T, configuredPaths []string, dataPlane string) []string {
	t.Helper()

	s := &Server{
		config: &ServerConfig{
			ConfigNeedHolds: holdsForEmptyConfig,
			ConfiguredPaths: configuredPaths,
			DataPlane:       dataPlane,
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	got, err := s.getConfigPathPlugins()
	require.NoError(t, err)

	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	return names
}

// dataPlaneWriterNames returns the registered plugins that declare a data plane.
// The test asks the registry rather than naming fib-kernel and fib-vpp, for the
// reason the feature exists: a FIB plugin added later must need no edit here.
func dataPlaneWriterNames() []string {
	var names []string
	for _, reg := range registry.All() {
		if reg.DataPlane != "" {
			names = append(names, reg.Name)
		}
	}
	return names
}

// TestStaticWithNoFIBBlockAutoLoadsTheWriterForTheDataPlane proves the owner's
// decision: an operator writes static routes, writes no `fib { ... }` block, and
// the engine loads the writer for the data plane they already chose.
func TestStaticWithNoFIBBlockAutoLoadsTheWriterForTheDataPlane(t *testing.T) {
	writer, ok := registry.PluginForDataPlane("kernel")
	require.True(t, ok, "no plugin declares the kernel data plane")

	names := autoLoadNames(t, []string{"static"}, "kernel")

	assert.Contains(t, names, "static", "the static config path must still load static")
	assert.Contains(t, names, writer,
		"a static config with no fib block must load the writer for the kernel data plane")
}

// TestStaticNeedsNoWriterWhenNothingNeedsADataPlane proves the resolution is
// driven by the producer's declaration and not by the config path: a config with
// no plugin declaring NeedsDataPlane loads no FIB plugin.
func TestStaticNeedsNoWriterWhenNothingNeedsADataPlane(t *testing.T) {
	names := autoLoadNames(t, []string{"interface"}, "kernel")

	for _, writer := range dataPlaneWriterNames() {
		assert.NotContains(t, names, writer,
			"an interface-only config must not load a FIB plugin")
	}
}

// TestExplicitFIBBlockIsNotOverriddenByTheAutoLoad proves an operator who chose
// a backend gets exactly that one. fib-p4 programs no switch, so an auto-load
// that ignored the explicit block would be visible as a second FIB plugin
// beside it.
func TestExplicitFIBBlockIsNotOverriddenByTheAutoLoad(t *testing.T) {
	require.NotNil(t, registry.Lookup("fib-p4"), "fib-p4 is not registered")

	names := autoLoadNames(t, []string{"static", "fib/p4"}, "kernel")

	assert.Contains(t, names, "fib-p4", "the operator's explicit fib { p4 { } } block must load")
	writer, ok := registry.PluginForDataPlane("kernel")
	require.True(t, ok)
	assert.NotContains(t, names, writer,
		"the auto-load must not add a second FIB plugin beside the operator's choice")
}

// TestNoWriterForTheSelectedDataPlaneLoadsNoFIBPlugin proves the resolution
// fails closed. A data plane no plugin programs refuses the producer too,
// so neither an unwritten route nor a writer for the wrong data plane starts.
func TestNoWriterForTheSelectedDataPlaneLoadsNoFIBPlugin(t *testing.T) {
	_, ok := registry.PluginForDataPlane("p4-runtime")
	require.False(t, ok, "p4-runtime must declare no writer for this test to mean anything")

	s := &Server{
		config: &ServerConfig{
			ConfigNeedHolds: holdsForEmptyConfig,
			ConfiguredPaths: []string{"static"},
			DataPlane:       "p4-runtime",
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	got, err := s.getConfigPathPlugins()
	require.Error(t, err)
	assert.ErrorContains(t, err, "p4-runtime")
	assert.Empty(t, got)
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	for _, writer := range dataPlaneWriterNames() {
		assert.NotContains(t, names, writer,
			"no FIB plugin may load for a data plane none of them programs")
	}
}

// reloadFixtureNames holds isolated test plugin names and roots registered
// under a registry snapshot so reload writer judgement tests never depend on
// concrete shipped FIB plugin names.
type reloadFixtureNames struct {
	producerA     string
	producerARoot string
	producerB     string
	producerBRoot string
	planeA        string
	writerA       string
	writerARoot   string
	planeB        string
	writerB       string
	writerBRoot   string
	noPlaneFIB    string
	noPlaneRoot   string
	observerA     string
	observerB     string
}

func registerReloadWriterFixtures(t *testing.T) reloadFixtureNames {
	t.Helper()
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	f := reloadFixtureNames{
		producerA:     "reload-producer-a",
		producerARoot: "rp-a",
		producerB:     "reload-producer-b",
		producerBRoot: "rp-b",
		planeA:        "reload-plane-a",
		writerA:       "reload-writer-a",
		writerARoot:   "rfib/writer-a",
		planeB:        "reload-plane-b",
		writerB:       "reload-writer-b",
		writerBRoot:   "rfib/writer-b",
		noPlaneFIB:    "reload-noplane-fib",
		noPlaneRoot:   "rfib/noplane",
		observerA:     "reload-observer-a",
		observerB:     "reload-observer-b",
	}

	run := func(_ net.Conn) int { return 0 }
	cli := func([]string) int { return 0 }
	for _, reg := range []registry.Registration{
		{
			Name: f.producerA, Description: "test producer A", RunEngine: run, CLIHandler: cli,
			NeedsDataPlane: true, ConfigRoots: []string{f.producerARoot},
		},
		{
			Name: f.producerB, Description: "test producer B", RunEngine: run, CLIHandler: cli,
			NeedsDataPlane: true, ConfigRoots: []string{f.producerBRoot},
		},
		{
			Name: f.writerA, Description: "test writer A", RunEngine: run, CLIHandler: cli,
			ProgramsFIB: true, DataPlane: f.planeA, ConfigRoots: []string{f.writerARoot},
		},
		{
			Name: f.writerB, Description: "test writer B", RunEngine: run, CLIHandler: cli,
			ProgramsFIB: true, DataPlane: f.planeB, ConfigRoots: []string{f.writerBRoot},
		},
		{
			Name: f.noPlaneFIB, Description: "test no-plane FIB", RunEngine: run, CLIHandler: cli,
			ProgramsFIB: true, NoDataPlaneChoice: true, ConfigRoots: []string{f.noPlaneRoot},
		},
		{
			Name: "reload-primary-selector", Description: "test primary selector", RunEngine: run, CLIHandler: cli,
			DataPlaneSelector: &registry.DataPlaneSelector{
				LeafPath:           "/interface/backend",
				Roots:              []string{"interface"},
				SelectsDataPlane:   true,
				RegisteredBackends: func() []string { return []string{f.planeA, f.planeB} },
			},
		},
		{
			Name: f.observerA, Description: "test observer A", RunEngine: run, CLIHandler: cli,
		},
		{
			Name: f.observerB, Description: "test observer B", RunEngine: run, CLIHandler: cli,
		},
	} {
		require.NoError(t, registry.Register(reg))
	}
	return f
}

func newReloadWriterTestServer(
	t *testing.T,
	bootPlugins []plugin.PluginConfig,
	runningSelection []plugin.PluginConfig,
	hasRunningSelection bool,
	runningProcs ...plugin.PluginConfig,
) *Server {
	t.Helper()
	s := &Server{
		config: &ServerConfig{
			Plugins:               bootPlugins,
			CandidateSelection:    runningSelection,
			HasCandidateSelection: hasRunningSelection,
		},
		registry:            plugin.NewPluginRegistry(),
		loadedPlugins:       make(map[string]bool),
		runningSelection:    slices.Clone(runningSelection),
		hasRunningSelection: hasRunningSelection,
	}
	pm := process.NewProcessManager(nil)
	for _, cfg := range runningProcs {
		require.NoError(t, pm.AddProcess(cfg.Name, process.NewProcess(cfg)))
	}
	s.procManager.Store(pm)
	return s
}

func autoLoadedNames(entries []AutoLoadedEndpoint) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Config.Name)
	}
	return names
}

// TestReloadStopsEngineAddedWriter verifies that adding a chosen ProgramsFIB
// plugin by reload starts that plugin and stops the running engine-added writer.
//
// VALIDATES: spec-fib-dummy AC-5 -- adding a chosen FIB block puts the running
// engine-added writer in the reload stop set and the chosen FIB plugin in the
// reload start set.
// PREVENTS: a reload leaving both the engine-added writer and the operator's
// newly chosen FIB plugin running together.
func TestReloadStopsEngineAddedWriter(t *testing.T) {
	f := registerReloadWriterFixtures(t)
	s := newReloadWriterTestServer(t, nil, nil, true,
		plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
		plugin.PluginConfig{Name: f.writerA, CompiledPlugin: f.writerA, Internal: true},
	)

	runningTree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
	}
	newTree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
		"rfib":          map[string]any{"noplane": map[string]any{}},
	}

	starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, []string{"rfib", f.noPlaneRoot}, nil, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []string{f.noPlaneFIB}, autoLoadedNames(starts))

	stops := s.collectReloadStopProcesses(runningTree, newTree, []string{"rfib", f.noPlaneRoot}, nil, nil, true)
	assert.True(t, stops[f.writerA], "engine-added writer must be stopped when a chosen FIB plugin is added")
	assert.False(t, stops[f.producerA], "running producer must stay up")
}

// TestReloadStartsWriterWhenChoiceRemoved verifies that removing the last
// chosen ProgramsFIB plugin while a NeedsDataPlane producer still runs starts
// the configured data plane's writer and stops the removed FIB plugin.
//
// VALIDATES: spec-fib-dummy AC-6 -- remove-only reload starts the plane writer.
// PREVENTS: removing a chosen FIB block leaving live routes with no writer.
func TestReloadStartsWriterWhenChoiceRemoved(t *testing.T) {
	f := registerReloadWriterFixtures(t)
	s := newReloadWriterTestServer(t, nil, nil, true,
		plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
		plugin.PluginConfig{Name: f.noPlaneFIB, CompiledPlugin: f.noPlaneFIB, Internal: true},
	)

	runningTree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
		"rfib":          map[string]any{"noplane": map[string]any{}},
	}
	newTree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
	}

	starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, nil, []string{f.noPlaneRoot}, nil, true)
	require.NoError(t, err)
	require.Len(t, starts, 1)
	assert.Equal(t, f.writerA, starts[0].Config.Name)
	assert.Equal(t, AutoLoadReasonDataPlane, starts[0].Reason.Kind)

	stops := s.collectReloadStopProcesses(runningTree, newTree, nil, []string{f.noPlaneRoot}, nil, true)
	assert.True(t, stops[f.noPlaneFIB], "removed chosen FIB plugin must be stopped")
	assert.False(t, stops[f.producerA], "running producer must stay up")
}

// TestReloadKeepsChosenWriter verifies that an unrelated reload starts and
// stops no FIB plugin, whether the running writer is engine-added or chosen.
//
// VALIDATES: spec-fib-dummy AC-7, R-2 -- unrelated reload preserves FIB state.
// PREVENTS: spurious FIB writer churn on unrelated config edits.
func TestReloadKeepsChosenWriter(t *testing.T) {
	f := registerReloadWriterFixtures(t)

	t.Run("engine-added writer stays up", func(t *testing.T) {
		s := newReloadWriterTestServer(t, nil, nil, true,
			plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
			plugin.PluginConfig{Name: f.writerA, CompiledPlugin: f.writerA, Internal: true},
		)
		runningTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
		}
		newTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
			"system":        map[string]any{"hostname": "r1"},
		}
		starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, []string{"system"}, nil, nil, true)
		require.NoError(t, err)
		assert.Empty(t, starts)
		stops := s.collectReloadStopProcesses(runningTree, newTree, []string{"system"}, nil, nil, true)
		assert.Empty(t, stops)
	})

	t.Run("chosen FIB plugin stays up", func(t *testing.T) {
		s := newReloadWriterTestServer(t, nil, nil, true,
			plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
			plugin.PluginConfig{Name: f.noPlaneFIB, CompiledPlugin: f.noPlaneFIB, Internal: true},
		)
		runningTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
			"rfib":          map[string]any{"noplane": map[string]any{}},
		}
		newTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
			"rfib":          map[string]any{"noplane": map[string]any{}},
			"system":        map[string]any{"hostname": "r1"},
		}
		starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, []string{"system"}, nil, nil, true)
		require.NoError(t, err)
		assert.Empty(t, starts)
		stops := s.collectReloadStopProcesses(runningTree, newTree, []string{"system"}, nil, nil, true)
		assert.Empty(t, stops)
	})
}

// TestReloadKeepsCommandLineFIBPlugin verifies that a ProgramsFIB plugin held
// in the candidate selection (e.g. via --plugin) stays chosen across unrelated
// reloads and last-producer removal, and that boot-time s.config.Plugins never
// counts as chosen when the candidate selection omits it.
//
// VALIDATES: spec-fib-dummy AC-7, AC-16, R-2, A-9 -- reload writer judgement
// reads the candidate selection and never s.config.Plugins.
// PREVENTS: --plugin FIB choices being stopped on reload, or stale boot
// s.config.Plugins overriding the candidate selection.
func TestReloadKeepsCommandLineFIBPlugin(t *testing.T) {
	f := registerReloadWriterFixtures(t)
	cliChoice := []plugin.PluginConfig{
		{Name: f.noPlaneFIB, CompiledPlugin: f.noPlaneFIB, Internal: true},
	}

	t.Run("candidate selection keeps command-line FIB plugin", func(t *testing.T) {
		s := newReloadWriterTestServer(t, nil, cliChoice, true,
			plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
			plugin.PluginConfig{Name: f.noPlaneFIB, CompiledPlugin: f.noPlaneFIB, Internal: true},
		)
		runningTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
		}

		// Unrelated reload keeps the CLI-chosen FIB plugin and adds no writer.
		unrelatedTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
			"system":        map[string]any{"hostname": "r1"},
		}
		starts, err := s.selectReloadAutoLoadWithReasons(runningTree, unrelatedTree, []string{"system"}, nil, cliChoice, true)
		require.NoError(t, err)
		assert.Empty(t, starts)
		stops := s.collectReloadStopProcesses(runningTree, unrelatedTree, []string{"system"}, nil, cliChoice, true)
		assert.Empty(t, stops)

		// Removing the last producer stops the producer but keeps the CLI-chosen FIB plugin.
		noProducerTree := map[string]any{
			"interface": map[string]any{"backend": f.planeA},
		}
		starts, err = s.selectReloadAutoLoadWithReasons(runningTree, noProducerTree, nil, []string{f.producerARoot}, cliChoice, true)
		require.NoError(t, err)
		assert.Empty(t, starts)
		stops = s.collectReloadStopProcesses(runningTree, noProducerTree, nil, []string{f.producerARoot}, cliChoice, true)
		assert.True(t, stops[f.producerA])
		assert.False(t, stops[f.noPlaneFIB], "CLI-chosen FIB plugin must stay up even when the last producer leaves")
	})

	t.Run("boot s.config.Plugins gives no chosen answer when omitted from candidate selection", func(t *testing.T) {
		// Server booted with noPlaneFIB in s.config.Plugins, but candidate
		// selection is empty: judgement must NOT treat noPlaneFIB as chosen.
		s := newReloadWriterTestServer(t, cliChoice, cliChoice, true,
			plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
			plugin.PluginConfig{Name: f.noPlaneFIB, CompiledPlugin: f.noPlaneFIB, Internal: true},
		)
		runningTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
		}
		newTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
		}
		starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, nil, nil, nil, true)
		require.NoError(t, err)
		assert.Equal(t, []string{f.writerA}, autoLoadedNames(starts),
			"s.config.Plugins must not suppress the plane writer when candidate selection omits the FIB plugin")

		stops := s.collectReloadStopProcesses(runningTree, newTree, nil, nil, nil, true)
		assert.True(t, stops[f.noPlaneFIB],
			"running FIB plugin absent from candidate selection must be stopped despite s.config.Plugins")
	})
}

// TestReloadStopsWriterWhenLastProducerRemoved verifies that removing the last
// NeedsDataPlane producer stops the engine-added writer, and stopOrderGraph
// schedules the producer ahead of the writer.
//
// VALIDATES: spec-fib-dummy AC-16 -- last producer removal stops engine-added
// writer after the producer withdraws its routes.
// PREVENTS: orphaned engine-added writers or stopping the writer before the
// producer can withdraw its routes.
func TestReloadStopsWriterWhenLastProducerRemoved(t *testing.T) {
	f := registerReloadWriterFixtures(t)
	producerCfg := plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true}
	writerCfg := plugin.PluginConfig{Name: f.writerA, CompiledPlugin: f.writerA, Internal: true}
	s := newReloadWriterTestServer(t, nil, nil, true, producerCfg, writerCfg)

	runningTree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
	}
	newTree := map[string]any{
		"interface": map[string]any{"backend": f.planeA},
	}

	starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, nil, []string{f.producerARoot}, nil, true)
	require.NoError(t, err)
	assert.Empty(t, starts)

	stops := s.collectReloadStopProcesses(runningTree, newTree, nil, []string{f.producerARoot}, nil, true)
	assert.True(t, stops[f.producerA], "removed producer must be stopped")
	assert.True(t, stops[f.writerA], "engine-added writer must be stopped when no producer remains")

	tiers, err := registry.TopologicalGraphTiers(stopOrderGraph([]plugin.PluginConfig{producerCfg, writerCfg}))
	require.NoError(t, err)
	var stopOrder []string
	for _, tier := range slices.Backward(tiers) {
		stopOrder = append(stopOrder, tier...)
	}
	prodIdx := slices.Index(stopOrder, f.producerA)
	writerIdx := slices.Index(stopOrder, f.writerA)
	require.GreaterOrEqual(t, prodIdx, 0)
	require.GreaterOrEqual(t, writerIdx, 0)
	assert.Less(t, prodIdx, writerIdx, "producer must stop before the FIB writer (AC-16)")
}

// TestReloadKeepsEngineAddedWriterAcrossBackendChange verifies row 3 of the
// reload writer judgement: an already running engine-added writer is kept when
// interface backend changes with no FIB plugin chosen, while starting from no
// running writer picks the new backend's writer.
//
// VALIDATES: spec-fib-dummy Data Flow step 2 row 3 -- reload does not hot-swap
// data-plane writers across an interface backend change.
// PREVENTS: reload attempting an unsupported live writer swap across backends.
func TestReloadKeepsEngineAddedWriterAcrossBackendChange(t *testing.T) {
	f := registerReloadWriterFixtures(t)

	t.Run("running engine-added writer is kept when backend changes", func(t *testing.T) {
		s := newReloadWriterTestServer(t, nil, nil, true,
			plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
			plugin.PluginConfig{Name: f.writerA, CompiledPlugin: f.writerA, Internal: true},
		)
		runningTree := map[string]any{
			f.producerARoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeA},
		}
		newTree := map[string]any{
			f.producerARoot: map[string]any{},
			f.producerBRoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeB},
		}
		starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, []string{f.producerBRoot}, nil, nil, true)
		require.NoError(t, err)
		assert.Equal(t, []string{f.producerB}, autoLoadedNames(starts),
			"plane B writer must not be started while plane A's engine-added writer is running")

		stops := s.collectReloadStopProcesses(runningTree, newTree, []string{f.producerBRoot}, nil, nil, true)
		assert.Empty(t, stops, "running engine-added writer must stay up across a backend change")
	})

	t.Run("no running writer starts the second plane writer", func(t *testing.T) {
		s := newReloadWriterTestServer(t, nil, nil, true)
		runningTree := map[string]any{
			"interface": map[string]any{"backend": f.planeA},
		}
		newTree := map[string]any{
			f.producerBRoot: map[string]any{},
			"interface":     map[string]any{"backend": f.planeB},
		}
		starts, err := s.selectReloadAutoLoadWithReasons(runningTree, newTree, []string{f.producerBRoot}, nil, nil, true)
		require.NoError(t, err)
		got := autoLoadedNames(starts)
		assert.Contains(t, got, f.producerB)
		assert.Contains(t, got, f.writerB)
	})
}

// TestReloadStopsWriterOfChangedFIBEndpoint verifies AC-20 for compiled-in
// ProgramsFIB bus endpoints both in-daemon and with SeparateProcess=true.
//
// VALIDATES: spec-fib-dummy AC-20 -- removing or changing a ProgramsFIB bus
// endpoint stops the old process and starts the candidate's choice; keeping
// the endpoint unchanged starts and stops nothing.
// PREVENTS: reload leaving stale compiled-in FIB bus endpoints running or
// restarting unchanged ones.
func TestReloadStopsWriterOfChangedFIBEndpoint(t *testing.T) {
	f := registerReloadWriterFixtures(t)

	for _, separate := range []bool{false, true} {
		mode := "in-daemon"
		if separate {
			mode = "separate-process"
		}
		t.Run(mode, func(t *testing.T) {
			epA := plugin.PluginConfig{
				Name:            "k",
				CompiledPlugin:  f.writerA,
				Internal:        !separate,
				SeparateProcess: separate,
			}
			epB := plugin.PluginConfig{
				Name:            "k",
				CompiledPlugin:  f.noPlaneFIB,
				Internal:        !separate,
				SeparateProcess: separate,
			}
			s := newReloadWriterTestServer(t, nil, []plugin.PluginConfig{epA}, true,
				plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
				epA,
			)
			runningTree := map[string]any{
				f.producerARoot: map[string]any{},
				"interface":     map[string]any{"backend": f.planeA},
			}

			// Unchanged endpoint: neither started nor stopped.
			starts, err := s.selectReloadAutoLoadWithReasons(runningTree, runningTree, nil, nil, []plugin.PluginConfig{epA}, true)
			require.NoError(t, err)
			assert.Empty(t, starts)
			stops := s.collectReloadStopProcesses(runningTree, runningTree, nil, nil, []plugin.PluginConfig{epA}, true)
			assert.Empty(t, stops)

			// Endpoint removed and replaced by config-root noPlaneFIB.
			treeWithNoPlane := map[string]any{
				f.producerARoot: map[string]any{},
				"interface":     map[string]any{"backend": f.planeA},
				"rfib":          map[string]any{"noplane": map[string]any{}},
			}
			starts, err = s.selectReloadAutoLoadWithReasons(runningTree, treeWithNoPlane, []string{"rfib", f.noPlaneRoot}, nil, nil, true)
			require.NoError(t, err)
			assert.Equal(t, []string{f.noPlaneFIB}, autoLoadedNames(starts))
			stops = s.collectReloadStopProcesses(runningTree, treeWithNoPlane, []string{"rfib", f.noPlaneRoot}, nil, nil, true)
			assert.True(t, stops["k"], "removed FIB endpoint process must be stopped")

			// Endpoint internal target changed on the same key "k".
			starts, err = s.selectReloadAutoLoadWithReasons(runningTree, runningTree, nil, nil, []plugin.PluginConfig{epB}, true)
			require.NoError(t, err)
			require.Len(t, starts, 1)
			assert.Equal(t, "k", starts[0].Config.Name)
			assert.Equal(t, f.noPlaneFIB, starts[0].Config.CompiledIdentity())
			assert.Equal(t, separate, starts[0].Config.SeparateProcess)
			stops = s.collectReloadStopProcesses(runningTree, runningTree, nil, nil, []plugin.PluginConfig{epB}, true)
			assert.True(t, stops["k"], "replaced FIB endpoint process must be stopped")

			// Endpoint removed with no other FIB chosen: plane writer starts, "k" stops.
			starts, err = s.selectReloadAutoLoadWithReasons(runningTree, runningTree, nil, nil, nil, true)
			require.NoError(t, err)
			assert.Equal(t, []string{f.writerA}, autoLoadedNames(starts))
			stops = s.collectReloadStopProcesses(runningTree, runningTree, nil, nil, nil, true)
			assert.True(t, stops["k"])
		})
	}
}

// TestReloadStartsChosenFIBEndpoint verifies that adding a new compiled-in
// ProgramsFIB bus endpoint starts it under its endpoint name and stops the
// running engine-added writer.
//
// VALIDATES: spec-fib-dummy AC-20 -- new FIB bus endpoint replaces engine-added writer.
// PREVENTS: newly configured FIB bus endpoints failing to start on reload or
// running alongside the engine-added writer.
func TestReloadStartsChosenFIBEndpoint(t *testing.T) {
	f := registerReloadWriterFixtures(t)
	s := newReloadWriterTestServer(t, nil, nil, true,
		plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
		plugin.PluginConfig{Name: f.writerA, CompiledPlugin: f.writerA, Internal: true},
	)

	runningTree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
	}
	ep := plugin.PluginConfig{
		Name:           "custom-fib",
		CompiledPlugin: f.noPlaneFIB,
		Internal:       true,
	}

	starts, err := s.selectReloadAutoLoadWithReasons(runningTree, runningTree, nil, nil, []plugin.PluginConfig{ep}, true)
	require.NoError(t, err)
	require.Len(t, starts, 1)
	assert.Equal(t, "custom-fib", starts[0].Config.Name)
	assert.Equal(t, f.noPlaneFIB, starts[0].Config.CompiledIdentity())

	stops := s.collectReloadStopProcesses(runningTree, runningTree, nil, nil, []plugin.PluginConfig{ep}, true)
	assert.True(t, stops[f.writerA], "engine-added writer must be stopped when a FIB bus endpoint is added")
	assert.False(t, stops[f.producerA])
}

// TestReloadLeavesNonFIBEndpointAlone verifies that adding, removing, or
// changing a compiled-in bus endpoint whose registration does not declare
// ProgramsFIB adds nothing to the writer judgement's start or stop sets.
//
// VALIDATES: spec-fib-dummy AC-20, R-13 -- writer judgement touches only
// ProgramsFIB processes.
// PREVENTS: reload stopping or restarting unrelated compiled-in bus endpoints.
func TestReloadLeavesNonFIBEndpointAlone(t *testing.T) {
	f := registerReloadWriterFixtures(t)
	obsA := plugin.PluginConfig{Name: "obs", CompiledPlugin: f.observerA, Internal: true}
	obsB := plugin.PluginConfig{Name: "obs", CompiledPlugin: f.observerB, Internal: true}
	s := newReloadWriterTestServer(t, nil, []plugin.PluginConfig{obsA}, true,
		plugin.PluginConfig{Name: f.producerA, CompiledPlugin: f.producerA, Internal: true},
		plugin.PluginConfig{Name: f.writerA, CompiledPlugin: f.writerA, Internal: true},
		obsA,
	)

	tree := map[string]any{
		f.producerARoot: map[string]any{},
		"interface":     map[string]any{"backend": f.planeA},
	}

	for name, cand := range map[string][]plugin.PluginConfig{
		"non-FIB endpoint changed": {obsB},
		"non-FIB endpoint removed": nil,
		"non-FIB endpoint added":   {obsA, {Name: "obs2", CompiledPlugin: f.observerB, Internal: true}},
	} {
		t.Run(name, func(t *testing.T) {
			starts, err := s.selectReloadAutoLoadWithReasons(tree, tree, nil, nil, cand, true)
			require.NoError(t, err)
			assert.Empty(t, starts)
			stops := s.collectReloadStopProcesses(tree, tree, nil, nil, cand, true)
			assert.Empty(t, stops)
		})
	}
}

// TestStopCollectedProcessesEmitsWriterStoppedForFIB verifies that
// stopCollectedProcesses emits mplsfib.WriterStopped once after stopping a
// ProgramsFIB process (and only after that process exits), and emits nothing
// when stopping only non-ProgramsFIB processes or when stopping fails.
//
// VALIDATES: spec-fib-dummy AC-24, R-15 -- mplsfib.WriterStopped fires once
// after a ProgramsFIB process exits cleanly, never on non-FIB stops or errors.
// PREVENTS: missed label replays after a displaced FIB writer stops, or
// premature/spurious replays while the old writer is still subscribed.
func TestStopCollectedProcessesEmitsWriterStoppedForFIB(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	var fibExited atomic.Bool
	const (
		nonFIBName    = "stop-nonfib-fixture"
		fibOKName     = "stop-fib-ok-fixture"
		fibRefuseName = "stop-fib-refuse-fixture"
	)

	require.NoError(t, registry.Register(registry.Registration{
		Name:        nonFIBName,
		Description: "non-FIB stop fixture",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(nonFIBName, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fibOKName,
		Description: "FIB stop fixture",
		ProgramsFIB: true,
		RunEngine: func(conn net.Conn) int {
			defer fibExited.Store(true)
			p := sdk.NewWithConn(fibOKName, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fibRefuseName,
		Description: "FIB stop refusal fixture",
		ProgramsFIB: true,
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(fibRefuseName, conn)
			p.OnBye(func(_ string) error {
				return errors.New("refuse removal")
			})
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, _ = newLifecycleStartupServer(t)
	s.SetShutdownFunc(func() {})
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: nonFIBName, CompiledPlugin: nonFIBName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: fibOKName, CompiledPlugin: fibOKName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: fibRefuseName, CompiledPlugin: fibRefuseName, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	var emits atomic.Int32
	var exitedAtEmit atomic.Bool
	unsub := mplsfib.WriterStopped.Subscribe(s, func() {
		emits.Add(1)
		exitedAtEmit.Store(fibExited.Load())
	})
	t.Cleanup(unsub)

	// Stopping only a non-FIB process emits nothing.
	require.NoError(t, s.stopCollectedProcesses(map[string]bool{nonFIBName: true}))
	assert.Equal(t, int32(0), emits.Load(), "stopping non-FIB process must not emit WriterStopped")

	// Stopping a ProgramsFIB process emits WriterStopped once, after exit.
	require.NoError(t, s.stopCollectedProcesses(map[string]bool{fibOKName: true}))
	assert.Equal(t, int32(1), emits.Load(), "stopping ProgramsFIB process must emit WriterStopped once")
	assert.True(t, exitedAtEmit.Load(), "WriterStopped must fire only after the stopped FIB process exits")

	// Stopping a ProgramsFIB process that refuses Bye returns an error and emits nothing.
	err := s.stopCollectedProcesses(map[string]bool{fibRefuseName: true})
	require.Error(t, err)
	assert.Equal(t, int32(1), emits.Load(), "failed FIB stop must not emit WriterStopped")
}
