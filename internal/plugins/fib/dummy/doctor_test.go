// VALIDATES: fib-dummy-holds-routes doctor check registration and AC-8 behavior
// for configs with fib dummy plus NeedsDataPlane producers, fib dummy alone,
// and configs without fib dummy.
// PREVENTS: fib-dummy doctor check emitting a row without fib dummy, missing
// NeedsDataPlane producers from either config roots or ctx.Plugins, or
// reporting the wrong severity or fix.
package fibdummy

import (
	"net"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorHoldsRoutes proves AC-8: with fib dummy and one or more
// NeedsDataPlane plugins selected it emits one WARNING naming each producer;
// with fib dummy alone it emits one INFO; without fib dummy (or with an
// inactive fib dummy container or nil tree) it emits nothing.
//
// VALIDATES: checkFIBDummyHoldsRoutes registration, severity, message, and fix.
// PREVENTS: silent fib-dummy misreporting or false rows on non-dummy configs.
func TestDoctorHoldsRoutes(t *testing.T) {
	// Verify check registration metadata in DoctorPhasePostConfig.
	var found *diagnostic.DoctorCheck
	for _, c := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if c.Name == doctorHoldsRoutesCheckName {
			cc := c
			found = &cc
			break
		}
	}
	require.NotNil(t, found, "doctor check %q must be registered in DoctorPhasePostConfig", doctorHoldsRoutesCheckName)
	assert.Equal(t, pluginName, found.Component)
	assert.Equal(t, []string{"config-tree"}, found.Dependencies)
	assert.Equal(t, []string{diagnostic.DoctorPlatformAny}, found.Platforms)
	assert.Equal(t, []string{codeDoctorHoldsRoutes}, found.Codes)
	assert.NotNil(t, diagnostic.Lookup(codeDoctorHoldsRoutes), "diagnostic code metadata must be registered")

	// Register test NeedsDataPlane producers and a non-producer plugin in an
	// isolated registry snapshot so the test does not depend on other plugin packages.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	registerTestPlugin := func(name string, roots []string, needsDataPlane bool) {
		t.Helper()
		err := registry.Register(registry.Registration{
			Name:           name,
			Description:    "test plugin " + name,
			ConfigRoots:    roots,
			NeedsDataPlane: needsDataPlane,
			RunEngine:      func(net.Conn) int { return 0 },
			CLIHandler:     func([]string) int { return 0 },
		})
		require.NoError(t, err)
	}
	registerTestPlugin("static", []string{"static"}, true)
	registerTestPlugin("ldp", []string{"ldp"}, true)
	registerTestPlugin("helper", []string{"helper"}, false)

	makeTree := func(withDummy, dummyInactive, withStatic, withHelper bool) *config.Tree {
		tree := config.NewTree()
		if withDummy {
			fib := tree.GetOrCreateContainer("fib")
			dummy := fib.GetOrCreateContainer("dummy")
			if dummyInactive {
				dummy.SetInactive(true)
			}
		}
		if withStatic {
			tree.GetOrCreateContainer("static")
		}
		if withHelper {
			tree.GetOrCreateContainer("helper")
		}
		return tree
	}

	t.Run("nil or non-tree context returns no rows", func(t *testing.T) {
		assert.Empty(t, checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{}))
		assert.Empty(t, checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{Tree: "not-a-tree"}))
	})

	t.Run("config without fib dummy returns no rows", func(t *testing.T) {
		tree := makeTree(false, false, true, false)
		assert.Empty(t, checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{Tree: tree}))
	})

	t.Run("inactive fib dummy returns no rows", func(t *testing.T) {
		tree := makeTree(true, true, true, false)
		assert.Empty(t, checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{Tree: tree}))
	})

	t.Run("fib dummy alone emits info", func(t *testing.T) {
		tree := makeTree(true, false, false, true)
		diags := checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{Tree: tree})
		require.Len(t, diags, 1)
		assert.Equal(t, codeDoctorHoldsRoutes, diags[0].Code)
		assert.Equal(t, diagnostic.SeverityInfo, diags[0].Severity)
		assert.Equal(t,
			"fib dummy is configured: ze writes no route the system RIB selects to any forwarding table "+
				"(named-table static routes, policy routing, IPsec tunnel-source guards, and interface addresses are outside the system RIB and untouched)",
			diags[0].Message)
		assert.Equal(t, "delete fib dummy", diags[0].Fix)
	})

	t.Run("fib dummy with static config root emits warning naming static", func(t *testing.T) {
		tree := makeTree(true, false, true, false)
		diags := checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{Tree: tree})
		require.Len(t, diags, 1)
		assert.Equal(t, codeDoctorHoldsRoutes, diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.Equal(t,
			"fib dummy is configured, and this config loads static, whose routes reach forwarding only through a data-plane writer: "+
				"fib dummy holds those routes and writes them to no forwarding table",
			diags[0].Message)
		assert.Equal(t, "delete fib dummy", diags[0].Fix)
	})

	t.Run("fib dummy with config-root and ctx.Plugins producers deduplicates and sorts", func(t *testing.T) {
		tree := makeTree(true, false, true, false)
		plugins := []plugin.PluginConfig{
			{Name: "static-ep", CompiledPlugin: "static", Internal: true},
			{Name: "ldp-ep", CompiledPlugin: "ldp", Internal: true},
			{Name: "ext-fake", Run: "/usr/local/bin/unrelated"},
		}
		diags := checkFIBDummyHoldsRoutes(diagnostic.DoctorCheckContext{Tree: tree, Plugins: plugins})
		require.Len(t, diags, 1)
		assert.Equal(t, codeDoctorHoldsRoutes, diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		wantOrder := []string{"ldp", "static"}
		slices.Sort(wantOrder)
		assert.Contains(t, diags[0].Message, "this config loads ldp, static, whose routes reach forwarding only through a data-plane writer")
		assert.Equal(t, "delete fib dummy", diags[0].Fix)
	})
}
