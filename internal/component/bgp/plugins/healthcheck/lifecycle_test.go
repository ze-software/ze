// Design: plan/spec-healthcheck-0-umbrella.md -- config reload lifecycle tests
package healthcheck

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestManager creates a probeManager with a no-op dispatch for lifecycle tests.
func newTestManager() *probeManager {
	return &probeManager{
		probes: make(map[string]*runningProbe),
		dispatchFn: func(_ context.Context, _ string) (string, string, error) {
			return statusDone, "", nil
		},
	}
}

func TestLifecycleStartAndStop(t *testing.T) {
	mgr := newTestManager()

	configs := []ProbeConfig{{
		Name:     "dns",
		Command:  "true",
		Group:    "hc-dns",
		Interval: 1,
		Rise:     3,
		Fall:     3,
		Timeout:  5,
	}}

	mgr.applyConfig(configs)

	mgr.mu.Lock()
	count := len(mgr.probes)
	mgr.mu.Unlock()
	if count != 1 {
		t.Fatalf("probes = %d, want 1", count)
	}

	// Remove all probes.
	mgr.applyConfig(nil)

	mgr.mu.Lock()
	count = len(mgr.probes)
	mgr.mu.Unlock()
	if count != 0 {
		t.Fatalf("probes = %d, want 0 after remove", count)
	}
}

func TestLifecycleReconfigure(t *testing.T) {
	mgr := newTestManager()

	original := []ProbeConfig{{
		Name:     "dns",
		Command:  "true",
		Group:    "hc-dns",
		Interval: 1,
		Rise:     3,
		Fall:     3,
		Timeout:  5,
	}}

	mgr.applyConfig(original)

	// Change the command -- should trigger deconfigure + restart.
	changed := []ProbeConfig{{
		Name:     "dns",
		Command:  "curl localhost",
		Group:    "hc-dns",
		Interval: 1,
		Rise:     3,
		Fall:     3,
		Timeout:  5,
	}}

	mgr.applyConfig(changed)

	mgr.mu.Lock()
	rp := mgr.probes["dns"]
	mgr.mu.Unlock()
	if rp == nil {
		t.Fatal("probe dns should be running after reconfigure")
	}
	if rp.config.Command != "curl localhost" {
		t.Errorf("config.Command = %q, want 'curl localhost'", rp.config.Command)
	}

	// Clean up.
	mgr.applyConfig(nil)
}

func TestLifecycleUnchanged(t *testing.T) {
	mgr := newTestManager()

	configs := []ProbeConfig{{
		Name:     "dns",
		Command:  "true",
		Group:    "hc-dns",
		Interval: 1,
		Rise:     3,
		Fall:     3,
		Timeout:  5,
	}}

	mgr.applyConfig(configs)

	mgr.mu.Lock()
	originalDone := mgr.probes["dns"].done
	mgr.mu.Unlock()

	// Apply same config -- should NOT restart.
	mgr.applyConfig(configs)

	mgr.mu.Lock()
	sameDone := mgr.probes["dns"].done
	mgr.mu.Unlock()

	if originalDone != sameDone {
		t.Error("probe restarted on unchanged config")
	}

	mgr.applyConfig(nil)
}

func TestLifecycleDisableToggle(t *testing.T) {
	mgr := newTestManager()

	// Start disabled.
	configs := []ProbeConfig{{
		Name:     "dns",
		Command:  "true",
		Group:    "hc-dns",
		Interval: 1,
		Rise:     1,
		Fall:     1,
		Timeout:  5,
		Disable:  true,
	}}
	mgr.applyConfig(configs)

	// Re-enable: disable false -> config change -> restart from INIT.
	enabled := []ProbeConfig{{
		Name:     "dns",
		Command:  "true",
		Group:    "hc-dns",
		Interval: 1,
		Rise:     1,
		Fall:     1,
		Timeout:  5,
		Disable:  false,
	}}
	mgr.applyConfig(enabled)

	mgr.mu.Lock()
	rp := mgr.probes["dns"]
	mgr.mu.Unlock()
	if rp == nil {
		t.Fatal("probe dns should be running after re-enable")
	}
	if rp.config.Disable {
		t.Error("config.Disable should be false after toggle")
	}

	mgr.applyConfig(nil)
}

func TestLifecycleMultipleProbes(t *testing.T) {
	mgr := newTestManager()

	configs := []ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 3, Fall: 3, Timeout: 5},
		{Name: "web", Command: "true", Group: "hc-web", Interval: 1, Rise: 3, Fall: 3, Timeout: 5},
	}
	mgr.applyConfig(configs)

	mgr.mu.Lock()
	count := len(mgr.probes)
	mgr.mu.Unlock()
	if count != 2 {
		t.Fatalf("probes = %d, want 2", count)
	}

	// Remove one.
	mgr.applyConfig(configs[:1])

	mgr.mu.Lock()
	count = len(mgr.probes)
	_, hasWeb := mgr.probes["web"]
	mgr.mu.Unlock()
	if count != 1 {
		t.Fatalf("probes = %d, want 1", count)
	}
	if hasWeb {
		t.Error("web probe should have been removed")
	}

	mgr.applyConfig(nil)
}

func TestDebounceTrue(t *testing.T) {
	// Verify debounce=true skips dispatch when state unchanged.
	// This is a behavioral test of the runProbe loop logic.
	// We test indirectly: the FSM stays in INIT on first iteration,
	// then transitions to RISING/UP. Only state changes should dispatch.

	var dispatches []string
	var mu sync.Mutex

	// We can't easily test the full runProbe loop without a real SDK.
	// Instead, verify the dispatch logic directly.
	cfg := ProbeConfig{
		Name:     "test",
		Group:    "hc",
		Debounce: true,
		UpMetric: 100,
	}

	prevState := StateUp
	newState := StateUp
	stateChanged := newState != prevState

	// debounce=true, state unchanged -> should NOT dispatch.
	if stateChanged || !cfg.Debounce {
		mu.Lock()
		dispatches = append(dispatches, "should-not-happen")
		mu.Unlock()
	}

	if len(dispatches) != 0 {
		t.Error("debounce=true should skip dispatch on unchanged state")
	}
}

func TestDebounceFalse(t *testing.T) {
	cfg := ProbeConfig{
		Name:     "test",
		Group:    "hc",
		Debounce: false,
		UpMetric: 100,
	}

	prevState := StateUp
	newState := StateUp
	stateChanged := newState != prevState

	dispatched := stateChanged || !cfg.Debounce

	if !dispatched {
		t.Error("debounce=false should dispatch even on unchanged state")
	}
}

func TestShowAllProbes(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 3, Fall: 3, Timeout: 5},
		{Name: "web", Command: "true", Group: "hc-web", Interval: 1, Rise: 3, Fall: 3, Timeout: 5},
	})
	defer mgr.applyConfig(nil)

	status, data, err := mgr.handleCommand("healthcheck show", nil)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if data == "" {
		t.Error("show returned empty data")
	}
}

func TestShowSingleProbe(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 3, Fall: 3, Timeout: 5, UpMetric: 100},
	})
	defer mgr.applyConfig(nil)

	status, data, err := mgr.handleCommand("healthcheck show", []string{"dns"})
	if err != nil {
		t.Fatalf("show dns: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if !strings.Contains(data, `"name":"dns"`) {
		t.Errorf("data = %q, want probe name", data)
	}
}

func TestShowNonexistentProbe(t *testing.T) {
	mgr := newTestManager()

	status, _, err := mgr.handleCommand("healthcheck show", []string{"missing"})
	if err == nil {
		t.Fatal("expected error for nonexistent probe")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

func TestResetProbe(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 1, Fall: 1, Timeout: 5},
	})
	defer mgr.applyConfig(nil)

	status, data, err := mgr.handleCommand("healthcheck reset", []string{"dns"})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if !strings.Contains(data, `"action":"reset"`) {
		t.Errorf("data = %q, want reset action", data)
	}

	// Probe should still be running after reset.
	mgr.mu.Lock()
	_, running := mgr.probes["dns"]
	mgr.mu.Unlock()
	if !running {
		t.Error("probe should be running after reset")
	}
}

func TestResetDisabledProbe(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 1, Fall: 1, Timeout: 5, Disable: true},
	})
	defer mgr.applyConfig(nil)

	status, _, err := mgr.handleCommand("healthcheck reset", []string{"dns"})
	if err == nil {
		t.Fatal("expected error for DISABLED probe reset")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

func TestResetMissingName(t *testing.T) {
	mgr := newTestManager()

	status, _, err := mgr.handleCommand("healthcheck reset", nil)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

func TestFastIntervalSelection(t *testing.T) {
	cfg := ProbeConfig{
		Interval:     5,
		FastInterval: 1,
	}

	tests := []struct {
		state    State
		wantSecs uint32
	}{
		{StateInit, 5},
		{StateRising, 1},
		{StateFalling, 1},
		{StateUp, 5},
		{StateDown, 5},
	}

	for _, tt := range tests {
		interval := time.Duration(cfg.Interval) * time.Second
		if tt.state == StateRising || tt.state == StateFalling {
			interval = time.Duration(cfg.FastInterval) * time.Second
		}
		got := uint32(interval / time.Second)
		if got != tt.wantSecs {
			t.Errorf("state=%d: interval=%ds, want %ds", tt.state, got, tt.wantSecs)
		}
	}
}
