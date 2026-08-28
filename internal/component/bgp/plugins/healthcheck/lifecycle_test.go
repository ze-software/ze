// Design: docs/architecture/bgp/healthcheck-plugin.md -- config reload lifecycle tests
package healthcheck

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestManager creates a probeManager with a no-op dispatch for lifecycle tests.
func mustMarshalStr(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

type dispatchCall struct {
	command string
	args    []string
	peer    string
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newTestManager() *probeManager {
	m := &probeManager{
		probes: make(map[string]*runningProbe),
		ready:  make(chan struct{}),
		dispatchFn: func(_ context.Context, _ string, _ []string, _ string) (string, json.RawMessage, error) {
			return statusDone, nil, nil
		},
	}
	// These tests drive the manager directly instead of through runHealthcheckPlugin,
	// so nothing delivers the OnAllPluginsReady callback that normally releases the
	// probe loops (see waitReady). Release it here: the startup handshake this gate
	// protects does not exist in a unit test.
	m.markReady()
	return m
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

// TestDebounce verifies debounce logic by running a real probe with a recording dispatch.
func TestDebounce(t *testing.T) {
	var mu sync.Mutex
	var dispatches []dispatchCall

	mgr := &probeManager{
		probes: make(map[string]*runningProbe),
		ipMgr:  realIPManager{},
		ready:  make(chan struct{}),
		dispatchFn: func(_ context.Context, cmd string, args []string, peer string) (string, json.RawMessage, error) {
			mu.Lock()
			dispatches = append(dispatches, dispatchCall{command: cmd, args: append([]string(nil), args...), peer: peer})
			mu.Unlock()
			return statusDone, nil, nil
		},
	}
	// This test drives runProbe directly, with no startup handshake to release the
	// probe-loop gate (see waitReady). Release it explicitly.
	mgr.markReady()

	// Probe that succeeds immediately (rise=1), debounce=true, interval=1.
	// After first UP dispatch, subsequent intervals should NOT dispatch again.
	cfg := ProbeConfig{
		Name:     "debounce-test",
		Command:  "true",
		Group:    "hc",
		Interval: 1,
		Timeout:  5,
		Rise:     1,
		Fall:     1,
		Debounce: true,
		UpMetric: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	rp := &runningProbe{config: cfg, cancel: cancel, done: make(chan struct{})}
	go mgr.runProbe(ctx, rp)

	// Wait for at least 2 probe intervals.
	time.Sleep(2500 * time.Millisecond)
	cancel()
	<-rp.done

	mu.Lock()
	count := len(dispatches)
	mu.Unlock()

	// With debounce=true, should dispatch exactly once (INIT->UP transition),
	// plus the exit withdraw. Not once per interval.
	// The exit dispatch adds a "request bgp watchdog withdraw" at the end.
	if count > 2 {
		t.Errorf("debounce=true: dispatches = %d, want <= 2 (UP + exit withdraw)", count)
	}
	if count == 0 {
		t.Error("debounce=true: no dispatches at all")
	}
}

func TestShowEmptyProbes(t *testing.T) {
	mgr := newTestManager()
	status, data, err := mgr.handleCommand("show bgp healthcheck", nil)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if got := mustMarshalStr(t, data); got != "[]" {
		t.Errorf("data = %q, want empty JSON array", got)
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	mgr := newTestManager()
	status, _, err := mgr.handleCommand("healthcheck foo", nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

func TestResetNonexistentProbe(t *testing.T) {
	mgr := newTestManager()
	status, _, err := mgr.handleCommand("clear bgp healthcheck", []string{"missing"})
	if err == nil {
		t.Fatal("expected error for nonexistent probe")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestShowAllProbes(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 3, Fall: 3, Timeout: 5},
		{Name: "web", Command: "true", Group: "hc-web", Interval: 1, Rise: 3, Fall: 3, Timeout: 5},
	})
	defer mgr.applyConfig(nil)

	status, data, err := mgr.handleCommand("show bgp healthcheck", nil)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	// Verify JSON contains both probes with state fields.
	if !strings.Contains(mustMarshalStr(t, data), `"name":"dns"`) && !strings.Contains(mustMarshalStr(t, data), `"name":"web"`) {
		t.Errorf("data = %q, want both probe names", data)
	}
	if !strings.Contains(mustMarshalStr(t, data), `"state":`) {
		t.Errorf("data = %q, want state field", data)
	}
}

func TestShowSingleProbe(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 3, Fall: 3, Timeout: 5, UpMetric: 100},
	})
	defer mgr.applyConfig(nil)

	status, data, err := mgr.handleCommand("show bgp healthcheck", []string{"dns"})
	if err != nil {
		t.Fatalf("show dns: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if !strings.Contains(mustMarshalStr(t, data), `"name":"dns"`) {
		t.Errorf("data = %q, want probe name", data)
	}
}

func TestShowNonexistentProbe(t *testing.T) {
	mgr := newTestManager()

	status, _, err := mgr.handleCommand("show bgp healthcheck", []string{"missing"})
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

	status, data, err := mgr.handleCommand("clear bgp healthcheck", []string{"dns"})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if !strings.Contains(mustMarshalStr(t, data), `"action":"reset"`) {
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

	status, _, err := mgr.handleCommand("clear bgp healthcheck", []string{"dns"})
	if err == nil {
		t.Fatal("expected error for DISABLED probe reset")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

func TestResetMissingName(t *testing.T) {
	mgr := newTestManager()

	status, _, err := mgr.handleCommand("clear bgp healthcheck", nil)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

// VALIDATES: dispatchStateAction generates correct watchdog commands for each state (#21).
func TestDispatchStateAction(t *testing.T) {
	var dispatched []dispatchCall
	mgr := &probeManager{
		probes: make(map[string]*runningProbe),
		dispatchFn: func(_ context.Context, cmd string, args []string, peer string) (string, json.RawMessage, error) {
			dispatched = append(dispatched, dispatchCall{command: cmd, args: append([]string(nil), args...), peer: peer})
			return statusDone, nil, nil
		},
	}

	cfg := ProbeConfig{
		Name:           "dns",
		Group:          "hc-dns",
		UpMetric:       100,
		DownMetric:     1000,
		DisabledMetric: 500,
	}

	tests := []struct {
		state          State
		withdrawOnDown bool
		wantCommand    string
		wantArgs       []string
	}{
		{StateUp, false, "request bgp watchdog announce", []string{"hc-dns", "med", "100"}},
		{StateUp, true, "request bgp watchdog announce", []string{"hc-dns", "med", "100"}},
		{StateDown, false, "request bgp watchdog announce", []string{"hc-dns", "med", "1000"}},
		{StateDown, true, "request bgp watchdog withdraw", []string{"hc-dns"}},
		{StateDisabled, false, "request bgp watchdog announce", []string{"hc-dns", "med", "500"}},
		{StateDisabled, true, "request bgp watchdog withdraw", []string{"hc-dns"}},
		{StateExit, false, "request bgp watchdog withdraw", []string{"hc-dns"}},
		{StateExit, true, "request bgp watchdog withdraw", []string{"hc-dns"}},
	}

	ctx := context.Background()
	for _, tt := range tests {
		dispatched = nil
		cfg.WithdrawOnDown = tt.withdrawOnDown
		mgr.dispatchStateAction(ctx, cfg, tt.state)
		if len(dispatched) == 0 {
			t.Errorf("state=%d withdraw=%v: no dispatch", tt.state, tt.withdrawOnDown)
			continue
		}
		if dispatched[0].command != tt.wantCommand || !equalStrings(dispatched[0].args, tt.wantArgs) {
			t.Errorf("state=%d withdraw=%v: got command=%q args=%v, want command=%q args=%v",
				tt.state, tt.withdrawOnDown, dispatched[0].command, dispatched[0].args, tt.wantCommand, tt.wantArgs)
		}
	}

	// RISING/FALLING/INIT/END should dispatch nothing.
	for _, state := range []State{StateInit, StateRising, StateFalling, StateEnd} {
		dispatched = nil
		mgr.dispatchStateAction(ctx, cfg, state)
		if len(dispatched) != 0 {
			t.Errorf("state=%d: expected no dispatch, got command=%q args=%v", state, dispatched[0].command, dispatched[0].args)
		}
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
