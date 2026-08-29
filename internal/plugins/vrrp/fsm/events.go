// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRP FSM input events and instance config
// RFC: rfc/short/rfc9568.md (VRRPv3) and rfc/short/rfc3768.md (VRRPv2)
//
// Typed input events consumed by Instance.Handle. Each event is a small value
// type carrying only decoded, pre-validated fields (no wire bytes, no packet
// types from spec-vrrp-1); the codec boundary lives entirely in the engine
// (spec-vrrp-5), which builds these events.
package fsm

import "net/netip"

// Config is the resolved per-instance VRRP configuration embedded in the Startup
// and ConfigUpdated events. The engine (spec-vrrp-5) builds it from validated
// YANG config; the FSM treats every field as a precondition (ranges are checked
// upstream, asserted here only in tests).
//
// RFC 9568 Section 5.2.4 / RFC 3768 Section 5.3.4: "The priority value for the
// VRRP Router that owns the IPvX address ... MUST be 255"; Backups "MUST use
// priority values between 1-254".
type Config struct {
	// Version is the wire protocol version, 2 (RFC 3768) or 3 (RFC 9568),
	// fixed per instance at Startup.
	Version uint8 `json:"version"`
	// IsOwner is true when this router owns the virtual address(es); it forces
	// Priority 255 and unconditional preemption (RFC 9568 Section 6.1).
	IsOwner bool `json:"is-owner"`
	// Priority is 1..254 for a Backup, 255 iff IsOwner. 0 is wire-only
	// (Master releasing) and is never a configured local priority (A-5).
	Priority uint8 `json:"priority"`
	// Preempt defaults true (RFC). Consulted only in Backup.
	Preempt bool `json:"preempt"`
	// PreemptDelayMs is the Junos-style hold-time in milliseconds; 0 disables
	// delayed preemption (the RFC-pure default). No RFC basis; see the spec's
	// Preempt-Delay Semantics section.
	PreemptDelayMs int `json:"preempt-delay-ms"`
	// AdvertIntervalMs is this router's own advertisement interval in
	// milliseconds (validated per version: v3 10..40950, v2 1000..255000).
	AdvertIntervalMs int `json:"advert-interval-ms"`
	// LocalPrimaryIP is the tie-break operand: the primary IPv4 address or the
	// link-local IPv6 address of the sending interface (RFC 9568 Section 6.4.3).
	LocalPrimaryIP netip.Addr `json:"local-primary-ip"`
	// VIPs is the full desired virtual-address set, used for the
	// InstallVIPs/RemoveVIPs payloads.
	VIPs []netip.Addr `json:"vips"`
	// AcceptMode is the EFFECTIVE Accept_Mode: the configured v3-only leaf with
	// the RFC 9568 Section 6.1 ownership exemption already folded in, so it is
	// true for an address owner whatever the operator configured
	// (EffectiveAcceptMode, internal/plugins/vrrp/groups.go).
	//
	// It reaches the dataplane. A change to it while Active re-emits InstallVIPs
	// (masterConfigUpdated, fsm.go), and the executor turns it into the packet
	// filter RFC 9568 Section 6.4.3 requires
	// (doInstallVIPs, internal/plugins/vrrp/instance.go).
	AcceptMode bool `json:"accept-mode"`
}

// Event is the closed set of typed inputs the FSM consumes. Exactly one Event is
// passed to Instance.Handle per call.
type Event interface {
	isEvent()
}

// Startup begins an instance. Produced by the engine on instance start (config
// commit/apply and parent-link readiness).
//
// RFC 9568 Section 6.4.1 / RFC 3768 Section 6.4.1: Initialize transitions on
// Startup, the address owner to Active(Master), a non-owner to Backup.
type Startup struct {
	Config Config `json:"config"`
}

// Shutdown stops an instance. Produced by the engine on config removal, plugin
// stop, or parent-link down.
//
// RFC 9568 Section 6.4.2/6.4.3 / RFC 3768 Section 6.4.2/6.4.3: Backup cancels its
// timer; Active(Master) sends a Priority-0 advertisement first.
type Shutdown struct{}

// AdvertReceived carries a decoded, receive-validated VRRP ADVERTISEMENT. The
// engine (spec-vrrp-5) runs packet.Decode + validation (spec-vrrp-1) over raw
// packets from the transport (spec-vrrp-4); malformed or failed-validation
// packets never become events.
type AdvertReceived struct {
	// Priority is the sender's advertised priority; 0 means the sender is
	// relinquishing (RFC 9568 Section 5.2.4).
	Priority uint8 `json:"priority"`
	// SrcIP is the sender's primary IPvX source address, the tie-break operand
	// consulted only in Master state.
	SrcIP netip.Addr `json:"src-ip"`
	// IntervalMs is the sender's Max Advertise Interval already converted to
	// milliseconds by the codec (spec-vrrp-1); v3 Backups adopt it.
	IntervalMs int `json:"interval-ms"`
	// VIPCount is the advertised IPvX address count (diagnostic; >= 1 enforced
	// upstream per RFC 9568 erratum 8299).
	VIPCount int `json:"vip-count"`
}

// MasterDownExpired is delivered by the engine's master-down clock.Timer,
// echoing the Gen of the arming StartMasterDownTimer action.
//
// RFC 9568 Section 6.4.2 / RFC 3768 Section 6.4.2: Backup promotes when the
// down timer fires.
type MasterDownExpired struct {
	Gen uint64 `json:"gen"`
}

// AdvertTimerExpired is delivered by the engine's advert clock.Timer, echoing the
// Gen of the arming StartAdvertTimer action.
//
// RFC 9568 Section 6.4.3 / RFC 3768 Section 6.4.3: Active(Master) sends an
// advertisement each interval and resets the timer.
type AdvertTimerExpired struct {
	Gen uint64 `json:"gen"`
}

// PreemptDelayExpired is delivered by the engine's preempt-delay clock.Timer,
// echoing the Gen of the arming StartPreemptDelayTimer action. No RFC basis;
// vendor extension (Junos hold-time semantics).
type PreemptDelayExpired struct {
	Gen uint64 `json:"gen"`
}

// ConfigUpdated re-applies configuration to a running instance. Produced by the
// engine on config apply.
type ConfigUpdated struct {
	Config Config `json:"config"`
}

func (Startup) isEvent()             {}
func (Shutdown) isEvent()            {}
func (AdvertReceived) isEvent()      {}
func (MasterDownExpired) isEvent()   {}
func (AdvertTimerExpired) isEvent()  {}
func (PreemptDelayExpired) isEvent() {}
func (ConfigUpdated) isEvent()       {}
