// Design: docs/architecture/core-design.md -- bounded retry of pending FIB installs
// Related: fibkernel.go -- processEvent, run, flushRoutes

package fibkernel

import (
	"context"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	defaultRetryBaseDelay   = 500 * time.Millisecond
	defaultRetryMaxDelay    = 30 * time.Second
	defaultRetryMaxAttempts = 10
)

// pendingRetry holds the latest failed install or replace change for a prefix
// together with its bounded retry schedule.
type pendingRetry struct {
	change    incomingChange
	attempts  int
	nextRetry time.Time
}

// trackPendingChangeLocked records a failed install or replace for retry and
// wakes runRetryLoop so it arms its timer to the earliest deadline. Caller MUST
// hold f.mu.
func (f *fibKernel) trackPendingChangeLocked(pfx string, c *incomingChange, now time.Time) {
	f.trackPendingLocked(pfx, now)
	if _, exists := f.pendingChanges[pfx]; !exists {
		if len(f.pendingChanges) >= maxPendingEntries {
			return
		}
	}
	f.pendingChanges[pfx] = &pendingRetry{
		change:    cloneForwardingChange(c),
		attempts:  0,
		nextRetry: now.Add(f.retryBackoff(0)),
	}
	f.wakeRetryLoop()
}

func (f *fibKernel) wakeRetryLoop() {
	if f.retryWake == nil {
		return
	}
	select {
	case f.retryWake <- struct{}{}:
	default:
	}
}

// retryBackoff doubles retryBaseDelay for each failed retry attempt up to
// retryMaxDelay without overflowing time.Duration.
func (f *fibKernel) retryBackoff(attempts int) time.Duration {
	delay := f.retryBaseDelay
	if delay <= 0 {
		delay = defaultRetryBaseDelay
	}
	maxDelay := max(f.retryMaxDelay, delay)
	for range attempts {
		if delay >= maxDelay/2 {
			return maxDelay
		}
		delay *= 2
	}
	return delay
}

// nextPendingRetry returns the earliest nextRetry deadline among entries that
// have remaining retry attempts.
func (f *fibKernel) nextPendingRetry() (time.Time, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.stopped.Load() {
		return time.Time{}, false
	}
	var earliest time.Time
	hasNext := false
	for _, entry := range f.pendingChanges {
		if entry.attempts >= f.retryMaxAttempts {
			continue
		}
		if entry.nextRetry.IsZero() {
			continue
		}
		if !hasNext || entry.nextRetry.Before(earliest) {
			earliest = entry.nextRetry
			hasNext = true
		}
	}
	return earliest, hasNext
}

// runRetryLoop runs bounded retries for failed FIB installs on the caller's
// goroutine until ctx is canceled. It uses one reusable timer armed to the
// earliest eligible entry's nextRetry.
func (f *fibKernel) runRetryLoop(ctx context.Context) {
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		next, hasNext := f.nextPendingRetry()
		var timerC <-chan time.Time
		if hasNext {
			wait := max(time.Until(next), 0)
			if timer == nil {
				timer = time.NewTimer(wait)
			} else {
				timer.Stop()
				timer.Reset(wait)
			}
			timerC = timer.C
		} else if timer != nil {
			timer.Stop()
		}

		select {
		case <-ctx.Done():
			return
		case <-f.retryWake:
		case <-timerC:
			f.retryDuePending(time.Now())
		}
	}
}

func (f *fibKernel) retryDuePending(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped.Load() {
		return
	}
	due := make([]string, 0, len(f.pendingChanges))
	for pfx, entry := range f.pendingChanges {
		if entry.attempts >= f.retryMaxAttempts {
			continue
		}
		if entry.nextRetry.IsZero() {
			continue
		}
		if entry.nextRetry.After(now) {
			continue
		}
		due = append(due, pfx)
	}
	if len(due) == 0 {
		return
	}
	slices.Sort(due)
	rb := f.asRichBackend()
	for _, pfx := range due {
		if f.stopped.Load() {
			return
		}
		f.retryOnePendingLocked(pfx, now, rb)
	}
	f.checkPendingLagLocked(now)
	f.reportBackupsLocked()
}

// retryPendingAfterProgressLocked makes one bounded pass over pending installs
// that did not fail in the current batch after another route installed cleanly
// (for example, a covering gateway route arriving in a later batch). Caller
// MUST hold f.mu.
func (f *fibKernel) retryPendingAfterProgressLocked(now time.Time, rb richRouteBackend, failedInBatch map[string]bool) {
	candidates := make([]string, 0, len(f.pendingChanges))
	for pfx, entry := range f.pendingChanges {
		if failedInBatch[pfx] {
			continue
		}
		if entry.attempts >= f.retryMaxAttempts {
			continue
		}
		candidates = append(candidates, pfx)
	}
	if len(candidates) == 0 {
		return
	}
	slices.Sort(candidates)
	for _, pfx := range candidates {
		if f.stopped.Load() {
			return
		}
		f.retryOnePendingLocked(pfx, now, rb)
	}
}

// retryOnePendingLocked attempts to program one parked change. On success it
// clears pending state and the fib-sync-failure warning. On failure it
// refreshes only the warning (not the error ring), increments attempts, and
// doubles backoff up to retryMaxDelay until retryMaxAttempts is reached.
// Caller MUST hold f.mu.
func (f *fibKernel) retryOnePendingLocked(pfx string, now time.Time, rb richRouteBackend) {
	entry, exists := f.pendingChanges[pfx]
	if !exists {
		return
	}
	isReplace := entry.change.Action.Verb() == routeaction.VerbReplace
	op, err := f.programRouteLocked(&entry.change, pfx, 0, isReplace, rb)
	if err == nil {
		delete(f.pending, pfx)
		delete(f.pendingChanges, pfx)
		report.ClearWarning(reportSourceFIB, reportCodeFIBSyncFailure, pfx)
		return
	}
	entry.attempts++
	if entry.attempts < f.retryMaxAttempts {
		entry.nextRetry = now.Add(f.retryBackoff(entry.attempts))
	} else {
		entry.nextRetry = time.Time{}
	}
	report.RaiseWarning(reportSourceFIB, reportCodeFIBSyncFailure, pfx,
		"FIB "+op+" retry failed: "+err.Error(),
		map[string]any{keyOperation: op, keyPrefix: pfx})
}
