// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon.
// Related: dispatch_report.go -- the engine side under test
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
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
		{"name the bus cannot hold", reportingProcess(strings.Repeat("n", report.SourceMax+1)), raise(`"subject":"s","message":"m"`),
			[]string{"name", strconv.Itoa(report.SourceMax)}},
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

// refusalsOf answers the doctor-plugin-request-refused events on the bus
// whose subject is plugin, and counts those under doctor-internal-defect.
func refusalsOf(plugin string) (refused []report.Issue, defects int) {
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginRequestRefused && issue.Subject == plugin {
			refused = append(refused, issue)
		}
		if issue.Code == diagnostic.CodeDoctorInternalDefect {
			defects++
		}
	}
	return refused, defects
}

// TestReportRaiseEscapedAtBoundsBothTransports proves the engine's encoded
// size bound agrees with the SDK's field bounds for the worst input: every
// string field at its bound and made only of characters json.Marshal writes
// as six-octet escapes. The SDK's validation passes it, so it is sent, and
// the engine accepts it and raises it, on both transports.
//
// VALIDATES: the report-issue route bounds every field (the bus limits),
// enforced by the SDK and the engine alike.
// PREVENTS: a request the SDK validated refused by the engine as oversized
// (the 4 KiB bound, which ignored escaping).
func TestReportRaiseEscapedAtBoundsBothTransports(t *testing.T) {
	code := diagnostic.CodeDoctorInternalDefect
	issue := sdk.Issue{
		Severity: rpc.ReportWarning, Code: code,
		Subject: strings.Repeat("&", rpc.ReportSubjectMax),
		Message: strings.Repeat("<", rpc.ReportMessageMax),
		Fix:     strings.Repeat(">", rpc.ReportFixMax),
	}
	in := rpc.ReportIssueInput{Action: rpc.ReportRaise, Severity: issue.Severity, Code: issue.Code,
		Subject: issue.Subject, Message: issue.Message, Fix: issue.Fix}
	require.NoError(t, rpc.ValidateReportIssue(&in), "the SDK's validation refuses the worst-case raise")
	encoded, err := json.Marshal(&in)
	require.NoError(t, err)
	require.Greater(t, len(encoded), 4<<10, "the worst case must exceed the old 4 KiB bound, or this test proves nothing")
	require.LessOrEqual(t, len(encoded), reportIssueEncodedMax)

	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			s := reportTestServer(t)
			proc := reportingProcess("report-escaped-" + name)
			p := reportRPCClient(t, s, proc, direct)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// Red: reportIssueEncodedMax back at 4 << 10.
			require.NoError(t, p.RaiseIssue(ctx, issue))
			warning, found := findWarning(proc.Name(), code, issue.Subject)
			require.True(t, found, "the escaped raise is not on the bus")
			assert.Equal(t, issue.Message, warning.Message)
			assert.Equal(t, issue.Fix, warning.Fix)
		})
	}
}

// TestReportActiveWarningsSharePerPlugin proves one plugin keeps at most its
// share of the bus's active warnings: a new warning past the share is
// refused with an answer naming the bound and the fix, the refusal is raised
// once as doctor-plugin-request-refused naming the plugin, a refresh of an
// active warning and a raise after a clear are accepted, and another
// plugin's warning survives a plugin that tries to flood the bus, which
// evicts nothing.
//
// The method drives the handler on a clock it moves past the raise rate,
// so only the share bounds the flood.
func TestReportActiveWarningsSharePerPlugin(t *testing.T) {
	s := reportTestServer(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	s.pluginReports.now = func() time.Time { return now }
	handle := lookupEngineOp(rpc.MethodReportIssue).handle
	code := diagnostic.CodeDoctorInternalDefect
	raise := func(subject string) json.RawMessage {
		return json.RawMessage(`{"action":"raise","severity":"warning","code":"` + code + `","subject":"` + subject + `","message":"m"}`)
	}
	call := func(proc *process.Process, params json.RawMessage) error {
		now = now.Add(rpc.ReportRaiseWindow / rpc.ReportRaisesPerWindow)
		_, err := handle(s, t.Context(), proc, params)
		return err
	}

	other := reportingProcess("report-share-other")
	require.NoError(t, call(other, raise("kept")))

	flood := reportingProcess("report-share-flood")
	_, _, warningCap := report.SourceWarnings(flood.Name(), code, "x")
	limit := activeWarningsMax(warningCap)
	for i := range limit {
		require.NoError(t, call(flood, raise("s"+strconv.Itoa(i))), "warning %d of the share %d", i+1, limit)
	}
	// Red: the share check in raiseWarningWithin.
	err := call(flood, raise("over"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(limit))
	assert.Contains(t, err.Error(), "ClearIssue")
	_, found := findWarning(flood.Name(), code, "over")
	assert.False(t, found, "a warning past the share reached the bus")

	require.NoError(t, call(flood, raise("s0")), "a refresh of an active warning adds none, so it is accepted")
	require.NoError(t, call(flood, json.RawMessage(`{"action":"clear","code":"`+code+`","subject":"s1"}`)))
	require.NoError(t, call(flood, raise("after-clear")), "a clear frees a slot")

	for i := range 2 * warningCap {
		assert.Error(t, call(flood, raise("flood"+strconv.Itoa(i))))
	}
	_, found = findWarning(other.Name(), code, "kept")
	assert.True(t, found, "another plugin's warning was evicted by the flood")
	assert.Zero(t, report.Losses().WarningsEvicted, "the flood evicted warnings from the bus")
	active, _, _ := report.SourceWarnings(flood.Name(), code, "x")
	assert.Equal(t, limit, active)

	refused, _ := refusalsOf(flood.Name())
	assert.Len(t, refused, 1, "the share refusal is raised once per window, never once per refused raise")
}

// TestUndecodablePluginEmitIsRefusedAndBounded proves a typed event a plugin
// emits with a payload that does not decode is that plugin's refused
// request: the emit answers an error naming the event, no subscriber receives
// it, and however many times the plugin repeats it the bus holds one
// doctor-plugin-request-refused event naming the plugin and no
// doctor-internal-defect event.
//
// VALIDATES: item 1 of the plugin-infra review, refusal at the dispatch site.
// PREVENTS: a plugin wrapping the error ring with internal-defect events
// that name the event, not the plugin.
func TestUndecodablePluginEmitIsRefusedAndBounded(t *testing.T) {
	s := reportTestServer(t)
	s.engineSubscribers = newEngineEventSubscribers()
	received := 0
	unsubscribe := forkedBGPRouteChange.Subscribe(s, func(*redistevents.RouteChangeBatch) { received++ })
	defer unsubscribe()
	proc := reportingProcess("emit-undecodable")
	params, err := json.Marshal(rpc.EmitEventInput{
		Namespace: forkedBGPRouteChange.Namespace(), EventType: forkedBGPRouteChange.EventType(), Event: "{not json",
	})
	require.NoError(t, err)

	// Red: refuseUndecodedPayload, or its refusal bound.
	for range 20 {
		_, emitErr := s.emitEvent(proc, params)
		require.Error(t, emitErr)
		assert.Contains(t, emitErr.Error(), forkedBGPRouteChange.Namespace()+"/"+forkedBGPRouteChange.EventType())
	}
	assert.Zero(t, received, "a subscriber received an undecodable payload")
	refused, defects := refusalsOf(proc.Name())
	assert.Len(t, refused, 1, "20 refused emits raised %d events", len(refused))
	assert.Zero(t, defects, "a plugin's payload was reported as a defect in ze")
}

// TestUndecodablePluginEmitToForkedSubscriberIsRefused proves the refusal of
// an undecodable typed emit does not depend on where the consumer runs: with
// a plugin-process subscriber and no engine subscriber on the event, the emit
// answers an error, reaches no process, and the bus holds one
// doctor-plugin-request-refused event naming the emitter.
//
// PREVENTS: the same emit being refused when an in-process consumer listens,
// and delivered with success and nothing raised when the consumer is forked.
func TestUndecodablePluginEmitToForkedSubscriberIsRefused(t *testing.T) {
	s := reportTestServer(t)
	s.engineSubscribers = newEngineEventSubscribers()
	s.subscriptions = newSubscriptionManager()
	namespace, eventType := forkedBGPRouteChange.Namespace(), forkedBGPRouteChange.EventType()
	subscriber := process.NewProcess(plugin.PluginConfig{Name: "forked-consumer"})
	subscriber.StartDelivery(t.Context())
	s.subscriptions.Add(subscriber, &Subscription{
		Namespace: events.LookupNamespaceID(namespace),
		EventType: events.LookupEventTypeID(eventType),
		Direction: events.DirBoth,
	})
	emitter := reportingProcess("emit-undecodable-forked")

	// Red: the decode in deliverEvent gated on an engine subscriber listening.
	for range 20 {
		delivered, err := s.deliverEvent(emitter, namespace, eventType, "", "", "{not json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), namespace+"/"+eventType)
		assert.Zero(t, delivered, "the forked subscriber received an undecodable payload")
	}
	refused, defects := refusalsOf(emitter.Name())
	assert.Len(t, refused, 1, "20 refused emits raised %d events", len(refused))
	assert.Zero(t, defects, "a plugin's payload was reported as a defect in ze")
}

// TestReportRefusalsLogOncePerWindow proves each refusal of the report-issue
// route writes one log line per rate window of the plugin even when another
// kind of refused request already raised the plugin's
// doctor-plugin-request-refused event, which stays the only one. Method:
// capture the package logger, raise the plugin's event through a failing
// dispatch-command, flood the rate on a fixed clock with one refreshed
// warning (so the share never refuses), then fill the share through
// raiseWarningWithin, the seam below the rate.
//
// PREVENTS: the shared refusal bound silencing the log of a report-issue
// flood for up to a window, so the operator loses its cause.
func TestReportRefusalsLogOncePerWindow(t *testing.T) {
	orig := logger
	t.Cleanup(func() { logger = orig })
	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = func() *slog.Logger { return captured }

	s := reportTestServer(t)
	s.dispatcher = NewDispatcher()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	s.pluginReports.now = func() time.Time { return now }
	handle := lookupEngineOp(rpc.MethodReportIssue).handle
	code := diagnostic.CodeDoctorInternalDefect
	proc := reportingProcess("report-log-looper")

	_, err := s.dispatchCommand(t.Context(), proc, "nonexistent command")
	require.Error(t, err)
	refused, _ := refusalsOf(proc.Name())
	require.Len(t, refused, 1, "the failing command raised no event, so this test proves nothing")

	params := json.RawMessage(`{"action":"raise","severity":"warning","code":"` + code + `","subject":"same","message":"m"}`)
	for range rpc.ReportRaisesPerWindow {
		_, err = handle(s, t.Context(), proc, params)
		require.NoError(t, err)
	}
	for range 3 {
		_, err = handle(s, t.Context(), proc, params)
		require.Error(t, err)
	}
	// Red: the rate refusal's log gated on refusePluginRequest raising.
	assert.Equal(t, 1, strings.Count(buf.String(), "plugin report raises refused past the rate bound"),
		"the rate refusals wrote no line, or one per refusal:\n%s", buf.String())

	_, _, warningCap := report.SourceWarnings(proc.Name(), code, "x")
	shareRefusals := 0
	for i := range activeWarningsMax(warningCap) + 3 {
		in := &rpc.ReportIssueInput{Severity: rpc.ReportWarning, Code: code, Subject: "s" + strconv.Itoa(i), Message: "m"}
		if s.pluginReports.raiseWarningWithin(proc.Name(), in) != nil {
			shareRefusals++
		}
	}
	require.Positive(t, shareRefusals, "no warning was refused past the share, so this test proves nothing")
	// Red: the share refusal's log gated on refusePluginRequest raising.
	assert.Equal(t, 1, strings.Count(buf.String(), "plugin report warning refused past its share of the bus"),
		"the share refusals wrote no line, or one per refusal:\n%s", buf.String())

	refused, _ = refusalsOf(proc.Name())
	assert.Len(t, refused, 1, "the bus event is bounded by the shared ledger, so it stays the command's")
}

// TestReportRefusalsLogAgainInNextWindow proves the once-per-window bound on
// each report-issue refusal's log line resets with the plugin's rate window:
// a rate refusal and a share refusal each write one line in the first
// window, and one more line each, only one, once the clock has moved past
// rpc.ReportRaiseWindow.
//
// METHOD: capture the package logger and drive the handler on a clock moved
// by hand. Window one floods the rate with one refreshed warning, then fills
// the share through raiseWarningWithin. The clock then moves one window on:
// a raise of a new warning opens the new window through admit and is refused
// by the share, a second share refusal follows, and the rate is flooded
// again with the refreshed warning.
//
// PREVENTS: rateRefused and shareRefused surviving the window roll, so a
// plugin refused once is never logged again for the rest of its life.
// Red: the reset of the whole window (`*window = reportWindow{start: now}`)
// in pluginReportRate.admit narrowed to start and raises.
func TestReportRefusalsLogAgainInNextWindow(t *testing.T) {
	orig := logger
	t.Cleanup(func() { logger = orig })
	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = func() *slog.Logger { return captured }

	s := reportTestServer(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	s.pluginReports.now = func() time.Time { return now }
	handle := lookupEngineOp(rpc.MethodReportIssue).handle
	code := diagnostic.CodeDoctorInternalDefect
	proc := reportingProcess("report-log-windows")
	raise := func(subject string) json.RawMessage {
		return json.RawMessage(`{"action":"raise","severity":"warning","code":"` + code + `","subject":"` + subject + `","message":"m"}`)
	}
	rateLines := func() int { return strings.Count(buf.String(), "plugin report raises refused past the rate bound") }
	shareLines := func() int {
		return strings.Count(buf.String(), "plugin report warning refused past its share of the bus")
	}

	// Window one: the rate, then the share.
	for range rpc.ReportRaisesPerWindow {
		_, err := handle(s, t.Context(), proc, raise("same"))
		require.NoError(t, err)
	}
	for range 3 {
		_, err := handle(s, t.Context(), proc, raise("same"))
		require.Error(t, err)
	}
	require.Equal(t, 1, rateLines(), "window one's rate refusals wrote no line, or one per refusal:\n%s", buf.String())
	_, _, warningCap := report.SourceWarnings(proc.Name(), code, "x")
	shareRefusals := 0
	for i := range activeWarningsMax(warningCap) + 3 {
		in := &rpc.ReportIssueInput{Severity: rpc.ReportWarning, Code: code, Subject: "s" + strconv.Itoa(i), Message: "m"}
		if s.pluginReports.raiseWarningWithin(proc.Name(), in) != nil {
			shareRefusals++
		}
	}
	require.Positive(t, shareRefusals, "no warning was refused past the share, so this test proves nothing")
	require.Equal(t, 1, shareLines(), "window one's share refusals wrote no line, or one per refusal:\n%s", buf.String())

	// Window two: the share is still full, so a new warning is refused by it.
	now = now.Add(rpc.ReportRaiseWindow)
	_, err := handle(s, t.Context(), proc, raise("window-two"))
	require.Error(t, err, "a new warning past the share was accepted")
	require.ErrorContains(t, err, "ClearIssue", "the raise was refused by the rate, not the share, so the window did not roll")
	in := &rpc.ReportIssueInput{Severity: rpc.ReportWarning, Code: code, Subject: "window-two-again", Message: "m"}
	require.Error(t, s.pluginReports.raiseWarningWithin(proc.Name(), in))
	assert.Equal(t, 2, shareLines(), "the share refusal's line did not reset with the window, or was written per refusal:\n%s", buf.String())

	// The new warning took one raise of window two; the refreshed one takes the rest.
	for range rpc.ReportRaisesPerWindow - 1 {
		_, err = handle(s, t.Context(), proc, raise("same"))
		require.NoError(t, err)
	}
	for range 3 {
		_, err = handle(s, t.Context(), proc, raise("same"))
		require.Error(t, err)
	}
	assert.Equal(t, 2, rateLines(), "the rate refusal's line did not reset with the window, or was written per refusal:\n%s", buf.String())
}

// TestFailingDispatchCommandIsBounded proves a plugin whose dispatch-command
// fails over and over leaves one doctor-plugin-request-refused event naming
// it, and each call still answers its error.
//
// PREVENTS: a plugin looping on a failing command overwriting the error ring.
func TestFailingDispatchCommandIsBounded(t *testing.T) {
	s := reportTestServer(t)
	s.dispatcher = NewDispatcher()
	proc := reportingProcess("dispatch-looper")
	// Red: refusePluginRequest in dispatchCommandResponse's failure branch.
	for range 20 {
		_, err := s.dispatchCommand(t.Context(), proc, "nonexistent command")
		require.Error(t, err)
	}
	refused, _ := refusalsOf(proc.Name())
	require.Len(t, refused, 1, "20 failed commands raised %d events", len(refused))
	assert.Contains(t, refused[0].Message, "dispatch-looper")
}

// TestMalformedForwardDestinationsAreBounded proves a plugin that sends
// forward-cached malformed destinations over and over leaves one
// doctor-plugin-request-refused event naming it, while each call still drops
// the malformed destinations.
//
// PREVENTS: one bus event per malformed destination overwriting the ring.
func TestMalformedForwardDestinationsAreBounded(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	// Red: refusePluginRequest in parseForwardDestinations.
	for range 20 {
		assert.Empty(t, parseForwardDestinations("forward-looper", []string{"bad-1", "bad-2"}))
	}
	refused, _ := refusalsOf("forward-looper")
	assert.Len(t, refused, 1, "20 malformed requests raised %d events", len(refused))
}

// TestAdHocSessionEndClearsItsReports proves an ad-hoc plugin session that
// raised a warning leaves nothing when it ends: no active warning under its
// name and no report-issue rate window. Method: an SDK
// plugin drives the session over a pipe, raises from OnStarted, and closes.
//
// PREVENTS: warnings and map entries piling up as SSH plugin sessions, each
// under a fresh random name, come and go.
func TestAdHocSessionEndClearsItsReports(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	pluginSide, engineSide := net.Pipe()
	s := &Server{
		subscriptions: newSubscriptionManager(),
		dispatcher:    NewDispatcher(),
		registry:      plugin.NewPluginRegistry(),
		capInjector:   plugin.NewCapabilityInjector(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	sessionDone := make(chan error, 1)
	go func() { sessionDone <- s.HandleAdHocPluginSession(engineSide, engineSide) }()

	p := sdk.NewWithConn("adhoc-reporter", pluginSide)
	var source string
	var raiseErr error
	p.OnStarted(func(ctx context.Context) error {
		raiseErr = p.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportWarning, Code: diagnostic.CodeDoctorInternalDefect,
			Subject: "adhoc", Message: "raised by an ad-hoc session"})
		for _, issue := range report.Warnings() {
			if strings.HasPrefix(issue.Source, "cli-debug-") {
				source = issue.Source
			}
		}
		return p.Close()
	})
	_ = p.Run(context.Background(), &sdk.Registration{}) // closed from OnStarted: Run's error carries nothing

	select {
	case <-sessionDone:
	case <-time.After(5 * time.Second):
		t.Fatal("HandleAdHocPluginSession did not return")
	}
	require.NoError(t, raiseErr)
	require.NotEmpty(t, source, "the ad-hoc raise never reached the bus, so this test proves nothing")

	// Red: endAdHocSession.
	active, _, _ := report.SourceWarnings(source, diagnostic.CodeDoctorInternalDefect, "adhoc")
	assert.Zero(t, active, "the ended session left its warnings on the bus")
	s.pluginReports.mu.Lock()
	_, window := s.pluginReports.windows[source]
	s.pluginReports.mu.Unlock()
	assert.False(t, window, "the ended session left its rate window")
}
