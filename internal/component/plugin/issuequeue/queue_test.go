// Design: docs/architecture/doctor-and-health-checks.md -- a plugin's problems reach the report bus through one queue
// Related: queue.go -- the queue these tests drive
// Related: raises.go -- RunRaises, which TestRunRaisesRetriesOnlyARefusedRaisePerWindow drives

package issuequeue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	testCode    = "doctor-test-warning"
	testCodeErr = "doctor-test-event"
)

// recordingPublisher is a Publisher that records each call in order and
// answers refuse for every one. gate, when not nil, holds every call until it
// is closed. Safe for concurrent use.
type recordingPublisher struct {
	mu     sync.Mutex
	calls  []string
	issues []sdk.Issue
	refuse error
	gate   chan struct{}
}

func (p *recordingPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	if p.gate != nil {
		<-p.gate
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, "raise "+issue.Code+" "+issue.Subject)
	p.issues = append(p.issues, issue)
	return p.refuse
}

func (p *recordingPublisher) ClearIssue(_ context.Context, code, subject string) error {
	if p.gate != nil {
		<-p.gate
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, "clear "+code+" "+subject)
	return p.refuse
}

// hungPublisher stands in for a daemon that never answers: each call blocks
// until its context ends, as the SDK's report-issue call does.
type hungPublisher struct{}

func (hungPublisher) RaiseIssue(ctx context.Context, _ sdk.Issue) error {
	<-ctx.Done()
	return ctx.Err()
}

func (hungPublisher) ClearIssue(ctx context.Context, _, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

// VALIDATES: the queue sends the raises and clears through the publisher in
// the order they were queued, with the severity each method names, Flush
// returns once the reports before it were sent, and Stop returns only after
// every queued report was sent.
// PREVENTS: a clear overtaking its raise and leaving the warning listed, an
// error event raised as a warning, and reports queued before a plugin stop
// that never reach the daemon.
// Method: a publisher gated shut while three reports queue, then Flush and
// Stop with the gate open.
func TestQueueSendsInOrderAndDrainsOnStop(t *testing.T) {
	publisher := &recordingPublisher{gate: make(chan struct{})}
	q := Start(publisher)
	failed := func(err error) { t.Errorf("a report the publisher took was handed to failed: %v", err) }

	if err := q.RaiseWarning(testCode, "s1", "m", "f", failed); err != nil {
		t.Fatalf("RaiseWarning: %v", err)
	}
	if err := q.ClearWarning(testCode, "s1", failed); err != nil {
		t.Fatalf("ClearWarning: %v", err)
	}
	close(publisher.gate)
	q.Flush()
	publisher.mu.Lock()
	flushed := len(publisher.calls)
	publisher.mu.Unlock()
	if flushed != 2 {
		t.Fatalf("Flush returned with %d reports sent, want the 2 queued before it", flushed)
	}
	if err := q.RaiseError(testCodeErr, "s2", "m", "ze doctor", failed); err != nil {
		t.Fatalf("RaiseError: %v", err)
	}
	q.Stop()

	want := []string{
		"raise " + testCode + " s1",
		"clear " + testCode + " s1",
		"raise " + testCodeErr + " s2",
	}
	if len(publisher.calls) != len(want) {
		t.Fatalf("calls %v, want %v", publisher.calls, want)
	}
	for i := range want {
		if publisher.calls[i] != want[i] {
			t.Fatalf("calls %v, want %v", publisher.calls, want)
		}
	}
	if got := publisher.issues[0].Severity; got != rpc.ReportWarning {
		t.Errorf("RaiseWarning sent severity %v, want %v", got, rpc.ReportWarning)
	}
	if got := publisher.issues[1].Severity; got != rpc.ReportError {
		t.Errorf("RaiseError sent severity %v, want %v", got, rpc.ReportError)
	}
}

// VALIDATES: a report the daemon refuses reaches its failed callback, a
// stopped queue refuses a report at once with ErrStopped, a full queue
// refuses one with ErrFull rather than blocking, and a nil queue refuses with
// ErrNotStarted.
// PREVENTS: a refused raise vanishing, and a caller that holds a lock waiting
// on a daemon that does not answer.
// Method: a refusing publisher; a publisher gated shut while the queue fills.
func TestQueueReportsRefusalsAndNeverBlocks(t *testing.T) {
	errRefused := errors.New("refused by the daemon")
	q := Start(&recordingPublisher{refuse: errRefused})
	var got error
	if err := q.RaiseWarning(testCode, "s", "m", "f", func(err error) { got = err }); err != nil {
		t.Fatalf("RaiseWarning: %v", err)
	}
	q.Stop()
	if !errors.Is(got, errRefused) {
		t.Fatalf("failed callback got %v, want the daemon's refusal", got)
	}
	if err := q.ClearWarning(testCode, "s", func(error) {}); !errors.Is(err, ErrStopped) {
		t.Fatalf("ClearWarning after Stop = %v, want ErrStopped", err)
	}
	q.Stop()

	gated := &recordingPublisher{gate: make(chan struct{})}
	full := Start(gated)
	// One report is held by the worker inside the gated call, and the channel
	// holds queueDepth more; the next one finds the queue full.
	var refused error
	for range queueDepth + 2 {
		if err := full.RaiseError(testCodeErr, "s", "m", "f", func(error) {}); err != nil {
			refused = err
			break
		}
	}
	close(gated.gate)
	full.Stop()
	if !errors.Is(refused, ErrFull) {
		t.Fatalf("a report past the queue's depth answered %v, want ErrFull", refused)
	}

	var unstarted *Queue
	if err := unstarted.RaiseWarning(testCode, "s", "m", "f", func(error) {}); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("RaiseWarning on a nil queue = %v, want ErrNotStarted", err)
	}
	unstarted.Flush()
	unstarted.Stop()
}

// VALIDATES: Stop against a daemon that never answers returns within one
// report's bound plus the drain bound, however many reports are queued, and
// hands every unsent report to its failed callback, the ones the drain had no
// time for with ErrStopDeadline.
// PREVENTS: a plugin stop that takes one report timeout per queued report
// (256 seconds for a full queue), and a report dropped at stop with no line.
// Method: a publisher that blocks until its context ends, ten reports, bounds
// shortened to 200 ms each; one report per bound would take two seconds.
func TestQueueStopBoundsTheDrainAgainstAHungDaemon(t *testing.T) {
	const bound = 200 * time.Millisecond
	q := start(hungPublisher{}, bound, bound)
	var (
		mu       sync.Mutex
		failures []error
	)
	failed := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		failures = append(failures, err)
	}
	const queued = 10
	for range queued {
		if err := q.RaiseError(testCodeErr, "s", "m", "f", failed); err != nil {
			t.Fatalf("RaiseError: %v", err)
		}
	}
	began := time.Now()
	q.Stop()
	took := time.Since(began)

	// The bound is one report's 200 ms plus the drain's 200 ms; the limit
	// leaves slack for a loaded machine and stays below the two seconds a
	// drain bounded per report would take.
	const limit = 1500 * time.Millisecond
	if took >= limit {
		t.Fatalf("Stop took %v against a hung daemon, want under %v: one report bound and one drain bound (%v)", took, limit, 2*bound)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(failures) != queued {
		t.Fatalf("failed callbacks = %d, want one per queued report (%d)", len(failures), queued)
	}
	var unsent int
	for _, err := range failures {
		if errors.Is(err, ErrStopDeadline) {
			unsent++
		}
	}
	// The worker may hold the first report in a call bounded by its own
	// timeout when Stop is called; every other one is the drain's.
	if unsent < queued-1 {
		t.Errorf("reports refused with ErrStopDeadline = %d, want at least %d: %v", unsent, queued-1, failures)
	}
}

// VALIDATES: RunRaises asks for a retry only when the latest raise of the run
// was refused, and at most once per window from that raise; a raise accepted
// (or not yet sent) asks for none, and a late refusal of an earlier raise does
// not mark the later one.
// PREVENTS: a refused raise of a lasting condition leaving it absent from
// show warnings for as long as it lasts, and a raise on every failure.
// Method: drive Offer and the refusal callbacks it answers on a fake clock.
func TestRunRaisesRetriesOnlyARefusedRaisePerWindow(t *testing.T) {
	const window = 10 * time.Second
	epoch := time.Unix(1_700_000_000, 0)
	var run RunRaises
	if run.RetryDue(epoch, window) {
		t.Fatal("a run with no raise asks for a retry")
	}
	refuseFirst := run.Offer(epoch)
	if run.RetryDue(epoch.Add(window), window) {
		t.Fatal("a raise not refused asks for a retry")
	}
	refuseFirst(ErrFull)
	if run.RetryDue(epoch.Add(window-time.Second), window) {
		t.Fatal("a refused raise asks for a retry inside its window")
	}
	if !run.RetryDue(epoch.Add(window), window) {
		t.Fatal("a refused raise asks for no retry once its window passed")
	}
	refuseSecond := run.Offer(epoch.Add(window))
	if run.RetryDue(epoch.Add(3*window), window) {
		t.Fatal("the retry, not refused, asks for another")
	}
	refuseFirst(ErrFull)
	if run.RetryDue(epoch.Add(3*window), window) {
		t.Fatal("a late refusal of the first raise marks the retry refused")
	}
	refuseSecond(ErrFull)
	if !run.RetryDue(epoch.Add(2*window), window) {
		t.Fatal("a refused retry asks for no further retry once its window passed")
	}
}
