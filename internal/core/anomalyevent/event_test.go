// VALIDATES: AC-1 the anomaly event contract -- a distinct "anomaly-detect"
// namespace with source/entity-oriented events, and GradeSeverity's 1x/2x/5x
// tiering of an incident score against the emit threshold.
// PREVENTS: collision with the ddos-detect namespace, and a severity misgrade at
// the tier boundaries or on a non-positive threshold.

package anomalyevent

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: child-5 AC-5/AC-7 and R-4 -- the entity-kind tag widens the contract
// WITHOUT changing what a source incident puts on the bus, the zero value reads as
// source so a producer that predates the field stays correct, and a port incident
// carries its port and protocol at both ends of their ranges.
// PREVENTS: a subscriber seeing new keys on a source incident, a port incident
// arriving with no identity, and a zero-value kind reading as "unknown".
func TestEventKindOmitemptyForSource(t *testing.T) {
	if EntityKindSource != "" {
		t.Errorf("EntityKindSource = %q, want the zero value", EntityKindSource)
	}
	if got := EntityKindSource.String(); got != "source" {
		t.Errorf("zero kind renders %q, want source", got)
	}
	if got := EntityKindDest.String(); got != "dest" {
		t.Errorf("dest kind renders %q", got)
	}

	// A source incident: no entity-kind, no port, no proto key in the JSON.
	src := AnomalyDetected{Entity: netip.MustParsePrefix("198.51.100.9/32"), Score: 7}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal source incident: %v", err)
	}
	for _, key := range []string{"entity-kind", "port", "proto"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("source incident JSON carries %q, want it omitted: %s", key, raw)
		}
	}

	// A port incident: kind and identity present, entity the zero prefix. Both
	// extremes of the numeric fields survive the round trip.
	for _, c := range []struct {
		port  uint16
		proto uint8
	}{{0, 0}, {65535, 255}, {31337, 17}} {
		pe := AnomalyDetected{EntityKind: EntityKindPort, Port: c.port, Proto: c.proto, Score: 9}
		raw, err := json.Marshal(pe)
		if err != nil {
			t.Fatalf("marshal port incident: %v", err)
		}
		if !strings.Contains(string(raw), `"entity-kind":"port"`) {
			t.Errorf("port incident JSON missing the kind: %s", raw)
		}
		var back AnomalyDetected
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal port incident: %v", err)
		}
		if back.Port != c.port || back.Proto != c.proto || back.EntityKind != EntityKindPort {
			t.Errorf("port incident round trip = %+v, want port %d proto %d", back, c.port, c.proto)
		}
		if back.Entity.IsValid() {
			t.Errorf("port incident entity = %v, want the zero prefix", back.Entity)
		}
	}

	// Ongoing and Cleared carry the same identity, so a subscriber can tell a dest
	// incident from a source incident on the SAME prefix.
	pfx := netip.MustParsePrefix("203.0.113.7/32")
	on := AnomalyOngoing{EntityKind: EntityKindDest, Entity: pfx}
	cl := AnomalyCleared{EntityKind: EntityKindDest, Entity: pfx}
	if on.EntityKind == EntityKindSource || cl.EntityKind == EntityKindSource {
		t.Error("dest ongoing/cleared must not read as source")
	}
}

func TestAnomalyEventRegisterAndGrade(t *testing.T) {
	if Namespace != "anomaly-detect" {
		t.Errorf("namespace = %q, want anomaly-detect (distinct from ddos-detect)", Namespace)
	}
	if Detected == nil || Ongoing == nil || Cleared == nil {
		t.Fatal("event handles must be registered at init")
	}

	cases := []struct {
		score, threshold float64
		want             Severity
	}{
		{0.5, 1, SeverityMedium},
		{1, 1, SeverityMedium},
		{1.9, 1, SeverityMedium},
		{2, 1, SeverityHigh},
		{4.9, 1, SeverityHigh},
		{5, 1, SeverityCritical},
		{100, 1, SeverityCritical},
		{30, 0, SeverityMedium}, // non-positive threshold -> medium, no divide-by-zero
	}
	for _, c := range cases {
		if got := GradeSeverity(c.score, c.threshold); got != c.want {
			t.Errorf("GradeSeverity(%v, %v) = %q, want %q", c.score, c.threshold, got, c.want)
		}
	}

	ev := &AnomalyDetected{
		Entity:        netip.MustParsePrefix("198.51.100.0/24"),
		FiredFeatures: []FeatureSignal{{Name: "out-in-ratio", Z: 6.2}},
		Score:         6.2,
		Severity:      SeverityCritical,
	}
	if !ev.Entity.IsValid() {
		t.Error("AnomalyDetected.Entity must be a valid source prefix")
	}
	if ev.Score != 6.2 || len(ev.FiredFeatures) != 1 || ev.Severity != SeverityCritical {
		t.Errorf("AnomalyDetected fields not carried: %+v", ev)
	}
}
