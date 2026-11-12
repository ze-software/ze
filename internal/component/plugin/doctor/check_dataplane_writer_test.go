// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: check_dataplane_writer.go -- checkDataPlaneWriter, the check under test
// Related: check_reload_dataplane_test.go -- the row builder over selections

package doctor

import (
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Fixture names for TestDataPlaneWriterCheckFromTree. The roots are owned by
// no real plugin, so the selection the check makes holds only the fixtures.
const (
	dataPlaneProducerFixture = "dataplane-producer-fixture"
	dataPlaneProducerRoot    = "dataplane-producer-fixture-root"
	dataPlaneFIBFixture      = "dataplane-fib-fixture"
	dataPlaneFIBName         = "fixture-writer"
	dataPlaneUnprogrammed    = "unprogrammed-fixture-plane"
)

// dataPlaneWriterTree builds a config tree holding the producer fixture's
// root and `interface { backend <unprogrammed> }`. fib adds a `fib { }`
// block; fibChild, when non-empty, adds `fib { <fibChild> { } }` inside it.
func dataPlaneWriterTree(fib bool, fibChild string) *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer(dataPlaneProducerRoot)
	tree.GetOrCreateContainer("interface").Set("backend", dataPlaneUnprogrammed)
	if !fib {
		return tree
	}
	fibTree := tree.GetOrCreateContainer("fib")
	if fibChild != "" {
		fibTree.GetOrCreateContainer(fibChild)
	}
	return tree
}

// TestDataPlaneWriterCheckFromTree proves the registered check, given a parsed
// config, reports a NeedsDataPlane producer whose data plane no plugin
// programs, that an empty `fib { }` block selects no writer and so still
// reports, and that a `fib { <name> { } }` block naming a ProgramsFIB plugin
// is the writer and reports nothing. Method: fixture registrations under a
// registry snapshot (a producer owning one root, a FIB plugin owning
// `fib/<name>`), then checkDataPlaneWriter over three trees, the path
// `ze doctor` runs, from the tree through autoload selection and the
// interface backend to registry.DataPlaneWriter.
//
// VALIDATES: AC-34 (spec-doctor-coverage-12-plugin-infra), the cases the
// deleted static-fib-writer tests held at the generic check.
// PREVENTS: an empty `fib { }` block read as a chosen writer, which hides a
// config the daemon refuses to start.
func TestDataPlaneWriterCheckFromTree(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	run := func(_ net.Conn) int { return 0 }
	cli := func([]string) int { return 0 }
	for _, reg := range []registry.Registration{
		{
			Name: dataPlaneProducerFixture, Description: "test", RunEngine: run, CLIHandler: cli,
			NeedsDataPlane: true, ConfigRoots: []string{dataPlaneProducerRoot},
		},
		{
			Name: dataPlaneFIBFixture, Description: "test", RunEngine: run, CLIHandler: cli,
			ProgramsFIB: true, ConfigRoots: []string{config.AppendPath("fib", dataPlaneFIBName)},
		},
	} {
		if err := registry.Register(reg); err != nil {
			t.Fatalf("register %s: %v", reg.Name, err)
		}
	}
	if writer, ok := registry.PluginForDataPlane(dataPlaneUnprogrammed); ok {
		t.Fatalf("%s programs %q, so the fixture data plane is not unprogrammed", writer, dataPlaneUnprogrammed)
	}
	if writer, gated := registry.GatedDataPlaneWriter(dataPlaneUnprogrammed); gated {
		t.Fatalf("gated writer %s owns %q, so the refusal would be the compiled-out one", writer, dataPlaneUnprogrammed)
	}
	if owners := registry.PluginsForConfigPaths([]string{"fib"}); slices.ContainsFunc(owners, registry.ProgramsFIB) {
		t.Fatalf("plugins %v own the bare fib root, so an empty fib block selects a writer in this binary", owners)
	}

	for name, tree := range map[string]*config.Tree{
		"producer and unprogrammed backend": dataPlaneWriterTree(false, ""),
		"plus an empty fib block":           dataPlaneWriterTree(true, ""),
	} {
		rows := checkDataPlaneWriter(diagnostic.DoctorCheckContext{Tree: tree})
		if len(rows) != 1 {
			t.Fatalf("%s: rows = %+v; want exactly one", name, rows)
		}
		if rows[0].Code != diagnostic.CodeDoctorPluginNoDataPlaneWriter {
			t.Errorf("%s: code = %q; want %q", name, rows[0].Code, diagnostic.CodeDoctorPluginNoDataPlaneWriter)
		}
		if rows[0].Severity != diagnostic.SeverityError {
			t.Errorf("%s: severity = %q; want %q", name, rows[0].Severity, diagnostic.SeverityError)
		}
		if !strings.Contains(rows[0].Message, dataPlaneUnprogrammed) {
			t.Errorf("%s: message %q does not name the data plane %q", name, rows[0].Message, dataPlaneUnprogrammed)
		}
	}

	rows := checkDataPlaneWriter(diagnostic.DoctorCheckContext{Tree: dataPlaneWriterTree(true, dataPlaneFIBName)})
	if len(rows) != 0 {
		t.Errorf("fib block naming %s: rows = %+v; want none, its plugin is the writer", dataPlaneFIBFixture, rows)
	}
}
