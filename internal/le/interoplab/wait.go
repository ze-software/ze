// Design: docs/architecture/testing/interop.md -- bounded waits over external peer state
// Related: lab.go -- peer readiness uses this wait contract.
package interoplab

import (
	"context"
	"errors"
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
// can read an unmeasured zero value as state.
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
	var lastErr error
	for {
		report.Attempts++
		value, err := probe(waitCtx)
		if err != nil {
			report.TransientFailures++
			lastErr = err
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
			report.Elapsed = time.Since(started)
			if !measured {
				return zero, report, waitNeverMeasuredError(options.Description, lastErr)
			}
			return last, report, waitNotReadyError(options.Description)
		case <-ticker.C:
		}
	}
}

func waitNeverMeasuredError(description string, last error) error {
	var tb textbuf.Buffer
	tb.Str("wait")
	if description != "" {
		tb.Str(" for ").Str(description)
	}
	tb.Str(" never measured peer state")
	if last != nil {
		tb.Str(": ").Err(last)
	}
	return errors.New(tb.String())
}

func waitNotReadyError(description string) error {
	var tb textbuf.Buffer
	tb.Str("wait")
	if description != "" {
		tb.Str(" for ").Str(description)
	}
	tb.Str(" timed out before the peer became ready")
	return errors.New(tb.String())
}
