// Design: docs/architecture/anomaly/anomaly-1-detect.md -- behavioral anomaly event contract
//
// The SECURITY anomaly domain's event contract: source/entity-oriented, deliberately
// SEPARATE from the destination-oriented ddosevent (they share no namespace, struct,
// or detector). A behavioral detector emits these; a responder (Spec 2b anomaly/shape)
// subscribes. Value types only -- no wire or schema change.

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
type FeatureSignal struct {
	Name string  `json:"name"`
	Z    float64 `json:"z"`
}

// AnomalyDetected is the confirmed-incident signal: emitted once when an entity's
// correlated deviation score crosses the emit threshold for the confirm window.
// The entity is a SOURCE prefix (behavioral subject), not a destination tuple.
type AnomalyDetected struct {
	Interface     string          `json:"interface,omitempty"`
	Entity        netip.Prefix    `json:"entity"`
	Cohort        string          `json:"cohort,omitempty"`
	FiredFeatures []FeatureSignal `json:"fired-features,omitempty"`
	Score         float64         `json:"score"`
	Severity      Severity        `json:"severity,omitempty"`
	At            time.Time       `json:"at"`
	Observable    bool            `json:"observable"`
}

// AnomalyOngoing is the lightweight "still anomalous" update while an incident is
// active.
type AnomalyOngoing struct {
	Entity     netip.Prefix `json:"entity"`
	Score      float64      `json:"score"`
	Observable bool         `json:"observable"`
}

// AnomalyCleared marks an incident resolved (score fell below threshold for the
// clear window).
type AnomalyCleared struct {
	Entity     netip.Prefix `json:"entity"`
	Observable bool         `json:"observable"`
}
