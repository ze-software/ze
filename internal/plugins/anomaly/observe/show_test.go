package observe

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/anomalyevent"
)

// showData runs the handler and returns its response map, failing the test when the
// handler errors or answers with anything else.
func showData(t *testing.T) plugin.Map {
	t.Helper()
	resp, err := handleShowAnomalyObserve(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %v, want done", resp.Status)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	return m
}

// wantEmptyIncidentsJSON asserts the response encodes an empty JSON ARRAY, never
// null. A nil []incident type-asserts to []incident and reports length 0, so only
// the marshaled form tells the two apart, and only the marshaled form is what the
// CLI and every JSON consumer actually reads: `null` breaks a caller that iterates.
func wantEmptyIncidentsJSON(t *testing.T, m plugin.Map) {
	t.Helper()
	encoded, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"incidents":[]`) {
		t.Errorf("encoded response = %s, want an empty incidents array", encoded)
	}
}

// VALIDATES: AC-6 -- with no store published (the plugin is not running) the
// handler answers done with enabled false, a zero active count, and an empty
// incident list rather than a nil one.
// PREVENTS: the CLI seeing an error or a null list when the plugin is absent, and a
// nil map dereference in the handler.
func TestShowAnomalyObserveNoStore(t *testing.T) {
	activeStore.Store(nil)

	m := showData(t)
	if m["enabled"] != false {
		t.Errorf("enabled = %v, want false", m["enabled"])
	}
	if m["active-count"] != 0 {
		t.Errorf("active-count = %v, want 0", m["active-count"])
	}
	incidents, ok := m["incidents"].([]incident)
	if !ok {
		t.Fatalf("incidents is %T, want []incident", m["incidents"])
	}
	if len(incidents) != 0 {
		t.Errorf("incidents = %v, want an empty list", incidents)
	}
	wantEmptyIncidentsJSON(t, m)
}

// VALIDATES: AC-6 -- a running plugin with no traffic answers enabled true,
// active-count zero, and an empty list.
// PREVENTS: the operator being unable to tell "not running" from "running and
// quiet", which is what the .ci asserts on the real daemon.
func TestShowAnomalyObserveEmptyStore(t *testing.T) {
	activeStore.Store(newStore(10, time.Hour))
	t.Cleanup(func() { activeStore.Store(nil) })

	m := showData(t)
	if m["enabled"] != true {
		t.Errorf("enabled = %v, want true", m["enabled"])
	}
	if m["active-count"] != 0 {
		t.Errorf("active-count = %v, want 0", m["active-count"])
	}
	if incidents, ok := m["incidents"].([]incident); !ok || len(incidents) != 0 {
		t.Errorf("incidents = %v (%T), want an empty []incident", m["incidents"], m["incidents"])
	}
	wantEmptyIncidentsJSON(t, m)
}

// VALIDATES: AC-7 -- an active incident surfaces with its entity, cohort, score,
// severity, start time and active flag, and active-count counts it.
// PREVENTS: the store staying unreachable from the CLI, which is what left the ddos
// template's list() dead for a release.
func TestShowAnomalyObserveWithStore(t *testing.T) {
	entity := netip.MustParsePrefix("10.0.0.9/32")
	confirmed := time.Now().Add(-time.Minute)
	s := newStore(10, time.Hour)
	s.open(&anomalyevent.AnomalyDetected{
		Interface: "xe0",
		Entity:    entity,
		Cohort:    "10.0.0.0/24",
		Score:     7.25,
		Severity:  anomalyevent.SeverityCritical,
		At:        confirmed,
	})
	activeStore.Store(s)
	t.Cleanup(func() { activeStore.Store(nil) })

	m := showData(t)
	if m["enabled"] != true {
		t.Errorf("enabled = %v, want true", m["enabled"])
	}
	if m["active-count"] != 1 {
		t.Errorf("active-count = %v, want 1", m["active-count"])
	}
	incidents, ok := m["incidents"].([]incident)
	if !ok {
		t.Fatalf("incidents is %T, want []incident", m["incidents"])
	}
	if len(incidents) != 1 {
		t.Fatalf("got %d incidents, want 1", len(incidents))
	}
	got := incidents[0]
	if got.Entity != entity {
		t.Errorf("entity = %s, want %s", got.Entity, entity)
	}
	if got.Cohort != "10.0.0.0/24" || got.Score != 7.25 {
		t.Errorf("incident = %+v, want the cohort and score of the event", got)
	}
	if got.Severity != anomalyevent.SeverityCritical {
		t.Errorf("severity = %q, want critical", got.Severity)
	}
	if !got.StartTime.Equal(confirmed) {
		t.Errorf("start-time = %s, want %s", got.StartTime, confirmed)
	}
	if !got.Active {
		t.Error("active = false, want true for an open incident")
	}
}

// VALIDATES: a finalized incident stays in the answer and carries its end time --
// the history the detect report ring cannot show, which is why this plugin exists.
// PREVENTS: the handler filtering to active incidents only, which would leave the
// operator with exactly the surface the detect ring already gives.
func TestShowAnomalyObserveKeepsFinalizedIncident(t *testing.T) {
	entity := netip.MustParsePrefix("10.0.0.9/32")
	s := newStore(10, time.Hour)
	s.open(&anomalyevent.AnomalyDetected{Entity: entity, At: time.Now()})
	s.finalize(entity)
	activeStore.Store(s)
	t.Cleanup(func() { activeStore.Store(nil) })

	m := showData(t)
	if m["active-count"] != 0 {
		t.Errorf("active-count = %v, want 0", m["active-count"])
	}
	incidents, ok := m["incidents"].([]incident)
	if !ok {
		t.Fatalf("incidents is %T, want []incident", m["incidents"])
	}
	if len(incidents) != 1 {
		t.Fatalf("got %d incidents, want the finalized one to remain", len(incidents))
	}
	if incidents[0].Active {
		t.Error("the finalized incident must report active false")
	}
	if incidents[0].EndTime.IsZero() {
		t.Error("the finalized incident must carry an end-time")
	}
}
