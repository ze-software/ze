# Spec: bgp-mrai-pacing

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Unapproved interval policy; candidate tests must not be counted as mandatory conformance failures until their interval assumption is adopted |
| RFC gaps | RFC4271-9.2.1.1-2, RFC4271-9.2.1.1-3 |
| Depends | - |
| Updated | 2026-11-15 |

## Task

Decide Ze's route-advertisement interval policy and turn the test campaign's
draft pacing probes into tests of that adopted policy. Keep this work visible
for later review while the remaining RFC test batches proceed, as the owner
instructed on 15 November. No timer design or nonzero default is approved here.

No existing MRAI feature spec was found. The current three tests in
`internal/component/bgp/reactor/rfc4271_mrai_candidate_test.go` propose the
suggested 30-second eBGP interval, an immediately eligible first advertisement,
and jitter between 22.5 and 30 seconds. They are explicitly untagged policy
candidates, not proof that current output violates a mandatory RFC obligation.
They compiled and ran in
`tmp/session/2026-11-14-210034/scratch/job-rfc-campaign-new-batch-reds-37b87864.log`.
Each fails at its early-route assertion after the first positive route reaches
the socket. Later coalescing and cancellation assertions are not yet reached.
Retain that log until an adopted-policy run supersedes the candidate result.

## Authority

- RFC 4271 Section 9.2.1.1:
  "Two UPDATE messages sent by a BGP speaker to a peer that advertise
  feasible routes and/or withdrawal of unfeasible routes to some common
  set of destinations MUST be separated by at least
  MinRouteAdvertisementIntervalTimer."
  → Constraint: with a positive interval, announcements and withdrawals for
  common destinations share the spacing requirement. Conservative batching
  across unrelated destinations is permitted; do not require unrelated
  prefixes to transmit immediately.
- RFC 4271 Section 9.2.1.1:
  "If new routes are selected multiple times while awaiting the expiration of
  MinRouteAdvertisementIntervalTimer, the last route selected SHALL be
  advertised at the end of MinRouteAdvertisementIntervalTimer."
  → Constraint: queued work must retain the latest selection and cannot
  resurrect a route after withdrawal.
- RFC 4271 Section 10 suggests 30 seconds for eBGP and five seconds for iBGP.
  It also states: "An implementation of BGP MUST allow the HoldTimer to be
  configurable on a per-peer basis, and MAY allow the other timers to be
  configurable."
  → Constraint: configurable MRAI and the suggested default are distinct
  policy choices; neither may be promoted into an unconditional MUST.

## Existing Mechanisms and Research

| Mechanism | Current evidence | Design constraint |
|-----------|------------------|-------------------|
| Session timers | `internal/component/bgp/fsm/timer.go` documents MRAI as absent | Do not count an unused timer declaration as pacing |
| Actual writer | `internal/component/bgp/reactor/session_write.go`, `sendUpdateCounted` | Tests observe real UPDATE bytes after a real Peer handshake |
| Current candidate tests | Three tests cover spacing, latest replacement and withdrawal cancellation | Bind them to the adopted interval before RFC tags or passing proof claims |
| FRR reference | Upstream master `bgpd/bgp_fsm.c`, `bgp_adjust_routeadv`, inspected 2026-11-15 | Zero bypasses pacing; a positive interval schedules remaining time. This is an unpinned research snapshot, not a versioned interop result |
| BIRD reference | Upstream master `proto/bgp/attrs.c`, `bgp_get_prefix` and `bgp_rt_notify`, inspected 2026-11-15 | Existing bucket replacement illustrates coalescing; no nonzero default was established by this review |

## Questions for Later Design Review

1. Which fixed/default interval should Ze adopt, and should operators be able
   to configure it? A zero interval must be distinguished from an absent or
   ineffective positive interval.
2. Should the first selected route be immediately eligible? The draft assumes
   this, but that assumption is not a requirement extracted from the RFC.
3. Which existing destination ownership and writer lifecycle should hold pending
   work, so peer teardown and withdrawals cancel it without bypassing ownership?
4. Which versioned FRR/BIRD scenario will prove the chosen wire behavior?

## Validation Owed

The three candidate tests ran and failed at their early-route assertion after
the first valid route reached the socket, as recorded above. After policy
approval, cover the chosen nonzero interval, latest selection at expiry,
withdrawal cancellation and teardown/reconnect ownership. Keep positive live
route controls. Record actual passing-baseline/failing-break evidence before
adding requirement tags or upgrading any mandatory row. No full RFC support or
completed mandatory batch is claimed by this proposal.
