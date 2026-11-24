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
	_ "github.com/ze-software/ze/internal/component/iface/yang"
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
			Name: "dataplane-selector-fixture", Description: "test", RunEngine: run, CLIHandler: cli,
			DataPlaneSelector: &registry.DataPlaneSelector{
				LeafPath:           "/interface/backend",
				Roots:              []string{"interface"},
				SelectsDataPlane:   true,
				RegisteredBackends: func() []string { return []string{"kernel"} },
			},
		},
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
		if rows[0].Code != codeDoctorPluginNoDataPlaneWriter {
			t.Errorf("%s: code = %q; want %q", name, rows[0].Code, codeDoctorPluginNoDataPlaneWriter)
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

// TestDoctorWriterRowUsesTheJudgement proves dataPlaneWriterRows agrees with
// registry.JudgeDataPlaneWriter across producer/no-producer, chosen FIB plugin
// in selected or configured lists, programmed plane, and unprogrammed plane.
//
// VALIDATES: spec-fib-dummy Phase 5 -- doctor's writer row calls the shared
// registry.JudgeDataPlaneWriter judgement so doctor and boot/reload agree.
// PREVENTS: doctor drifting from the engine's writer judgement when a FIB
// plugin is chosen via config root, bus endpoint, or --plugin.
func TestDoctorWriterRowUsesTheJudgement(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	run := func(_ net.Conn) int { return 0 }
	cli := func([]string) int { return 0 }

	const (
		producer     = "doc-judge-producer"
		observer     = "doc-judge-observer"
		planeA       = "doc-judge-plane-a"
		writerA      = "doc-judge-writer-a"
		chosenFIB    = "doc-judge-chosen-fib"
		unprogrammed = "doc-judge-unprogrammed"
	)

	for _, reg := range []registry.Registration{
		{Name: producer, Description: "test", RunEngine: run, CLIHandler: cli, NeedsDataPlane: true},
		{Name: observer, Description: "test", RunEngine: run, CLIHandler: cli},
		{Name: writerA, Description: "test", RunEngine: run, CLIHandler: cli, ProgramsFIB: true, DataPlane: planeA},
		{Name: chosenFIB, Description: "test", RunEngine: run, CLIHandler: cli, ProgramsFIB: true, NoDataPlaneChoice: true},
	} {
		if err := registry.Register(reg); err != nil {
			t.Fatalf("register %s: %v", reg.Name, err)
		}
	}

	cases := []struct {
		name       string
		selected   []string
		configured []string
		dataPlane  string
		wantRows   int
		wantCode   string
	}{
		{
			name:      "producer on unprogrammed plane reports error",
			selected:  []string{producer},
			dataPlane: unprogrammed,
			wantRows:  1,
			wantCode:  codeDoctorPluginNoDataPlaneWriter,
		},
		{
			name:      "producer on programmed plane reports nothing",
			selected:  []string{producer},
			dataPlane: planeA,
			wantRows:  0,
		},
		{
			name:      "chosen FIB in selected suppresses error",
			selected:  []string{producer, chosenFIB},
			dataPlane: unprogrammed,
			wantRows:  0,
		},
		{
			name:       "chosen FIB in configured suppresses error",
			selected:   []string{producer},
			configured: []string{chosenFIB},
			dataPlane:  unprogrammed,
			wantRows:   0,
		},
		{
			name:      "no producer on unprogrammed plane reports nothing",
			selected:  []string{observer},
			dataPlane: unprogrammed,
			wantRows:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := dataPlaneWriterRows(tc.selected, tc.configured, tc.dataPlane)
			if len(rows) != tc.wantRows {
				t.Fatalf("rows = %+v; want %d", rows, tc.wantRows)
			}
			if tc.wantRows > 0 && rows[0].Code != tc.wantCode {
				t.Errorf("code = %q; want %q", rows[0].Code, tc.wantCode)
			}
			chosen := make([]string, 0, len(tc.selected)+len(tc.configured))
			chosen = append(chosen, tc.selected...)
			chosen = append(chosen, tc.configured...)
			_, judgeErr := registry.JudgeDataPlaneWriter(tc.selected, chosen, tc.dataPlane)
			if (judgeErr != nil) != (len(rows) > 0) {
				t.Errorf("doctor rows (%d) disagree with JudgeDataPlaneWriter err (%v)", len(rows), judgeErr)
			}
		})
	}
}
