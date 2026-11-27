// Design: docs/architecture/diagnostics/production-diagnostics.md -- core live runtime support bundle collector tests

package support

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/badpacket"
	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/supportreg"
)

var (
	staticModuleNames = []string{
		"config", "conntrack", "crashes", "disk", "dmesg", "dns", "doctor",
		"env", "fds", "firewall", "host", "interfaces", "modules", "neighbors",
		"platform", "routes", "runtime", "sockets", "sysctl", "version",
	}
	coreRuntimeModuleNames = []string{
		"bad-packets", "drift", "goroutines", "health", "log-ring", "report", "watchdog",
	}
)

// TestSupportRuntimeCollectors_AllModulesRegistered verifies that all 20 static
// modules and all 7 core runtime modules are registered via supportreg.
//
// VALIDATES: AC-7. ModuleNames() returns a sorted list containing every static
// and core live runtime module.
// PREVENTS: dropping a static module or missing a core runtime registration
// during supportreg migration.
func TestSupportRuntimeCollectors_AllModulesRegistered(t *testing.T) {
	names := ModuleNames()
	if !slices.IsSorted(names) {
		t.Fatalf("ModuleNames() not sorted: %v", names)
	}

	present := make(map[string]bool, len(names))
	for _, name := range names {
		present[name] = true
	}

	for _, want := range staticModuleNames {
		if !present[want] {
			t.Errorf("missing static module %q in ModuleNames()", want)
		}
	}
	for _, want := range coreRuntimeModuleNames {
		if !present[want] {
			t.Errorf("missing core runtime module %q in ModuleNames()", want)
		}
	}
}

// TestSupportRuntimeCollectors_EachCollectorValidJSON verifies that every core
// live runtime collector runs cleanly, enforces bounds, and marshals to JSON.
//
// VALIDATES: AC-7. All 7 core runtime collectors execute without error and
// produce bounded, JSON-serializable payloads.
// PREVENTS: panics, unmarshalable fields, or unbounded slices in runtime
// support modules.
func TestSupportRuntimeCollectors_EachCollectorValidJSON(t *testing.T) {
	unregDrift := drift.Register("support-test-drift", func(_ context.Context) drift.Report {
		return drift.Report{
			Subsystem: "support-test-drift",
			Status:    drift.StatusClean,
			Checked:   3,
		}
	})
	t.Cleanup(unregDrift)

	pktID := badpacket.Record("support-test", "192.0.2.1", "UPDATE", "treat-as-withdraw", "test malformed frame", []byte{0xff, 0x00, 0x13})

	opts := &collectOptions{}
	cases := []struct {
		name  string
		fn    moduleCollector
		check func(t *testing.T, raw []byte)
	}{
		{
			name: "log-ring",
			fn:   collectLogRing,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					Available bool             `json:"available"`
					Entries   []map[string]any `json:"entries"`
					Count     int              `json:"count"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal log-ring: %v", err)
				}
				if !payload.Available {
					t.Error("log-ring reported unavailable")
				}
				if payload.Count > runtimeLogRingMaxEntries {
					t.Errorf("log-ring count %d exceeds cap %d", payload.Count, runtimeLogRingMaxEntries)
				}
				if len(payload.Entries) != payload.Count {
					t.Errorf("log-ring entries len %d != count %d", len(payload.Entries), payload.Count)
				}
			},
		},
		{
			name: "report",
			fn:   collectReport,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					Errors       []any          `json:"errors"`
					ErrorCount   int            `json:"error-count"`
					Warnings     []any          `json:"warnings"`
					WarningCount int            `json:"warning-count"`
					Losses       map[string]any `json:"losses"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal report: %v", err)
				}
				if payload.Errors == nil {
					t.Error("report errors slice is nil")
				}
				if payload.Warnings == nil {
					t.Error("report warnings slice is nil")
				}
				if payload.ErrorCount > runtimeReportMaxEntries {
					t.Errorf("report error-count %d exceeds cap %d", payload.ErrorCount, runtimeReportMaxEntries)
				}
				if payload.WarningCount > runtimeReportMaxEntries {
					t.Errorf("report warning-count %d exceeds cap %d", payload.WarningCount, runtimeReportMaxEntries)
				}
				if payload.Losses == nil {
					t.Error("report losses map is nil")
				}
			},
		},
		{
			name: "health",
			fn:   collectHealth,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					Status     string `json:"status"`
					CheckedAt  string `json:"checked-at"`
					Components []any  `json:"components"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal health: %v", err)
				}
				if payload.Status == "" {
					t.Error("health status is empty")
				}
				if payload.CheckedAt == "" {
					t.Error("health checked-at is empty")
				}
				if payload.Components == nil {
					t.Error("health components slice is nil")
				}
			},
		},
		{
			name: "watchdog",
			fn:   collectWatchdog,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					IntervalMS   int64 `json:"interval-ms"`
					Probes       []any `json:"probes"`
					RecentStalls []any `json:"recent-stalls"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal watchdog: %v", err)
				}
				if payload.IntervalMS <= 0 {
					t.Errorf("watchdog interval-ms = %d, want > 0", payload.IntervalMS)
				}
				if payload.Probes == nil {
					t.Error("watchdog probes slice is nil")
				}
				if payload.RecentStalls == nil {
					t.Error("watchdog recent-stalls slice is nil")
				}
			},
		},
		{
			name: "bad-packets",
			fn:   collectBadPackets,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					Packets       []badpacket.PacketRecord `json:"packets"`
					Count         int                      `json:"count"`
					TotalRecorded uint64                   `json:"total-recorded"`
					Capacity      int                      `json:"capacity"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal bad-packets: %v", err)
				}
				if payload.Capacity != badpacket.MaxRecords {
					t.Errorf("bad-packets capacity = %d, want %d", payload.Capacity, badpacket.MaxRecords)
				}
				if payload.Count > badpacket.MaxRecords {
					t.Errorf("bad-packets count %d exceeds capacity %d", payload.Count, badpacket.MaxRecords)
				}
				if payload.TotalRecorded == 0 {
					t.Error("bad-packets total-recorded is zero after Record")
				}
				found := false
				for i := range payload.Packets {
					if payload.Packets[i].ID == pktID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("bad-packets snapshot did not include recorded packet %s", pktID)
				}
			},
		},
		{
			name: "drift",
			fn:   collectDrift,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					Subsystems []drift.Report `json:"subsystems"`
					Count      int            `json:"count"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal drift: %v", err)
				}
				if payload.Count != len(payload.Subsystems) {
					t.Errorf("drift count %d != len(subsystems) %d", payload.Count, len(payload.Subsystems))
				}
				found := false
				for i := range payload.Subsystems {
					if payload.Subsystems[i].Subsystem == "support-test-drift" {
						found = true
						break
					}
				}
				if !found {
					t.Error("drift collector did not include registered subsystem support-test-drift")
				}
			},
		},
		{
			name: "goroutines",
			fn:   collectGoroutines,
			check: func(t *testing.T, raw []byte) {
				t.Helper()
				var payload struct {
					Count     int    `json:"count"`
					Truncated bool   `json:"truncated"`
					Stacks    string `json:"stacks"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("unmarshal goroutines: %v", err)
				}
				if payload.Count <= 0 {
					t.Errorf("goroutines count = %d, want > 0", payload.Count)
				}
				if payload.Stacks == "" {
					t.Error("goroutines stacks is empty")
				}
				if len(payload.Stacks) > runtimeGoroutineStackMax {
					t.Errorf("goroutines stacks size %d exceeds cap %d", len(payload.Stacks), runtimeGoroutineStackMax)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := tc.fn(opts)
			if err != nil {
				t.Fatalf("%s collector failed: %v", tc.name, err)
			}
			raw, err := json.Marshal(val)
			if err != nil {
				t.Fatalf("%s JSON marshal failed: %v", tc.name, err)
			}
			tc.check(t, raw)
		})
	}
}

// TestSupportRuntimeCollectors_LogRingSinceFilter verifies that collectLogRing
// respects opts.SinceTime when filtering log ring entries.
//
// VALIDATES: AC-7. A future SinceTime filters out older log entries and records
// the requested since cutoff in the payload.
// PREVENTS: --since being silently ignored by the log-ring collector.
func TestSupportRuntimeCollectors_LogRingSinceFilter(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	val, err := collectLogRing(&collectOptions{SinceTime: future})
	if err != nil {
		t.Fatalf("collectLogRing: %v", err)
	}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatalf("marshal log-ring: %v", err)
	}
	var payload struct {
		Count int    `json:"count"`
		Since string `json:"since"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal log-ring: %v", err)
	}
	if payload.Count != 0 {
		t.Errorf("future SinceTime returned %d entries, want 0", payload.Count)
	}
	if payload.Since == "" {
		t.Error("expected since field when SinceTime is set")
	}
}

// TestSupportRuntimeCollectors_DynamicRegistration verifies that a collector
// registered via supportreg.Register appears in ModuleNames() and filterModules
// and disappears cleanly when unregistered.
//
// VALIDATES: AC-7. Daemon and external components can dynamically register and
// unregister support modules through internal/core/supportreg.
// PREVENTS: stale central enumerations or leaked registrations across daemon
// restarts.
func TestSupportRuntimeCollectors_DynamicRegistration(t *testing.T) {
	const dynName = "test-dynamic-support-module"

	unreg := supportreg.Register(dynName, func(_ *supportreg.CollectOptions) (any, error) {
		return map[string]any{"dynamic": true}, nil
	})
	t.Cleanup(unreg)

	if !slices.Contains(ModuleNames(), dynName) {
		t.Fatalf("ModuleNames() missing dynamically registered %q", dynName)
	}

	selected, errMsg := filterModules([]string{dynName}, nil)
	if errMsg != "" {
		t.Fatalf("filterModules(%q) failed: %s", dynName, errMsg)
	}
	if _, ok := selected[dynName]; !ok {
		t.Fatalf("filterModules(%q) omitted dynamic collector", dynName)
	}

	unreg()

	if slices.Contains(ModuleNames(), dynName) {
		t.Errorf("ModuleNames() still contains %q after unregister", dynName)
	}
	if _, errMsg := filterModules([]string{dynName}, nil); errMsg == "" {
		t.Errorf("filterModules(%q) succeeded after unregister, want unknown module error", dynName)
	}
}

// TestSupportRuntimeCollectors_CollectToMemoryArchive verifies that collecting
// all 7 core runtime modules produces a valid gzip tar archive well below
// command.BlobRemoteOctetsMax.
//
// VALIDATES: AC-7, R-4. Selecting all 7 core runtime modules builds a valid
// tar.gz with all 7 JSON payloads and stays safely under the remote blob cap.
// PREVENTS: runtime modules breaking archive packaging or overflowing the
// daemon-routed blob limit.
func TestSupportRuntimeCollectors_CollectToMemoryArchive(t *testing.T) {
	selected, errMsg := filterModules(coreRuntimeModuleNames, nil)
	if errMsg != "" {
		t.Fatalf("filterModules(coreRuntimeModuleNames): %s", errMsg)
	}

	manifest, archive, err := collectToMemory(context.Background(), selected, &collectOptions{}, "runtime-test")
	if err != nil {
		t.Fatalf("collectToMemory: %v", err)
	}
	if len(archive) == 0 {
		t.Fatal("archive is empty")
	}
	if len(archive)*4 > command.BlobRemoteOctetsMax {
		t.Errorf("runtime modules archive is %d bytes, over a quarter of remote bound %d", len(archive), command.BlobRemoteOctetsMax)
	}

	entries := tarEntries(t, archive)
	if _, ok := entries["manifest.json"]; !ok {
		t.Fatal("archive missing manifest.json")
	}

	for _, name := range coreRuntimeModuleNames {
		mod, ok := manifest.Modules[name]
		if !ok {
			t.Errorf("manifest missing module %q", name)
			continue
		}
		if !mod.Collected {
			t.Errorf("module %q not collected: %s", name, mod.Error)
		}
		entryName := name + ".json"
		data, ok := entries[entryName]
		if !ok {
			t.Errorf("archive missing %s", entryName)
			continue
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("archive entry %s is not valid JSON: %v", entryName, err)
		}
	}

	// Verify ModuleList includes every core runtime module name.
	list := ModuleList()
	for _, name := range coreRuntimeModuleNames {
		if !strings.Contains(list, name) {
			t.Errorf("ModuleList() missing %q", name)
		}
	}
}
