// Design: docs/architecture/testing/interop.md -- bounded waits over external peer state
// Related: lab.go -- peer readiness uses this wait contract.
// Related: output.go -- the bound on the last answer a timeout carries.
package interoplab

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// WaitOptions bounds a wait by a wall-clock timeout and a poll interval.
type WaitOptions struct {
	Timeout     time.Duration `json:"timeout"`
	Interval    time.Duration `json:"interval"`
	Description string        `json:"description"`
}

// WaitReport distinguishes probes that ran from probes that never measured state.
type WaitReport struct {
	Attempts          int           `json:"attempts"`
	TransientFailures int           `json:"transient-failures"`
	Elapsed           time.Duration `json:"elapsed"`
}

// Wait calls probe until ready accepts a value or the timeout expires. The
// probe MUST stop when its context is done. Wait returns an error when every
// probe fails and when successful probes never become ready. Thus, no caller
// can read an unmeasured zero value as state. A not-ready timeout carries the
// last measured value after "last output:", bounded by OutputExcerpt, so every
// caller's red names the state the peer was in, not only the state it missed.
// A never-measured error whose final probe the wait's end cut short also
// names the failure of the last probe that ran to its end.
// Wait MUST NOT start another probe after observing cancellation, even when a
// poll tick is also ready.
func Wait[T any](ctx context.Context, options WaitOptions, probe func(context.Context) (T, error), ready func(T) bool) (T, WaitReport, error) {
	var zero T
	if options.Timeout <= 0 {
		return zero, WaitReport{}, errors.New("wait timeout must be positive")
	}
	if options.Interval <= 0 {
		return zero, WaitReport{}, errors.New("wait interval must be positive")
	}
	if probe == nil {
		return zero, WaitReport{}, errors.New("wait probe is nil")
	}
	if ready == nil {
		return zero, WaitReport{}, errors.New("wait readiness predicate is nil")
	}

	started := time.Now()
	waitCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	ticker := time.NewTicker(options.Interval)
	defer ticker.Stop()

	report := WaitReport{}
	var last T
	measured := false
	var causeErr, cutErr error
	for waitCtx.Err() == nil {
		report.Attempts++
		value, err := probe(waitCtx)
		if err != nil {
			report.TransientFailures++
			// A probe the wait's own end cut short failed for that end, not
			// for the peer: an exec killed at the deadline names only its own
			// timeout. The failure of the last probe that ran to its end is
			// kept beside it, so the red names why every probe failed.
			if waitCtx.Err() == nil {
				causeErr, cutErr = err, nil
			} else {
				cutErr = err
			}
		}
		if err == nil {
			measured = true
			last = value
			if ready(value) {
				report.Elapsed = time.Since(started)
				return value, report, nil
			}
		}

		select {
		case <-waitCtx.Done():
			// The loop guard gives cancellation precedence over a ready tick.
		case <-ticker.C:
		}
	}
	report.Elapsed = time.Since(started)
	if !measured {
		return zero, report, waitNeverMeasuredError(options.Description, causeErr, cutErr)
	}
	return last, report, waitNotReadyError(options.Description, last)
}

// waitNeverMeasuredError names why the probes failed: cause is the failure of
// the last probe that ran to its end, and cut the failure of a final probe the
// wait's end cut short. Either may be nil.
func waitNeverMeasuredError(description string, cause, cut error) error {
	var tb textbuf.Buffer
	tb.Str("wait")
	if description != "" {
		tb.Str(" for ").Str(description)
	}
	tb.Str(" never measured peer state")
	if cause != nil {
		tb.Str(": ").Err(cause)
	}
	if cut == nil {
		return errors.New(tb.String())
	}
	if cause != nil {
		tb.Str("; the final probe, cut short by the wait's end")
	}
	tb.Str(": ").Err(cut)
	return errors.New(tb.String())
}

func waitNotReadyError(description string, last any) error {
	var tb textbuf.Buffer
	tb.Str("wait")
	if description != "" {
		tb.Str(" for ").Str(description)
	}
	tb.Str(" timed out before the peer became ready; last output:\n").Str(waitValueExcerpt(last))
	return errors.New(tb.String())
}

// waitValueExcerpt renders the last measured value for a timeout. Text is quoted
// as the peer printed it; any other value prints as fmt's %v, which recovers a
// panicking String method. Both pass through OutputExcerpt, so a value that
// renders to megabytes stays bounded.
func waitValueExcerpt(value any) string {
	switch typed := value.(type) {
	case string:
		return OutputExcerpt(typed)
	case []byte:
		return OutputExcerpt(string(typed))
	default:
		return OutputExcerpt(fmt.Sprint(value))
	}
}
