// Design: docs/architecture/diagnostics/production-diagnostics.md -- always-on daemon liveness watchdog
// Related: doctor_codes.go -- doctor diagnostic code registration

// Package watchdog runs a single bounded background goroutine that monitors
// scheduler wake-up drift and registered subsystem liveness probes. When a
// stall is detected outside the cooldown window, it captures bounded goroutine
// stacks and triggers an automatic flight-recorder trace snapshot into a
// fixed-size in-memory ring.
package watchdog

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultInterval       = 2 * time.Second
	defaultStallThreshold = 5 * time.Second
	defaultCooldown       = 30 * time.Second
	maxStallRecords       = 8
	maxStackBytes         = 256 * 1024

	schedulerProbeName = "scheduler"
	flightrecTrigger   = "watchdog-stall"
	reportSource       = "watchdog"
	reportFix          = "ze show system watchdog"
)

// ProbeFunc evaluates one subsystem's liveness on a watchdog tick.
// Implementations MUST be safe for concurrent use and MUST avoid acquiring
// locks that may be held across blocking I/O.
type ProbeFunc func() (stalled bool, detail string, stallDuration time.Duration)

// ProbeState reports the latest observed health of one registered probe.
type ProbeState struct {
	Name    string `json:"name"`
	Stalled bool   `json:"stalled"`
	Detail  string `json:"detail,omitempty"`
	StallMS int64  `json:"stall-ms,omitempty"`
}

// StallRecord captures one bounded stall event with goroutine stacks and any
// correlated flight-recorder snapshot ID.
type StallRecord struct {
	ID              string    `json:"id"`
	CapturedAt      time.Time `json:"captured-at"`
	Probe           string    `json:"probe"`
	Detail          string    `json:"detail"`
	StallMS         int64     `json:"stall-ms"`
	TraceSnapshotID string    `json:"trace-snapshot-id,omitempty"`
	GoroutineCount  int       `json:"goroutine-count"`
	StacksTruncated bool      `json:"stacks-truncated"`
	Stacks          string    `json:"stacks"`
}

// Snapshot is a point-in-time read of the daemon liveness watchdog.
type Snapshot struct {
	Running          bool          `json:"running"`
	IntervalMS       int64         `json:"interval-ms"`
	StallThresholdMS int64         `json:"stall-threshold-ms"`
	Ticks            uint64        `json:"ticks"`
	StallCount       uint64        `json:"stall-count"`
	LastTickAt       time.Time     `json:"last-tick-at,omitzero"`
	Probes           []ProbeState  `json:"probes"`
	RecentStalls     []StallRecord `json:"recent-stalls"`
}

type probeEntry struct {
	id    uint64
	name  string
	fn    ProbeFunc
	state ProbeState
}

type probeCall struct {
	id   uint64
	name string
	fn   ProbeFunc
}

type probeResult struct {
	id       uint64
	name     string
	stalled  bool
	detail   string
	duration time.Duration
}

type watchdogState struct {
	// lifecycleMu serializes Start, Stop, and resetForTest so Status only
	// ever acquires the leaf mutex mu and never waits on goroutine exit.
	lifecycleMu sync.Mutex

	mu             sync.Mutex
	interval       time.Duration
	stallThreshold time.Duration
	cooldown       time.Duration
	cancel         context.CancelFunc
	done           chan struct{}

	ticks         uint64
	stallCount    uint64
	stallSeq      uint64
	lastTickAt    time.Time
	lastCaptureAt time.Time

	nextProbeID uint64
	probes      map[string]probeEntry

	stalls    [maxStallRecords]StallRecord
	stallHead int
	stallLen  int
}

var (
	global      = watchdogState{probes: make(map[string]probeEntry)}
	watchdogLog = slogutil.LazyLogger("watchdog")
)

// Start launches the always-on daemon liveness watchdog with default intervals.
// Repeated calls while running are idempotent. Callers MUST pair Start with Stop.
// Safe for concurrent use.
func Start() {
	startWithConfig(defaultInterval, defaultStallThreshold, defaultCooldown)
}

// Stop cancels the watchdog worker and waits until its goroutine exits.
// Repeated calls when stopped are idempotent. Callers MUST call Stop after Start.
// Safe for concurrent use.
func Stop() {
	global.lifecycleMu.Lock()
	defer global.lifecycleMu.Unlock()
	stopLocked()
}

// RegisterProbe registers a named liveness probe and returns an idempotent
// unregister function that removes only that specific registration. Empty names
// or nil probe functions are ignored and return a no-op unregister function.
// Safe for concurrent use.
func RegisterProbe(name string, fn ProbeFunc) func() {
	if name == "" {
		return func() {}
	}
	if fn == nil {
		return func() {}
	}

	global.mu.Lock()
	global.nextProbeID++
	id := global.nextProbeID
	if global.probes == nil {
		global.probes = make(map[string]probeEntry)
	}
	global.probes[name] = probeEntry{
		id:    id,
		name:  name,
		fn:    fn,
		state: ProbeState{Name: name},
	}
	global.mu.Unlock()

	return func() {
		global.mu.Lock()
		defer global.mu.Unlock()
		cur, ok := global.probes[name]
		if !ok {
			return
		}
		if cur.id != id {
			return
		}
		delete(global.probes, name)
	}
}

// Status returns a point-in-time snapshot of the watchdog under its leaf mutex.
// Probes are sorted by name and RecentStalls are ordered newest-first; both
// slices are always non-nil.
// Safe for concurrent use.
func Status() Snapshot {
	global.mu.Lock()
	defer global.mu.Unlock()

	interval := global.interval
	if interval == 0 {
		interval = defaultInterval
	}
	threshold := global.stallThreshold
	if threshold == 0 {
		threshold = defaultStallThreshold
	}

	probes := make([]ProbeState, 0, len(global.probes))
	for _, entry := range global.probes {
		probes = append(probes, entry.state)
	}
	slices.SortFunc(probes, func(a, b ProbeState) int {
		return strings.Compare(a.Name, b.Name)
	})

	stalls := make([]StallRecord, 0, global.stallLen)
	for i := range global.stallLen {
		idx := (global.stallHead - 1 - i + maxStallRecords) % maxStallRecords
		stalls = append(stalls, global.stalls[idx])
	}

	return Snapshot{
		Running:          global.cancel != nil,
		IntervalMS:       interval.Milliseconds(),
		StallThresholdMS: threshold.Milliseconds(),
		Ticks:            global.ticks,
		StallCount:       global.stallCount,
		LastTickAt:       global.lastTickAt,
		Probes:           probes,
		RecentStalls:     stalls,
	}
}

func startWithConfig(interval, stallThreshold, cooldown time.Duration) {
	if interval <= 0 {
		interval = defaultInterval
	}
	if stallThreshold <= 0 {
		stallThreshold = defaultStallThreshold
	}
	if cooldown < 0 {
		cooldown = defaultCooldown
	}

	global.lifecycleMu.Lock()
	defer global.lifecycleMu.Unlock()

	global.mu.Lock()
	if global.cancel != nil {
		global.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	global.interval = interval
	global.stallThreshold = stallThreshold
	global.cooldown = cooldown
	global.cancel = cancel
	global.done = done
	global.mu.Unlock()

	go runLoop(ctx, done, interval, stallThreshold, cooldown)
}

func stopLocked() {
	global.mu.Lock()
	if global.cancel == nil {
		global.mu.Unlock()
		return
	}
	cancel := global.cancel
	done := global.done
	global.cancel = nil
	global.done = nil
	global.mu.Unlock()

	cancel()
	<-done
}

func resetForTest() {
	global.lifecycleMu.Lock()
	defer global.lifecycleMu.Unlock()

	stopLocked()

	global.mu.Lock()
	defer global.mu.Unlock()
	global.interval = 0
	global.stallThreshold = 0
	global.cooldown = 0
	global.ticks = 0
	global.stallCount = 0
	global.stallSeq = 0
	global.lastTickAt = time.Time{}
	global.lastCaptureAt = time.Time{}
	global.nextProbeID = 0
	global.probes = make(map[string]probeEntry)
	global.stalls = [maxStallRecords]StallRecord{}
	global.stallHead = 0
	global.stallLen = 0
}

func runLoop(ctx context.Context, done chan struct{}, interval, stallThreshold, cooldown time.Duration) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	prevTick := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			runTick(now, prevTick, interval, stallThreshold, cooldown)
			prevTick = now
		}
	}
}

func runTick(now, prevTick time.Time, interval, stallThreshold, cooldown time.Duration) {
	drift := now.Sub(prevTick) - interval

	calls := snapshotProbeCalls(now)
	results := make([]probeResult, 0, len(calls))
	for _, call := range calls {
		stalled, detail, dur := call.fn()
		results = append(results, probeResult{
			id:       call.id,
			name:     call.name,
			stalled:  stalled,
			detail:   detail,
			duration: dur,
		})
	}
	updateProbeStates(results)

	stalledProbe, stallDetail, stallDur, detected := detectStall(drift, stallThreshold, results)
	if !detected {
		return
	}

	seq, allowed := reserveStallCapture(now, cooldown)
	if !allowed {
		return
	}

	recordStall(now, seq, stalledProbe, stallDetail, stallDur)
}

func snapshotProbeCalls(now time.Time) []probeCall {
	global.mu.Lock()
	defer global.mu.Unlock()

	global.ticks++
	global.lastTickAt = now.UTC()

	calls := make([]probeCall, 0, len(global.probes))
	for _, entry := range global.probes {
		calls = append(calls, probeCall{
			id:   entry.id,
			name: entry.name,
			fn:   entry.fn,
		})
	}
	slices.SortFunc(calls, func(a, b probeCall) int {
		return strings.Compare(a.name, b.name)
	})
	return calls
}

func updateProbeStates(results []probeResult) {
	global.mu.Lock()
	defer global.mu.Unlock()

	for _, res := range results {
		cur, ok := global.probes[res.name]
		if !ok {
			continue
		}
		if cur.id != res.id {
			continue
		}
		stallMS := max(int64(0), res.duration.Milliseconds())
		if !res.stalled {
			stallMS = 0
		}
		cur.state = ProbeState{
			Name:    res.name,
			Stalled: res.stalled,
			Detail:  res.detail,
			StallMS: stallMS,
		}
		global.probes[res.name] = cur
	}
}

func detectStall(drift, stallThreshold time.Duration, results []probeResult) (probeName, detail string, duration time.Duration, detected bool) {
	if drift > stallThreshold {
		var tb textbuf.Buffer
		detail = tb.Reset().Str("scheduler wake-up drift ").Int(drift.Milliseconds()).Str("ms exceeded threshold ").Int(stallThreshold.Milliseconds()).Str("ms").String()
		return schedulerProbeName, detail, drift, true
	}
	for _, res := range results {
		if !res.stalled {
			continue
		}
		d := res.detail
		if d == "" {
			d = "probe reported stall"
		}
		return res.name, d, res.duration, true
	}
	return "", "", 0, false
}

func reserveStallCapture(now time.Time, cooldown time.Duration) (uint64, bool) {
	global.mu.Lock()
	defer global.mu.Unlock()

	if !cooldownElapsedLocked(now, cooldown) {
		return 0, false
	}
	global.lastCaptureAt = now
	global.stallSeq++
	return global.stallSeq, true
}

func cooldownElapsedLocked(now time.Time, cooldown time.Duration) bool {
	if cooldown == 0 {
		return true
	}
	if global.lastCaptureAt.IsZero() {
		return true
	}
	return now.Sub(global.lastCaptureAt) >= cooldown
}

func recordStall(now time.Time, seq uint64, probeName, detail string, stallDur time.Duration) {
	buf := make([]byte, maxStackBytes)
	n := runtime.Stack(buf, true)
	truncated := n == len(buf)
	stacks := string(buf[:n])
	goroutines := runtime.NumGoroutine()

	var traceID string
	if flightrec.CaptureAuto(flightrecTrigger, probeName) {
		traceID = latestWatchdogTraceID(probeName)
	}

	var idBuf textbuf.Buffer
	stallID := idBuf.Reset().Str("stall-").Uint(seq).String()
	stallMS := max(int64(0), stallDur.Milliseconds())

	rec := StallRecord{
		ID:              stallID,
		CapturedAt:      now.UTC(),
		Probe:           probeName,
		Detail:          detail,
		StallMS:         stallMS,
		TraceSnapshotID: traceID,
		GoroutineCount:  goroutines,
		StacksTruncated: truncated,
		Stacks:          stacks,
	}

	global.mu.Lock()
	global.stalls[global.stallHead] = rec
	global.stallHead = (global.stallHead + 1) % maxStallRecords
	if global.stallLen < maxStallRecords {
		global.stallLen++
	}
	global.stallCount++
	global.mu.Unlock()

	var msgBuf textbuf.Buffer
	msgBuf.Reset().Str("daemon liveness stall detected on probe ").Str(probeName)
	if detail != "" {
		msgBuf.Str(": ").Str(detail)
	}
	if stallMS > 0 {
		msgBuf.Str(" (").Int(stallMS).Str("ms)")
	}
	msg := report.FitMessage(msgBuf.String())

	report.RaiseErrorFix(reportSource, CodeDoctorDaemonStallDetected, probeName, msg, reportFix)
	watchdogLog().Error(msg,
		"code", CodeDoctorDaemonStallDetected,
		"probe", probeName,
		"stall-id", stallID,
		"stall-ms", stallMS,
		"goroutines", goroutines,
		"trace-snapshot-id", traceID,
	)
}

func latestWatchdogTraceID(probeName string) string {
	snaps := flightrec.Snapshots()
	for i := range snaps {
		if snaps[i].Trigger != flightrecTrigger {
			continue
		}
		if snaps[i].Detail != probeName {
			continue
		}
		return snaps[i].ID
	}
	return ""
}
