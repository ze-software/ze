// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRP FSM output actions (closed set)
// RFC: rfc/short/rfc9568.md (VRRPv3) and rfc/short/rfc3768.md (VRRPv2)
//
// The FSM returns ordered action VALUES instead of performing effects. The
// engine (spec-vrrp-5) is the sole executor: packet sends via the transport
// (spec-vrrp-4), VIP install/remove via the iface address-owner registry
// (spec-vrrp-3), timers via internal/core/clock. Action order is part of the
// contract; a closed set keeps the executor a dumb dispatcher that cannot
// corrupt protocol logic.
package fsm

import (
	"net/netip"
	"time"
)

// EmitStateChange reason tokens. These are fixed, enumerated strings that feed
// eventbus consumers, metrics labels, and logs in spec-vrrp-5; they are never
// attacker-controlled and must match the State Transition Table verbatim.
const (
	// ReasonStartupOwner: Initialize->Master on owner Startup (RFC 9568 Section 6.4.1).
	ReasonStartupOwner = "startup-owner"
	// ReasonStartup: Initialize->Backup on non-owner Startup (RFC 9568 Section 6.4.1).
	ReasonStartup = "startup"
	// ReasonMasterDownExpired: Backup->Master on down-timer expiry (RFC 9568 Section 6.4.2).
	ReasonMasterDownExpired = "master-down-expired"
	// ReasonPreemptDelayExpired: Backup->Master on preempt-delay expiry (no RFC basis).
	ReasonPreemptDelayExpired = "preempt-delay-expired"
	// ReasonShutdown: Backup/Master->Initialize on Shutdown (RFC 9568 Section 6.4.2/6.4.3).
	ReasonShutdown = "shutdown"
	// ReasonHigherPriority: Master->Backup on a higher-priority advert (RFC 9568 Section 6.4.3).
	ReasonHigherPriority = "higher-priority"
	// ReasonTieBreakLost: Master->Backup on equal priority with greater sender IP (RFC 9568 Section 6.4.3).
	ReasonTieBreakLost = "tie-break-lost"
)

// Action is the closed set of ordered side-effect values the FSM emits. The
// engine executes them in the returned order.
type Action interface {
	isAction()
}

// SendAdvert asks the engine to build and send a VRRP ADVERTISEMENT from THESE
// fields (never from a cached packet, per R-5 / holo bug 8).
//
// RFC 9568 Section 7.2 / RFC 3768 Section 7.2: fill fields from the Virtual
// Router configuration state and transmit.
type SendAdvert struct {
	Priority         uint8 `json:"priority"`
	AdvertIntervalMs int   `json:"advert-interval-ms"`
}

// SendAdvertZeroPriority asks the engine to send a Priority-0 ADVERTISEMENT
// (Master relinquishing).
//
// RFC 9568 Section 6.4.3 / RFC 3768 Section 6.4.3: on Shutdown, Active(Master)
// MUST send an ADVERTISEMENT with Priority = 0 before Initialize.
type SendAdvertZeroPriority struct{}

// InstallVIPs asks the engine to register the full desired virtual-address set
// via the iface address-owner registry (spec-vrrp-3/5).
type InstallVIPs struct {
	VIPs []netip.Addr `json:"vips"`
}

// RemoveVIPs asks the engine to deregister the virtual addresses.
type RemoveVIPs struct {
	VIPs []netip.Addr `json:"vips"`
}

// AnnounceFailover asks the engine to send the gratuitous ARP (per IPv4 VIP) /
// unsolicited NA (per IPv6 VIP) burst that repoints learning bridges and host
// caches (spec-vrrp-4).
//
// RFC 9568 Section 6.4.2 (erratum 7949) / RFC 3768 Section 6.4.2.
type AnnounceFailover struct{}

// StartMasterDownTimer asks the engine to arm/reset the master-down clock.Timer.
// Duration is a time.Duration (nanoseconds); Gen is the staleness generation.
type StartMasterDownTimer struct {
	Duration time.Duration `json:"duration"`
	Gen      uint64        `json:"gen"`
}

// StartAdvertTimer asks the engine to arm/reset the advert clock.Timer at the
// router's own configured interval.
type StartAdvertTimer struct {
	Interval time.Duration `json:"interval"`
	Gen      uint64        `json:"gen"`
}

// StartPreemptDelayTimer asks the engine to arm the preempt-delay clock.Timer
// (Junos hold-time; no RFC basis).
type StartPreemptDelayTimer struct {
	Duration time.Duration `json:"duration"`
	Gen      uint64        `json:"gen"`
}

// StopPreemptDelayTimer asks the engine to cancel only the preempt-delay timer.
type StopPreemptDelayTimer struct{}

// StopTimers asks the engine to cancel all three timers (master-down, advert,
// preempt-delay).
type StopTimers struct{}

// EmitStateChange asks the engine to publish a state transition on the eventbus,
// increment metrics, and log it. Reason is one of the Reason* constants.
type EmitStateChange struct {
	From   State  `json:"from"`
	To     State  `json:"to"`
	Reason string `json:"reason"`
}

func (SendAdvert) isAction()             {}
func (SendAdvertZeroPriority) isAction() {}
func (InstallVIPs) isAction()            {}
func (RemoveVIPs) isAction()             {}
func (AnnounceFailover) isAction()       {}
func (StartMasterDownTimer) isAction()   {}
func (StartAdvertTimer) isAction()       {}
func (StartPreemptDelayTimer) isAction() {}
func (StopPreemptDelayTimer) isAction()  {}
func (StopTimers) isAction()             {}
func (EmitStateChange) isAction()        {}
