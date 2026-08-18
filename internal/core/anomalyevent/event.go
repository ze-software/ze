// Design: docs/architecture/anomaly/anomaly-1-detect.md -- behavioral anomaly event contract
//
// The SECURITY anomaly domain's event contract: entity-oriented, deliberately
// SEPARATE from the destination-oriented ddosevent (they share no namespace, struct,
// or detector). A behavioral detector emits these; a responder (Spec 2b anomaly/shape)
// subscribes. Value types only -- no wire or schema change.
//
// An incident's entity sits on one of three axes (EntityKind): the SENDER, the
// RECEIVER, or the destination SERVICE PORT. The kind decides what a subscriber may
// do with it, and a response that acts on an address MUST act on the sender kind
// alone: acting on a receiver would throttle the victim.

package anomalyevent

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/events"
)

// Namespace is the anomaly domain's event namespace, distinct from ddosevent's
// "ddos-detect".
const Namespace = "anomaly-detect"

var (
	// Detected fires once per incident on confirmation.
	Detected = events.Register[*AnomalyDetected](Namespace, "anomaly-detected")
	// Ongoing fires while an incident remains above threshold.
	Ongoing = events.Register[*AnomalyOngoing](Namespace, "anomaly-ongoing")
	// Cleared fires when an incident resolves below threshold.
	Cleared = events.Register[*AnomalyCleared](Namespace, "anomaly-cleared")
)

// Severity grades incident intensity from the score-to-threshold ratio so a
// responder can escalate. NetHawk-style 1x/2x/5x tiering, mirroring ddosevent.
type Severity string

const (
	SeverityMedium   Severity = "medium"   // >= 1x threshold (the emitted floor)
	SeverityHigh     Severity = "high"     // >= 2x threshold
	SeverityCritical Severity = "critical" // >= 5x threshold
)

// GradeSeverity grades an incident score against the emit threshold using 1x/2x/5x
// tiering. Sub-threshold incidents are never emitted, so SeverityMedium is the
// floor. A non-positive threshold (no baseline yet) grades medium rather than
// dividing by zero.
func GradeSeverity(score, threshold float64) Severity {
	if threshold <= 0 {
		return SeverityMedium
	}
	switch r := score / threshold; {
	case r >= 5:
		return SeverityCritical
	case r >= 2:
		return SeverityHigh
	default:
		return SeverityMedium
	}
}

// FeatureSignal names one neutral feature that deviated for an entity and the
// normalized deviation (z-score) that made it fire. Annotative; the responder
// acts on Entity, not the individual signals.
//
// The six names are one fixed vocabulary across every entity kind, and each reads
// in the kind's own direction: "fan-out" is a source's destination count, a
// destination's source count, and a port's source count.
type FeatureSignal struct {
	Name string  `json:"name"`
	Z    float64 `json:"z"`
}

// EntityKind names which axis an incident's entity sits on, so a subscriber knows
// what Entity, Port and Proto mean before it acts on them.
//
// The zero value is EntityKindSource. That is deliberate: an event a producer built
// before this field existed reads as a source incident, and a responder that acts
// only on sources stays correct without a version check. A guard MUST test for
// EntityKindSource rather than against the other kinds, so a kind added later is
// refused rather than silently acted on.
type EntityKind string

const (
	// EntityKindSource is an anomalous SENDER, identified by Entity. Zero value.
	EntityKindSource EntityKind = ""
	// EntityKindDest is an anomalous RECEIVER (a distributed sink, a probed host),
	// identified by Entity. It is the victim, never the actor.
	EntityKindDest EntityKind = "dest"
	// EntityKindPort is an anomalous destination SERVICE PORT, identified by Port
	// and Proto. Entity is the zero prefix: a port is not an address.
	EntityKindPort EntityKind = "port"
)

// String names the kind for display. The zero value renders "source".
func (k EntityKind) String() string {
	if k == EntityKindSource {
		return "source"
	}
	return string(k)
}

// AnomalyDetected is the confirmed-incident signal: emitted once when an entity's
// correlated deviation score crosses the emit threshold for the confirm window.
//
// EntityKind says which axis the entity sits on. For a source or a destination the
// subject is Entity, a prefix; for a port it is Port and Proto, and Entity is the
// zero prefix. A source incident marshals exactly as it did before the kind existed,
// because the source kind and the zero port are all omitted.
type AnomalyDetected struct {
	Interface     string          `json:"interface,omitempty"`
	EntityKind    EntityKind      `json:"entity-kind,omitempty"`
	Entity        netip.Prefix    `json:"entity"`
	Port          uint16          `json:"port,omitempty"`
	Proto         uint8           `json:"proto,omitempty"`
	Cohort        string          `json:"cohort,omitempty"`
	FiredFeatures []FeatureSignal `json:"fired-features,omitempty"`
	Score         float64         `json:"score"`
	Severity      Severity        `json:"severity,omitempty"`
	At            time.Time       `json:"at"`
	Observable    bool            `json:"observable"`
}

// AnomalyOngoing is the lightweight "still anomalous" update while an incident is
// active. It carries the same entity identity as the AnomalyDetected that opened
// the incident, so a subscriber can match the two without keeping its own index.
type AnomalyOngoing struct {
	EntityKind EntityKind   `json:"entity-kind,omitempty"`
	Entity     netip.Prefix `json:"entity"`
	Port       uint16       `json:"port,omitempty"`
	Proto      uint8        `json:"proto,omitempty"`
	Score      float64      `json:"score"`
	Observable bool         `json:"observable"`
}

// AnomalyCleared marks an incident resolved (score fell below threshold for the
// clear window). It carries the entity identity for the same reason AnomalyOngoing
// does: a destination incident and a source incident can name the SAME prefix, and
// a responder that ignored the kind would withdraw the wrong one.
type AnomalyCleared struct {
	EntityKind EntityKind   `json:"entity-kind,omitempty"`
	Entity     netip.Prefix `json:"entity"`
	Port       uint16       `json:"port,omitempty"`
	Proto      uint8        `json:"proto,omitempty"`
	Observable bool         `json:"observable"`
}
