// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon.
// Related: dispatch_report.go -- the engine side under test
package server

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// reportRPCClient connects an SDK plugin named after proc to s. direct uses
// the DirectBridge an internal plugin gets; otherwise every call is framed on
// the connection, as a forked plugin's is. Both reach the one engineOps entry.
func reportRPCClient(t *testing.T, s *Server, proc *process.Process, direct bool) *sdk.Plugin {
	t.Helper()
	client, engine := net.Pipe()
	conn := client
	if direct {
		bridge := rpc.NewDirectBridge()
		bridge.SetDeliverEvents(func([]string) error { return nil })
		bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
			return s.dispatchPluginRPCDirect(ctx, proc, method, params)
		})
		bridge.SetReady()
		conn = rpc.NewBridgedConn(client, bridge)
	}
	p := sdk.NewWithConn(proc.Name(), conn)
	done := make(chan struct{})
	if direct {
		close(done)
	} else {
		pc := plugipc.NewPluginConn(engine, engine)
		go func() {
			defer close(done)
			for {
				req, err := pc.ReadRequest(s.ctx)
				if err != nil {
					return
				}
				s.dispatchPluginRPC(proc, pc, req)
			}
		}()
	}
	t.Cleanup(func() {
		assert.NoError(t, p.Close())
		assert.NoError(t, engine.Close())
		<-done
	})
	return p
}

func reportTestServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	return s
}

func reportingProcess(name string) *process.Process {
	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	proc.SetStage(plugin.StageRunning)
	return proc
}

// findWarning answers the active warning of source, code and subject.
func findWarning(source, code, subject string) (report.Issue, bool) {
	for _, issue := range report.Warnings() {
		if issue.Source == source && issue.Code == code && issue.Subject == subject {
			return issue, true
		}
	}
	return report.Issue{}, false
}

// TestReportRaiseRouteBothTransports proves the report-issue route end to end
// on the connection a forked plugin uses and on the DirectBridge an internal
// plugin uses: a raise lands on the daemon's bus under the plugin's own name
// with its fix, a clear removes the warning, and every refusal is an answer
// naming its cause, never a raise the bus drops in silence.
//
// The method calls the SDK's RaiseIssue and ClearIssue over each transport
// and reads the bus; the refusals the SDK would catch first are sent to the
// entry's handler directly, which both transports call.
func TestReportRaiseRouteBothTransports(t *testing.T) {
	code := diagnostic.CodeDoctorInternalDefect
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			s := reportTestServer(t)
			proc := reportingProcess("report-route-" + name)
			p := reportRPCClient(t, s, proc, direct)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// A warning lands under the authenticated name, with its fix.
			require.NoError(t, p.RaiseIssue(ctx, sdk.Issue{
				Severity: rpc.ReportWarning, Code: code, Subject: "peer 192.0.2.1",
				Message: "the decoder met a state only a defect produces", Fix: "show bgp peer 192.0.2.1",
			}))
			warning, found := findWarning(proc.Name(), code, "peer 192.0.2.1")
			require.True(t, found, "the raise is not on the bus: %v", report.Warnings())
			assert.Equal(t, "show bgp peer 192.0.2.1", warning.Fix)
			assert.Equal(t, "the decoder met a state only a defect produces", warning.Message)

			// A clear removes it.
			require.NoError(t, p.ClearIssue(ctx, code, "peer 192.0.2.1"))
			_, found = findWarning(proc.Name(), code, "peer 192.0.2.1")
			assert.False(t, found, "the clear left the warning active")

			// An error event lands in the ring, with its fix.
			require.NoError(t, p.RaiseIssue(ctx, sdk.Issue{
				Severity: rpc.ReportError, Code: code, Subject: "decoder", Message: "a defect", Fix: "show errors",
			}))
			errorsOnBus := report.Errors(0)
			require.NotEmpty(t, errorsOnBus)
			assert.Equal(t, proc.Name(), errorsOnBus[0].Source)
			assert.Equal(t, code, errorsOnBus[0].Code)
			assert.Equal(t, "show errors", errorsOnBus[0].Fix)

			// Every wire bound is within the bus's own: a raise at each bound lands.
			atBounds := sdk.Issue{
				Severity: rpc.ReportWarning, Code: code,
				Subject: strings.Repeat("s", rpc.ReportSubjectMax),
				Message: strings.Repeat("m", rpc.ReportMessageMax),
				Fix:     strings.Repeat("f", rpc.ReportFixMax),
			}
			require.NoError(t, p.RaiseIssue(ctx, atBounds))
			_, found = findWarning(proc.Name(), code, atBounds.Subject)
			assert.True(t, found, "a raise at every wire bound was dropped by the bus")

			// A code the daemon does not describe is refused, naming it.
			err := p.RaiseIssue(ctx, sdk.Issue{
				Severity: rpc.ReportWarning, Code: "doctor-report-route-unregistered", Subject: "x", Message: "m",
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "doctor-report-route-unregistered")
			assert.Contains(t, err.Error(), "not a registered event code")
			_, found = findWarning(proc.Name(), "doctor-report-route-unregistered", "x")
			assert.False(t, found, "an unregistered code reached the bus")

			// A raise before the handshake ends is refused, naming the stage.
			early := process.NewProcess(plugin.PluginConfig{Name: "report-route-early-" + name})
			early.SetStage(plugin.StageReady)
			pEarly := reportRPCClient(t, s, early, direct)
			err = pEarly.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportWarning, Code: code, Subject: "x", Message: "m"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "before its startup handshake ended")
			_, found = findWarning(early.Name(), code, "x")
			assert.False(t, found, "a raise before the handshake reached the bus")
		})
	}
}

// TestReportIssueHandlerRefusals proves the refusals the SDK never lets
// through are still made by the engine, because a plugin in another language
// sends the request without the SDK: an oversized request, a field past its
// bound, a source the bus could not hold, and a missing owner. Each answer
// names the field and the bound.
func TestReportIssueHandlerRefusals(t *testing.T) {
	s := reportTestServer(t)
	handle := lookupEngineOp(rpc.MethodReportIssue).handle
	proc := reportingProcess("report-refusals")
	raise := func(fields string) json.RawMessage {
		return json.RawMessage(`{"action":"raise","severity":"warning","code":"` + diagnostic.CodeDoctorInternalDefect + `",` + fields + `}`)
	}
	cases := []struct {
		name   string
		proc   *process.Process
		params json.RawMessage
		want   []string
	}{
		{"request above bound", proc, json.RawMessage(`{"pad":"` + strings.Repeat("x", reportIssueEncodedMax) + `"}`),
			[]string{"octets", strconv.Itoa(reportIssueEncodedMax)}},
		{"undecodable", proc, json.RawMessage(`{"action":`), []string{"decode report-issue"}},
		{"subject above bound", proc, raise(`"subject":"` + strings.Repeat("s", rpc.ReportSubjectMax+1) + `","message":"m"`),
			[]string{"subject", strconv.Itoa(rpc.ReportSubjectMax)}},
		{"fix above bound", proc, raise(`"subject":"s","message":"m","fix":"` + strings.Repeat("f", rpc.ReportFixMax+1) + `"`),
			[]string{"fix", strconv.Itoa(rpc.ReportFixMax)}},
		{"no owner", nil, raise(`"subject":"s","message":"m"`), []string{"no plugin owner"}},
		{"name the bus cannot hold", reportingProcess(strings.Repeat("n", reportSourceMax+1)), raise(`"subject":"s","message":"m"`),
			[]string{"name", strconv.Itoa(reportSourceMax)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handle(s, t.Context(), tc.proc, tc.params)
			require.Error(t, err)
			for _, part := range tc.want {
				assert.Contains(t, err.Error(), part)
			}
		})
	}
	assert.Empty(t, report.Warnings(), "a refused request reached the bus")
}

// TestReportIssueRaiseRate proves R-2: past rpc.ReportRaisesPerWindow raises
// in one window a plugin's raise is refused with an answer naming the bound,
// the refusal is raised once per window as doctor-plugin-request-refused so
// the operator learns of it, a clear is never refused, and the next window
// accepts raises again.
//
// The method drives the handler on a clock it moves by hand.
func TestReportIssueRaiseRate(t *testing.T) {
	s := reportTestServer(t)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	s.pluginReports.now = func() time.Time { return now }
	handle := lookupEngineOp(rpc.MethodReportIssue).handle
	proc := reportingProcess("report-flood")
	raise := json.RawMessage(`{"action":"raise","severity":"error","code":"` + diagnostic.CodeDoctorInternalDefect + `","subject":"s","message":"m"}`)
	clearReq := json.RawMessage(`{"action":"clear","code":"` + diagnostic.CodeDoctorInternalDefect + `","subject":"s"}`)

	for i := range rpc.ReportRaisesPerWindow {
		_, err := handle(s, t.Context(), proc, raise)
		require.NoError(t, err, "raise %d of %d", i+1, rpc.ReportRaisesPerWindow)
	}
	for range 3 {
		_, err := handle(s, t.Context(), proc, raise)
		require.Error(t, err)
		assert.Contains(t, err.Error(), strconv.Itoa(rpc.ReportRaisesPerWindow))
	}
	_, err := handle(s, t.Context(), proc, clearReq)
	require.NoError(t, err, "a clear is refused by the raise bound")

	refusals := 0
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginRequestRefused {
			refusals++
			assert.Equal(t, proc.Name(), issue.Subject)
		}
	}
	assert.Equal(t, 1, refusals, "the refusal is raised once per window, never once per refused raise")

	now = now.Add(rpc.ReportRaiseWindow)
	_, err = handle(s, t.Context(), proc, raise)
	require.NoError(t, err, "the next window accepts raises again")
}
