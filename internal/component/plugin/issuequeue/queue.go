// Design: docs/architecture/doctor-and-health-checks.md -- a plugin's problems reach the report bus through one queue
// Related: raises.go -- RunRaises, the record a warning that is raised again keeps
//
// Package issuequeue carries a plugin's raises and clears to the daemon's
// report bus without making the code that met the problem wait for the daemon.
//
// A plugin raises through the SDK (sdk.Plugin.RaiseIssue and ClearIssue),
// never through internal/core/report: the plugin may run forked, and a forked
// process's own bus is a copy the daemon never reads. An SDK call waits for
// the daemon's answer, and a plugin meets its problems on goroutines that MUST
// NOT wait: an express loop, a Hello sender, code that holds a lock every
// received packet takes. So each report enters a bounded queue that one worker
// per plugin drains, and the plugin in the daemon and the forked plugin take
// the same path.
package issuequeue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Publisher is the SDK method pair a queue reports through. *sdk.Plugin
// implements it. Each method MUST return once ctx is done: the queue bounds
// every call, and its whole drain at Stop, through ctx alone.
type Publisher interface {
	RaiseIssue(ctx context.Context, issue sdk.Issue) error
	ClearIssue(ctx context.Context, code, subject string) error
}

// queueDepth bounds the reports waiting for the daemon. A plugin raises once
// per change of a condition or per rate window, never per packet, so a full
// queue means the daemon stopped answering. A report that finds it full is
// refused, and the Warn line of the problem says so (Outcome).
const queueDepth = 256

// reportTimeout bounds one report-issue call, so a daemon that does not answer
// holds each report behind it for at most this long.
const reportTimeout = time.Second

// stopDrainMax bounds the whole drain at Stop, whatever the number of reports
// still queued: against a daemon that does not answer, a stop sends what it
// can in this time and hands every other report to its failed callback, so a
// plugin stops in seconds rather than one reportTimeout per queued report.
const stopDrainMax = 3 * time.Second

var (
	// ErrNotStarted answers a report made before the plugin's SDK entry
	// started its queue: only a test that drives an engine without the SDK
	// entry makes one.
	ErrNotStarted = errors.New("the report queue to the daemon is not started, the plugin's engine is not running")
	// ErrFull answers a report that found queueDepth reports waiting.
	ErrFull = errors.New("the report queue to the daemon is full, the daemon is not taking reports")
	// ErrStopped answers a report made after Stop.
	ErrStopped = errors.New("the report queue to the daemon is stopped, the plugin is stopping")
	// ErrStopDeadline is handed to the failed callback of a report the stop
	// drain had no time left to send.
	ErrStopDeadline = errors.New("the plugin stopped before the daemon took the report")
)

// action is what a queued report asks the daemon's report bus to do, or the
// flush marker Flush waits on.
type action uint8

const (
	actionUnspecified action = iota
	actionRaise
	actionClear
	actionFlush
)

// report is one queued raise or clear with the callback for a report the
// daemon did not take, or a flush marker with the channel its worker closes.
type report struct {
	action  action
	issue   sdk.Issue
	failed  func(error)
	flushed chan struct{}
}

// Queue is one plugin's report queue. Start creates the channel and starts
// the worker; RaiseWarning, RaiseError and ClearWarning enqueue and never
// block; the plugin's SDK entry owns the queue and MUST call Stop once, after
// every engine that reports through it has stopped, so the clears its stop
// queued are still sent. Stop returns once the worker has exited.
//
// A nil *Queue refuses every report with ErrNotStarted, so a plugin that keeps
// its queue in a package variable answers a report made before its SDK entry
// ran, rather than dropping it. Safe for concurrent use.
type Queue struct {
	publisher Publisher
	reports   chan report
	// reportBound and drainBound are reportTimeout and stopDrainMax; a test
	// in this package shortens them.
	reportBound time.Duration
	drainBound  time.Duration
	// mu orders enqueue against Stop, so no report enters the queue after the
	// worker's last drain.
	mu       sync.Mutex
	stopOnce sync.Once
	quit     chan struct{}
	done     chan struct{}
}

// Start creates the queue over publisher and starts its worker. The caller
// owns the queue and MUST call Stop once, when the plugin exits.
func Start(publisher Publisher) *Queue {
	return start(publisher, reportTimeout, stopDrainMax)
}

func start(publisher Publisher, reportBound, drainBound time.Duration) *Queue {
	q := &Queue{
		publisher:   publisher,
		reports:     make(chan report, queueDepth),
		reportBound: reportBound,
		drainBound:  drainBound,
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	go q.run()
	return q
}

// RaiseWarning queues an active warning, which stays on the bus until it is
// cleared or the plugin's process is removed. failed MUST NOT be nil: the
// worker calls it, on its own goroutine, with the reason the daemon did not
// take the report. The answer is why the report was not queued.
func (q *Queue) RaiseWarning(code, subject, message, fix string, failed func(error)) error {
	return q.enqueue(report{action: actionRaise, failed: failed, issue: sdk.Issue{
		Severity: rpc.ReportWarning, Code: code, Subject: subject, Message: message, Fix: fix,
	}})
}

// RaiseError queues an error event, which the bus keeps as history. failed is
// as for RaiseWarning.
func (q *Queue) RaiseError(code, subject, message, fix string, failed func(error)) error {
	return q.enqueue(report{action: actionRaise, failed: failed, issue: sdk.Issue{
		Severity: rpc.ReportError, Code: code, Subject: subject, Message: message, Fix: fix,
	}})
}

// ClearWarning queues the clear of the warning of code and subject. failed is
// as for RaiseWarning.
func (q *Queue) ClearWarning(code, subject string, failed func(error)) error {
	return q.enqueue(report{action: actionClear, failed: failed, issue: sdk.Issue{
		Code: code, Subject: subject,
	}})
}

func (q *Queue) enqueue(r report) error {
	if r.failed == nil {
		panic("BUG: a report queued with no failed callback")
	}
	if q == nil {
		return ErrNotStarted
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-q.quit:
		return ErrStopped
	default:
	}
	select {
	case q.reports <- r:
		return nil
	default:
		return ErrFull
	}
}

// Stop ends the worker after it sent the reports queued before the call, and
// returns once the worker has exited: within the bound of the one report the
// worker may be sending, plus stopDrainMax for all the others. It MUST
// be called after every engine reporting through the queue has stopped; a
// second call returns at once. The daemon also clears every warning a plugin
// raised when it removes the plugin's process, so a clear Stop could not send
// leaves nothing behind for long.
func (q *Queue) Stop() {
	if q == nil {
		return
	}
	q.stopOnce.Do(func() {
		q.mu.Lock()
		close(q.quit)
		q.mu.Unlock()
	})
	<-q.done
}

// Flush returns once the worker has handled every report queued before the
// call, or at once when the queue is stopped. It is for a test that reads its
// publisher after a raise: production code has no reason to wait for the
// daemon.
func (q *Queue) Flush() {
	if q == nil {
		return
	}
	flushed := make(chan struct{})
	select {
	case q.reports <- report{action: actionFlush, flushed: flushed}:
	case <-q.done:
		return
	}
	select {
	case <-flushed:
	case <-q.done:
	}
}

// run is the worker. It ends only when Stop closes quit, after the drain. A
// closed quit is looked at before each report, because a select picks at
// random among ready cases: without the look, a worker with reports waiting
// could go on sending them one reportBound each, outside the drain's deadline.
func (q *Queue) run() {
	defer close(q.done)
	for {
		select {
		case <-q.quit:
			q.drain()
			return
		default:
		}
		select {
		case r := <-q.reports:
			q.handle(context.Background(), r)
		case <-q.quit:
			q.drain()
			return
		}
	}
}

// drain handles the reports queued before Stop, at most queueDepth since
// enqueue refuses every report once quit is closed, all under one deadline of
// drainBound: a report reached after it is handed to its failed callback
// unsent.
func (q *Queue) drain() {
	ctx, cancel := context.WithTimeout(context.Background(), q.drainBound)
	defer cancel()
	for {
		select {
		case r := <-q.reports:
			q.handle(ctx, r)
		default:
			return
		}
	}
}

// handle makes one report-issue call, bounded by reportBound and by parent,
// and hands a refusal to the report's failed callback.
func (q *Queue) handle(parent context.Context, r report) {
	if r.action == actionFlush {
		close(r.flushed)
		return
	}
	if parent.Err() != nil {
		r.failed(ErrStopDeadline)
		return
	}
	ctx, cancel := context.WithTimeout(parent, q.reportBound)
	defer cancel()
	var err error
	switch r.action {
	case actionRaise:
		err = q.publisher.RaiseIssue(ctx, r.issue)
	case actionClear:
		err = q.publisher.ClearIssue(ctx, r.issue.Code, r.issue.Subject)
	case actionFlush:
		panic("BUG: a flush marker reached the report-issue call")
	case actionUnspecified:
		panic("BUG: a report queued with no action")
	default:
		panic("BUG: a report queued with an unknown action")
	}
	if err == nil {
		return
	}
	if parent.Err() != nil {
		err = fmt.Errorf("%w: %w", ErrStopDeadline, err)
	}
	r.failed(err)
}

// Outcome is the attribute a problem's Warn line carries about its report:
// that the queue took it, or why it did not, so a report that reached no
// report bus is on the line the operator reads rather than dropped. A report
// the daemon refuses later reaches its failed callback, which logs its own
// line.
func Outcome(err error) slog.Attr {
	if err != nil {
		return slog.Any("report-error", err)
	}
	return slog.String("report", "queued")
}
