package doctor

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestReloadRecordRows proves the two reload checks answer one error per
// unrestored removal and per failed capability restore, each with its fix,
// and say so when no server is published. Method: the row builders over a
// records value; the checks with no published server.
//
// VALIDATES: AC-28, AC-29 (spec-doctor-coverage-12-plugin-infra).
func TestReloadRecordRows(t *testing.T) {
	records := server.ReloadDoctorRecords{
		RemovalsUnrecovered: []server.RemovalUnrecovered{{Plugin: "p1", Err: "restart refused"}},
		CapabilitiesNotRestored: []server.CapabilityRestoreFailure{{
			Err: "decode stored capabilities", Time: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC),
		}},
	}
	removal := removalUnrecoveredRows(&records)
	if len(removal) != 1 || removal[0].Code != diagnostic.CodeDoctorPluginRemovalUnrecovered ||
		removal[0].Severity != diagnostic.SeverityError || removal[0].Fix != reloadRetryCommand ||
		!strings.Contains(removal[0].Message, "p1") || !strings.Contains(removal[0].Message, "restart refused") {
		t.Fatalf("removal rows = %+v; want one error naming p1 and its cause, fix %q", removal, reloadRetryCommand)
	}
	restore := capabilitiesNotRestoredRows(&records)
	if len(restore) != 1 || restore[0].Code != diagnostic.CodeDoctorPluginCapabilitiesNotRestored ||
		restore[0].Severity != diagnostic.SeverityError || restore[0].Fix != reloadRetryCommand ||
		!strings.Contains(restore[0].Message, "2026-11-11T09:00:00Z") {
		t.Fatalf("restore rows = %+v; want one error naming the time, fix %q", restore, reloadRetryCommand)
	}
	for _, check := range []func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic{
		checkRemovalUnrecovered, checkCapabilitiesNotRestored,
	} {
		rows := check(diagnostic.DoctorCheckContext{})
		if len(rows) != 1 || rows[0].Severity != diagnostic.SeverityWarning {
			t.Fatalf("check with no published server = %+v; want one warning saying it cannot tell", rows)
		}
	}
}

// TestDataPlaneWriterRowMatchesRefusal proves `ze doctor` reports a selected
// NeedsDataPlane producer with no writer, in the words of the refusal boot
// and reload give (registry.DataPlaneWriter), and reports nothing when a
// writer programs the data plane or an explicit FIB plugin is chosen. Method:
// fixture registrations of a producer and a writer; the row builder over
// selections.
//
// VALIDATES: AC-34, A-3.
// PREVENTS: a config `ze doctor` passes that the daemon refuses to start.
func TestDataPlaneWriterRowMatchesRefusal(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	run := func(_ net.Conn) int { return 0 }
	cli := func([]string) int { return 0 }
	for _, reg := range []registry.Registration{
		{Name: "producer-fixture", Description: "test", RunEngine: run, CLIHandler: cli, NeedsDataPlane: true},
		{Name: "writer-fixture", Description: "test", RunEngine: run, CLIHandler: cli, ProgramsFIB: true, DataPlane: "fixture-plane"},
		{Name: "chosen-fib-fixture", Description: "test", RunEngine: run, CLIHandler: cli, ProgramsFIB: true},
	} {
		if err := registry.Register(reg); err != nil {
			t.Fatalf("register %s: %v", reg.Name, err)
		}
	}

	_, refusal := registry.DataPlaneWriter("producer-fixture", "absent-plane")
	if refusal == nil {
		t.Fatal("DataPlaneWriter answered a writer for a data plane nobody programs")
	}
	rows := dataPlaneWriterRows([]string{"producer-fixture"}, nil, "absent-plane")
	if len(rows) != 1 {
		t.Fatalf("rows = %+v; want one", rows)
	}
	var operr *opererr.Error
	if !errors.As(refusal, &operr) {
		t.Fatalf("refusal %v is no operator error", refusal)
	}
	if rows[0].Code != diagnostic.CodeDoctorPluginNoDataPlaneWriter || rows[0].Severity != diagnostic.SeverityError ||
		!strings.Contains(rows[0].Message, refusal.Error()) || !strings.Contains(rows[0].Message, "producer-fixture") ||
		rows[0].Fix != operr.Fix() {
		t.Fatalf("row = %+v; want an error carrying the refusal %q and its fix", rows[0], refusal)
	}

	for name, selection := range map[string][]string{
		"writer programs the data plane": {"producer-fixture"},
		"no producer selected":           {"writer-fixture"},
	} {
		if rows := dataPlaneWriterRows(selection, nil, "fixture-plane"); len(rows) != 0 {
			t.Errorf("%s: rows = %+v; want none", name, rows)
		}
	}
	if rows := dataPlaneWriterRows([]string{"producer-fixture", "chosen-fib-fixture"}, nil, "absent-plane"); len(rows) != 0 {
		t.Errorf("explicit fib block: rows = %+v; want none, its plugin is the writer", rows)
	}
	if rows := dataPlaneWriterRows([]string{"producer-fixture"}, []string{"chosen-fib-fixture"}, "absent-plane"); len(rows) != 0 {
		t.Errorf("configured fib plugin: rows = %+v; want none", rows)
	}
}
