// Design: docs/architecture/isis/isis-5-adjacency.md -- a failed Hello send warns once per failure run.
//
// VALIDATES: a Hello the circuit cannot send is logged at WARN, which the default
// level shows, naming the interface and the level, why it matters (the neighbors
// drop the adjacency at hold-time expiry) and the command that shows the
// interface, under doctor-isis-hello-send-failed, and is raised as a warning
// through the SDK publisher, never on the process's own report bus; the next
// failures at that level stay quiet until a Hello goes out again, which logs at
// Info with the count, clears the warning and arms the WARN again. Removing the
// circuit clears every warning still raised. A send that finds the circuit
// already closed by the transport reports nothing.
// PREVENTS: the failure reaching only a Debug line the default level hides, a
// broken link writing one WARN per hello-interval, a forked IS-IS raising on a
// report bus the daemon never reads, and a recovered or removed interface
// leaving its warning behind.

package isis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// clearedIssue is one ClearIssue call a recordingPublisher saw.
type clearedIssue struct{ code, subject string }

// recordingPublisher is an issuequeue.Publisher that records every raise it
// takes and every clear, answers the errors of raiseErrs in order to the first
// raises, which it then does not record, and answers clearErr to each clear.
// Its readers flush the report queue first, so they see every report queued
// before the call. Safe for concurrent use.
type recordingPublisher struct {
	mu        sync.Mutex
	queue     *issuequeue.Queue
	raises    []sdk.Issue
	refusals  int
	raiseErrs []error
	clears    []clearedIssue
	clearErr  error
}

func (p *recordingPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.raiseErrs) > 0 {
		err := p.raiseErrs[0]
		p.raiseErrs = p.raiseErrs[1:]
		p.refusals++
		return err
	}
	p.raises = append(p.raises, issue)
	return nil
}

func (p *recordingPublisher) ClearIssue(_ context.Context, code, subject string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clears = append(p.clears, clearedIssue{code: code, subject: subject})
	return p.clearErr
}

// flush waits until the queue has handed every report queued so far to this
// publisher, and each refusal to its failed callback.
func (p *recordingPublisher) flush() { p.queue.Flush() }

func (p *recordingPublisher) raised() []sdk.Issue {
	p.flush()
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.raises)
}

func (p *recordingPublisher) refused() int {
	p.flush()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.refusals
}

func (p *recordingPublisher) cleared() []clearedIssue {
	p.flush()
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.clears)
}

// useRecordingPublisher starts a report queue over a recordingPublisher for one
// test, in place of the one runISISPlugin starts, resets the process report bus
// so the test can show nothing reached it, and restores both when the test
// ends. Its tests MUST NOT run in parallel: both are package state.
func useRecordingPublisher(t *testing.T) *recordingPublisher {
	t.Helper()
	previous := issueQueue.Load()
	report.ResetForTest()
	publisher := &recordingPublisher{}
	publisher.queue = issuequeue.Start(publisher)
	issueQueue.Store(publisher.queue)
	t.Cleanup(func() {
		issueQueue.Store(previous)
		publisher.queue.Stop()
		report.ResetForTest()
	})
	return publisher
}

// requireLocalBusEmpty fails the test when a raise reached the process's own
// report bus, which a forked IS-IS's daemon never reads.
func requireLocalBusEmpty(t *testing.T) {
	t.Helper()
	if errs, warns := report.Errors(0), report.Warnings(); len(errs) != 0 || len(warns) != 0 {
		t.Errorf("the process report bus holds errors %+v and warnings %+v, want nothing: raises go through the SDK", errs, warns)
	}
}

// infoLines returns the INFO lines of a text-handler log.
func infoLines(log string) []string {
	var out []string
	for line := range strings.SplitSeq(log, "\n") {
		if strings.Contains(line, "level=INFO") {
			out = append(out, line)
		}
	}
	return out
}

// TestISISHelloSendFailureWarnsOncePerRun drives one circuit's warner through two
// failures at level 1, one at level 2, a success at level 1, a new failure at
// level 1 and the circuit's removal. Each level's first failure of a run writes
// one WARN line and raises one warning for its interface and level; the success
// writes one Info line counting the two lost Hellos and clears the level-1
// warning; the failure after it WARNs and raises again; the removal clears both
// levels' warnings.
func TestISISHelloSendFailureWarnsOncePerRun(t *testing.T) {
	publisher := useRecordingPublisher(t)
	logged := &lockedBuffer{}
	warner := newHelloSendWarn(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelInfo})), "eth0")
	refused := errors.New("sendto: network is down")

	warner.observe(adjacency.Level1, "hello send", refused)
	warner.observe(adjacency.Level1, "hello send", refused)
	lines := warnLines(logged.String())
	if len(lines) != 1 {
		t.Fatalf("after two level-1 failures: WARN lines %q, want 1", lines)
	}
	for _, want := range []string{
		"IS-IS cannot send the l1 Hello on interface eth0",
		"Without Hellos the neighbors on this interface drop their adjacency when the hold time expires",
		`fix="ze show isis interface"`,
		"code=" + codeHelloSendFailed,
		"interface=eth0",
		"level=l1",
		"network is down",
		"report=queued",
	} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("WARN line %q lacks %q", lines[0], want)
		}
	}

	warner.observe(adjacency.Level2, "hello send", refused)
	if lines = warnLines(logged.String()); len(lines) != 2 || !strings.Contains(lines[1], "level=l2") {
		t.Fatalf("a level-2 failure opens its own run: WARN lines %q, want a second line at level l2", lines)
	}

	warner.observe(adjacency.Level1, "hello send", nil)
	info := infoLines(logged.String())
	if len(info) != 1 || !strings.Contains(info[0], "failed-sends=2") || !strings.Contains(info[0], "level=l1") {
		t.Fatalf("the level-1 success after the run: INFO lines %q, want one counting failed-sends=2", info)
	}
	if got, want := publisher.cleared(), []clearedIssue{{codeHelloSendFailed, "eth0 l1"}}; !slices.Equal(got, want) {
		t.Fatalf("the level-1 success cleared %+v, want only the level-1 warning %+v", got, want)
	}

	warner.observe(adjacency.Level1, "hello send", refused)
	if lines = warnLines(logged.String()); len(lines) != 3 {
		t.Fatalf("a failure after the success: WARN lines %q, want a third line", lines)
	}

	raised := publisher.raised()
	wantSubjects := []string{"eth0 l1", "eth0 l2", "eth0 l1"}
	if len(raised) != len(wantSubjects) {
		t.Fatalf("raises %+v, want one per WARN line (%d)", raised, len(wantSubjects))
	}
	for index, issue := range raised {
		if issue.Severity != rpc.ReportWarning || issue.Code != codeHelloSendFailed ||
			issue.Subject != wantSubjects[index] || issue.Fix != showInterfacesCommand {
			t.Errorf("raise %+v, want a %s under %q for %q with fix %q", issue,
				rpc.ReportWarning, codeHelloSendFailed, wantSubjects[index], showInterfacesCommand)
		}
		level := strings.TrimPrefix(wantSubjects[index], "eth0 ")
		if want := "IS-IS cannot send the " + level + " Hello on interface eth0"; issue.Message != want {
			t.Errorf("raise %d message = %q, want the WARN line's problem %q", index, issue.Message, want)
		}
	}

	warner.stop()
	removal := publisher.cleared()[1:]
	slices.SortFunc(removal, func(a, b clearedIssue) int { return strings.Compare(a.subject, b.subject) })
	if want := []clearedIssue{{codeHelloSendFailed, "eth0 l1"}, {codeHelloSendFailed, "eth0 l2"}}; !slices.Equal(removal, want) {
		t.Fatalf("the circuit's removal cleared %+v, want both failing levels %+v", removal, want)
	}
	warner.stop()
	if got := len(publisher.cleared()); got != 3 {
		t.Errorf("a second stop with no failing level made %d clears in all, want still 3", got)
	}
	requireLocalBusEmpty(t)
}

// TestISISHelloSendClearRefusalIsWarned: a clear the daemon refuses leaves the
// warning listed, so the recovery logs a WARN under the same code naming the
// interface, the level and ze show warnings.
func TestISISHelloSendClearRefusalIsWarned(t *testing.T) {
	publisher := useRecordingPublisher(t)
	publisher.clearErr = errors.New("report-issue: connection closed")
	logged := &lockedBuffer{}
	warner := newHelloSendWarn(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelInfo})), "eth0")

	warner.observe(adjacency.Level2, "hello send", errors.New("sendto: network is down"))
	warner.observe(adjacency.Level2, "hello send", nil)
	// The refused clear is logged by its failed callback, on the queue's worker.
	publisher.flush()

	lines := warnLines(logged.String())
	if len(lines) != 2 {
		t.Fatalf("WARN lines %q, want the failure and the refused clear (2)", lines)
	}
	for _, want := range []string{
		"the Hello send warning could not be cleared",
		"code=" + codeHelloSendFailed,
		"interface=eth0",
		"level=l2",
		"connection closed",
		`fix="ze show warnings"`,
	} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("WARN line %q lacks %q", lines[1], want)
		}
	}
}

// TestISISHelloSendOnClosedCircuitIsQuiet: the transport closes a circuit before
// onCircuitDown stops its Hello goroutine, so a send in that gap answers
// ErrCircuitNotOpen. That send writes no WARN and opens no run, so the next
// success writes no Info line and clears nothing either.
func TestISISHelloSendOnClosedCircuitIsQuiet(t *testing.T) {
	publisher := useRecordingPublisher(t)
	logged := &lockedBuffer{}
	warner := newHelloSendWarn(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelInfo})), "eth0")

	warner.observe(adjacency.Level1, "hello send", fmt.Errorf("send: %w", transport.ErrCircuitNotOpen))
	warner.observe(adjacency.Level1, "hello send", nil)

	if got := logged.String(); got != "" {
		t.Fatalf("a send on a closed circuit logged %q, want nothing", got)
	}
	if raised, cleared := publisher.raised(), publisher.cleared(); len(raised) != 0 || len(cleared) != 0 {
		t.Fatalf("a send on a closed circuit raised %+v and cleared %+v, want nothing", raised, cleared)
	}
}

// TestISISHelloSendRetriesARefusedRaiseOncePerWindow: the daemon refuses the
// first raise of a failure run, so the bus does not list the warning. The next
// failure inside authFailWarnInterval raises nothing; the first failure once
// the window passed raises again, which the daemon takes; the failures after it
// raise nothing more. The run writes one WARN line for the failure and one for
// the refused raise, and the bus ends up holding exactly one warning.
// Method: a publisher that refuses its first raise, a fake clock on the warner,
// and a flush of the report queue after each failure.
// PREVENTS: a link broken for hours absent from show warnings because the one
// raise of its run was refused, and a raise on every failed Hello.
func TestISISHelloSendRetriesARefusedRaiseOncePerWindow(t *testing.T) {
	publisher := useRecordingPublisher(t)
	publisher.raiseErrs = []error{errors.New("report-issue: daemon busy")}
	logged := &lockedBuffer{}
	warner := newHelloSendWarn(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelInfo})), "eth0")
	start := time.Unix(1_700_000_000, 0)
	clock := start
	warner.now = func() time.Time { return clock }
	down := errors.New("sendto: network is down")

	warner.observe(adjacency.Level1, "hello send", down)
	if got := publisher.refused(); got != 1 {
		t.Fatalf("the daemon refused %d raises after the first failure, want 1", got)
	}
	clock = start.Add(authFailWarnInterval - time.Second)
	warner.observe(adjacency.Level1, "hello send", down)
	if raised, refused := len(publisher.raised()), publisher.refused(); raised != 0 || refused != 1 {
		t.Fatalf("a failure inside the window: raises taken %d, refused %d, want 0 and still 1", raised, refused)
	}
	clock = start.Add(authFailWarnInterval)
	warner.observe(adjacency.Level1, "hello send", down)
	if raised := publisher.raised(); len(raised) != 1 || raised[0].Subject != "eth0 l1" || raised[0].Code != codeHelloSendFailed {
		t.Fatalf("the first failure once the window passed: raises taken %+v, want the eth0 l1 warning once", raised)
	}
	for _, later := range []time.Duration{2 * authFailWarnInterval, 10 * authFailWarnInterval} {
		clock = start.Add(later)
		warner.observe(adjacency.Level1, "hello send", down)
	}
	if raised, refused := len(publisher.raised()), publisher.refused(); raised != 1 || refused != 1 {
		t.Fatalf("failures after a raise the daemon took: raises taken %d, refused %d, want still 1 and 1", raised, refused)
	}

	lines := warnLines(logged.String())
	if len(lines) != 2 {
		t.Fatalf("WARN lines %q, want the run's failure and the refused raise (2)", lines)
	}
	// The refusal is logged by the queue's worker, which can run before the Hello
	// goroutine writes the run's failure line, so the line is found by its text.
	var refusedLine string
	for _, line := range lines {
		if strings.Contains(line, "could not be raised") {
			refusedLine = line
		}
	}
	for _, want := range []string{"could not be raised", "code=" + codeHelloSendFailed, "level=l1", "daemon busy"} {
		if !strings.Contains(refusedLine, want) {
			t.Errorf("the refused raise's WARN line %q lacks %q (WARN lines %q)", refusedLine, want, lines)
		}
	}
	requireLocalBusEmpty(t)
}

// failingSendBackend is a transport.Backend whose circuits refuse every frame,
// as a kernel does on a link that is down.
type failingSendBackend struct{}

func (failingSendBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	return &failingSendCircuit{fakeCircuit{name: name, recv: make(chan transport.RawFrame)}}, nil
}

type failingSendCircuit struct{ fakeCircuit }

func (*failingSendCircuit) Send(_, _ [transport.MACLen]byte, _ []byte) error {
	return errors.New("sendto: network is down")
}

// TestISISRemovedCircuitClearsHelloSendWarning opens a circuit whose every Hello
// fails, through the engine's own circuit path, waits for its warnings, then
// removes the interface: the removal clears each warning the circuit raised, so
// a removed interface leaves none behind.
func TestISISRemovedCircuitClearsHelloSendWarning(t *testing.T) {
	publisher := useRecordingPublisher(t)
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{}}}}}`))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	eng := newEngine(transport.New(failingSendBackend{}))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	defer eng.shutdown()

	deadline := time.Now().Add(5 * time.Second)
	for len(publisher.raised()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no Hello send warning was raised for the failing circuit within 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}
	eng.closeCircuit("eth0")

	var raised, cleared []string
	for _, issue := range publisher.raised() {
		raised = append(raised, issue.Subject)
	}
	for _, entry := range publisher.cleared() {
		if entry.code != codeHelloSendFailed {
			t.Errorf("cleared code %q, want %q", entry.code, codeHelloSendFailed)
		}
		cleared = append(cleared, entry.subject)
	}
	slices.Sort(raised)
	slices.Sort(cleared)
	if !slices.Equal(raised, cleared) {
		t.Fatalf("the removed circuit raised %q and cleared %q, want every raised warning cleared", raised, cleared)
	}
	requireLocalBusEmpty(t)
}
