// Design: docs/research/l2tpv2-ze-integration.md -- explicit per-counter availability
package ppp

import "errors"

// ErrCounterUnavailable means the data plane did not provide a valid measurement.
var ErrCounterUnavailable = errors.New("ppp: data plane did not provide a valid measurement for this counter")

// CounterReading is one raw diagnostic counter. Its zero value is Unknown,
// not a measured zero. Private fields keep that distinction through copying.
// Safe to copy and use concurrently.
type CounterReading struct {
	count    uint64
	measured bool
}

// MeasuredCounter retains a native measurement, including a legitimate zero.
// Producers MUST call it only after proving the native field exists and is valid.
func MeasuredCounter(count uint64) CounterReading {
	return CounterReading{count: count, measured: true}
}

// Value returns only a native measurement. Callers MUST handle an unavailable
// reading explicitly, never substitute zero; MeasuredCounter establishes validity.
func (reading CounterReading) Value() (uint64, error) {
	if !reading.measured {
		return 0, ErrCounterUnavailable
	}
	return reading.count, nil
}
