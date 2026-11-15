// Design: docs/architecture/isis/isis-4-component-config.md -- a refused event emit is a defect, warned.
//
// VALIDATES: a session-up, session-down or lsp-change event the event bus
// refuses to publish is logged at WARN, which the default level shows, naming
// what was lost (the event and the neighbor or LSP), why it matters (the plugins
// that follow IS-IS missed the change) and the command that shows the current
// state, under doctor-internal-defect, and is raised as an error event through
// the SDK publisher, never on the process's own report bus. The per-LSP
// lsp-change refusal WARNs once and counts every refusal.
// PREVENTS: a refused emit reaching only a Debug line the default level hides,
// a refused lsp-change filling the log once per LSP, and a raise landing on a
// report bus the daemon never reads.

package isis

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refusingBus refuses every emit the way the daemon's bus refuses an event pair
// its registry does not hold (plugin server deliverEvent).
type refusingBus struct{}

func (refusingBus) Emit(namespace, eventType string, _ any) (int, error) {
	return 0, errors.New("unknown event: " + namespace + "/" + eventType)
}

func (refusingBus) Subscribe(string, string, func(any)) func() { return func() {} }

// tallyCounter is a metrics.Counter that remembers how often it was bumped.
type tallyCounter struct{ count atomic.Int64 }

func (c *tallyCounter) Inc()          { c.count.Add(1) }
func (c *tallyCounter) Add(v float64) { c.count.Add(int64(v)) }

// captureISISLogger points the package logger at a text handler at Info, the
// level that hides Debug, and restores the previous logger when the test ends.
func captureISISLogger(t *testing.T) *lockedBuffer {
	t.Helper()
	logged := &lockedBuffer{}
	previous := loggerPtr.Load()
	setLogger(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	return logged
}

// TestISISSessionEventRefusalIsWarned: the event sink emits session-up and
// session-down on a bus that refuses both. Each refusal writes one WARN line
// naming the event, the neighbor and the level, why it matters and the fix, under
// doctor-internal-defect, and raises one error event through the publisher with
// the same fix.
func TestISISSessionEventRefusalIsWarned(t *testing.T) {
	logged := captureISISLogger(t)
	publisher := useRecordingPublisher(t)

	sink := newEventSink(refusingBus{})
	neighbor := adjacency.NeighborSnapshot{SystemID: "0000.0000.0002", Level: "l2", State: "up"}
	sink.SessionUp(neighbor)
	sink.SessionDown(neighbor)

	lines := warnLines(logged.String())
	if len(lines) != 2 {
		t.Fatalf("WARN lines %q, want one per refused event (2)", lines)
	}
	for index, event := range []string{EventSessionUp, EventSessionDown} {
		for _, want := range []string{
			"IS-IS did not publish the " + event + " event of neighbor 0000.0000.0002 at level l2",
			"Plugins that follow IS-IS adjacencies missed this change",
			`fix="ze show isis neighbor"`,
			"code=" + diagnostic.CodeDoctorInternalDefect,
			"event=" + event,
			"neighbor=0000.0000.0002",
			"unknown event: isis/" + event,
			"report=queued",
		} {
			if !strings.Contains(lines[index], want) {
				t.Errorf("WARN line %q lacks %q", lines[index], want)
			}
		}
	}

	raised := publisher.raised()
	if len(raised) != 2 {
		t.Fatalf("raises %+v, want one per refused event (2)", raised)
	}
	for index, issue := range raised {
		if issue.Severity != rpc.ReportError || issue.Code != diagnostic.CodeDoctorInternalDefect ||
			issue.Subject != "0000.0000.0002 l2" || issue.Fix != showNeighborsCommand {
			t.Errorf("raise %+v, want an %s under %q for %q with fix %q", issue, rpc.ReportError,
				diagnostic.CodeDoctorInternalDefect, "0000.0000.0002 l2", showNeighborsCommand)
		}
		event := []string{EventSessionUp, EventSessionDown}[index]
		if want := "IS-IS did not publish the " + event + " event of neighbor 0000.0000.0002 at level l2"; issue.Message != want {
			t.Errorf("raise %d message = %q, want the WARN line's problem %q", index, issue.Message, want)
		}
	}
	requireLocalBusEmpty(t)
}

// TestISISRefusedDefectRaisesAreWarned: the daemon refuses the error event of a
// refused session event and of a refused lsp-change event, after the report queue
// took both. Each failed callback writes one WARN line saying show errors does not
// list the defect, under doctor-internal-defect, naming the subject, the daemon's
// error and the fix, while the problem's own WARN line still reads report=queued.
// Method: a recording publisher that refuses its first two raises; one refused
// session-up through the sink and one refused lsp-change through the engine; flush,
// then read each logger's WARN lines.
// PREVENTS: a defect the daemon never listed vanishing with no line saying so.
func TestISISRefusedDefectRaisesAreWarned(t *testing.T) {
	sessionLog := captureISISLogger(t)
	publisher := useRecordingPublisher(t)
	refusal := errors.New("report-issue: the daemon refused the raise")
	publisher.raiseErrs = []error{refusal, refusal}
	previousBus := eventBusPtr.Load()
	setEventBus(refusingBus{})
	t.Cleanup(func() { eventBusPtr.Store(previousBus) })

	newEventSink(refusingBus{}).SessionUp(adjacency.NeighborSnapshot{SystemID: "0000.0000.0002", Level: "l2", State: "up"})
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	lspLog := &lockedBuffer{}
	eng.log = slog.New(slog.NewTextHandler(lspLog, &slog.HandlerOptions{Level: slog.LevelInfo}))
	eng.mu.Lock()
	eng.lspChangesRefused = &tallyCounter{}
	eng.mu.Unlock()
	const lspID = "0000.0000.0002.00-00"
	eng.publishLSPChange("l1", lspID, 7, "add")

	if got := publisher.refused(); got != 2 {
		t.Fatalf("the daemon refused %d raises, want both (2)", got)
	}
	for _, tc := range []struct {
		site, log, subject, fix string
	}{
		{"session event", sessionLog.String(), "0000.0000.0002 l2", showNeighborsCommand},
		{"lsp-change event", lspLog.String(), lspID, showDatabaseCommand},
	} {
		lines := warnLines(tc.log)
		if len(lines) != 2 {
			t.Fatalf("%s: WARN lines %q, want the refused emit and its refused raise (2)", tc.site, lines)
		}
		var refusalLine string
		for _, line := range lines {
			if strings.Contains(line, "isis could not raise an internal defect on the daemon's report bus, so show errors does not list it") {
				refusalLine = line
				continue
			}
			if !strings.Contains(line, "report=queued") {
				t.Errorf("%s: the refused emit's WARN line %q does not say the queue took its raise", tc.site, line)
			}
		}
		for _, want := range []string{
			"code=" + diagnostic.CodeDoctorInternalDefect,
			// slog's text handler quotes a value only when it needs quoting: the
			// session subject holds a space, an LSP ID does not.
			"subject=" + slogTextValue(tc.subject),
			`error="` + refusal.Error() + `"`,
			`fix="` + tc.fix + `"`,
		} {
			if !strings.Contains(refusalLine, want) {
				t.Errorf("%s: the refused raise's WARN line %q lacks %q", tc.site, refusalLine, want)
			}
		}
	}
}

// TestISISLSPChangeRefusalWarnsOnceAndCounts: publishLSPChange runs three times
// on a bus that refuses lsp-change. The first refusal writes the one WARN line,
// naming the LSP, the level, why it matters and the fix, under
// doctor-internal-defect, and raises one error event through the publisher; the
// counter counts all three refusals.
func TestISISLSPChangeRefusalWarnsOnceAndCounts(t *testing.T) {
	publisher := useRecordingPublisher(t)
	previousBus := eventBusPtr.Load()
	setEventBus(refusingBus{})
	t.Cleanup(func() { eventBusPtr.Store(previousBus) })

	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	logged := &lockedBuffer{}
	eng.log = slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelInfo}))
	refused := &tallyCounter{}
	eng.mu.Lock()
	eng.lspChangesRefused = refused
	eng.mu.Unlock()

	const lspID = "0000.0000.0002.00-00"
	for range 3 {
		eng.publishLSPChange("l1", lspID, 7, "add")
	}

	lines := warnLines(logged.String())
	if len(lines) != 1 {
		t.Fatalf("WARN lines %q, want only the first refusal logged (1)", lines)
	}
	for _, want := range []string{
		"IS-IS did not publish the lsp-change event of LSP " + lspID + " at level l1",
		"Plugins that follow the IS-IS database miss its changes",
		"ze_isis_lsp_change_events_refused_total counts the later refusals",
		`fix="ze show isis database"`,
		"code=" + diagnostic.CodeDoctorInternalDefect,
		"lsp-id=" + lspID,
		"unknown event: isis/lsp-change",
		"report=queued",
	} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("WARN line %q lacks %q", lines[0], want)
		}
	}
	if got := refused.count.Load(); got != 3 {
		t.Errorf("refused counter = %d, want every refusal counted (3)", got)
	}
	raised := publisher.raised()
	if len(raised) != 1 || raised[0].Severity != rpc.ReportError ||
		raised[0].Code != diagnostic.CodeDoctorInternalDefect || raised[0].Subject != lspID ||
		raised[0].Fix != showDatabaseCommand {
		t.Errorf("raises %+v, want one %s under %q for %q with fix %q", raised, rpc.ReportError,
			diagnostic.CodeDoctorInternalDefect, lspID, showDatabaseCommand)
	}
	if want := "IS-IS did not publish the lsp-change event of LSP " + lspID + " at level l1"; len(raised) == 1 && raised[0].Message != want {
		t.Errorf("raise message = %q, want the WARN line's problem %q", raised[0].Message, want)
	}
	requireLocalBusEmpty(t)
}

// slogTextValue renders value as slog's TextHandler writes it: bare when it holds
// no space, equals sign, quote or control character, quoted otherwise.
func slogTextValue(value string) string {
	if strings.ContainsFunc(value, func(r rune) bool { return r <= ' ' || r == '=' || r == '"' || r == 0x7f }) {
		return strconv.Quote(value)
	}
	return value
}
