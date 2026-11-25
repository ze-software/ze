// Design: docs/architecture/doctor-and-health-checks.md -- dormant bus reception check tests
// Related: check_bus_dormant.go -- dormantReceptionRows and dormantReceptionRow

package doctor

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testDormantCollectorPath = "/bus/endpoint/collector"
	testDormantAlerterPath   = "/bus/endpoint/alerter"
	testDormantAlertMsg      = "acme:alert"
)

// TestDormantReceptionCheckReportsMissingSender compiles a bus where a
// collector registers reception of a custom message with no sender configured,
// then recompiles with a sender present, and feeds each compiled result to
// dormantReceptionRows.
//
// VALIDATES: spec-explicit-plugin-and-attach-config S21 / AC-8, AC-25 --
// show doctor reports a dormant reception naming the receiving endpoint path,
// the message, "dormant", and the missing-sender reason, and emits zero
// diagnostics once a sender is configured.
// PREVENTS: dormant receptions going unreported in show doctor or leaving a
// stale diagnostic after a sender is added.
func TestDormantReceptionCheckReportsMissingSender(t *testing.T) {
	t.Run("dormant reception present", func(t *testing.T) {
		compiled, err := server.CompileBus(server.NewTagInterner(), []server.BusParticipantInput{{
			Path:         testDormantCollectorPath,
			EndpointName: "collector",
			Receives:     []rpc.MessageReceiveDecl{{Message: testDormantAlertMsg}},
		}})
		if err != nil {
			t.Fatalf("CompileBus without sender: %v", err)
		}
		filter := compiled.Filter(testDormantCollectorPath)
		if filter == nil {
			t.Fatalf("compiled bus missing filter for %s", testDormantCollectorPath)
		}
		snapshot := &server.DoctorSnapshot{
			DormantReceptions: slices.Clone(filter.DormantReceptions),
		}
		row := oneRow(t, dormantReceptionRows(snapshot), codeDoctorBusDormantReception,
			diagnostic.SeverityInfo, "dormant", testDormantCollectorPath, testDormantAlertMsg, "no sender")
		if !strings.Contains(row.Fix, testDormantAlertMsg) {
			t.Fatalf("fix %q does not name message %s", row.Fix, testDormantAlertMsg)
		}
	})

	t.Run("sender present emits zero diagnostics", func(t *testing.T) {
		compiled, err := server.CompileBus(server.NewTagInterner(), []server.BusParticipantInput{
			{
				Path:         testDormantAlerterPath,
				EndpointName: "alerter",
				Sends:        []rpc.MessageSendDecl{{Message: testDormantAlertMsg}},
			},
			{
				Path:         testDormantCollectorPath,
				EndpointName: "collector",
				Receives:     []rpc.MessageReceiveDecl{{Message: testDormantAlertMsg}},
			},
		})
		if err != nil {
			t.Fatalf("CompileBus with sender: %v", err)
		}
		filter := compiled.Filter(testDormantCollectorPath)
		if filter == nil {
			t.Fatalf("compiled bus missing filter for %s", testDormantCollectorPath)
		}
		snapshot := &server.DoctorSnapshot{
			DormantReceptions: slices.Clone(filter.DormantReceptions),
		}
		rows := dormantReceptionRows(snapshot)
		if len(rows) != 0 {
			t.Fatalf("want zero diagnostics when sender is present, got %+v", rows)
		}
	})
}

// TestDormantReceptionCheckRegistered verifies that the dormant reception check
// is registered in the runtime phase with its declared code and fails closed
// when no plugin server is published.
//
// VALIDATES: plugin-bus-dormant-receptions is registered at DoctorPhaseRuntime
// under codeDoctorBusDormantReception, and running it with no server published
// answers a warning under its own code rather than reporting zero dormant
// receptions.
// PREVENTS: a registered check missing its code declaration or silently
// answering healthy when live server state is unavailable.
func TestDormantReceptionCheckRegistered(t *testing.T) {
	var found *diagnostic.DoctorCheck
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != pluginBusDormantCheckName {
			continue
		}
		c := check
		found = &c
		break
	}
	if found == nil {
		t.Fatalf("runtime doctor check %q is not registered", pluginBusDormantCheckName)
	}
	if !slices.Contains(found.Codes, codeDoctorBusDormantReception) {
		t.Fatalf("check %q codes %v lack %q", found.Name, found.Codes, codeDoctorBusDormantReception)
	}
	rows := found.Check(diagnostic.DoctorCheckContext{})
	oneRow(t, rows, codeDoctorBusDormantReception, diagnostic.SeverityWarning, "cannot")
}
