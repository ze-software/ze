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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beevik/ntp"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	ntpevents "github.com/ze-software/ze/internal/plugins/ntp/events"
	"github.com/ze-software/ze/pkg/ze"
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
	ntpQueryFn = ntp.Query
	setClockFn = setClock
)

// ntpConfig holds the parsed NTP configuration.
type ntpConfig struct {
	Enabled         bool
	Servers         []string
	IntervalSec     int // sync interval in seconds (default 3600)
	MaxStepSec      int // max accepted clock step in seconds; 0 means unlimited
	SlewThresholdMs int // max offset in ms for slew (Adjtimex); 0 = always step
	// PersistPath is vestigial/back-compat. The last-known time now persists in the
	// shared zefs store (database.zefs) via internal/core/statestore, not this path;
	// the value no longer designates a file. A non-empty value still enables time
	// persistence (the default), so the YANG leaf stays parsed and validated.
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

// syncWorker is the long-lived NTP sync goroutine.
type syncWorker struct {
	cfg      ntpConfig
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	dhcpSrv  []string    // DHCP-discovered servers (lower priority)
	eventBus ze.EventBus // for emitting clock-synced event
	synced   atomic.Bool // true after first successful NTP sync
	peers    map[string]*serverState
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

// stopAndWait signals the worker to stop and waits for completion.
func (w *syncWorker) stopAndWait() {
	close(w.stop)
	<-w.done
}

func (w *syncWorker) run() {
	defer close(w.done)
	logger := loggerPtr.Load()

	// Publish initial "enabled but not synced" state.
	w.publishState(false, "", 0, 0)

	// Phase 1: restore saved time (rough clock for devices without RTC).
	// A non-empty persist-path enables persistence; the store location is the
	// shared zefs store, not the (vestigial) path value.
	if w.cfg.PersistPath != "" {
		if t, err := loadTime(); err == nil {
			if err := setClock(t); err != nil {
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
		if err := slewClock(best.Offset); err != nil {
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
			logger.Debug("ntp: clock-synced emit failed", "err", err)
		}
	}

	// Write RTC if available.
	if err := setRTC(clockTime); err != nil {
		logger.Debug("ntp: rtc write failed (non-fatal)", "err", err)
	}

	// Persist time to the shared zefs store (persist-path is vestigial: a non-empty
	// value enables persistence, but the location is the shared zefs store).
	if w.cfg.PersistPath != "" {
		if err := saveTime(clockTime); err != nil {
			logger.Debug("ntp: time persistence failed", "err", err)
		}
	}

	return true
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
		Enabled:      true,
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

// handleDHCPEvent processes a DHCP lease event to extract NTP servers.
func (w *syncWorker) handleDHCPEvent(data string) {
	var payload iface.DHCPPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		loggerPtr.Load().Debug("ntp: failed to parse DHCP lease event", "err", err)
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

	if v, ok := ntpMap["enabled"].(string); ok {
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
