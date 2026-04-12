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
	"fmt"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beevik/ntp"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

// ntpConfig holds the parsed NTP configuration.
type ntpConfig struct {
	Enabled     bool
	Servers     []string
	IntervalSec int    // sync interval in seconds (default 3600)
	PersistPath string // path to save time on shutdown
}

// defaultConfig returns an ntpConfig with sensible defaults.
func defaultConfig() ntpConfig {
	return ntpConfig{
		Enabled:     false,
		IntervalSec: 3600,
		PersistPath: "/perm/ze/timefile",
	}
}

// syncWorker is the long-lived NTP sync goroutine.
type syncWorker struct {
	cfg     ntpConfig
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
	dhcpSrv []string // DHCP-discovered servers (lower priority)
}

func newSyncWorker(cfg ntpConfig) *syncWorker {
	return &syncWorker{
		cfg:  cfg,
		stop: make(chan struct{}),
		done: make(chan struct{}),
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

	// Phase 1: restore saved time (rough clock for devices without RTC).
	if w.cfg.PersistPath != "" {
		if t, err := loadTime(w.cfg.PersistPath); err == nil {
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

// doSync queries one NTP server and sets the clock if offset is meaningful.
// Returns true on success.
func (w *syncWorker) doSync(logger *slog.Logger) bool {
	servers := w.servers()
	if len(servers) == 0 {
		logger.Warn("ntp: no servers configured")
		return false
	}

	// Anti-thundering-herd jitter: 0-250ms random delay.
	// RFC 5905 recommends randomizing client requests.
	// Cryptographic randomness is not needed for jitter/load balancing.
	jitter := time.Duration(rand.IntN(250)) * time.Millisecond //nolint:gosec // jitter, not security
	if !w.sleepOrStop(jitter) {
		return false
	}

	// Pick a random server for load distribution.
	server := servers[rand.IntN(len(servers))] //nolint:gosec // load balancing, not security

	resp, err := ntp.Query(server)
	if err != nil {
		logger.Warn("ntp: query failed", "server", server, "err", err)
		return false
	}

	// Validate response: reject absurd timestamps.
	now := resp.Time
	if now.Year() < 2020 || now.Year() > 2100 {
		logger.Warn("ntp: response rejected (absurd timestamp)",
			"server", server, "year", now.Year())
		return false
	}

	// Validate clock offset is reasonable.
	if err := resp.Validate(); err != nil {
		logger.Warn("ntp: response validation failed", "server", server, "err", err)
		return false
	}

	// Set system clock.
	clockTime := time.Now().Add(resp.ClockOffset)
	if err := setClock(clockTime); err != nil {
		logger.Warn("ntp: set clock failed", "server", server, "err", err)
		return false
	}
	logger.Info("ntp: clock synced", "server", server, "offset", resp.ClockOffset)

	// Write RTC if available.
	if err := setRTC(clockTime); err != nil {
		logger.Debug("ntp: rtc write failed (non-fatal)", "err", err)
	}

	// Persist time.
	if w.cfg.PersistPath != "" {
		if err := saveTime(w.cfg.PersistPath, clockTime); err != nil {
			logger.Debug("ntp: time persistence failed", "err", err)
		}
	}

	return true
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
		return fmt.Errorf("empty server address")
	}
	if len(addr) > maxServerAddrLen {
		return fmt.Errorf("server address too long (%d > %d)", len(addr), maxServerAddrLen)
	}
	for _, c := range addr {
		if c < 0x20 || c == 0x7f {
			return fmt.Errorf("server address contains control character")
		}
	}
	return nil
}

// subscribeDHCP sets up the event bus subscription for DHCP lease events.
func subscribeDHCP(eb ze.EventBus, w *syncWorker) func() {
	return eb.Subscribe(plugin.NamespaceInterface, plugin.EventInterfaceDHCPAcquired, w.handleDHCPEvent)
}
