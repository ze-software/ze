// Design: docs/functional-tests.md -- a compiled driver's decisions are unit-tested, its .ci proves the path
// Related: misc_fixture_managed_fleet_leave.go -- the session count and the refusal check under test

package fixture

import (
	"strings"
	"testing"
)

// TestFleetLeaveCountSessions proves which lines of a hub's output the leave
// and rejoin drivers count as a session admitted and as a session ended. The
// method is a hub log with one client's two sessions, another client's
// session, and a refusal of the first client: only the admissions and the
// ends of the named client may count, or "the hub saw the close" could pass
// on another client's line.
func TestFleetLeaveCountSessions(t *testing.T) {
	output := strings.Join([]string{
		`level=INFO msg="managed client connected" subsystem=hub.managed-server name=edge-01`,
		`level=INFO msg="managed client connected" subsystem=hub.managed-server name=edge-02`,
		`level=WARN msg="managed client rejected config" subsystem=hub.managed-server name=edge-01 error="holding"`,
		`level=INFO msg="managed client disconnected" subsystem=hub.managed-server name=edge-01`,
		`level=INFO msg="managed client connected" subsystem=hub.managed-server name=edge-01`,
		``,
	}, "\n")

	got := fleetLeaveCountSessions(output, "edge-01")
	want := fleetLeaveSessions{opened: 2, closed: 1}
	if got != want {
		t.Fatalf("sessions of edge-01 = %+v, want %+v", got, want)
	}
	other := fleetLeaveCountSessions(output, "edge-02")
	if other != (fleetLeaveSessions{opened: 1}) {
		t.Fatalf("sessions of edge-02 = %+v, want one admitted and none ended", other)
	}
	if none := fleetLeaveCountSessions("", "edge-01"); none != (fleetLeaveSessions{}) {
		t.Fatalf("sessions in an empty output = %+v, want none", none)
	}
}

// TestFleetLeaveRequireNoHubRefusal proves what the drivers accept as the
// refusal of a rejoin on a device whose configuration names no hub: the
// problem line, on either stream, with the fix on the last line of that
// stream. Each rejected case is an answer that would let a wrong refusal
// pass: no refusal, another problem, and a fix that is not last.
func TestFleetLeaveRequireNoHubRefusal(t *testing.T) {
	refusal := "error: " + fleetLeaveNoHubProblem + "\n" +
		"why: it has no plugin hub client block\n" +
		"fix: " + fleetLeaveNoHubFix + "\n"

	accepted := []struct {
		name           string
		stdout, stderr string
	}{
		{name: "on stderr", stderr: refusal},
		{name: "on stdout", stdout: refusal},
		{name: "on stderr beside other output", stdout: "connecting\n", stderr: refusal},
	}
	for _, c := range accepted {
		t.Run(c.name, func(t *testing.T) {
			if err := fleetLeaveRequireNoHubRefusal(c.stdout, c.stderr, "the route"); err != nil {
				t.Fatalf("a refusal with the fix on its last line was not accepted: %v", err)
			}
		})
	}

	rejected := []struct {
		name           string
		stdout, stderr string
	}{
		{name: "no output"},
		{name: "an answer and no refusal", stdout: fleetLeaveNoteStarted + "\n"},
		{name: "another problem", stderr: "error: request fleet enable: the store was not opened\nfix: ze doctor\n"},
		{name: "the fix is not the last line", stderr: "fix: " + fleetLeaveNoHubFix + "\nerror: " + fleetLeaveNoHubProblem + "\n"},
		{name: "another fix", stderr: "error: " + fleetLeaveNoHubProblem + "\nfix: ze doctor\n"},
	}
	for _, c := range rejected {
		t.Run(c.name, func(t *testing.T) {
			if err := fleetLeaveRequireNoHubRefusal(c.stdout, c.stderr, "the route"); err == nil {
				t.Fatal("an answer that is not the refusal with its fix last was accepted")
			}
		})
	}
}
