//go:build linux

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/host/trend"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/sim"
)

// The fixture box: 8 GiB of memory, 200 MiB of unreclaimable kernel memory
// and 4 GiB available at the start. What the kernel takes comes out of what
// is available, so the kernel-memory rule's limit stays at 4296 MiB.
const (
	trendBoxTotalKiB      = 8 << 20
	trendBoxKernelKiB     = 200 << 10
	trendBoxAvailableKiB  = 4 << 20
	trendBoxMebibyteKiB   = 1 << 10
	trendBoxKernelBytes   = trendBoxKernelKiB << 10
	trendBoxLimitBytes    = (trendBoxKernelKiB + trendBoxAvailableKiB) << 10
	trendKernelMemoryRule = "kernel-memory"
	// trendHandlerWait bounds, in real time, the wait for the monitor's
	// goroutine to publish a tick. Nothing waits on it unless the monitor is
	// broken.
	trendHandlerWait = 10 * time.Second
)

// trendBox is a fixture /proc the kernel-memory rule reads, on a fake clock.
type trendBox struct {
	t     *testing.T
	root  string
	clock *sim.FakeClock
}

// startTrendBox writes the start meminfo, starts the real trend monitor over
// it and stops the monitor when the test ends.
func startTrendBox(t *testing.T) *trendBox {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	box := &trendBox{t: t, root: root,
		clock: sim.NewFakeClock(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))}
	box.setGrown(0)
	monitor, err := trend.Start(box.clock, &host.Detector{Root: root})
	if err != nil {
		t.Fatalf("trend.Start: %v", err)
	}
	t.Cleanup(monitor.Stop)
	if !box.clock.AwaitTickers(1, trendHandlerWait) {
		t.Fatal("the trend monitor created no ticker")
	}
	return box
}

// setGrown writes a meminfo whose unreclaimable kernel memory is grownMiB
// above the start, taken from the memory available.
func (b *trendBox) setGrown(grownMiB uint64) {
	b.t.Helper()
	line := func(key string, kib uint64) string {
		return key + ": " + strconv.FormatUint(kib, 10) + " kB\n"
	}
	meminfo := line("MemTotal", trendBoxTotalKiB) +
		line("MemFree", trendBoxAvailableKiB/2) +
		line("MemAvailable", trendBoxAvailableKiB-grownMiB*trendBoxMebibyteKiB) +
		line("SReclaimable", 100*trendBoxMebibyteKiB) +
		line("SUnreclaim", trendBoxKernelKiB+grownMiB*trendBoxMebibyteKiB)
	if err := os.WriteFile(filepath.Join(b.root, "proc", "meminfo"), []byte(meminfo), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// stepHour moves the clock one hour, fires the monitor's ticker, and waits
// until the kernel-memory rule has been observed for hours. The sample of each
// step closes one observed hourly bucket, so the observed duration is the
// published state that says the tick was judged. The monitor exposes no
// publish signal outside its package, so the wait polls that state.
func (b *trendBox) stepHour(hours int64) {
	b.t.Helper()
	b.clock.Add(time.Hour)
	b.clock.FireTickers()
	want := time.Duration(hours) * time.Hour
	deadline := time.Now().Add(trendHandlerWait)
	for time.Now().Before(deadline) {
		statuses, err := trend.Rules()
		if err != nil {
			b.t.Fatalf("trend.Rules: %v", err)
		}
		for i := range statuses {
			if statuses[i].Name() == trendKernelMemoryRule && statuses[i].Observed() == want {
				return
			}
		}
		time.Sleep(time.Millisecond) // polls the published state; see the comment above
	}
	b.t.Fatalf("the kernel-memory rule was not observed for %s after the step", want)
}

// showTrendRule runs the show host trend handler and answers the JSON object
// of the rule called name, as every command surface receives it.
func showTrendRule(t *testing.T, name string) map[string]any {
	t.Helper()
	resp, err := handleShowTrend(nil, command.ValidatedArgs{})
	if err != nil {
		t.Fatalf("handleShowTrend: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %q (%s), want %q", resp.Status, resp.Error, plugin.StatusDone)
	}
	encoded, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal the answer: %v", err)
	}
	var answer struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(encoded, &answer); err != nil {
		t.Fatalf("unmarshal the answer %s: %v", encoded, err)
	}
	for _, rule := range answer.Rules {
		if rule["name"] == name {
			return rule
		}
	}
	t.Fatalf("the answer lists no rule %q: %s", name, encoded)
	return nil
}

// assertTrendKeysAbsent fails for each key the rule's answer carries.
func assertTrendKeysAbsent(t *testing.T, rule map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if value, present := rule[key]; present {
			t.Errorf("key %q = %v, want it absent", key, value)
		}
	}
}

// VALIDATES: AC-21, warming-up half -- a running monitor's rule is listed
// with its state, its unit, its level and limit in bytes as numbers and its
// observed duration, and a rule still warming up carries no growth, no
// projection and no warning: those keys are absent, never zero.
// Method: the real trend monitor and the real kernel-memory rule over a
// fixture /proc on a fake clock; the handler's answer is read through JSON
// right after start and after one observed hour.
func TestShowHostTrendListsWarmingUpRule(t *testing.T) {
	box := startTrendBox(t)

	rule := showTrendRule(t, trendKernelMemoryRule)
	if rule["state"] != "warming-up" {
		t.Errorf("state = %v, want warming-up", rule["state"])
	}
	if rule["unit"] != "bytes" {
		t.Errorf("unit = %v, want bytes", rule["unit"])
	}
	if rule["level"] != float64(trendBoxKernelBytes) {
		t.Errorf("level = %v, want %d", rule["level"], trendBoxKernelBytes)
	}
	if rule["limit"] != float64(trendBoxLimitBytes) {
		t.Errorf("limit = %v, want %d", rule["limit"], trendBoxLimitBytes)
	}
	if rule["observed-seconds"] != float64(0) {
		t.Errorf("observed-seconds = %v, want 0 before any bucket closed", rule["observed-seconds"])
	}
	assertTrendKeysAbsent(t, rule, "growth", "growth-window-seconds", "exhaustion-seconds",
		"warning", "failure-source", "failure-cause")

	box.stepHour(1)
	rule = showTrendRule(t, trendKernelMemoryRule)
	if rule["state"] != "warming-up" {
		t.Errorf("after one hour: state = %v, want warming-up", rule["state"])
	}
	if rule["observed-seconds"] != float64(3600) {
		t.Errorf("after one hour: observed-seconds = %v, want 3600", rule["observed-seconds"])
	}
	assertTrendKeysAbsent(t, rule, "growth", "growth-window-seconds", "exhaustion-seconds", "warning")
}

// VALIDATES: AC-21, projection half -- a rule whose growth holds lists the
// growth over the window it judged, the window, the projected time to
// exhaustion, and, once active, the warning it raised.
// Method: as above, with unreclaimable kernel memory growing 100 MiB an hour
// (above the 1% minimum of 81.92 MiB, and reaching the limit in about a day
// and a half, inside the 7 day horizon). The clock steps one hour at a time,
// bounded at 24 steps, until the rule is active.
func TestShowHostTrendListsProjectionOfActiveRule(t *testing.T) {
	box := startTrendBox(t)

	var rule map[string]any
	for hour := int64(1); hour <= 24; hour++ {
		box.setGrown(uint64(hour) * 100)
		box.stepHour(hour)
		rule = showTrendRule(t, trendKernelMemoryRule)
		if rule["state"] == "active" {
			break
		}
	}
	if rule["state"] != "active" {
		t.Fatalf("state after 24 hours of growth = %v, want active: %v", rule["state"], rule)
	}
	for _, key := range []string{"growth", "growth-window-seconds", "exhaustion-seconds"} {
		value, present := rule[key].(float64)
		if !present {
			t.Errorf("key %q = %v, want a number", key, rule[key])
			continue
		}
		if value <= 0 {
			t.Errorf("key %q = %v, want above 0", key, value)
		}
	}
	if warning, _ := rule["warning"].(string); warning == "" {
		t.Errorf("warning = %v, want the message the rule raised", rule["warning"])
	}
	assertTrendKeysAbsent(t, rule, "failure-source", "failure-cause")
}
