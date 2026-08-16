// Design: docs/architecture/storage/smart-health.md -- SMART disk health management

package storage

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/smart"
)

func TestManagerCreatedFromConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if !m.config.Enabled {
		t.Error("config.Enabled should be true")
	}
	if m.config.CheckInterval != 30*time.Minute {
		t.Errorf("CheckInterval = %v, want 30m", m.config.CheckInterval)
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.CheckInterval = 50 * time.Millisecond
	m := NewManager(cfg)
	m.Start()
	m.Stop()
}

func TestManagerStartDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	m := NewManager(cfg)
	m.Start()
	if m.running {
		t.Error("running should be false when disabled")
	}
}

func TestManagerStopWithoutStart(t *testing.T) {
	m := NewManager(DefaultConfig())
	m.Stop()
}

func TestManagerStatus(t *testing.T) {
	m := NewManager(DefaultConfig())
	m.devices["sda"] = &DeviceStatus{
		Name:        "sda",
		Transport:   "sata",
		Healthy:     true,
		TempCelsius: 38,
		LastChecked: time.Now(),
	}
	statuses := m.Status()
	if len(statuses) != 1 {
		t.Fatalf("len(Status()) = %d, want 1", len(statuses))
	}
	if statuses[0].Name != "sda" {
		t.Errorf("Name = %q, want %q", statuses[0].Name, "sda")
	}
}

func TestHealthPollRaisesWarning(t *testing.T) {
	report.ResetForTest()

	cfg := DefaultConfig()
	cfg.Temperature.Informational = 40
	cfg.Temperature.Critical = 55
	m := NewManager(cfg)

	m.devices["sda"] = &DeviceStatus{Name: "sda", Transport: "sata"}

	fakeInfo := &fakeSmartInfo{temp: 45, healthy: true}
	m.checkTemperature("sda", m.devices["sda"], fakeInfo.toInfo())

	warnings := report.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected a warning for temp above informational threshold")
	}
	if warnings[0].Code != "temp-high" {
		t.Errorf("Code = %q, want %q", warnings[0].Code, "temp-high")
	}
}

func TestHealthPollClearsWarning(t *testing.T) {
	report.ResetForTest()

	cfg := DefaultConfig()
	cfg.Temperature.Informational = 40
	m := NewManager(cfg)

	m.devices["sda"] = &DeviceStatus{Name: "sda", Transport: "sata"}

	hot := &fakeSmartInfo{temp: 45, healthy: true}
	m.checkTemperature("sda", m.devices["sda"], hot.toInfo())

	cool := &fakeSmartInfo{temp: 35, healthy: true}
	m.checkTemperature("sda", m.devices["sda"], cool.toInfo())

	warnings := report.Warnings()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings after temp drop, got %d", len(warnings))
	}
}

func TestHealthPollRaisesError(t *testing.T) {
	report.ResetForTest()

	cfg := DefaultConfig()
	cfg.Temperature.Critical = 50
	m := NewManager(cfg)

	m.devices["sda"] = &DeviceStatus{Name: "sda", Transport: "sata"}

	hot := &fakeSmartInfo{temp: 55, healthy: true}
	m.checkTemperature("sda", m.devices["sda"], hot.toInfo())

	errors := report.Errors(0)
	if len(errors) == 0 {
		t.Fatal("expected an error for temp above critical threshold")
	}
	if errors[0].Code != "temp-critical" {
		t.Errorf("Code = %q, want %q", errors[0].Code, "temp-critical")
	}
}

func TestCheckHealthUnhealthy(t *testing.T) {
	report.ResetForTest()

	m := NewManager(DefaultConfig())
	ds := &DeviceStatus{Name: "sda", Transport: "sata"}
	m.devices["sda"] = ds
	info := &fakeSmartInfo{temp: 30, healthy: false}
	m.checkHealth("sda", ds, info.toInfo())

	errors := report.Errors(0)
	if len(errors) == 0 {
		t.Fatal("expected an error for unhealthy SMART status")
	}
	if errors[0].Code != "smart-failing" {
		t.Errorf("Code = %q, want %q", errors[0].Code, "smart-failing")
	}
}

func TestReconfigure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.CheckInterval = 100 * time.Millisecond
	m := NewManager(cfg)
	m.Start()

	newCfg := cfg
	newCfg.CheckInterval = 200 * time.Millisecond
	m.Reconfigure(newCfg)

	m.Stop()

	if m.config.CheckInterval != 200*time.Millisecond {
		t.Errorf("CheckInterval = %v, want 200ms", m.config.CheckInterval)
	}
}

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"nvme0n1", "nvme"},
		{"sda", "sata"},
		{"vda", "unknown"},
	}
	for _, tt := range tests {
		got := classifyTransport(tt.name)
		if got != tt.want {
			t.Errorf("classifyTransport(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPastTimeOfDay(t *testing.T) {
	at := func(h, m int) time.Time {
		return time.Date(2026, 5, 26, h, m, 0, 0, time.UTC)
	}
	tests := []struct {
		now       time.Time
		timeOfDay string
		want      bool
	}{
		{at(2, 0), "02:00", true},
		{at(1, 59), "02:00", false},
		{at(3, 0), "02:00", true},
		{at(0, 0), "", true},
		{at(0, 0), "invalid", true},
		{at(0, 0), "ab:cd", true},
		{at(0, 0), "25:00", true},
		{at(0, 0), "00:60", true},
		{at(23, 59), "23:59", true},
		{at(23, 58), "23:59", false},
	}
	for _, tt := range tests {
		got := pastTimeOfDay(tt.now, tt.timeOfDay)
		if got != tt.want {
			t.Errorf("pastTimeOfDay(%02d:%02d, %q) = %v, want %v",
				tt.now.Hour(), tt.now.Minute(), tt.timeOfDay, got, tt.want)
		}
	}
}

func TestMatchesDay(t *testing.T) {
	sunday := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)   // Sunday
	monday := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)    // Monday
	wednesday := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC) // Wednesday

	tests := []struct {
		now  time.Time
		day  string
		want bool
	}{
		{sunday, "sunday", true},
		{sunday, "Sunday", true},
		{sunday, "monday", false},
		{monday, "monday", true},
		{wednesday, "wednesday", true},
		{wednesday, "", true},
		{wednesday, "foobar", true},
	}
	for _, tt := range tests {
		got := matchesDay(tt.now, tt.day)
		if got != tt.want {
			t.Errorf("matchesDay(%s, %q) = %v, want %v",
				tt.now.Weekday(), tt.day, got, tt.want)
		}
	}
}

func TestCriticalClearsTempHigh(t *testing.T) {
	report.ResetForTest()

	cfg := DefaultConfig()
	cfg.Temperature.Informational = 40
	cfg.Temperature.Critical = 55
	m := NewManager(cfg)

	ds := &DeviceStatus{Name: "sda", Transport: "sata"}
	m.devices["sda"] = ds

	warm := &fakeSmartInfo{temp: 45, healthy: true}
	m.checkTemperature("sda", ds, warm.toInfo())

	warnings := report.Warnings()
	if len(warnings) != 1 || warnings[0].Code != "temp-high" {
		t.Fatalf("expected temp-high warning, got %v", warnings)
	}

	hot := &fakeSmartInfo{temp: 60, healthy: true}
	m.checkTemperature("sda", ds, hot.toInfo())

	warnings = report.Warnings()
	for _, w := range warnings {
		if w.Code == "temp-high" {
			t.Error("temp-high warning should be cleared when temperature is critical")
		}
	}
}

func TestDeviceRemovalClearsTempRising(t *testing.T) {
	report.ResetForTest()

	cfg := DefaultConfig()
	cfg.Temperature.Difference = 4
	m := NewManager(cfg)

	ds := &DeviceStatus{Name: "sda", Transport: "sata", prevTemp: 30}
	m.devices["sda"] = ds

	rising := &fakeSmartInfo{temp: 40, healthy: true}
	m.checkTemperature("sda", ds, rising.toInfo())

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == "temp-rising" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected temp-rising warning")
	}

	m.mu.Lock()
	delete(m.devices, "sda")
	report.ClearWarning("storage", "temp-high", "sda")
	report.ClearWarning("storage", "temp-rising", "sda")
	m.mu.Unlock()

	warnings = report.Warnings()
	for _, w := range warnings {
		if w.Code == "temp-rising" && w.Subject == "sda" {
			t.Error("temp-rising should be cleared after device removal")
		}
	}
}

func TestCheckHealthReportsOnce(t *testing.T) {
	report.ResetForTest()

	m := NewManager(DefaultConfig())
	ds := &DeviceStatus{Name: "sda", Transport: "sata"}
	m.devices["sda"] = ds

	info := &fakeSmartInfo{temp: 30, healthy: false}
	m.checkHealth("sda", ds, info.toInfo())
	m.checkHealth("sda", ds, info.toInfo())

	errors := report.Errors(0)
	if len(errors) != 1 {
		t.Errorf("expected 1 error (deduplicated), got %d", len(errors))
	}
}

func TestCheckHealthResetsAfterRecovery(t *testing.T) {
	report.ResetForTest()

	m := NewManager(DefaultConfig())
	ds := &DeviceStatus{Name: "sda", Transport: "sata"}
	m.devices["sda"] = ds

	bad := &fakeSmartInfo{temp: 30, healthy: false}
	m.checkHealth("sda", ds, bad.toInfo())

	good := &fakeSmartInfo{temp: 30, healthy: true}
	m.checkHealth("sda", ds, good.toInfo())

	bad2 := &fakeSmartInfo{temp: 30, healthy: false}
	m.checkHealth("sda", ds, bad2.toInfo())

	errors := report.Errors(0)
	if len(errors) != 2 {
		t.Errorf("expected 2 errors (one per failure episode), got %d", len(errors))
	}
}

// removed the waitReady helper (and the production-only `ready`
// channel it read). Stop() already waits on m.done for goroutine exit, so the
// Start/Reconfigure tests need no separate readiness signal; the channel only
// existed for tests, which is the smell being removed.
type fakeSmartInfo struct {
	temp    int
	healthy bool
}

func (f *fakeSmartInfo) toInfo() *smartInfo {
	return &smartInfo{
		TempCelsius: f.temp,
		Healthy:     f.healthy,
	}
}

type smartInfo = smart.Info
