// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- DDoS event contract

package ddosevent

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/events"
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

// GradeConfidence scores 0-100 how strongly the characterized signals point to a
// real attack rather than a benign traffic spike, from data available at
// characterization time (attack start): the peak-to-threshold ratio, whether a
// specific family (dominant protocol + discriminator) was classified, the extra
// specificity of reflection/SYN floods, and whether the sources are distributed.
// Attack duration is deliberately NOT a factor -- ze grades at attack start, not
// end (unlike ftagent). Annotative: responders/observability consume it; the
// detector's mitigation path keys on Family/Target/Severity.
func GradeConfidence(peakPps, threshold float64, family AttackFamily, entropy, entropyThreshold float64) int {
	conf := 25 // base
	r := 1.0
	if threshold > 0 {
		r = peakPps / threshold
	}
	if r > 0 {
		conf += min(30, int(r*6)) // up to +30 at r>=5
	}
	if family != FamilyGenericFlood {
		conf += 25 // a dominant protocol + discriminator matched
	}
	if family == FamilyReflection || family == FamilySYNFlood {
		conf += 10 // highest-specificity, most actionable families
	}
	if entropyThreshold > 0 && entropy >= entropyThreshold {
		conf += 10 // distributed source spread
	}
	return max(0, min(100, conf))
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

// Direction classifies an attack's victim relative to this box. Local means the
// victim address is owned by the box (control-plane traffic that lands on the
// netfilter INPUT hook); Remote means it is a downstream/transit host reached
// through the FORWARD hook. Derived once at emit via iface.AddressIsLocal; an
// unresolved victim is Remote, because a local INPUT drop cannot protect an
// address the box does not terminate.
type Direction string

const (
	DirectionLocal  Direction = "local"
	DirectionRemote Direction = "remote"
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
	// Direction classifies the victim as local (box-owned) or remote (transit); it
	// routes which mitigation the local responder installs (INPUT vs FORWARD hook).
	Direction Direction `json:"direction,omitempty"`
	// SuppressMitigation is set by the detector's traffic policy when a matching rule
	// exempts this attack from the mitigation ACTION (an allow rule at mitigation scope,
	// or a deny rule at detection scope): observability still records the incident, but
	// responders install no drop/announce. The zero value (false) mitigates, so any
	// emitter that does not set it defends by default (fail-safe). Detection-scoped
	// allow rules suppress the event entirely and never reach a responder.
	SuppressMitigation bool    `json:"suppress-mitigation,omitempty"`
	PeakRxPps          float64 `json:"peak-rx-pps"`
	PeakRxBps          float64 `json:"peak-rx-bps"`
	Observable         bool    `json:"observable"`
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
	// Confidence (0-100) grades how strongly the characterized signals point to a
	// real attack vs a benign spike (see GradeConfidence). Annotative: observability
	// and the dashboard reporter display it, and responders may gate on it; the
	// mitigation vector itself keys on Target/Family/Severity.
	Confidence int `json:"confidence"`
	// Direction and SuppressMitigation carry the same meaning as on AttackDetected.
	// The characterized event is authoritative: when the two-stage policy evaluation
	// flips a decision (a source rule matched once the sources were known), this event
	// carries the final disposition and responders reconcile to it.
	Direction          Direction `json:"direction,omitempty"`
	SuppressMitigation bool      `json:"suppress-mitigation,omitempty"`
	PeakRxPps          float64   `json:"peak-rx-pps"`
	PeakRxBps          float64   `json:"peak-rx-bps"`
	Observable         bool      `json:"observable"`
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
