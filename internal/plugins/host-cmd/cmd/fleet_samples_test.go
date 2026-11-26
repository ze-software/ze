// Design: docs/architecture/fleet-config.md — tests for host fleet sample RPC handler

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

// VALIDATES: show fleet source host returns a JSON-encoded SourceSnapshot that
// passes fleet.ValidateSourceSnapshot and registers under ze-host:show-fleet-source-host.
// PREVENTS: broken wire-method registration or malformed JSON from the host source RPC.
func TestHandleShowFleetSourceHost(t *testing.T) {
	saved := sampleHostFleetMetrics
	t.Cleanup(func() { sampleHostFleetMetrics = saved })

	sampleHostFleetMetrics = func() fleet.SourceSnapshot {
		return fleet.SourceSnapshot{
			Owner: fleet.SampleOwnerHost,
			Readings: []fleet.SourceReading{
				{Figure: fleet.FigureCPUUsed, Unit: fleet.SampleUnitPercent, Value: 12.5},
				{Figure: fleet.FigureMemoryUsed, Unit: fleet.SampleUnitBytes, Value: 4096},
				{Figure: fleet.FigureMemoryTotal, Unit: fleet.SampleUnitBytes, Value: 16384},
				{Figure: fleet.FigureLoad1m, Unit: fleet.SampleUnitNumber, Value: 0.42},
				{Figure: fleet.FigureUptime, Unit: fleet.SampleUnitSeconds, Value: 120},
			},
		}
	}

	resp, err := handleShowFleetSourceHost(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("handleShowFleetSourceHost error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %q, error = %q, want done", resp.Status, resp.Error)
	}
	raw, ok := resp.Data.(plugin.RawJSON)
	if !ok {
		t.Fatalf("data type = %T, want plugin.RawJSON", resp.Data)
	}

	var decoded fleet.SourceSnapshot
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if err := fleet.ValidateSourceSnapshot(fleet.SampleOwnerHost, &decoded); err != nil {
		t.Fatalf("ValidateSourceSnapshot failed: %v", err)
	}
	if len(decoded.Readings) != 5 {
		t.Fatalf("got %d readings, want 5", len(decoded.Readings))
	}

	found := false
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == "ze-host:show-fleet-source-host" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ze-host:show-fleet-source-host not found in pluginserver.AllBuiltinRPCs()")
	}
}

// VALIDATES: an invalid snapshot returned by the host sampler is refused with
// an operator error instead of passing malformed data across the RPC boundary.
// PREVENTS: invalid unit or out-of-range host sample reaching the fleet sender.
func TestHandleShowFleetSourceHostRejectsInvalidSnapshot(t *testing.T) {
	saved := sampleHostFleetMetrics
	t.Cleanup(func() { sampleHostFleetMetrics = saved })

	sampleHostFleetMetrics = func() fleet.SourceSnapshot {
		return fleet.SourceSnapshot{
			Owner: fleet.SampleOwnerHost,
			Readings: []fleet.SourceReading{
				{Figure: fleet.FigureCPUUsed, Unit: fleet.SampleUnitBytes, Value: 10},
			},
		}
	}

	resp, err := handleShowFleetSourceHost(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %q, want error", resp.Status)
	}
}
