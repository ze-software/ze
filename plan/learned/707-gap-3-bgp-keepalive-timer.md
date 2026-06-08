# 707 -- gap-3-bgp-keepalive-timer

Spec: spec-gap-3-bgp-keepalive-timer.md
Date: 2026-05-15

## What this was

Configurable BGP keepalive timer. Ze derived keepalive as holdTime/3
(RFC 4271 Section 10 default) with no way to override it. Operators migrating
from VyOS need `timers keepalive 10` on BGP neighbors. This adds a `keepalive`
leaf to the YANG timer container: 0 = auto (hold/3), non-zero overrides.

## Architecture decisions

1. **Single override point in StartKeepaliveTimer.** The keepalive interval was
   computed at exactly one place (`keepaliveInterval := t.holdTime / 3`). The
   override is a three-line `if` immediately after that derivation. No new
   methods, no new timer, no structural change to the FSM.

2. **Negotiation clamping in session_negotiate.go.** RFC 4271 Section 4.2
   negotiates hold-time as min(local, peer). If the peer proposes a lower
   hold-time, a configured keepalive could exceed it, causing immediate session
   flap. The negotiation code clamps keepalive back to negotiatedHold/3 when
   this happens. This was not in the original spec; found during pre-implementation
   RFC cross-check.

3. **Validation at config parse, not at timer start.** Rejecting keepalive >=
   hold-time at parse time (config.go) gives the operator an immediate error on
   commit, rather than a silent runtime surprise. The hot-config path
   (reactor_api.go) warns and ignores instead of erroring, matching the pattern
   used by send-hold-time validation.

## What surprised us

1. **The spec missed two RFC constraints.** The original skeleton had the
   holdTime/3 default and the keepalive < hold-time validation, but missed:
   (a) the 1-second floor from RFC 4271 Section 4.4 ("KEEPALIVE messages MUST
   NOT be sent more frequently than one per second"), and (b) the negotiation
   clamping problem where a peer's lower hold-time invalidates the configured
   keepalive. Both were caught by reading the full RFC text before implementing.
   The 1-second floor turned out to be enforced implicitly by YANG uint16
   (minimum non-zero value is 1).

2. **Two config parsers, different tree shapes.** `config.go:parsePeerFromTree`
   reads from a nested YANG tree (timer fields under a `timer` map).
   `reactor_api.go:parsePeersFromTree` reads from a flattened tree (timer
   fields at peer level). Both needed the keepalive extraction, following their
   respective patterns. Easy to miss the second parser.

## Mistakes and corrections (review findings)

| Finding | Severity | Root cause |
|---------|----------|------------|
| Missing config validation test (AC-3) | ISSUE | Tests written for FSM but not for config rejection path |
| Missing negotiation clamping test | ISSUE | New RFC-driven logic had no dedicated test |
| Redundant YANG range `0..65535` on uint16 | NOTE | Copy-paste from receive-hold-time which has a real range constraint |
| Stale comment in save.go enumerating timer fields | NOTE | Comment listed individual fields instead of being generic |

All found and fixed during `/ze-review` passes before commit.

## Files

| Area | Files | Lines |
|------|-------|-------|
| YANG schema | `bgp/yang/ze-bgp-conf.yang` | +6 |
| FSM timer | `bgp/fsm/timer.go` | +21 |
| FSM timer tests | `bgp/fsm/timer_test.go` | +123 |
| PeerSettings | `bgp/reactor/peersettings.go` | +4 |
| Config parse | `bgp/reactor/config.go` | +7 |
| Config test | `bgp/reactor/config_test.go` | +40 |
| Session init | `bgp/reactor/session.go` | +1 |
| Negotiation | `bgp/reactor/session_negotiate.go` | +6 |
| Hot-config / API | `bgp/reactor/reactor_api.go` | +11 |
| PeerInfo type | `plugin/types_bgp.go` | +1 |
| CLI display | `bgp/plugins/cmd/peer/peer.go` | +1 |
| CLI save | `bgp/plugins/cmd/peer/save.go` | +6 |
| Total | 12 files | ~227 |
