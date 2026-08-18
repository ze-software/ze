// VALIDATES: AC-9, the `show anomaly detect` RENDER contract across all three
// entity kinds, driven through the real ze-show:anomaly handler rather than
// through entityLabel alone.
// PREVENTS: the vacuity that hid this surface's shape. test/plugin/anomaly-show.ci
// asserts `incidents` is a LIST and never reads a row, so it passes whatever the
// formatter returns -- the same field-presence gap that let `command-count` report
// 0 from 2026-03-27 to 2026-08-18.

package detect

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/anomalyevent"
)

// showRows drives the real RPC entry point over a detector holding inc, and returns
// the rendered incident rows. It restores the process-global detector, which the
// handler reads and every test in this package shares.
func showRows(t *testing.T, inc []anomalyevent.AnomalyDetected) []plugin.Map {
	t.Helper()
	prev := loadGlobalDetector()
	t.Cleanup(func() { setGlobalDetector(prev) })
	setGlobalDetector(&detector{inc: inc})

	resp, err := handleShowAnomaly(nil, nil)
	if err != nil {
		t.Fatalf("handleShowAnomaly: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %v, want done", resp.Status)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if enabled, _ := data["enabled"].(bool); !enabled {
		t.Fatal("a detector is loaded, so the report must read enabled")
	}
	rows, ok := data["incidents"].([]plugin.Map)
	if !ok {
		t.Fatalf("incidents is %T, want []plugin.Map", data["incidents"])
	}
	return rows
}

// A source row is unchanged, a dest row shows the dest prefix, and a port row shows
// `proto/port` with the two numbers also carried apart, so a reader never has to
// split the label. It asserts VALUES rather than field presence, and needs no
// traffic generator, so unlike anomaly-entity-matrix.ci it is not blocked on child
// 4's fakeflow.
func TestShowAnomalyEntityLabelByKind(t *testing.T) {
	at := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	rows := showRows(t, []anomalyevent.AnomalyDetected{
		{
			EntityKind: anomalyevent.EntityKindSource,
			Entity:     netip.MustParsePrefix("198.51.100.1/32"),
			Cohort:     "198.51.100.0/24",
			Score:      4.5,
			Severity:   anomalyevent.SeverityMedium,
			At:         at,
		},
		{
			EntityKind: anomalyevent.EntityKindDest,
			Entity:     netip.MustParsePrefix("203.0.113.7/32"),
			Cohort:     "203.0.113.0/24",
			Score:      5,
			Severity:   anomalyevent.SeverityMedium,
			At:         at,
		},
		{
			EntityKind: anomalyevent.EntityKindPort,
			Port:       53,
			Proto:      17,
			Score:      6,
			Severity:   anomalyevent.SeverityMedium,
			At:         at,
		},
	})

	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}

	// Source: identical to what this surface rendered before the entity matrix.
	if got := rows[0]["entity"]; got != "198.51.100.1/32" {
		t.Errorf("source entity = %v, want 198.51.100.1/32", got)
	}
	if got := rows[0]["entity-kind"]; got != anomalyevent.EntityKindSource.String() {
		t.Errorf("source entity-kind = %v, want %q", got, anomalyevent.EntityKindSource.String())
	}

	// Dest: the dest prefix, not the cohort and not a port.
	if got := rows[1]["entity"]; got != "203.0.113.7/32" {
		t.Errorf("dest entity = %v, want 203.0.113.7/32", got)
	}
	if got := rows[1]["entity-kind"]; got != "dest" {
		t.Errorf("dest entity-kind = %v, want dest", got)
	}

	// Port: `proto/port`. A port has no address, so rendering Entity here would
	// print the zero prefix and name no subject at all.
	if got := rows[2]["entity"]; got != "17/53" {
		t.Errorf("port entity = %v, want 17/53", got)
	}
	if got := rows[2]["entity-kind"]; got != "port" {
		t.Errorf("port entity-kind = %v, want port", got)
	}
	if got := rows[2]["port"]; got != uint16(53) {
		t.Errorf("port field = %v (%T), want uint16 53", got, got)
	}
	if got := rows[2]["proto"]; got != uint8(17) {
		t.Errorf("proto field = %v (%T), want uint8 17", got, got)
	}

	// The two numbers are carried APART only for a port incident; an address row
	// that gained them would be reporting a port it does not have.
	for i, kind := range []string{"source", "dest"} {
		if _, ok := rows[i]["port"]; ok {
			t.Errorf("%s row carries a port field", kind)
		}
		if _, ok := rows[i]["proto"]; ok {
			t.Errorf("%s row carries a proto field", kind)
		}
	}
}

// VALIDATES: with no detector loaded the surface says so, rather than reporting an
// empty incident list as if the detector were running and quiet.
func TestShowAnomalyWithNoDetector(t *testing.T) {
	prev := loadGlobalDetector()
	t.Cleanup(func() { setGlobalDetector(prev) })
	setGlobalDetector(nil)

	resp, err := handleShowAnomaly(nil, nil)
	if err != nil {
		t.Fatalf("handleShowAnomaly: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if enabled, _ := data["enabled"].(bool); enabled {
		t.Error("no detector is loaded, so the report must not read enabled")
	}
}
