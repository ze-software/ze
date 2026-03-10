// Design: docs/architecture/core-design.md — no-op metrics for disabled state
// Overview: metrics.go — metric collection interfaces

package metrics

// NopRegistry is a no-op Registry for tests and disabled metrics.
type NopRegistry struct{}

// Counter returns a no-op Counter.
func (NopRegistry) Counter(string, string) Counter { return nopCounter{} }

// Gauge returns a no-op Gauge.
func (NopRegistry) Gauge(string, string) Gauge { return nopGauge{} }

// CounterVec returns a no-op CounterVec.
func (NopRegistry) CounterVec(string, string, []string) CounterVec { return nopCounterVec{} }

// GaugeVec returns a no-op GaugeVec.
func (NopRegistry) GaugeVec(string, string, []string) GaugeVec { return nopGaugeVec{} }

type nopCounter struct{}

func (nopCounter) Inc()        {}
func (nopCounter) Add(float64) {}

type nopGauge struct{}

func (nopGauge) Set(float64) {}
func (nopGauge) Inc()        {}
func (nopGauge) Dec()        {}
func (nopGauge) Add(float64) {}

type nopCounterVec struct{}

func (nopCounterVec) With(...string) Counter { return nopCounter{} }
func (nopCounterVec) Delete(...string) bool  { return false }

type nopGaugeVec struct{}

func (nopGaugeVec) With(...string) Gauge  { return nopGauge{} }
func (nopGaugeVec) Delete(...string) bool { return false }
