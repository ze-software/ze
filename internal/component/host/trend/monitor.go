// Design: docs/architecture/host/observability.md -- the host trend monitor
// Related: trend.go -- the rule registry, the readings and the published state

package trend

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/stats"
)

const trendLogName = "host.trend"

// published is the running monitor the accessor, the doctor checks and the
// show handler read; nil when none runs.
var published atomic.Pointer[Monitor]

var (
	errAlreadyRunning = errors.New("a host trend monitor is already running")
	errNoClock        = errors.New("the host trend monitor needs a clock")
	errNoDetector     = errors.New("the host trend monitor needs a host detector")
	errNoRules        = errors.New("no host trend rule is registered")
)

// Monitor samples every registered rule on its own period, judges the growth
// pattern at each bucket close, and raises and clears the rule's warning on
// the report bus.
//
// Start builds it, samples every rule once, publishes it, then creates its
// ticker and starts its one goroutine. The goroutine owns every rule's history
// and state machine; readers see only the snapshot it publishes under mu.
// Safe for concurrent use by readers; only the goroutine writes.
// Stop stops the ticker, ends the goroutine, waits for it, unpublishes the
// monitor and clears the monitor's own warnings.
type Monitor struct {
	detector *host.Detector
	ticker   clock.Ticker
	runs     []*ruleRun // owned by the goroutine after Start

	stop     chan struct{} // closed by Stop
	done     chan struct{} // closed when the goroutine returns
	stopOnce sync.Once

	mu        sync.RWMutex
	snapshot  []RuleStatus  // guarded by mu
	sampledAt time.Time     // guarded by mu: the tick the snapshot follows
	changed   chan struct{} // guarded by mu: closed and replaced at each publish
}

// ruleRun is one rule's live state inside the monitor's goroutine.
type ruleRun struct {
	rule       rule
	history    *stats.LevelHistory
	hysteresis *stats.Hysteresis
	nextSample time.Time

	latest  reading      // the latest sample's answer
	growth  stats.Growth // the latest judgment
	message string       // the warning message the latest holding judgment wrote
}

// Start builds a monitor over every registered rule, reads each rule once,
// starts the monitor's goroutine on a ticker from clk, and publishes it to
// Rules and the doctor checks. detector carries the filesystem root the
// readers read under; the daemon passes the zero Detector, which reads /.
//
// A caller whose Start answered a Monitor MUST call Stop on it, once, at
// daemon stop. Start refuses while another monitor is published.
func Start(clk clock.Clock, detector *host.Detector) (*Monitor, error) {
	if clk == nil {
		return nil, errNoClock
	}
	if detector == nil {
		return nil, errNoDetector
	}
	rules := registeredRules()
	if len(rules) == 0 {
		return nil, errNoRules
	}
	m := &Monitor{
		detector: detector,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		changed:  make(chan struct{}),
	}
	now := clk.Now()
	period := rules[0].samplePeriod
	for _, r := range rules {
		history, err := stats.NewLevelHistory(r.history)
		if err != nil {
			// registerRule validated the parameters, so this is a Ze defect.
			panic("BUG: trend rule " + r.name + ": " + err.Error())
		}
		run := &ruleRun{rule: r, history: history,
			hysteresis: stats.NewHysteresis(r.confirmAfter, r.clearAfter)}
		run.hysteresis.OnConfirmed = func() { m.confirmed(run) }
		run.hysteresis.OnCleared = func() { m.cleared(run) }
		m.runs = append(m.runs, run)
		period = gcdDuration(period, r.samplePeriod)
	}
	if published.Load() != nil {
		return nil, errAlreadyRunning
	}
	// The first read happens before the monitor is published, so no reader
	// ever sees a running monitor with an empty snapshot.
	m.sample(now)
	if !published.CompareAndSwap(nil, m) {
		return nil, errAlreadyRunning
	}
	// The ticker exists before the goroutine starts, so a test driving a fake
	// clock can wait for it and fire it with no race against the start.
	m.ticker = clk.NewTicker(period)
	go m.run()
	return m, nil
}

// Stop ends the monitor: it stops the ticker, ends the goroutine and waits for
// it, unpublishes the monitor, then clears the warnings the monitor raises (each
// rule's warning and each rule's unreadable warning), key by key. Other
// warnings of the host source, such as a carrier change, stay. It MUST be
// called after the Start that answered m; later calls do nothing.
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() {
		m.ticker.Stop()
		close(m.stop)
		<-m.done
		published.CompareAndSwap(m, nil)
		for _, run := range m.runs {
			report.ClearWarning(reportSource, run.rule.code, run.rule.name)
			report.ClearWarning(reportSource, CodeUnreadable, run.rule.name)
		}
	})
}

// run is the monitor's goroutine. It has no bound by design: it lives until
// Stop, which ends it through the stop channel.
func (m *Monitor) run() {
	defer close(m.done)
	for {
		select {
		case <-m.stop:
			return
		case at := <-m.ticker.C():
			m.sample(at)
		}
	}
}

// sample reads every rule whose period has come at tick at, then publishes.
func (m *Monitor) sample(at time.Time) {
	for _, run := range m.runs {
		if at.Before(run.nextSample) {
			continue
		}
		run.nextSample = at.Add(run.rule.samplePeriod)
		m.sampleRule(run, at)
	}
	m.publish(at)
}

// sampleRule reads one rule and folds the answer in.
func (m *Monitor) sampleRule(run *ruleRun, at time.Time) {
	got := run.rule.read(m.detector)
	switch answer := got.(type) {
	case levelReading:
		m.recovered(run)
		run.latest = answer
		m.fold(run, at, answer)
	case absentReading:
		m.recovered(run)
		run.latest = answer
	case unreadableReading:
		m.unreadable(run, answer)
	case nil:
		panic("BUG: trend rule " + run.rule.name + " answered no reading")
	}
}

// fold adds a level to the rule's history and judges growth when the sample
// closed a bucket.
func (m *Monitor) fold(run *ruleRun, at time.Time, sample levelReading) {
	closed, err := run.history.Add(at, sample.level)
	if err != nil {
		// newLevelReading refused every level Add refuses.
		panic("BUG: trend rule " + run.rule.name + ": " + err.Error())
	}
	if !closed {
		return
	}
	growth, err := run.history.Growth(sample.riseMin, sample.limit)
	if err != nil {
		// newLevelReading refused every rise and limit Growth refuses.
		panic("BUG: trend rule " + run.rule.name + ": " + err.Error())
	}
	run.growth = growth
	switch growth.Outcome() {
	case stats.GrowthWarmingUp:
		// Not judged: too few consecutive buckets, at start or after a gap. The
		// state machine counts judgments only, so a rule neither confirms nor
		// clears on a window it cannot see.
	case stats.GrowthHolds, stats.GrowthLimitReached:
		// LimitReached holds too: the level rose enough and already stands at
		// the limit, which is the exhaustion the warning projects, arrived.
		run.message = run.rule.describe(newFinding(growth, sample))
		run.hysteresis.Tick(true)
	case stats.GrowthNotRising, stats.GrowthRiseTooSmall, stats.GrowthBeyondHorizon:
		run.hysteresis.Tick(false)
	case stats.GrowthUnspecified:
		panic("BUG: trend rule " + run.rule.name + ": growth judged nothing")
	}
	if warned(run) {
		// A re-raise refreshes the message and keeps the time of the first
		// raise; it also keeps the warning among the newest on the bus, so a
		// full bus evicts older warnings before it.
		report.RaiseWarningFix(reportSource, run.rule.code, run.rule.name, run.message, run.rule.fix)
	}
}

// newFinding gathers what a describe function words from a holding judgment.
func newFinding(growth stats.Growth, sample levelReading) finding {
	rise, _ := growth.Rise()
	span, _ := growth.Span()
	exhaustion, projected := growth.Exhaustion()
	return finding{outcome: growth.Outcome(), rise: rise, span: span,
		exhaustion: exhaustion, projected: projected && growth.Outcome() == stats.GrowthHolds,
		level: sample.level, limit: sample.limit}
}

// warned reports whether the rule's warning stands: the machine is active, or
// counting toward a clear it has not reached.
func warned(run *ruleRun) bool {
	state := run.hysteresis.State()
	return state == stats.HysteresisActive || state == stats.HysteresisClearing
}

// confirmed runs when a rule's machine confirms; fold raises the warning
// right after the Tick that called it.
func (m *Monitor) confirmed(run *ruleRun) {
	slogutil.Logger(trendLogName).Info("host trend warning raised", "rule", run.rule.name)
}

// cleared runs when a rule's machine clears: the growth stopped for as many
// judgments as the rule clears after.
func (m *Monitor) cleared(run *ruleRun) {
	report.ClearWarning(reportSource, run.rule.code, run.rule.name)
	slogutil.Logger(trendLogName).Info("host trend warning cleared", "rule", run.rule.name)
}

// unreadable records a failed read. The first failure after a good read logs
// a warning; every failed read raises the rule's unreadable warning, so its
// cause stays current.
func (m *Monitor) unreadable(run *ruleRun, failure unreadableReading) {
	if _, already := run.latest.(unreadableReading); !already {
		slogutil.Logger(trendLogName).Warn("host trend rule cannot read its resource",
			"code", CodeUnreadable, "rule", run.rule.name, "source", failure.source, "error", failure.err)
	}
	run.latest = failure
	report.RaiseWarningFix(reportSource, CodeUnreadable, run.rule.name, unreadableMessage(run.rule.name, failure), trendFix)
}

// recovered clears the unreadable warning when a read succeeds after a failure.
func (m *Monitor) recovered(run *ruleRun) {
	if _, was := run.latest.(unreadableReading); !was {
		return
	}
	report.ClearWarning(reportSource, CodeUnreadable, run.rule.name)
	slogutil.Logger(trendLogName).Info("host trend rule reads its resource again", "rule", run.rule.name)
}

// publish replaces the snapshot readers see, under one lock, and wakes any
// observer waiting on changed.
func (m *Monitor) publish(at time.Time) {
	snapshot := make([]RuleStatus, 0, len(m.runs))
	for _, run := range m.runs {
		snapshot = append(snapshot, statusOf(run))
	}
	m.mu.Lock()
	m.snapshot = snapshot
	m.sampledAt = at
	close(m.changed)
	m.changed = make(chan struct{})
	m.mu.Unlock()
}

// statuses answers a copy of the snapshot. Safe for concurrent use.
func (m *Monitor) statuses() []RuleStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RuleStatus(nil), m.snapshot...)
}

// statusOf builds a rule's published state from its live state.
func statusOf(run *ruleRun) RuleStatus {
	status := RuleStatus{
		name:     run.rule.name,
		code:     run.rule.code,
		unit:     run.rule.unit,
		observed: time.Duration(run.history.ObservedRun()) * run.rule.history.BucketPeriod,
		growth:   run.growth,
	}
	switch latest := run.latest.(type) {
	case levelReading:
		status.level, status.limit, status.hasReading = latest.level, latest.limit, true
	case unreadableReading:
		status.failureSource, status.failureCause, status.failed = latest.source, latest.err.Error(), true
	case absentReading:
	case nil:
		panic("BUG: trend rule " + run.rule.name + " published before its first read")
	}
	if warned(run) {
		status.message, status.fix, status.warned = run.message, run.rule.fix, true
	}
	status.state = stateOf(run)
	return status
}

// stateOf names a rule's state. A standing warning wins over the latest read,
// because the warning stays on the bus whatever that read answered.
func stateOf(run *ruleRun) State {
	switch run.hysteresis.State() {
	case stats.HysteresisActive, stats.HysteresisClearing:
		return StateActive
	case stats.HysteresisConfirming:
		return StateConfirming
	case stats.HysteresisIdle:
	case stats.HysteresisUnspecified:
		panic("BUG: trend rule " + run.rule.name + ": state machine not built")
	}
	switch run.latest.(type) {
	case absentReading:
		return StateNotPresent
	case unreadableReading:
		return StateUnreadable
	case levelReading:
	case nil:
		panic("BUG: trend rule " + run.rule.name + " published before its first read")
	}
	if run.growth.Outcome() == stats.GrowthUnspecified {
		return StateWarmingUp
	}
	if run.growth.Outcome() == stats.GrowthWarmingUp {
		return StateWarmingUp
	}
	return StateQuiet
}

// gcdDuration answers the greatest common divisor of two positive durations,
// the ticker period at which every rule's sample period falls on a tick.
func gcdDuration(a, b time.Duration) time.Duration {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
