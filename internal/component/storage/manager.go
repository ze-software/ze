// Design: docs/architecture/storage/smart-health.md -- SMART disk health management
// Related: config.go — Config struct for SMART management

package storage

import (
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/smart"
)

// The report detail keys a temperature finding carries.
const (
	keyTempCelsius = "temp-celsius"
	keyThreshold   = "threshold"
)

// DeviceStatus holds the current SMART state for one device.
type DeviceStatus struct {
	Name           string     `json:"name"`
	Transport      string     `json:"transport"`
	Healthy        bool       `json:"healthy"`
	TempCelsius    int        `json:"temp-celsius,omitempty"`
	PowerOnHours   uint64     `json:"power-on-hours,omitempty"`
	ErrorCount     uint64     `json:"error-count"`
	PercentUsed    int        `json:"percent-used,omitempty"`
	AvailableSpare int        `json:"available-spare,omitempty"`
	Unavailable    bool       `json:"unavailable,omitempty"`
	SmartEnabled   bool       `json:"smart-enabled"`
	LastChecked    time.Time  `json:"last-checked"`
	LastShortTest  *time.Time `json:"last-short-test,omitempty"`
	LastLongTest   *time.Time `json:"last-long-test,omitempty"`

	prevTemp       int  // previous reading for rate-of-change detection (not exported)
	healthReported bool // true once smart-failing has been raised (avoids ring flooding)
}

// Manager runs periodic SMART health checks and self-test scheduling.
type Manager struct {
	mu      sync.RWMutex
	config  Config
	devices map[string]*DeviceStatus
	stopCh  chan struct{}
	done    chan struct{}
	running bool
}

// NewManager creates a Manager from the given configuration.
func NewManager(cfg Config) *Manager {
	return &Manager{
		config:  cfg,
		devices: make(map[string]*DeviceStatus),
	}
}

// Start spawns the background health-poll goroutine. No-op if not enabled
// or already running.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled || m.running {
		return
	}
	m.stopCh = make(chan struct{})
	m.done = make(chan struct{})
	m.running = true
	go m.run()
}

// Stop terminates the background goroutine and waits for it to exit.
// Safe to call without Start.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	close(m.stopCh)
	m.running = false
	done := m.done
	m.mu.Unlock()

	<-done
}

// Reconfigure updates the manager's configuration and restarts the ticker.
func (m *Manager) Reconfigure(cfg Config) {
	m.Stop()
	m.mu.Lock()
	m.config = cfg
	m.mu.Unlock()
	m.Start()
}

// Status returns a snapshot of all tracked device statuses.
func (m *Manager) Status() []DeviceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]DeviceStatus, 0, len(m.devices))
	for _, d := range m.devices {
		out = append(out, *d)
	}
	return out
}

func (m *Manager) run() {
	defer close(m.done)

	m.poll()

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.poll()
		case <-m.stopCh:
			report.ClearSource("storage")
			return
		}
	}
}

func (m *Manager) poll() {
	devices := discoverBlockDevices()

	seen := make(map[string]bool, len(devices))
	for _, name := range devices {
		seen[name] = true
		m.checkDevice(name)
	}

	m.mu.Lock()
	for name := range m.devices {
		if !seen[name] {
			delete(m.devices, name)
			report.ClearWarning("storage", "temp-high", name)
			report.ClearWarning("storage", "temp-rising", name)
		}
	}
	m.mu.Unlock()
}

func (m *Manager) checkDevice(name string) {
	info := smart.Detect(name, "")
	if info == nil {
		return
	}
	if info.Unavailable {
		m.mu.Lock()
		if _, exists := m.devices[name]; !exists {
			m.devices[name] = &DeviceStatus{
				Name:        name,
				Transport:   classifyTransport(name),
				Unavailable: true,
				LastChecked: time.Now(),
			}
		}
		m.mu.Unlock()
		return
	}

	// AC-1: auto-enable SMART on detected devices.
	enabled := m.enableOnce(name)

	now := time.Now()

	m.mu.Lock()
	ds, exists := m.devices[name]
	if !exists {
		ds = &DeviceStatus{
			Name:      name,
			Transport: classifyTransport(name),
		}
		m.devices[name] = ds
	}
	ds.Healthy = info.Healthy
	ds.TempCelsius = info.TempCelsius
	ds.PowerOnHours = info.PowerOnHours
	ds.ErrorCount = info.ErrorCount
	ds.PercentUsed = info.PercentUsed
	ds.AvailableSpare = info.AvailableSpare
	ds.SmartEnabled = enabled
	ds.Unavailable = false
	ds.LastChecked = now
	m.mu.Unlock()

	m.checkTemperature(name, ds, info)
	m.checkHealth(name, ds, info)
	m.checkSelfTest(name, now)
}

func (m *Manager) enableOnce(name string) bool {
	m.mu.RLock()
	ds, exists := m.devices[name]
	m.mu.RUnlock()

	if exists {
		return ds.SmartEnabled
	}
	if err := smart.Enable(name); err != nil {
		slogutil.Logger("storage").Warn("SMART enable failed", "device", name, "error", err)
		return false
	}
	return true
}

func (m *Manager) checkTemperature(name string, ds *DeviceStatus, info *smart.Info) {
	if info.TempCelsius == 0 {
		return
	}

	if ds.prevTemp > 0 && m.config.Temperature.Difference > 0 {
		delta := info.TempCelsius - ds.prevTemp
		if delta >= m.config.Temperature.Difference {
			report.RaiseWarning("storage", "temp-rising", name,
				"disk temperature rising rapidly",
				map[string]any{keyTempCelsius: info.TempCelsius, "delta": delta, keyThreshold: m.config.Temperature.Difference})
		} else {
			report.ClearWarning("storage", "temp-rising", name)
		}
	}
	ds.prevTemp = info.TempCelsius

	switch {
	case info.TempCelsius >= m.config.Temperature.Critical:
		report.ClearWarning("storage", "temp-high", name)
		report.RaiseError("storage", "temp-critical", name,
			"disk temperature critical",
			map[string]any{keyTempCelsius: info.TempCelsius, keyThreshold: m.config.Temperature.Critical})
	case info.TempCelsius >= m.config.Temperature.Informational:
		report.RaiseWarning("storage", "temp-high", name,
			"disk temperature above informational threshold",
			map[string]any{keyTempCelsius: info.TempCelsius, keyThreshold: m.config.Temperature.Informational})
	default:
		report.ClearWarning("storage", "temp-high", name)
	}
}

func (m *Manager) checkHealth(name string, ds *DeviceStatus, info *smart.Info) {
	if !info.Healthy {
		if !ds.healthReported {
			report.RaiseError("storage", "smart-failing", name,
				"SMART health status: FAILING")
			ds.healthReported = true
		}
	} else {
		ds.healthReported = false
	}
}

func (m *Manager) checkSelfTest(name string, now time.Time) {
	if smart.IsSelfTestInProgress(name) {
		return
	}

	m.mu.RLock()
	ds := m.devices[name]
	m.mu.RUnlock()

	if m.config.SelfTest.Short.Interval > 0 {
		if ds.LastShortTest == nil || now.Sub(*ds.LastShortTest) >= m.config.SelfTest.Short.Interval {
			if pastTimeOfDay(now, m.config.SelfTest.Short.TimeOfDay) {
				if err := smart.StartSelfTest(name, smart.SelfTestShort); err == nil {
					m.mu.Lock()
					t := now
					ds.LastShortTest = &t
					m.mu.Unlock()
				}
			}
		}
	}

	if m.config.SelfTest.Long.Interval > 0 {
		if ds.LastLongTest == nil || now.Sub(*ds.LastLongTest) >= m.config.SelfTest.Long.Interval {
			if matchesDay(now, m.config.SelfTest.Long.Day) && pastTimeOfDay(now, m.config.SelfTest.Long.TimeOfDay) {
				if err := smart.StartSelfTest(name, smart.SelfTestExtended); err == nil {
					m.mu.Lock()
					t := now
					ds.LastLongTest = &t
					m.mu.Unlock()
				}
			}
		}
	}
}

var validDays = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// pastTimeOfDay returns true if the current time is at or past the
// configured HH:MM. Empty or malformed timeOfDay always matches
// (fail-open: do not silently block scheduling).
func pastTimeOfDay(now time.Time, timeOfDay string) bool {
	if timeOfDay == "" {
		return true
	}
	if len(timeOfDay) != 5 || timeOfDay[2] != ':' {
		return true
	}
	if !isDigit(timeOfDay[0]) || !isDigit(timeOfDay[1]) || !isDigit(timeOfDay[3]) || !isDigit(timeOfDay[4]) {
		return true
	}
	hh := int(timeOfDay[0]-'0')*10 + int(timeOfDay[1]-'0')
	mm := int(timeOfDay[3]-'0')*10 + int(timeOfDay[4]-'0')
	if hh > 23 || mm > 59 {
		return true
	}
	h, m, _ := now.Clock()
	return h > hh || (h == hh && m >= mm)
}

// matchesDay returns true if the current weekday matches the configured
// day name. Empty or unrecognized day always matches
// (fail-open: do not silently block scheduling).
func matchesDay(now time.Time, day string) bool {
	if day == "" {
		return true
	}
	wd, ok := validDays[strings.ToLower(day)]
	if !ok {
		return true
	}
	return now.Weekday() == wd
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func classifyTransport(name string) string {
	if strings.HasPrefix(name, "nvme") {
		return "nvme"
	}
	if strings.HasPrefix(name, "sd") {
		return "sata"
	}
	return "unknown"
}
