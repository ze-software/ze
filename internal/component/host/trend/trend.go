// Design: docs/architecture/host/observability.md -- the host trend monitor
// Detail: monitor.go -- the run loop that samples, judges, raises and clears
// Detail: register.go -- the monitor's codes, its runtime doctor check, the rule check helper
// Detail: register_kernel_memory.go -- the kernel-memory rule

// Package trend keeps a short history of host resources and warns when a
// built-in rule sees one heading for exhaustion. A rule is a value its own
// file registers from init(): the monitor, the doctor checks and the
// published state iterate the registry and name no rule.
//
// The package sits below internal/component/host rather than in it because
// internal/core/diagnostic imports the host component (DoctorCheckContext
// carries host.PlatformInfo), so a doctor check registered from the host
// package itself would be an import cycle. This package imports both.
package trend

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/stats"
)

// reportSource is the report-bus source of every trend warning. The host
// inventory diff raises carrier and throttle warnings under the same source,
// so the monitor clears its own keys one by one and never the whole source.
const reportSource = "host"

// trendFix is the command every trend finding names: it shows each rule's
// state, figures and projection.
const trendFix = "show host trend"

// ErrNotRunning answers a read of the trend state when no monitor runs:
// the hub never started one, or it stopped. A reader reports it; it never
// reads it as a box with no trend to report.
var ErrNotRunning = errors.New("the host trend monitor is not running")

// Unit says what a rule's level and limit count.
type Unit uint8

const (
	// UnitUnspecified is the zero value, which no rule declares.
	UnitUnspecified Unit = iota
	// UnitBytes: the level and the limit are sizes in bytes.
	UnitBytes
)

// String names the unit for an answer or a log line.
func (u Unit) String() string {
	switch u {
	case UnitUnspecified:
		return "unspecified"
	case UnitBytes:
		return "bytes"
	default:
		panic("BUG: trend: invalid unit")
	}
}

// rule is one built-in trend rule. Its own file builds it and registers it
// from init() with registerRule, together with its doctor code and its runtime
// doctor check.
type rule struct {
	name         string            // the warning's subject and the rule's row in show host trend
	code         string            // the doctor code of its warning
	unit         Unit              // what its level and limit count
	samplePeriod time.Duration     // how often the reader runs
	history      stats.LevelParams // bucket period, bucket count, rises compared, horizon
	confirmAfter int               // consecutive holding judgments that raise the warning
	clearAfter   int               // consecutive failing judgments that clear it
	read         func(*host.Detector) reading
	describe     func(finding) string // the warning message, from the judged figures
	fix          string               // the command the warning names
}

// finding is what a rule's describe function words: one judgment whose
// outcome is GrowthHolds or GrowthLimitReached, both of which measure the
// rise and its span.
type finding struct {
	outcome    stats.GrowthOutcome
	rise       float64
	span       time.Duration
	exhaustion time.Duration // set when projected
	projected  bool          // GrowthHolds: the line reaches the limit within the horizon
	level      float64
	limit      float64
}

// ruleRegistry holds the registered rules in registration order. Rules
// register from init(), and Start copies the list, so a monitor never sees a
// rule registered after it started.
var ruleRegistry struct {
	sync.Mutex
	list []rule
}

// registerRule adds r to the registry. An invalid rule or a second rule with
// the same name or code is a Ze defect in that rule's file, found at init.
func registerRule(r rule) {
	if err := validateRule(r); err != nil {
		panic("BUG: trend rule " + r.name + ": " + err.Error())
	}
	ruleRegistry.Lock()
	defer ruleRegistry.Unlock()
	for _, known := range ruleRegistry.list {
		if known.name == r.name {
			panic("BUG: trend rule " + r.name + " registered twice")
		}
		if known.code == r.code {
			panic("BUG: trend rules " + known.name + " and " + r.name + " share code " + r.code)
		}
	}
	ruleRegistry.list = append(ruleRegistry.list, r)
}

// registeredRules answers a copy of the registry.
func registeredRules() []rule {
	ruleRegistry.Lock()
	defer ruleRegistry.Unlock()
	return append([]rule(nil), ruleRegistry.list...)
}

var errRuleInvalid = errors.New("invalid trend rule")

// validateRule refuses a rule the monitor could not run.
func validateRule(r rule) error {
	if r.name == "" {
		return errors.Join(errRuleInvalid, errors.New("no name"))
	}
	if r.code == "" {
		return errors.Join(errRuleInvalid, errors.New("no doctor code"))
	}
	if r.unit == UnitUnspecified {
		return errors.Join(errRuleInvalid, errors.New("no unit"))
	}
	if r.samplePeriod < time.Second {
		return errors.Join(errRuleInvalid, errors.New("a sample period under one second"))
	}
	if err := r.history.Validate(); err != nil {
		return err
	}
	if r.samplePeriod > r.history.BucketPeriod {
		return errors.Join(errRuleInvalid, errors.New("a sample period longer than its bucket"))
	}
	if r.confirmAfter < 1 {
		return errors.Join(errRuleInvalid, errors.New("a confirm count under 1"))
	}
	if r.clearAfter < 1 {
		return errors.Join(errRuleInvalid, errors.New("a clear count under 1"))
	}
	if r.read == nil {
		return errors.Join(errRuleInvalid, errors.New("no reader"))
	}
	if r.describe == nil {
		return errors.Join(errRuleInvalid, errors.New("no message writer"))
	}
	if r.fix == "" {
		return errors.Join(errRuleInvalid, errors.New("no fix"))
	}
	return nil
}

// reading is what a rule's reader answers at one sample, and it is exactly one
// of three things: levelReading (the resource was read), absentReading (the
// resource does not exist on this box) or unreadableReading (it could not be
// read, and why). A sealed interface rather than one struct with flags, so a
// reader cannot answer a level and a failure at once, and no zero struct can
// pass for a level of zero.
type reading interface {
	isReading()
}

// levelReading is one sample of a resource: its level, the limit at which it
// is exhausted, and the least total rise the growth pattern accepts, all in
// the rule's unit. Only newLevelReading builds one.
type levelReading struct {
	level   float64
	limit   float64
	riseMin float64
}

// absentReading says the resource does not exist on this box (a kernel module
// that is not loaded). It is not a problem, and nothing reports it.
type absentReading struct{}

// unreadableReading says the reader could not read source, and err is why.
type unreadableReading struct {
	source string
	err    error
}

func (levelReading) isReading()      {}
func (absentReading) isReading()     {}
func (unreadableReading) isReading() {}

var errReadingInvalid = errors.New("the reader answered a value the growth pattern cannot judge")

// newLevelReading validates a sample: every figure finite, a level and a rise
// of at least 0, and a limit above 0, which the growth pattern divides by.
func newLevelReading(level, limit, riseMin float64) (levelReading, error) {
	if !finite(level) || level < 0 {
		return levelReading{}, errReadingInvalid
	}
	if !finite(limit) || limit <= 0 {
		return levelReading{}, errReadingInvalid
	}
	if !finite(riseMin) || riseMin < 0 {
		return levelReading{}, errReadingInvalid
	}
	return levelReading{level: level, limit: limit, riseMin: riseMin}, nil
}

// finite reports whether v is neither NaN nor an infinity.
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// State is a rule's state as the operator sees it.
type State uint8

const (
	// StateUnspecified is the zero value, which no published rule holds.
	StateUnspecified State = iota
	// StateNotPresent: the resource does not exist on this box.
	StateNotPresent
	// StateWarmingUp: read, but not watched long enough to judge.
	StateWarmingUp
	// StateQuiet: judged, and the resource is not heading for exhaustion.
	StateQuiet
	// StateConfirming: growth seen, not yet for as many judgments as the rule
	// confirms after.
	StateConfirming
	// StateActive: the warning stands. This includes the judgments while the
	// growth has stopped and the rule counts toward clearing, because the
	// warning stays on the report bus until it clears: a row that said
	// otherwise would disagree with show warnings and show doctor.
	StateActive
	// StateUnreadable: the reader could not read the resource.
	StateUnreadable
)

// String names the state as show host trend prints it.
func (s State) String() string {
	switch s {
	case StateUnspecified:
		return "unspecified"
	case StateNotPresent:
		return "not-present"
	case StateWarmingUp:
		return "warming-up"
	case StateQuiet:
		return "quiet"
	case StateConfirming:
		return "confirming"
	case StateActive:
		return "active"
	case StateUnreadable:
		return "unreadable"
	default:
		panic("BUG: trend: invalid state")
	}
}

// RuleStatus is one rule's published state: a value copy, safe to keep and to
// read from any goroutine.
type RuleStatus struct {
	name     string
	code     string
	unit     Unit
	state    State
	observed time.Duration

	level      float64 // set when hasReading
	limit      float64 // set when hasReading
	hasReading bool    // the latest sample read the resource

	growth stats.Growth // the latest judgment; GrowthUnspecified before the first

	failureSource string // set when failed
	failureCause  string // set when failed
	failed        bool   // the latest sample could not read the resource

	message string // set when warned
	fix     string // set when warned
	warned  bool   // the rule's warning stands on the report bus
}

// Name is the rule's name, the subject of its warning.
func (s RuleStatus) Name() string { return s.name }

// Code is the doctor code of the rule's warning.
func (s RuleStatus) Code() string { return s.code }

// Unit says what Reading's level and limit count.
func (s RuleStatus) Unit() Unit { return s.unit }

// State is the rule's state.
func (s RuleStatus) State() State { return s.state }

// Observed is how long the rule has watched the resource without a gap: the
// consecutive observed closed buckets ending at the newest, times the bucket
// period. The open bucket is not counted, because nothing is judged on it.
func (s RuleStatus) Observed() time.Duration { return s.observed }

// Reading is the level and the limit the latest sample read; ok is false when
// it read none (the resource is absent or could not be read).
func (s RuleStatus) Reading() (level, limit float64, ok bool) {
	return s.level, s.limit, s.hasReading
}

// Growth is how much the level rose over the window the latest judgment
// compared, and the span of that window; ok is false when that judgment
// measured no rise (nothing judged yet, or a bucket that did not rise).
func (s RuleStatus) Growth() (rise float64, span time.Duration, ok bool) {
	rise, measured := s.growth.Rise()
	if !measured {
		return 0, 0, false
	}
	span, _ = s.growth.Span()
	return rise, span, true
}

// Exhaustion is how long after the newest judged bucket the fitted growth
// reaches the limit; ok is false when the latest judgment projected none.
func (s RuleStatus) Exhaustion() (time.Duration, bool) {
	return s.growth.Exhaustion()
}

// Failure is what the latest sample could not read and why; ok is false when
// it read the resource or found it absent.
func (s RuleStatus) Failure() (source, cause string, ok bool) {
	return s.failureSource, s.failureCause, s.failed
}

// Warning is the message and the fix of the rule's standing warning; ok is
// false when no warning stands.
func (s RuleStatus) Warning() (message, fix string, ok bool) {
	return s.message, s.fix, s.warned
}

// Rules answers the published state of every rule, in registration order, or
// ErrNotRunning when no monitor runs. It never answers an empty list for a
// monitor that is not running. Safe for concurrent use.
func Rules() ([]RuleStatus, error) {
	m := published.Load()
	if m == nil {
		return nil, ErrNotRunning
	}
	return m.statuses(), nil
}
