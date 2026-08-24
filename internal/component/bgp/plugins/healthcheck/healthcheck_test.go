// Design: docs/architecture/bgp/healthcheck-plugin.md -- the answer shape of the show command
// Related: lifecycle_test.go -- the per-probe show tests that share newTestManager
package healthcheck

import (
	"encoding/json"
	"sort"
	"testing"
)

// showRows dispatches "show bgp healthcheck" and decodes its answer as a row set.
// It fails the test when the answer is not a JSON array, which is the whole point
// of the caller below: a bare object decodes into no row set at all.
func showRows(t *testing.T, m *probeManager, args []string) []map[string]any {
	t.Helper()
	status, data, err := m.handleCommand("show bgp healthcheck", args)
	if err != nil {
		t.Fatalf("show bgp healthcheck %v: %v", args, err)
	}
	if status != statusDone {
		t.Fatalf("show bgp healthcheck %v: status = %q, want %q", args, status, statusDone)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal answer for %v: %v", args, err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatalf("show bgp healthcheck %v answered %s, which is no row set: %v", args, encoded, err)
	}
	return rows
}

// keysOf returns a map's keys in sorted order, so two answers can be compared on
// the names they use rather than on the order a range happens to produce.
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// VALIDATES: AC-14 -- "show bgp healthcheck <name>" answers a one-element row set,
// in the same spelling the no-argument branch uses for the fields both carry.
// PREVENTS: the named-probe branch drifting back to a bare object, which no single
// answer-shape declaration can describe, so an operator such as `| count` would
// answer one thing for the probe list and something else for one probe.
//
// TestHealthcheckNamedProbeAnswersRows proves AC-14: "show bgp healthcheck"
// answers one shape whatever its argument. The method is to read BOTH branches
// and compare them against each other, so the test stays honest if a later change
// moves the no-argument branch instead of the named-probe one. A test that only
// read the named-probe answer would go green on two branches that disagree.
func TestHealthcheckNamedProbeAnswersRows(t *testing.T) {
	mgr := newTestManager()
	mgr.applyConfig([]ProbeConfig{
		{Name: "dns", Command: "true", Group: "hc-dns", Interval: 1, Rise: 3, Fall: 3, Timeout: 5, UpMetric: 100, DownMetric: 200, DisabledMetric: 300},
		{Name: "web", Command: "true", Group: "hc-web", Interval: 2, Rise: 4, Fall: 5, Timeout: 5},
	})
	defer mgr.applyConfig(nil)

	listRows := showRows(t, mgr, nil)
	if len(listRows) != 2 {
		t.Fatalf("no-argument rows = %d, want 2", len(listRows))
	}
	var listDNS map[string]any
	for _, row := range listRows {
		if row["name"] == "dns" {
			listDNS = row
		}
	}
	if listDNS == nil {
		t.Fatal("no-argument answer holds no row named dns")
	}

	namedRows := showRows(t, mgr, []string{"dns"})
	if len(namedRows) != 1 {
		t.Fatalf("named-probe rows = %d, want 1", len(namedRows))
	}
	named := namedRows[0]

	// Every field the no-argument branch writes is spelled the same way here and
	// carries the same value for the same probe.
	for _, key := range keysOf(listDNS) {
		got, present := named[key]
		if !present {
			t.Errorf("named-probe row has no %q field, which the no-argument row writes", key)
			continue
		}
		if got != listDNS[key] {
			t.Errorf("named-probe %s = %v, want %v from the no-argument row", key, got, listDNS[key])
		}
	}

	// The named-probe row keeps the ten fields it has always carried. Widening the
	// no-argument branch to match would change the answer every operator reads, so
	// the two row shapes stay different and the command declares "map" rather than
	// "tab" (internal/component/command/pipe_catalog.go).
	want := []string{
		"command", "disabled-metric", "down-metric", "fall", "group",
		"interval", "name", "rise", "state", "up-metric",
	}
	got := keysOf(named)
	if len(got) != len(want) {
		t.Fatalf("named-probe fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("named-probe fields = %v, want %v", got, want)
		}
	}
	if named["up-metric"] != float64(100) {
		t.Errorf("named-probe up-metric = %v, want 100", named["up-metric"])
	}
}
