// Design: docs/architecture/fleet-config.md — managed client periodic metric sample sender
// Related: client.go — runConnection and handleConfigChangedRequest drive sessionSampler
// Related: report.go — shared unknownMethodPrefix and isUnknownMethodError helpers

package managed

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	minSampleCallTimeout = 500 * time.Millisecond
	maxSampleCallTimeout = 5 * time.Second
)

// SampleCollectorFunc collects one device-local metric sample report for an
// authenticated managed session at time now. Standalone startup never calls it.
type SampleCollectorFunc func(ctx context.Context, now time.Time) (fleet.SampleReport, error)

// SampleMetrics holds process-wide instrumentation counters for the managed
// metric sample sender. Standalone startup tests read these counters to verify
// zero calls to the sampler clock or sender when no managed session negotiates
// sampling.
type SampleMetrics struct {
	ClockStarts  uint64
	Ticks        uint64
	ReportsSent  uint64
	ReportsEmpty uint64
	ReportsError uint64
}

var (
	sampleClockStartsCount  atomic.Uint64
	sampleTicksCount        atomic.Uint64
	sampleReportsSentCount  atomic.Uint64
	sampleReportsEmptyCount atomic.Uint64
	sampleReportsErrorCount atomic.Uint64
)

// FleetSampleMetrics returns a snapshot of the process-wide fleet metric
// sampling instrumentation counters.
func FleetSampleMetrics() SampleMetrics {
	return SampleMetrics{
		ClockStarts:  sampleClockStartsCount.Load(),
		Ticks:        sampleTicksCount.Load(),
		ReportsSent:  sampleReportsSentCount.Load(),
		ReportsEmpty: sampleReportsEmptyCount.Load(),
		ReportsError: sampleReportsErrorCount.Load(),
	}
}

// sessionSampler manages at most one long-lived sampling goroutine for a single
// authenticated managed session. It starts no goroutine or ticker until
// UpdateInterval receives a positive interval advertised by the hub, supports
// live interval updates from config-changed replies, drops superseded
// collector results by generation, and never buffers offline samples.
//
// Safe for concurrent use. Callers MUST invoke Stop before closing mc or
// returning from runConnection.
type sessionSampler struct {
	parentCtx context.Context
	mc        *rpc.MuxConn
	cfg       *ClientConfig

	mu               sync.Mutex
	stopped          bool
	started          bool
	disabled         bool
	intervalSeconds  int
	generation       uint64
	cancel           context.CancelFunc
	activeTickGen    uint64
	activeTickCancel context.CancelFunc
	done             chan struct{}
	notify           chan struct{}
}

// newSessionSampler creates an idle per-session metric sampler bound to
// parentCtx and mc. No goroutine or timer is started until UpdateInterval
// receives a positive interval.
func newSessionSampler(parentCtx context.Context, mc *rpc.MuxConn, cfg *ClientConfig) *sessionSampler {
	return &sessionSampler{
		parentCtx: parentCtx,
		mc:        mc,
		cfg:       cfg,
		done:      make(chan struct{}),
		notify:    make(chan struct{}, 1),
	}
}

// UpdateInterval applies the hub-advertised sample interval in seconds for this
// session. Zero (or negative) stops periodic sampling without closing the
// session; a positive value starts or resets the ticker. No-op when sampling
// was disabled on this session due to an unknown-method refusal or when
// SampleCollector is nil.
func (s *sessionSampler) UpdateInterval(seconds int) {
	if s == nil {
		return
	}
	if s.cfg == nil {
		return
	}
	if s.cfg.SampleCollector == nil {
		return
	}

	seconds = normalizeSampleIntervalSeconds(seconds)

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	if s.disabled {
		s.mu.Unlock()
		return
	}
	if seconds == s.intervalSeconds {
		if seconds == 0 {
			s.mu.Unlock()
			return
		}
		if s.started {
			s.mu.Unlock()
			return
		}
	}

	s.intervalSeconds = seconds
	s.generation++
	tickCancel := s.activeTickCancel
	s.activeTickCancel = nil

	startWorker := false
	var sessionCtx context.Context
	if !s.started && seconds > 0 {
		var cancel context.CancelFunc
		sessionCtx, cancel = context.WithCancel(s.parentCtx)
		s.cancel = cancel
		s.started = true
		startWorker = true
	}
	notifyWorker := s.started && !startWorker
	s.mu.Unlock()

	if tickCancel != nil {
		tickCancel()
	}
	if startWorker {
		go s.run(sessionCtx)
		return
	}
	if notifyWorker {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
}

// Stop cancels any in-flight collection or RPC and waits for the session's
// sampler goroutine (if started) to exit. Idempotent. MUST be called before
// closing mc or returning from runConnection.
func (s *sessionSampler) Stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.generation++
	cancel := s.cancel
	tickCancel := s.activeTickCancel
	s.activeTickCancel = nil
	started := s.started
	s.mu.Unlock()

	if tickCancel != nil {
		tickCancel()
	}
	if cancel != nil {
		cancel()
	}
	if started {
		<-s.done
	}
}

func (s *sessionSampler) run(ctx context.Context) {
	defer close(s.done)

	var ticker *time.Ticker
	var tickCh <-chan time.Time
	var activeInterval time.Duration
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	applySchedule := func() (interval time.Duration, gen uint64, immediate, exit bool) {
		s.mu.Lock()
		stopped := s.stopped
		disabled := s.disabled
		seconds := s.intervalSeconds
		gen = s.generation
		s.mu.Unlock()

		if stopped {
			if ticker != nil {
				ticker.Stop()
				ticker = nil
				tickCh = nil
			}
			activeInterval = 0
			return 0, gen, false, true
		}
		if disabled {
			if ticker != nil {
				ticker.Stop()
				ticker = nil
				tickCh = nil
			}
			activeInterval = 0
			return 0, gen, false, true
		}
		if seconds <= 0 {
			if ticker != nil {
				ticker.Stop()
				ticker = nil
				tickCh = nil
			}
			activeInterval = 0
			return 0, gen, false, false
		}

		nextInterval := s.intervalDuration(seconds)
		wasInactive := activeInterval == 0
		if ticker == nil {
			sampleClockStartsCount.Add(1)
			ticker = time.NewTicker(nextInterval)
			tickCh = ticker.C
			activeInterval = nextInterval
		} else if nextInterval != activeInterval {
			sampleClockStartsCount.Add(1)
			ticker.Reset(nextInterval)
			tickCh = ticker.C
			activeInterval = nextInterval
		}
		return activeInterval, gen, wasInactive, false
	}

	interval, gen, immediate, exit := applySchedule()
	if exit {
		return
	}
	if immediate {
		s.collectAndSend(ctx, gen, interval)
		if s.isDisabled() {
			return
		}
	}

	// Intentional session-scoped loop: exits when ctx is canceled, mc closes,
	// Stop is called, or the hub refuses sample-report as an unknown method.
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.mc.Done():
			return
		case <-s.notify:
			interval, gen, immediate, exit = applySchedule()
			if exit {
				return
			}
			if immediate {
				s.collectAndSend(ctx, gen, interval)
				if s.isDisabled() {
					return
				}
			}
		case <-tickCh:
			tickInterval, tickGen, ok := s.currentTickSpec()
			if !ok {
				continue
			}
			s.collectAndSend(ctx, tickGen, tickInterval)
			if s.isDisabled() {
				return
			}
		}
	}
}

func (s *sessionSampler) currentTickSpec() (time.Duration, uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return 0, 0, false
	}
	if s.disabled {
		return 0, 0, false
	}
	if s.intervalSeconds <= 0 {
		return 0, 0, false
	}
	return s.intervalDuration(s.intervalSeconds), s.generation, true
}

func (s *sessionSampler) collectAndSend(ctx context.Context, gen uint64, interval time.Duration) {
	if s.sessionEnded(ctx) {
		return
	}
	if !s.generationCurrent(gen) {
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout(interval))
	if !s.registerTickCancel(gen, cancel) {
		cancel()
		return
	}
	defer func() {
		s.clearTickCancel(gen)
		cancel()
	}()

	now := s.now()
	sampleTicksCount.Add(1)
	rep, collectErr := s.cfg.SampleCollector(callCtx, now)
	if !s.generationCurrent(gen) {
		return
	}
	if s.sessionEnded(ctx) {
		return
	}
	if collectErr != nil {
		recordSampleSendError(s.cfg.Name, "collect", collectErr)
		return
	}
	if len(rep.Samples) == 0 {
		sampleReportsEmptyCount.Add(1)
		return
	}

	body, encodeErr := fleet.EncodeSampleReport(rep)
	if encodeErr != nil {
		recordSampleSendError(s.cfg.Name, "encode", encodeErr)
		return
	}
	if !s.generationCurrent(gen) {
		return
	}
	if s.sessionEnded(ctx) {
		return
	}

	_, rpcErr := s.mc.CallRPC(callCtx, fleet.VerbSampleReport, json.RawMessage(body))
	if rpcErr == nil {
		sampleReportsSentCount.Add(1)
		return
	}
	if s.sessionEnded(ctx) {
		return
	}
	if isUnknownMethodError(rpcErr, fleet.VerbSampleReport) {
		s.disable()
		logger().Debug("hub does not support sample-report; disabling sampling until reconnect",
			"name", s.cfg.Name, "error", rpcErr)
		return
	}

	if _, refused := errors.AsType[*rpc.RPCCallError](rpcErr); refused {
		recordSampleSendError(s.cfg.Name, "refused", rpcErr)
		return
	}
	recordSampleSendError(s.cfg.Name, "rpc", rpcErr)
}

func recordSampleSendError(name, step string, err error) {
	sampleReportsErrorCount.Add(1)
	logger().Debug("managed sample report step failed; retrying on next interval",
		"name", name, "step", step, "error", err)
}

func (s *sessionSampler) registerTickCancel(gen uint64, cancel context.CancelFunc) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	if s.disabled {
		return false
	}
	if s.generation != gen {
		return false
	}
	s.activeTickGen = gen
	s.activeTickCancel = cancel
	return true
}

func (s *sessionSampler) isDisabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disabled
}

func (s *sessionSampler) clearTickCancel(gen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeTickGen == gen {
		s.activeTickCancel = nil
	}
}

func (s *sessionSampler) generationCurrent(gen uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.stopped && !s.disabled && s.generation == gen
}

func (s *sessionSampler) disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disabled = true
	s.generation++
	s.activeTickCancel = nil
}

func (s *sessionSampler) sessionEnded(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	select {
	case <-s.mc.Done():
		return true
	default:
		return false
	}
}

func (s *sessionSampler) intervalDuration(seconds int) time.Duration {
	unit := time.Second
	if s.cfg != nil && s.cfg.sampleIntervalUnit > 0 {
		unit = s.cfg.sampleIntervalUnit
	}
	return time.Duration(seconds) * unit
}

func (s *sessionSampler) callTimeout(interval time.Duration) time.Duration {
	if s.cfg != nil && s.cfg.sampleTimeout > 0 {
		return s.cfg.sampleTimeout
	}
	half := interval / 2
	bounded := min(half, maxSampleCallTimeout)
	return max(bounded, minSampleCallTimeout)
}

func (s *sessionSampler) now() time.Time {
	if s.cfg != nil && s.cfg.sampleNow != nil {
		return s.cfg.sampleNow()
	}
	return time.Now()
}

func normalizeSampleIntervalSeconds(seconds int) int {
	if seconds <= 0 {
		return 0
	}
	if seconds > fleet.MaxSampleIntervalSeconds {
		return fleet.MaxSampleIntervalSeconds
	}
	return seconds
}
