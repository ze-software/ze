// Design: docs/features/interfaces.md -- NTP client plugin

// Package ntp implements a lightweight NTP client plugin for ze.
// It queries configured NTP servers, sets the system clock via
// Settimeofday, writes to the hardware RTC when available, and
// persists time to disk for recovery on devices without RTC.
//
// The plugin subscribes to DHCP lease events to discover NTP servers
// via option 42. Configured servers take priority over DHCP-discovered ones.
package ntp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beevik/ntp"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/report"
	ntpevents "github.com/ze-software/ze/internal/plugins/ntp/events"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	fixShowDoctor         = "ze show doctor"
	fixShowSystemNTP      = "ze show system ntp"
	subjectClockSynced    = "clock-synced"
	subjectRTCDevice      = "/dev/rtc0"
	subjectLastTime       = "last-time"
	subjectDHCPLeaseEvent = "dhcp-lease-event"

	dhcpEventWarnInterval = time.Minute
	minRaiseRetryWindow   = time.Minute
)

var (
	errEmptyServerAddress                    = errors.New("empty server address")
	errServerAddressContainsControlCharacter = errors.New("server address contains control character")
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

// Test seams: ntpQueryFn is the per-server NTP query and setClockFn is the
// platform clock setter. They are variables (not direct calls) so unit tests
// can drive doSync/stopAndWait without real network I/O or a privileged
// Settimeofday. Matches the radiusAdminProbe seam convention in the radius
// component. Production code leaves them at their real implementations.
var (
	ntpQueryFn  = ntp.Query
	setClockFn  = setClock
	slewClockFn = slewClock
	setRTCFn    = setRTC
)

// ntpConfig holds the parsed NTP configuration.
type ntpConfig struct {
	Enabled         bool
	Servers         []string
	IntervalSec     int // sync interval in seconds (default 3600)
	MaxStepSec      int // max accepted clock step in seconds; 0 means unlimited
	SlewThresholdMs int // max offset in ms for slew (Adjtimex); 0 = always step
	// PersistPath is vestigial/back-compat. The last-known time now persists in the
	// daemon's managed store (database/) via the plugin SDK state RPCs
	// (p.StateGet/p.StatePut), not this path; the value no longer designates a
	// file. A non-empty value still enables time persistence (the default), so
	// the YANG leaf stays parsed and validated.
	// TODO: consider deprecating the persist-path YANG leaf.
	PersistPath string
}

// defaultConfig returns an ntpConfig with sensible defaults.
func defaultConfig() ntpConfig {
	return ntpConfig{
		Enabled:         false,
		IntervalSec:     3600,
		MaxStepSec:      3600,
		SlewThresholdMs: 128,
		PersistPath:     "/perm/ze/timefile",
	}
}

// ntpWarningRun tracks a run of consecutive failures for one persistent NTP
// warning so the worker warns once per run, retries refused raises via
// issuequeue.RunRaises, and clears on recovery. Owned by syncWorker.doSync.
type ntpWarningRun struct {
	failures uint64
	raises   *issuequeue.RunRaises
}

// syncWorker is the long-lived NTP sync goroutine.
type syncWorker struct {
	cfg             ntpConfig
	stop            chan struct{}
	stopOnce        sync.Once // requestStop closes stop once
	done            chan struct{}
	ready           <-chan struct{}   // closed on SDK OnStarted; nil in direct unit tests
	state           stateClient       // daemon state RPC client (*sdk.Plugin); nil in unit tests without state
	issues          *issuequeue.Queue // report-bus queue over *sdk.Plugin; nil in unit tests without publisher
	mu              sync.Mutex
	dhcpSrv         []string    // DHCP-discovered servers (lower priority)
	dhcpEventWarned time.Time   // last rate-limited malformed DHCP event WARN/raise (guarded by mu)
	eventBus        ze.EventBus // for emitting clock-synced event
	synced          atomic.Bool // true after first successful NTP sync
	peers           map[string]*serverState
	rtcWarn         ntpWarningRun
	persistWarn     ntpWarningRun
}

// lastErrorUnsynchronized is the last-error of a server whose reply carries
// the leap indicator alarm: the server has no time to give, so the operator
// fixes the server's own time source or names another server.
const lastErrorUnsynchronized = "server is not synchronized (leap indicator alarm): it has no time source of its own, so its time is not used; fix the time source on that server, or add another server under environment ntp server"

// serverUnsynchronized reports a reply from a server that is not synchronized
// itself. beevik/ntp Response.Validate tests stratum 0 before the leap
// indicator and answers "kiss of death received" for such a reply, which
// sends the operator after a refusal that never happened. A stratum 0 reply
// that carries a printable kiss code is a real kiss-o'-death packet, and
// Validate keeps naming it.
//
// RFC 5905 Section 7.3: "3     | unknown (clock unsynchronized)".
// RFC 5905 Section 7.3: "For packet stratum 0 (unspecified or invalid), this
// is a four-character ASCII [RFC1345] string, called the "kiss code",".
func serverUnsynchronized(resp *ntp.Response) bool {
	if resp.Leap != ntp.LeapNotInSync {
		return false
	}
	if resp.Stratum != 0 {
		return true
	}
	return resp.KissCode == ""
}

func newSyncWorker(cfg ntpConfig, eb ze.EventBus) *syncWorker {
	return &syncWorker{
		cfg:      cfg,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		eventBus: eb,
		peers:    make(map[string]*serverState),
	}
}

// start begins the NTP sync loop in a background goroutine.
func (w *syncWorker) start() {
	go w.run()
}

// requestStop signals the worker to stop and does not wait for it. The worker
// ends within one in-flight ntp.Query (about 5 seconds). It is safe to call
// more than once; the caller MUST call stopAndWait afterwards to join the
// worker.
func (w *syncWorker) requestStop() {
	w.stopOnce.Do(func() { close(w.stop) })
}

// stopAndWait signals the worker to stop and waits for completion. It MAY
// follow requestStop: the signal is sent once.
func (w *syncWorker) stopAndWait() {
	w.requestStop()
	<-w.done
}

func (w *syncWorker) run() {
	defer close(w.done)
	logger := loggerPtr.Load()
	// Clear standing RTC/persistence warnings before closing w.done so
	// stopAndWait() returns only after clears are queued (before issues.Stop()
	// drains them), preventing stale warnings on disable or worker reload.
	defer func() {
		w.clearRTCFailure(logger)
		w.clearPersistFailure(logger)
	}()

	// Publish initial "enabled but not synced" state.
	w.publishState(false, "", 0, 0)

	// SDK state RPCs MUST begin no earlier than OnStarted (Stage 5 complete).
	// Waiting here keeps Stage-2 configure fast and prevents state-get RPCs
	// from racing the startup handshake.
	if w.ready != nil {
		select {
		case <-w.ready:
		case <-w.stop:
			return
		}
	}

	// Phase 1: restore saved time (rough clock for devices without RTC).
	// A non-empty persist-path enables persistence; the store location is the
	// daemon's managed zefs store, not the (vestigial) path value.
	if w.cfg.PersistPath != "" {
		if t, err := loadTime(context.Background(), w.state); err == nil {
			if err := setClock(t); err != nil { //nolint:staticcheck // SA4023 holds only on darwin: the !linux stub always errors, the linux implementation can return nil
				logger.Warn("ntp: restore clock failed", "err", err)
			} else {
				logger.Info("ntp: clock restored from saved time", "time", t)
			}
		}
	}

	// Phase 2: initial sync (retry every 1s until success).
	for {
		if w.isStopped() {
			return
		}
		if w.doSync(logger) {
			break
		}
		if !w.sleepOrStop(time.Second) {
			return
		}
	}

	// Phase 3: periodic sync at configured interval.
	interval := time.Duration(w.cfg.IntervalSec) * time.Second
	for {
		if !w.sleepOrStop(interval) {
			return
		}
		w.doSync(logger)
	}
}

// doSync queries all NTP servers, updates per-server state, selects the
// best server, and adjusts the clock. Returns true if the clock was set.
func (w *syncWorker) doSync(logger *slog.Logger) bool {
	servers := w.servers()
	if len(servers) == 0 {
		logger.Warn("ntp: no servers configured")
		return false
	}

	// Anti-thundering-herd jitter: 0-250ms random delay.
	// RFC 5905 recommends randomizing client requests.
	jitter := time.Duration(rand.IntN(250)) * time.Millisecond //nolint:gosec // jitter, not security
	if !w.sleepOrStop(jitter) {
		return false
	}

	// Query all servers and update per-server state. Check the stop channel
	// before each server so a reload or shutdown that lands mid-sync waits out
	// at most one in-flight query (~one query timeout), not
	// len(servers) x timeout. ntp.Query has no context/cancel, so stop-checks
	// between the serial per-server queries are the bound (see startup-resilience
	// spec, FIX 1). Returning false here is the normal "did not sync" signal:
	// phase 2 then exits via sleepOrStop(stop) and phase 3 ignores the result.
	now := time.Now()
	for _, addr := range servers {
		if w.isStopped() {
			return false
		}
		ps := w.getOrCreatePeer(addr)
		ps.LastQuery = now

		resp, err := ntpQueryFn(addr)
		if err != nil {
			ps.Reach = reachShift(ps.Reach, false)
			ps.LastError = err.Error()
			logger.Warn("ntp: query failed", "server", addr, "err", err)
			continue
		}

		if resp.Time.Year() < 2020 || resp.Time.Year() > 2100 {
			ps.Reach = reachShift(ps.Reach, false)
			ps.LastError = "absurd timestamp year " + strconv.Itoa(resp.Time.Year())
			logger.Warn("ntp: response rejected (absurd timestamp)", "server", addr, "year", resp.Time.Year())
			continue
		}

		// RFC 5905 Section 7.3: the leap indicator alarm is checked before
		// Validate, which tests stratum 0 first and names this reply a kiss
		// of death.
		if serverUnsynchronized(resp) {
			ps.Reach = reachShift(ps.Reach, false)
			ps.LastError = lastErrorUnsynchronized
			logger.Warn("ntp: server is not synchronized", "server", addr, "stratum", resp.Stratum)
			continue
		}

		if err := resp.Validate(); err != nil {
			ps.Reach = reachShift(ps.Reach, false)
			ps.LastError = err.Error()
			logger.Warn("ntp: response validation failed", "server", addr, "err", err)
			continue
		}

		ps.Reach = reachShift(ps.Reach, true)
		ps.Offset = resp.ClockOffset
		ps.RTT = resp.RTT
		ps.Stratum = resp.Stratum
		ps.RootDelay = resp.RootDelay
		ps.RootDispersion = resp.RootDispersion
		ps.LastSuccess = now
		ps.LastError = ""
	}

	// Select best server and adjust clock.
	peerSlice := w.peerSlice()
	best := selectBestServer(peerSlice)
	if best == nil {
		w.publishState(false, "", 0, 0)
		logger.Warn("ntp: no reachable servers")
		return false
	}

	action := decideClockAction(best.Offset, w.cfg.SlewThresholdMs, w.cfg.MaxStepSec)
	if action == actionReject {
		logger.Warn("ntp: offset exceeds max-step",
			"server", best.Address, "offset", best.Offset, "max-step", w.cfg.MaxStepSec)
		w.publishState(false, best.Address, 0, best.Stratum)
		return false
	}

	clockTime := time.Now().Add(best.Offset)
	if action == actionSlew {
		if err := slewClockFn(best.Offset); err != nil {
			logger.Warn("ntp: slew failed, falling back to step", "server", best.Address, "err", err)
			if err := setClockFn(clockTime); err != nil {
				logger.Warn("ntp: set clock failed", "server", best.Address, "err", err)
				return false
			}
			logger.Info("ntp: clock stepped (slew fallback)", "server", best.Address, "offset", best.Offset)
		} else {
			logger.Info("ntp: clock slewed", "server", best.Address, "offset", best.Offset)
		}
	} else {
		if err := setClockFn(clockTime); err != nil {
			logger.Warn("ntp: set clock failed", "server", best.Address, "err", err)
			return false
		}
		logger.Info("ntp: clock stepped", "server", best.Address, "offset", best.Offset)
	}

	w.publishState(true, best.Address, best.Offset, best.Stratum)

	// Emit clock-synced event once after first successful NTP sync.
	if w.synced.CompareAndSwap(false, true) && w.eventBus != nil {
		if _, err := w.eventBus.Emit(ntpevents.Namespace, ntpevents.EventClockSynced, ""); err != nil {
			msg := report.FitMessage("NTP synchronized the system clock, but could not emit the clock-synced event on the event bus; downstream plugins waiting for initial clock synchronization may not learn that the clock is ready. Cause: " + err.Error())
			failed := func(rErr error) {
				logger.Warn("NTP could not deliver the clock-synced emit failure report to the daemon; check daemon health",
					"code", codeDoctorNTPClockSyncedEmitFailed, "error", rErr, "fix", fixShowDoctor)
			}
			qErr := w.issues.RaiseError(codeDoctorNTPClockSyncedEmitFailed, subjectClockSynced, msg, fixShowDoctor, failed)
			logger.Warn("NTP synchronized the system clock, but could not emit the clock-synced event; downstream plugins waiting for clock readiness may not be notified",
				"code", codeDoctorNTPClockSyncedEmitFailed, "error", err, "fix", fixShowDoctor, issuequeue.Outcome(qErr))
		}
	}

	// Write RTC if available.
	if err := setRTCFn(clockTime); err != nil {
		if !isUnsupportedRTCError(err) {
			w.observeRTCFailure(logger, err)
		}
	} else {
		w.clearRTCFailure(logger)
	}

	// Persist time to the daemon's managed zefs store via the SDK state RPC
	// (persist-path is vestigial: a non-empty value enables persistence, but
	// the location is the managed zefs store).
	if w.cfg.PersistPath != "" {
		if err := saveTime(context.Background(), w.state, clockTime); err != nil {
			w.observePersistFailure(logger, err)
		} else {
			w.clearPersistFailure(logger)
		}
	}

	return true
}

// isUnsupportedRTCError reports whether err came from the non-Linux setRTC stub
// in clock_other.go, which is an expected platform absence rather than a device
// write failure.
func isUnsupportedRTCError(err error) bool {
	return strings.HasPrefix(err.Error(), "ntp: rtc not supported on ")
}

func (w *syncWorker) raiseRetryWindow() time.Duration {
	return max(time.Duration(w.cfg.IntervalSec)*time.Second, minRaiseRetryWindow)
}

func (w *syncWorker) observeRTCFailure(logger *slog.Logger, cause error) {
	now := time.Now()
	w.rtcWarn.failures++
	msg := report.FitMessage("NTP synchronized the system clock, but writing the synced time to /dev/rtc0 failed; hardware clock drift will persist across reboots until the RTC device is writable. Cause: " + cause.Error())
	if w.rtcWarn.failures == 1 {
		w.rtcWarn.raises = &issuequeue.RunRaises{}
		qErr := w.raiseRTCWarning(logger, w.rtcWarn.raises, now, msg)
		logger.Warn("NTP could not update the hardware RTC; hardware clock drift will persist across reboots until /dev/rtc0 is writable",
			"code", codeDoctorNTPRTCWriteFailed, "error", cause, "fix", fixShowDoctor, issuequeue.Outcome(qErr))
		return
	}
	if !w.rtcWarn.raises.RetryDue(now, w.raiseRetryWindow()) {
		return
	}
	if qErr := w.raiseRTCWarning(logger, w.rtcWarn.raises, now, msg); qErr != nil {
		logger.Warn("NTP could not re-queue the RTC write warning for the daemon; check daemon health",
			"code", codeDoctorNTPRTCWriteFailed, "fix", fixShowDoctor, issuequeue.Outcome(qErr))
	}
}

func (w *syncWorker) raiseRTCWarning(logger *slog.Logger, raises *issuequeue.RunRaises, now time.Time, message string) error {
	refused := raises.Offer(now)
	failed := func(rErr error) {
		refused(rErr)
		logger.Warn("NTP could not deliver the RTC write warning to the daemon; check daemon health",
			"code", codeDoctorNTPRTCWriteFailed, "error", rErr, "fix", fixShowDoctor)
	}
	qErr := w.issues.RaiseWarning(codeDoctorNTPRTCWriteFailed, subjectRTCDevice, message, fixShowDoctor, failed)
	if qErr != nil {
		refused(qErr)
	}
	return qErr
}

func (w *syncWorker) clearRTCFailure(logger *slog.Logger) {
	if w.rtcWarn.failures == 0 {
		return
	}
	w.rtcWarn.failures = 0
	w.rtcWarn.raises = nil
	failed := func(rErr error) {
		logger.Warn("NTP could not clear the RTC write warning on the daemon; check daemon health",
			"code", codeDoctorNTPRTCWriteFailed, "error", rErr, "fix", fixShowDoctor)
	}
	if cErr := w.issues.ClearWarning(codeDoctorNTPRTCWriteFailed, subjectRTCDevice, failed); cErr != nil {
		logger.Warn("NTP could not queue the RTC write warning clear; stale warning may persist",
			"code", codeDoctorNTPRTCWriteFailed, "fix", fixShowDoctor, issuequeue.Outcome(cErr))
	}
}

func (w *syncWorker) observePersistFailure(logger *slog.Logger, cause error) {
	now := time.Now()
	w.persistWarn.failures++
	msg := report.FitMessage("NTP synchronized the system clock, but writing the last-known timestamp to the daemon state store failed; devices without a battery-backed RTC will boot with a stale timestamp. Cause: " + cause.Error())
	if w.persistWarn.failures == 1 {
		w.persistWarn.raises = &issuequeue.RunRaises{}
		qErr := w.raisePersistWarning(logger, w.persistWarn.raises, now, msg)
		logger.Warn("NTP could not persist the last-known clock time; devices without a battery-backed RTC will boot with a stale timestamp",
			"code", codeDoctorNTPTimePersistFailed, "error", cause, "fix", fixShowDoctor, issuequeue.Outcome(qErr))
		return
	}
	if !w.persistWarn.raises.RetryDue(now, w.raiseRetryWindow()) {
		return
	}
	if qErr := w.raisePersistWarning(logger, w.persistWarn.raises, now, msg); qErr != nil {
		logger.Warn("NTP could not re-queue the time-persistence warning for the daemon; check daemon health",
			"code", codeDoctorNTPTimePersistFailed, "fix", fixShowDoctor, issuequeue.Outcome(qErr))
	}
}

func (w *syncWorker) raisePersistWarning(logger *slog.Logger, raises *issuequeue.RunRaises, now time.Time, message string) error {
	refused := raises.Offer(now)
	failed := func(rErr error) {
		refused(rErr)
		logger.Warn("NTP could not deliver the time-persistence warning to the daemon; check daemon health",
			"code", codeDoctorNTPTimePersistFailed, "error", rErr, "fix", fixShowDoctor)
	}
	qErr := w.issues.RaiseWarning(codeDoctorNTPTimePersistFailed, subjectLastTime, message, fixShowDoctor, failed)
	if qErr != nil {
		refused(qErr)
	}
	return qErr
}

func (w *syncWorker) clearPersistFailure(logger *slog.Logger) {
	if w.persistWarn.failures == 0 {
		return
	}
	w.persistWarn.failures = 0
	w.persistWarn.raises = nil
	failed := func(rErr error) {
		logger.Warn("NTP could not clear the time-persistence warning on the daemon; check daemon health",
			"code", codeDoctorNTPTimePersistFailed, "error", rErr, "fix", fixShowDoctor)
	}
	if cErr := w.issues.ClearWarning(codeDoctorNTPTimePersistFailed, subjectLastTime, failed); cErr != nil {
		logger.Warn("NTP could not queue the time-persistence warning clear; stale warning may persist",
			"code", codeDoctorNTPTimePersistFailed, "fix", fixShowDoctor, issuequeue.Outcome(cErr))
	}
}

// getOrCreatePeer returns the serverState for addr, creating it if needed.
func (w *syncWorker) getOrCreatePeer(addr string) *serverState {
	if ps, ok := w.peers[addr]; ok {
		return ps
	}
	ps := &serverState{Address: addr}
	w.peers[addr] = ps
	return ps
}

// peerSlice returns a snapshot of all peer states as a slice.
func (w *syncWorker) peerSlice() []serverState {
	out := make([]serverState, 0, len(w.peers))
	for _, ps := range w.peers {
		out = append(out, *ps)
	}
	return out
}

// publishState stores a syncState snapshot for the show handlers.
func (w *syncWorker) publishState(synced bool, source string, offset time.Duration, stratum uint8) {
	st := &syncState{
		Enabled:      w.cfg.Enabled,
		Synced:       synced,
		Source:       source,
		Offset:       offset,
		Stratum:      stratum,
		PollInterval: w.cfg.IntervalSec,
		Servers:      w.peerSlice(),
	}
	if synced {
		st.LastSync = time.Now()
	}
	storeState(st)
}

func clockOffsetAllowed(offset, maxStep time.Duration) bool {
	if maxStep <= 0 {
		return true
	}
	if offset < 0 {
		offset = -offset
	}
	return offset <= maxStep
}

// clockAction represents the clock adjustment decision.
type clockAction int

const (
	actionReject clockAction = iota
	actionSlew
	actionStep
)

// decideClockAction returns the appropriate action for the given offset.
func decideClockAction(offset time.Duration, slewThresholdMs, maxStepSec int) clockAction {
	absOff := offset
	if absOff < 0 {
		absOff = -absOff
	}
	maxStep := time.Duration(maxStepSec) * time.Second
	if maxStep > 0 && absOff > maxStep {
		return actionReject
	}
	if slewThresholdMs > 0 && absOff <= time.Duration(slewThresholdMs)*time.Millisecond {
		return actionSlew
	}
	return actionStep
}

// servers returns the effective server list: configured servers first,
// then DHCP-discovered servers as fallback.
func (w *syncWorker) servers() []string {
	if len(w.cfg.Servers) > 0 {
		return w.cfg.Servers
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dhcpSrv
}

// addDHCPServers adds NTP servers discovered from a DHCP lease.
func (w *syncWorker) addDHCPServers(servers []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.dhcpSrv = servers
}

// isStopped checks whether the stop channel has been closed.
// Non-blocking check used in loop conditions, not a silent ignore.
func (w *syncWorker) isStopped() bool {
	select {
	case <-w.stop:
		return true
	default: // non-blocking check, not a silent ignore
		return false
	}
}

func (w *syncWorker) sleepOrStop(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-w.stop:
		return false
	}
}

func (w *syncWorker) dhcpEventWarnDue() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if !w.dhcpEventWarned.IsZero() {
		if now.Sub(w.dhcpEventWarned) < dhcpEventWarnInterval {
			return false
		}
	}
	w.dhcpEventWarned = now
	return true
}

// handleDHCPEvent processes a DHCP lease event to extract NTP servers.
func (w *syncWorker) handleDHCPEvent(data string) {
	var payload iface.DHCPPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		if !w.dhcpEventWarnDue() {
			return
		}
		log := loggerPtr.Load()
		msg := report.FitMessage("NTP could not decode a DHCP lease event payload on the event bus; NTP servers advertised via DHCP option 42 in that event were ignored. Cause: " + err.Error())
		failed := func(rErr error) {
			log.Warn("NTP could not deliver the malformed DHCP-event report to the daemon; check daemon health",
				"code", codeDoctorNTPDHCPEventMalformed, "error", rErr, "fix", fixShowSystemNTP)
		}
		qErr := w.issues.RaiseError(codeDoctorNTPDHCPEventMalformed, subjectDHCPLeaseEvent, msg, fixShowSystemNTP, failed)
		log.Warn("NTP could not decode a DHCP lease event payload; DHCP option 42 NTP servers in that event were ignored",
			"code", codeDoctorNTPDHCPEventMalformed, "error", err, "fix", fixShowSystemNTP, issuequeue.Outcome(qErr))
		return
	}
	if len(payload.NTPServers) > 0 {
		loggerPtr.Load().Info("ntp: received NTP servers from DHCP", "servers", payload.NTPServers)
		w.addDHCPServers(payload.NTPServers)
	}
}

// parseNTPConfig extracts NTP configuration from the environment config section.
func parseNTPConfig(data string) (ntpConfig, error) {
	cfg := defaultConfig()

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("ntp config: unmarshal: %w", err)
	}

	envMap, ok := root["environment"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	ntpMap, ok := envMap["ntp"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	if v, ok := ntpMap[fieldEnabled].(string); ok {
		cfg.Enabled = v == "true"
	}
	if v, ok := ntpMap["interval"].(string); ok {
		var sec int
		if _, err := fmt.Sscanf(v, "%d", &sec); err == nil && sec >= 60 && sec <= 86400 {
			cfg.IntervalSec = sec
		}
	}
	if v, ok := ntpMap["max-step"].(string); ok {
		var sec int
		if _, err := fmt.Sscanf(v, "%d", &sec); err != nil {
			return cfg, fmt.Errorf("ntp config: max-step: %w", err)
		}
		if sec < 0 || sec > 86400 {
			return cfg, fmt.Errorf("ntp config: max-step %d out of range 0..86400", sec)
		}
		cfg.MaxStepSec = sec
	}
	if v, ok := ntpMap["slew-threshold"].(string); ok {
		ms, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("ntp config: slew-threshold: %w", err)
		}
		if ms < 0 || ms > 1000 {
			return cfg, fmt.Errorf("ntp config: slew-threshold %d out of range 0..1000", ms)
		}
		cfg.SlewThresholdMs = ms
	}
	if v, ok := ntpMap["persist-path"].(string); ok && v != "" {
		if err := validatePersistPath(v); err != nil {
			return cfg, fmt.Errorf("ntp config: persist-path: %w", err)
		}
		cfg.PersistPath = v
	}

	if serverMap, ok := ntpMap["server"].(map[string]any); ok {
		for name, sv := range serverMap {
			sm, _ := sv.(map[string]any)
			if sm == nil {
				continue
			}
			if addr, ok := sm["address"].(string); ok && addr != "" {
				if err := validateServerAddress(addr); err != nil {
					return cfg, fmt.Errorf("ntp config: server %q: %w", name, err)
				}
				cfg.Servers = append(cfg.Servers, addr)
			}
		}
	}

	return cfg, nil
}

// validatePersistPath rejects path traversal and non-absolute paths.
func validatePersistPath(path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("must be absolute path, got %q", path)
	}
	cleaned := filepath.Clean(path)
	if cleaned != path {
		return fmt.Errorf("path contains traversal or redundant separators: %q", path)
	}
	return nil
}

// validateServerAddress rejects obviously invalid server addresses.
// Accepts hostnames and IPs; rejects empty, overly long, and control chars.
const maxServerAddrLen = 253 // max DNS hostname length

func validateServerAddress(addr string) error {
	if addr == "" {
		return errEmptyServerAddress
	}
	if len(addr) > maxServerAddrLen {
		return fmt.Errorf("server address too long (%d > %d)", len(addr), maxServerAddrLen)
	}
	for _, c := range addr {
		if c < 0x20 || c == 0x7f {
			return errServerAddressContainsControlCharacter
		}
	}
	return nil
}

// subscribeDHCP sets up the event bus subscription for DHCP lease events.
func subscribeDHCP(eb ze.EventBus, w *syncWorker) func() {
	return eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDHCPAcquired, events.AsString(w.handleDHCPEvent))
}
