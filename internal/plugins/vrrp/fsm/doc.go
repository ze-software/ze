// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRP instance state machine and timers
// RFC: rfc/short/rfc9568.md (VRRPv3) and rfc/short/rfc3768.md (VRRPv2)
//
// Package fsm implements the per-group VRRP instance state machine and timer
// arithmetic for RFC 9568 (VRRPv3, default) and RFC 3768 (VRRPv2, opt-in).
//
// # Invariants
//
// The FSM is a pure, deterministic, single-threaded, actions-as-values machine:
//
//   - Inputs are typed events (Startup, Shutdown, AdvertReceived,
//     MasterDownExpired, AdvertTimerExpired, PreemptDelayExpired, ConfigUpdated)
//     plus the current time read from an injected clock.Clock (timestamps only,
//     never scheduling).
//   - Outputs are an ordered slice of action values (SendAdvert,
//     SendAdvertZeroPriority, InstallVIPs, RemoveVIPs, AnnounceFailover,
//     StartMasterDownTimer, StartAdvertTimer, StartPreemptDelayTimer,
//     StopPreemptDelayTimer, StopTimers, EmitStateChange). Action order is part
//     of the contract.
//   - The FSM performs NO I/O: no sockets, no netlink, no direct wall-clock
//     scheduling, no goroutines, no locks. Every side effect is an action value
//     the engine (spec-vrrp-5) executes.
//   - It reads clock.Now() only, for state-entry timestamps and last-advert
//     bookkeeping surfaced by Snapshot for `show vrrp`.
//
// The engine owns the three clock.Timer values (master-down, advert,
// preempt-delay), selects on their channels, and re-enters Handle with the
// matching expiry event. Because clock.Timer.Reset can leave an already-fired
// tick queued, every Start* action and every expiry event carries a monotonic
// Gen; the FSM ignores any expiry whose Gen does not match the currently armed
// generation for that timer role (see timers.go).
//
// The FSM trusts its caller's threading contract: exactly one goroutine calls
// Handle. It contains no locks by design and is NOT safe for concurrent use.
//
// # Terminology
//
// RFC 9568 renamed "Master" to "Active Router". This package keeps
// Initialize/Backup/Master to match the operator vocabulary of keepalived,
// Junos, and the `show vrrp` CLI surface. In the RFC citations below, Master ==
// Active.
package fsm
