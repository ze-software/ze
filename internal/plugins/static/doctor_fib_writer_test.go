// VALIDATES: spec-connected-static-reach-the-locrib -- after the owner's
// 2026-09-06 decision the FIB plugin is AUTO-LOADED for a main-table static
// route, so the static-fib-writer check reports only the state auto-loading
// cannot repair: no plugin programs the data plane the config selects.
// PREVENTS: the check refusing a config Ze now starts, and the opposite failure
// of staying silent about static routes that can reach no data plane at all.

package static

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// treeWithStaticRoute builds a config tree with one static route in the named
// table. An empty table name writes the route under `default`, the main table.
// backend, when non-empty, adds the `interface { backend <name> }` stanza; fib,
// when non-empty, adds a `fib { <name> { } }` block.
func treeWithStaticRoute(table, backend, fib string) *config.Tree {
	if table == "" {
		table = "default"
	}
	tree := config.NewTree()
	static := tree.GetOrCreateContainer("static")

	route := config.NewTree()
	next := route.GetOrCreateContainer("next")
	hop := config.NewTree()
	hop.Set("address", "192.0.2.1")
	next.AddListEntry("hop", "192.0.2.1", hop)

	tableTree := config.NewTree()
	tableTree.AddListEntry("route", "10.0.0.0/8", route)
	static.AddListEntry("table", table, tableTree)

	if backend != "" {
		tree.GetOrCreateContainer("interface").Set("backend", backend)
	}
	if fib != "" {
		tree.GetOrCreateContainer("fib").GetOrCreateContainer(fib)
	}
	return tree
}

// registerWriterFor makes some plugin the writer for a data plane, for the life
// of the test. The static test binary links no FIB plugin, and skipping instead
// would leave the case this feature exists for unproven, so the test supplies
// the declaration fib-kernel makes in a daemon.
func registerWriterFor(t *testing.T, dataPlane string) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	err := registry.Register(registry.Registration{
		Name:        "fib-test-writer",
		Description: "stands in for the FIB plugin that programs " + dataPlane,
		ProgramsFIB: true,
		DataPlane:   dataPlane,
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register the %s writer: %v", dataPlane, err)
	}
}

// TestFIBWriterCheckIsSilentWhenTheDataPlaneHasAWriter is the owner's decision
// at the doctor surface: a main-table static route with no `fib { }` block is a
// config Ze runs, so the check says nothing about it.
func TestFIBWriterCheckIsSilentWhenTheDataPlaneHasAWriter(t *testing.T) {
	registerWriterFor(t, "netlink")

	diags := checkFIBWriter(registry.DoctorCheckContext{
		Tree: treeWithStaticRoute("", "netlink", ""),
	})
	if len(diags) != 0 {
		t.Fatalf("check reported %d diagnostics for a config the engine now serves: %+v", len(diags), diags)
	}
}

// TestFIBWriterCheckIsSilentWithNoInterfaceBlock proves the common config: no
// `interface { }` stanza and no `fib { }` block, which is what every static
// functional test writes.
func TestFIBWriterCheckIsSilentWithNoInterfaceBlock(t *testing.T) {
	registerWriterFor(t, "netlink")

	diags := checkFIBWriter(registry.DoctorCheckContext{
		Tree: treeWithStaticRoute("", "", ""),
	})
	if len(diags) != 0 {
		t.Fatalf("check reported %d diagnostics: %+v", len(diags), diags)
	}
}

// TestFIBWriterCheckReportsADataPlaneNoPluginProgram proves the state that is
// left to report. The routes reach arbitration and stop there, and the message
// names the data plane so the operator knows which one has no writer.
func TestFIBWriterCheckReportsADataPlaneNoPluginProgram(t *testing.T) {
	const dataPlane = "p4-runtime"
	if _, ok := registry.PluginForDataPlane(dataPlane); ok {
		t.Fatalf("a plugin programs %q, so this config is not the unwritable one", dataPlane)
	}

	diags := checkFIBWriter(registry.DoctorCheckContext{
		Tree: treeWithStaticRoute("", dataPlane, ""),
	})
	if len(diags) != 1 {
		t.Fatalf("check reported %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != doctorCodeNoFIBWriter {
		t.Fatalf("code = %q, want %q", diags[0].Code, doctorCodeNoFIBWriter)
	}
	if diags[0].Severity != "error" {
		t.Fatalf("severity = %q, want error", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, dataPlane) {
		t.Fatalf("message does not name the data plane: %q", diags[0].Message)
	}
}

// TestFIBWriterCheckIsSilentForANamedTableOnlyConfig proves the boundary the
// static half keeps: a named-table route is programmed directly, so it needs no
// FIB plugin whatever the data plane is.
func TestFIBWriterCheckIsSilentForANamedTableOnlyConfig(t *testing.T) {
	diags := checkFIBWriter(registry.DoctorCheckContext{
		Tree: treeWithStaticRoute("blue", "p4-runtime", ""),
	})
	if len(diags) != 0 {
		t.Fatalf("check reported %d diagnostics for a named-table route: %+v", len(diags), diags)
	}
}

// TestFIBWriterCheckDefersToAnExplicitFIBBackend proves the operator's own
// choice ends the question. `fib { p4 { } }` loads fib-p4 whatever this check
// thinks of the data plane, so reporting would be reporting a config that has
// its writer.
func TestFIBWriterCheckDefersToAnExplicitFIBBackend(t *testing.T) {
	diags := checkFIBWriter(registry.DoctorCheckContext{
		Tree: treeWithStaticRoute("", "p4-runtime", "p4"),
	})
	if len(diags) != 0 {
		t.Fatalf("check reported %d diagnostics for an explicit fib backend: %+v", len(diags), diags)
	}
}

// TestFIBWriterCheckReportsAnEmptyFIBBlock proves an empty `fib { }` selects no
// backend. It loads no plugin, so a config that carries one and an unwritable
// data plane is exactly as unrouted as one with no block at all.
func TestFIBWriterCheckReportsAnEmptyFIBBlock(t *testing.T) {
	tree := treeWithStaticRoute("", "p4-runtime", "")
	tree.GetOrCreateContainer("fib")

	diags := checkFIBWriter(registry.DoctorCheckContext{Tree: tree})
	if len(diags) != 1 {
		t.Fatalf("check reported %d diagnostics for an empty fib block, want 1: %+v", len(diags), diags)
	}
}
