// Design: docs/architecture/plugin/rib-storage-design.md -- recoverable output faults.
// Related: rib_advertise_diagnostic.go -- one runtime doctor producer.
package rib

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func (d *selectedDispatch) doctorFailures() []rpc.DoctorCheckDiagnostic {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var findings []rpc.DoctorCheckDiagnostic
	for _, destination := range d.destinations {
		if destination.failure == nil {
			continue
		}
		findings = append(findings, rpc.DoctorCheckDiagnostic{Code: codeDoctorBGPSelectedOutput,
			Severity: "error", Message: selectedDestinationFailure(selectedDestinationID{peer: destination.peer, session: destination.session}, destination.failure), Fix: "ze show bgp peers"})
	}
	return findings
}

func selectedDestinationFailure(id selectedDestinationID, cause error) string {
	return fmt.Sprintf("Selected output to destination %s Session %d remains owed: %s. Complete its selecting process's attach grants or restore the named transport, then retry the change.", id.peer, id.session, cause)
}

const selectedReportSubject = "selected destinations"

func (r *RIBManager) reportSelectedDestinations(ctx context.Context, cause error) error {
	if cause == nil {
		return r.plugin.ClearIssue(ctx, codeDoctorBGPSelectedOutput, selectedReportSubject)
	}
	return r.plugin.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportWarning, Code: codeDoctorBGPSelectedOutput,
		Subject: selectedReportSubject, Message: report.FitMessage(cause.Error()), Fix: "ze show bgp peers"})
}

// selectedReporter owns one current aggregate issue and its bounded retry.
// It is created once with the dispatch lifetime; stop MUST cancel and join it.
// No route payload, destination pointer or ended-Session identity is retained.
type selectedReporter struct {
	dispatch *selectedDispatch
	send     func(context.Context, error) error
	wake     chan struct{}
	done     chan struct{}
}

func newSelectedReporter(dispatch *selectedDispatch, send func(context.Context, error) error) *selectedReporter {
	r := &selectedReporter{dispatch: dispatch, send: send, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go r.run()
	return r
}

func (d *selectedDispatch) signalReportLocked() {
	if d.reporter == nil {
		return
	}
	select {
	case d.reporter.wake <- struct{}{}:
	default:
	}
}

func (d *selectedDispatch) reportCauseLocked() error {
	var first *selectedDestination
	count := 0
	for _, destination := range d.destinations {
		if destination.failure == nil {
			continue
		}
		count++
		if first == nil {
			first = destination
			continue
		}
		comparison := destination.peer.Compare(first.peer)
		if comparison < 0 {
			first = destination
			continue
		}
		if comparison == 0 {
			if destination.session < first.session {
				first = destination
			}
		}
	}
	if first == nil {
		return nil
	}
	return fmt.Errorf("selected output remains owed at %d destinations; destination %s Session %d: %w", count, first.peer, first.session, first.failure)
}

func (r *selectedReporter) run() {
	defer close(r.done)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var tick <-chan time.Time
	acknowledged := ""
	uncertain := false
	var raiseAfter, clearAfter time.Time
	for {
		select {
		case <-r.dispatch.ctx.Done():
			return
		case <-r.wake:
		case <-tick:
		}
		tick = nil
		for {
			r.dispatch.mu.Lock()
			cause := r.dispatch.reportCauseLocked()
			r.dispatch.mu.Unlock()
			message := ""
			if cause != nil {
				message = cause.Error()
			}
			if !uncertain {
				if message == acknowledged {
					timer.Stop()
					break
				}
			}
			after := clearAfter
			if cause != nil {
				after = raiseAfter
			}
			if now := time.Now(); now.Before(after) {
				timer.Reset(after.Sub(now))
				tick = timer.C
				break
			}
			ctx, cancel := context.WithTimeout(r.dispatch.ctx, advertiseReportBudget)
			err := r.send(ctx, cause)
			cancel()
			if r.dispatch.ctx.Err() != nil {
				return
			}
			if cause != nil {
				// Every attempted raise may spend the owner's shared allowance,
				// including an acknowledged refresh of this same warning.
				raiseAfter = time.Now().Add(rpc.ReportRaiseWindow)
			}
			if err != nil {
				// The request may have applied without its answer. In particular,
				// a later empty aggregate still owes a clear after a lost raise.
				uncertain = true
				delay := time.Second
				if cause != nil {
					delay = time.Until(raiseAfter)
				} else {
					clearAfter = time.Now().Add(delay)
				}
				logger().Warn("BGP selected destination finding was not acknowledged", "error", err,
					"code", codeDoctorBGPSelectedOutput, "fix", "ze show bgp peers")
				timer.Reset(delay)
				tick = timer.C
				break
			}
			acknowledged, uncertain = message, false
			clearAfter = time.Time{}
		}
	}
}
