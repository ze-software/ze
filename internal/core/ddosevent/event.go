// Design: plan/spec-cp-survival-5-detect-0-umbrella.md -- DDoS event contract

package ddosevent

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

const Namespace = "ddos-detect"

var (
	Detected = events.Register[*AttackDetected](Namespace, "attack-detected")
	Ongoing  = events.Register[*AttackOngoing](Namespace, "attack-ongoing")
	Cleared  = events.Register[*AttackCleared](Namespace, "attack-cleared")
)

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

type AttackDetected struct {
	Interface  string       `json:"interface"`
	Target     VectorTuple  `json:"target"`
	Family     AttackFamily `json:"family"`
	TopSources []netip.Addr `json:"top-sources,omitempty"`
	PeakRxPps  float64      `json:"peak-rx-pps"`
	PeakRxBps  float64      `json:"peak-rx-bps"`
	Observable bool         `json:"observable"`
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
