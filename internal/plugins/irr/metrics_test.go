package irr

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/metrics"
)

// countingMetrics counts ze_irr_refresh_outcomes_total by result and keeps the
// last value of ze_irr_data_age_seconds; every other instrument is a no-op.
// Safe for concurrent use: the worker goroutine writes it while a test reads.
type countingMetrics struct {
	metrics.NopRegistry
	mu       sync.Mutex
	outcomes map[string]int
	age      float64
	ageSet   bool
}

// useCountingMetrics installs a counting registry as the owner's, through the
// same setMetricsRegistry the plugin registry calls, and removes it after.
func useCountingMetrics(t *testing.T) *countingMetrics {
	t.Helper()
	reg := &countingMetrics{outcomes: make(map[string]int)}
	setMetricsRegistry(reg)
	t.Cleanup(func() { ownerMetricsPtr.Store(nil) })
	return reg
}

func (c *countingMetrics) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name != "ze_irr_refresh_outcomes_total" {
		return c.NopRegistry.CounterVec(name, help, labels)
	}
	return countingOutcomeVec{reg: c}
}

func (c *countingMetrics) Gauge(name, help string) metrics.Gauge {
	if name != "ze_irr_data_age_seconds" {
		return c.NopRegistry.Gauge(name, help)
	}
	return countingAgeGauge{reg: c}
}

func (c *countingMetrics) outcome(result string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.outcomes[result]
}

// dataAge returns the last data age set, and whether one was set at all.
func (c *countingMetrics) dataAge() (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.age, c.ageSet
}

type countingOutcomeVec struct{ reg *countingMetrics }

func (v countingOutcomeVec) With(labels ...string) metrics.Counter {
	if len(labels) != 1 {
		return metrics.NopRegistry{}.Counter("", "")
	}
	return countingOutcome{reg: v.reg, result: labels[0]}
}

func (countingOutcomeVec) Delete(...string) bool { return false }

type countingOutcome struct {
	reg    *countingMetrics
	result string
}

func (o countingOutcome) Inc() {
	o.reg.mu.Lock()
	defer o.reg.mu.Unlock()
	o.reg.outcomes[o.result]++
}

func (o countingOutcome) Add(float64) {}

type countingAgeGauge struct{ reg *countingMetrics }

func (g countingAgeGauge) Set(value float64) {
	g.reg.mu.Lock()
	defer g.reg.mu.Unlock()
	g.reg.age, g.reg.ageSet = value, true
}

func (countingAgeGauge) Inc()        {}
func (countingAgeGauge) Dec()        {}
func (countingAgeGauge) Add(float64) {}

// metricsPass runs one worker pass and the metrics publish that run makes
// after it.
func (h *ownerHarness) metricsPass() {
	h.pass()
	h.owner.publishMetrics()
}

// VALIDATES: a fetch whose answer holds no prefix is counted as empty, apart
// from a stored answer, and the data age keeps growing from the last answer
// that carried prefixes. (Replaces firewall-irr TestRefreshOutcomeCountsEmptyDistinctly
// and TestRefreshOutcomeCountsSuccess, whose fetch moved to the owner.)
// METHOD: boot the owner over a fake resolver that answers AS-TEST, then move
// the clock an hour and answer nothing; read the counting registry after each.
// PREVENTS: an operator reading a green stored counter, and a data age of
// zero, while the firewall and BGP run on a list nobody confirmed.
func TestRefreshOutcomeCountsEmptyDistinctly(t *testing.T) {
	reg := useCountingMetrics(t)
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	h.owner.publishMetrics()

	if got := reg.outcome(outcomeStored); got != 1 {
		t.Fatalf("after the first answer, stored = %d, want 1", got)
	}
	if got := reg.outcome(outcomeEmpty); got != 0 {
		t.Fatalf("after the first answer, empty = %d, want 0", got)
	}

	h.clock.at = h.clock.at.Add(time.Hour)
	h.resolver.answers["AS-TEST"] = fakeAnswer{}
	h.metricsPass()

	if got := reg.outcome(outcomeEmpty); got != 1 {
		t.Errorf("after the empty answer, empty = %d, want 1", got)
	}
	if got := reg.outcome(outcomeStored); got != 1 {
		t.Errorf("after the empty answer, stored = %d, want it still 1", got)
	}
	age, set := reg.dataAge()
	if !set || age < 3599 || age > 3601 {
		t.Errorf("ze_irr_data_age_seconds = %v (set %v), want about 3600: an empty answer must not reset the age", age, set)
	}
}

// VALIDATES: a fetch that stores prefixes counts as stored and leaves a data
// age of zero, and a fetch the server does not answer counts as an error.
// METHOD: boot over a fake resolver that answers AS-TEST, then move the clock
// an hour and make the server refuse; read the counting registry after each.
// PREVENTS: the empty-answer count swallowing the healthy path, or a refused
// fetch that no counter shows.
func TestRefreshOutcomeCountsStored(t *testing.T) {
	reg := useCountingMetrics(t)
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	h.owner.publishMetrics()

	if got := reg.outcome(outcomeStored); got != 1 {
		t.Fatalf("stored = %d, want 1", got)
	}
	if age, set := reg.dataAge(); !set || age != 0 {
		t.Fatalf("ze_irr_data_age_seconds = %v (set %v) right after the store, want 0", age, set)
	}

	h.clock.at = h.clock.at.Add(time.Hour)
	h.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("connection refused")}
	h.metricsPass()

	if got := reg.outcome(outcomeError); got != 1 {
		t.Errorf("after the refused fetch, error = %d, want 1", got)
	}
	if got := reg.outcome(outcomeStored); got != 1 {
		t.Errorf("after the refused fetch, stored = %d, want it still 1", got)
	}
}

// VALIDATES: a fetch that panics costs that fetch and nothing more: the worker
// keeps answering commands, the panic is counted, and the pointer still names
// the copy stored before. (Replaces firewall-irr TestRefreshAllSurvivesPanic,
// whose fetch moved to the owner.)
// METHOD: store AS-TEST, then make the fake resolver panic for it, run the
// worker and send it `update irr as-set AS-TEST`, then a second command.
// PREVENTS: a malformed IRR answer ending the worker goroutine, which the
// SDK's dispatch recovery does not cover, and with it the plugin process: a
// panic that repeats crash-loops it into the respawn limit.
func TestFetchPanicKeepsTheWorker(t *testing.T) {
	reg := useCountingMetrics(t)
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	before, ok := h.pointer("AS-TEST")
	if !ok {
		t.Fatal("boot stored no pointer for AS-TEST")
	}

	h.resolver.panicOn = "AS-TEST"
	stop := h.runWorker()
	status, _, err := h.owner.handleCommand(cmdUpdateIRRAsSet, []string{"AS-TEST"})
	if status != statusError || err == nil {
		t.Errorf("update of a set whose fetch panics answered %q %v, want %q and an error", status, err, statusError)
	}
	// The worker answers the next command: it did not die with the panic.
	status, data, err := h.owner.handleCommand(cmdClearIRRAsSet, []string{"AS-NEVER"})
	stop()
	if status != statusDone || err != nil {
		t.Fatalf("the command after the panic answered %q %+v %v, want %q", status, data, err, statusDone)
	}

	if got := reg.outcome(outcomePanic); got != 1 {
		t.Errorf("panic outcomes = %d, want 1", got)
	}
	after, ok := h.pointer("AS-TEST")
	if !ok || after != before {
		t.Errorf("pointer after the panic %+v (present %v), want it unchanged %+v", after, ok, before)
	}
}
