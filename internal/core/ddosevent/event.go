// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- DDoS event contract

package ddosevent

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

const Namespace = "ddos-detect"

var (
	Detected      = events.Register[*AttackDetected](Namespace, "attack-detected")
	Characterized = events.Register[*AttackCharacterized](Namespace, "attack-characterized")
	Ongoing       = events.Register[*AttackOngoing](Namespace, "attack-ongoing")
	Cleared       = events.Register[*AttackCleared](Namespace, "attack-cleared")
)

// Severity grades attack intensity from the peak-to-threshold ratio so
// responders can escalate (e.g. the flowspec blackhole-fallback at critical)
// without waiting for characterization. NetHawk-derived 1x/2x/5x tiering; value
// type, no wire/schema change.
type Severity string

const (
	SeverityMedium   Severity = "medium"   // >= 1x threshold (the emitted floor)
	SeverityHigh     Severity = "high"     // >= 2x threshold
	SeverityCritical Severity = "critical" // >= 5x threshold
)

// GradeSeverity grades the peak-to-threshold ratio using NetHawk's 1x/2x/5x
// tiering. Sub-threshold traffic is never emitted, so SeverityMedium is the
// floor for any ratio the detector actually reports. A non-positive threshold
// (no baseline yet) also grades medium rather than dividing by zero.
func GradeSeverity(peakPps, threshold float64) Severity {
	if threshold <= 0 {
		return SeverityMedium
	}
	switch r := peakPps / threshold; {
	case r >= 5:
		return SeverityCritical
	case r >= 2:
		return SeverityHigh
	default:
		return SeverityMedium
	}
}

type AttackFamily string

const (
	FamilyUDPFlood     AttackFamily = "udp-flood"
	FamilySYNFlood     AttackFamily = "syn-flood"
	FamilyICMPFlood    AttackFamily = "icmp-flood"
	FamilyReflection   AttackFamily = "reflection"
	FamilyFragFlood    AttackFamily = "fragment-flood"
	FamilyGenericFlood AttackFamily = "generic-flood"
)

type VectorTuple struct {
	DstPrefix netip.Prefix `json:"dst-prefix"`
	Proto     uint8        `json:"proto"`
	DstPort   uint16       `json:"dst-port"`
	SrcPort   uint16       `json:"src-port,omitempty"`
	TCPFlags  uint8        `json:"tcp-flags,omitempty"`
}

// AttackDetected is the fast "victim identified" signal: emitted on the rate
// trigger once the target prefix is resolved, before full characterization. Its
// Family is generic-flood and Target carries only DstPrefix; the narrowed vector
// arrives in the follow-up AttackCharacterized.
type AttackDetected struct {
	Interface  string       `json:"interface"`
	Target     VectorTuple  `json:"target"`
	Family     AttackFamily `json:"family"`
	TopSources []netip.Addr `json:"top-sources,omitempty"`
	Severity   Severity     `json:"severity,omitempty"`
	PeakRxPps  float64      `json:"peak-rx-pps"`
	PeakRxBps  float64      `json:"peak-rx-bps"`
	Observable bool         `json:"observable"`
}

// AttackCharacterized is the Stage-2 refinement of AttackDetected: it carries the
// classified attack family and the narrowest discriminating VectorTuple (proto +
// discriminating ports + TCP flags) plus ranked top sources and source-address
// entropy, so responders install a surgical rule. Emitted after AttackDetected
// once on-box flow characterization completes; absent when no flow source is
// available (responders keep acting on the coarse AttackDetected).
type AttackCharacterized struct {
	Interface  string       `json:"interface"`
	Target     VectorTuple  `json:"target"`
	Family     AttackFamily `json:"family"`
	TopSources []netip.Addr `json:"top-sources,omitempty"`
	Severity   Severity     `json:"severity,omitempty"`
	// SourceEntropy is the Shannon entropy (bits) of the attack's source-address
	// packet distribution: ~0 for a single source, higher for a distributed or
	// spoofed flood. Annotative; responders act on Target/Family.
	SourceEntropy float64 `json:"source-entropy"`
	PeakRxPps     float64 `json:"peak-rx-pps"`
	PeakRxBps     float64 `json:"peak-rx-bps"`
	Observable    bool    `json:"observable"`
}

type AttackOngoing struct {
	Interface  string      `json:"interface"`
	Target     VectorTuple `json:"target"`
	CurrentPps float64     `json:"current-pps"`
	CurrentBps float64     `json:"current-bps"`
	Observable bool        `json:"observable"`
}

type AttackCleared struct {
	Interface  string      `json:"interface"`
	Target     VectorTuple `json:"target"`
	Observable bool        `json:"observable"`
}
