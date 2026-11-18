// Design: docs/architecture/l2tp/l2tp-10-metrics.md -- original physical series custody
// Related: observer.go -- one interest per bounded login slot

package l2tp

import (
	"errors"
	"log/slog"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

var errCQMSeriesUnsupported = errors.New("the registered quality metrics do not support retained series")

type cqmSeriesLease struct {
	rtt   metrics.SeriesLease
	loss  metrics.SeriesLease
	state metrics.SeriesLease
}

// Release MUST follow this slot's final update; native leases MUST release only
// their original vectors. Value copies remain idempotent through each native lease.
func (series cqmSeriesLease) Release() {
	series.state.Release()
	series.loss.Release()
	series.rtt.Release()
}

// retainCQMSeries retains before any publication and never creates a sample.
// Callers MUST release after their final update; partial acquisitions MUST be
// released before the original admission error is returned. No reporter runs here.
func (m *l2tpMetrics) retainCQMSeries(username string) (cqmSeriesLease, error) {
	if m == nil {
		return cqmSeriesLease{}, nil
	}
	rtt, ok := m.lcpEchoRTTSecs.(metrics.SeriesRetainer)
	if !ok {
		return cqmSeriesLease{}, errCQMSeriesUnsupported
	}
	loss, ok := m.lcpEchoLossRatio.(metrics.SeriesRetainer)
	if !ok {
		return cqmSeriesLease{}, errCQMSeriesUnsupported
	}
	state, ok := m.bucketState.(metrics.SeriesRetainer)
	if !ok {
		return cqmSeriesLease{}, errCQMSeriesUnsupported
	}
	var held cqmSeriesLease
	var err error
	held.rtt, err = rtt.RetainSeries(username)
	if err != nil {
		return cqmSeriesLease{}, err
	}
	held.loss, err = loss.RetainSeries(username)
	if err != nil {
		held.Release()
		return cqmSeriesLease{}, err
	}
	held.state, err = state.RetainSeries(username)
	if err != nil {
		held.Release()
		return cqmSeriesLease{}, err
	}
	return held, nil
}

// observeCQMBucket updates the binding captured by this observer, never the
// replaceable global pointer. Its caller MUST hold the login's original leases.
func (m *l2tpMetrics) observeCQMBucket(username string, bucket CQMBucket, expectedEchoes uint64) {
	if m == nil {
		return
	}
	if bucket.EchoCount > 0 {
		m.lcpEchoRTTSecs.With(username).Observe(bucket.AvgRTT().Seconds())
	}
	if expectedEchoes > 0 {
		var lost uint64
		if bucket.EchoCount < expectedEchoes {
			lost = expectedEchoes - bucket.EchoCount
		}
		m.lcpEchoLossRatio.With(username).Set(float64(lost) / float64(expectedEchoes))
	}
	m.bucketState.With(username).Set(float64(bucket.State))
}

func reportObserverMetricRefusal(birth subscriberid.L2TPBirth, login string, cause error) {
	const problem = "L2TP could not record connection quality"
	const why = "The subscriber's monitoring metrics could not be retained"
	const fix = "ze doctor"
	refusal := opererr.Wrap(cause, problem, why, fix)
	id := birth.String()
	slogutil.Logger(SubsystemName).Warn(refusal.Error(), "code", report.CodeInternalDefect,
		"subscriber-id", id, "username", login)
	report.RaiseWarningFix(SubsystemName, report.CodeInternalDefect, "quality metrics", problem+": "+why, fix,
		map[string]any{"subscriber-id": id, "username": login, "cause": cause.Error()})
}

// reportObserverStartFailure MUST run after Subsystem.mu is released; startup
// MUST retain the actual cause without publishing a partially admitted root.
func reportObserverStartFailure(logger *slog.Logger, cause error) {
	const problem = "L2TP could not start session monitoring"
	const why = "Sessions need an original observation scope before publication"
	const fix = "ze doctor"
	refusal := opererr.Wrap(cause, problem, why, fix)
	logger.Warn(refusal.Error(), "code", report.CodeInternalDefect)
	report.RaiseWarningFix(SubsystemName, report.CodeInternalDefect, "session monitoring", problem+": "+why, fix,
		map[string]any{"cause": cause.Error()})
}
