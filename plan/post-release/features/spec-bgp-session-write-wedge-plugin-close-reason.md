# Spec: a session ze closed for a write budget reaches plugins as a local close

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Superseded on 2026-11-14: the work is release work, owned by residual R-2 of `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`. This file holds no work and is to be removed |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Superseded on 2026-11-14 by residual R-2 of
`plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` (its OD-3 and "R-2:
causes and readers"), by the owner's decision: "Move it to the BMP".** Everything
recorded below moved there. Do not design or implement from this file: it holds
no work and is to be removed. The text below is kept as written until then.

Created after the closure of `spec-bgp-session-write-wedge` (closure record in
commit 8de10c4d54), whose Known Limitations recorded: "A budget expiry is still
reported to plugins as a transport failure (`rpc.ReasonTCPFailure`), as a
failed write was before this spec; whether a local-close reason fits better
was not decided here".

The owner's first answer, 2026-11-14, superseded the same day (see above). Asked
"Should a write-budget close reach plugins as a local close rather than a TCP
failure?", Thomas answered: "Yes, it is an improvement we should probably do
after release."

What is true today, read at the producers on 2026-11-14:

- The reason a plugin reads on a session down is derived once, by
  `sessionCloseFor` (`internal/component/bgp/reactor/peer_close.go`), from four
  inputs: whether a NOTIFICATION crossed the wire, whether the receive
  publication failed, the FSM event that left Established, and the session's
  recorded close reason.
- The reasons are the `Reason*` constants of `pkg/plugin/rpc/enums.go`:
  `notification`, `adj-rib-in-invalid`, `tcp-failure`, `local-close`, and
  `removed` for a peer taken out of the configuration.
  `docs/architecture/api/architecture.md`, "Close Reasons", is their page.
- `local-close` already exists (`rpc.ReasonLocalClose`). `sessionCloseFor`
  gives it for every event but Event 18, and for Event 18 only when the
  recorded cause is `ErrCongestionTeardown`. Any other cause on Event 18 gives
  `tcp-failure`, a recorded `ErrWriteBudgetExpired`
  (`session_write_budget.go` `recordWriteFailure`) included.
- A close that met a held write path adds `ErrWritePathHeld` to the cause
  (`leaveCause`, `peer_close.go`). `sessionCloseFor` does not read it: the
  reason follows the event of the closer. A teardown that recorded its stop
  event leaves on that event (`Session.endEvent`, `session_connection.go`) and
  is `local-close` today; a close with no teardown leaves on Event 18 and is
  `tcp-failure`.
- The operator is told the cause on both paths (`doctor-bgp-peer-not-reading`,
  `doctor-bgp-write-path-held`), and `sessionDroppedOwed` raises no second
  event. Only the write budget has a last-error record
  (`plugin.CloseCauseWriteBudget`, `PeerInfo.LastCloseCause`); the held path
  has none.

What a plugin cannot tell: a session the peer or the network dropped from a
session ze closed because the peer accepted no data for the write budget. Both
arrive as `tcp-failure` with Event 18. Readers of the reason today:

- BMP: `peerDownFor` (`internal/component/bgp/plugins/bmp/bmp_events.go`) sends
  Peer Down reason 4 (the remote system closed) for `tcp-failure`, and reason 2
  (the local system closed) for anything else.
- Graceful Restart: `rpc.ReasonForbidsRetention` forbids retention only for
  `notification` and `adj-rib-in-invalid`, so the two reasons retain alike.
- The ExaBGP bridge: `exabgpDownReasons`
  (`internal/exabgp/bridge/bridge_event_state.go`) writes `closing connection`
  for both, so a migrated script sees no change.
- A third-party plugin, a route server or a monitor, that treats a transport
  failure differently from a close ze chose.

Goal: a session ze closed itself for a write budget expiry, or whose close met
a held write path, reaches plugins with a reason that says ze closed it.

Constraint found while writing this skeleton, settled on 2026-11-14 (see the
top of this section). The mechanism already has an owner in the release bucket:
`plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` (Status `design`,
P1), residual R-2: "A local close that fires Event 18 with no NOTIFICATION,
other than the congestion drop, may be reported as reason 4 (remote)", producer
`sessionCloseFor`. A write budget expiry is a member of that class, and R-2 now
takes it.

What the design must settle, now in R-2 of that spec. No choice is made here:

- Whether `local-close` is the reason, extending the congestion arm of
  `sessionCloseFor`, or whether plugins need the cause itself (budget expiry,
  held path) beside the reason. No new reason name is needed for the first
  form.
- Whether the held-path case changes at all: where a teardown ran it is
  already `local-close`, and where the peer dropped the transport `tcp-failure`
  may be the true answer.
- BMP: the same session changes from Peer Down reason 4 to reason 2
  (RFC 7854 Section 4.9).
- The ExaBGP bridge's words for the down, checked in the same change
  (`ai/doors/bgp-engine.md`).

## Required Reading

- `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-plugins.md`, `ai/doors/system-plugins.md`
- `docs/architecture/api/architecture.md`, "Close Reasons"
- `docs/architecture/api/json-format.md`, `docs/architecture/api/process-protocol.md`
- `docs/architecture/behavior/peer-lifecycle.md`
- `docs/architecture/exabgp-bridge.md`, the down reason words
- `rfc/short/rfc7854.md`, `rfc/short/rfc4724.md`

## Current Behavior (MANDATORY)

- [ ] `internal/component/bgp/reactor/peer_close.go` -> `sessionCloseFor`, `leaveCause`, `sessionDroppedOwed`: one reason per down; Event 18 is `local-close` only for `ErrCongestionTeardown`
- [ ] `internal/component/bgp/reactor/session_write_budget.go` -> `recordWriteFailure`, `reportWriteBudgetExpiry`: the expiry becomes the close reason and is reported to the operator
- [ ] `pkg/plugin/rpc/enums.go` -> the `Reason*` constants, `ReasonForbidsRetention`
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` -> `peerDownFor`
- [ ] `internal/exabgp/bridge/bridge_event_state.go` -> `exabgpDownReasons`

## Existing Mechanisms (MANDATORY)

Superseded: the live rows are in the Existing Mechanisms table of
`plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`.

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Close derivation | `peer_close.go` `sessionCloseFor` | One reason per down; a local cause on Event 18 is honored for congestion alone | Does not apply: owned by R-2 of the BMP spec |
| Local-close reason | `pkg/plugin/rpc/enums.go` `ReasonLocalClose` | The reason for a session ze closed with no NOTIFICATION reaching the peer | Does not apply: owned by R-2 of the BMP spec |
| Operator-side cause | `plugin.CloseCause`, `PeerInfo.LastCloseCause` | Records a write budget close for `show bgp peer` | Does not apply: owned by R-2 of the BMP spec |
| BMP residual R-2 | `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` | Owns the local closes that leave on Event 18, the write budget expiry and the held write path included (its OD-3) | Replaced by it: this spec holds no work |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A write to a peer that accepts no data for the write budget, or a close that meets a held write path, followed by the session's down event to plugins.

### Transformation Path
Superseded: designed in R-2 of the BMP spec.

### Boundaries Crossed
Superseded: designed in R-2 of the BMP spec. Engine to plugin: the `reason` of the state down event, on both transports.

### Integration Points
Superseded: designed in R-2 of the BMP spec.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Superseded: AC-27 and AC-28 of the BMP spec | → | - | - |

## Acceptance Criteria

Superseded: AC-27 and AC-28 of
`plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` carry these.

- AC-1: a plugin subscribed to state events, on a session ze closed for a write budget expiry, reads a down whose reason says ze closed it, on the internal and the forked transport.
- AC-2: a session the peer or the network dropped still reads `tcp-failure`.
- AC-3: the pages that state the reasons, and the BMP and ExaBGP bridge behavior for them, match the code.

## Risks & Assumptions

Superseded: none recorded here.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| Superseded: the owed tests are in the BMP spec's TDD plan | - |

## Files to Modify

None: this spec holds no work.

## Implementation Steps

None: this spec holds no work.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

Superseded: the work and its tests are owned by R-2 of the BMP spec. The rows
below are the original ones, kept because the spec format requires them.

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/reactor/`, the BMP and bridge package tests, and the functional test the design names under `./le test bgp plugin`
- [ ] This file is removed once the removal is authorised (it is a tracked spec, so the commit tool treats its removal as a closure)
