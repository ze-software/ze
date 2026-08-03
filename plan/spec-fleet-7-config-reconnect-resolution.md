# Spec: fleet-7 -- Diverged Config Resolution

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fleet-6-config-freeze |
| Phase | - |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- config-fetch / config-changed / config-ack protocol
4. `pkg/fleet/envelope.go` -- RPC verb payload types (four verbs, no up-channel today)
5. `internal/component/managed/client.go` -- `FetchConfig`, `SendConfigAck`, `OnCommit`
6. `internal/component/plugin/server/managed.go` -- `HandleConfigFetch`, `BuildConfigChanged`, `connected`
7. `internal/component/config/diff/` -- diff engine reused by the resolution UI
8. `spec-fleet-6-config-freeze.md` -- provides the persisted baseline, the dirty check, the hold, and the `diverged` state this spec resolves
9. `spec-fleet-1-device-registry.md` -- where the `diverged` device state is surfaced

## Task

Resolve a device that `fleet-6` has marked `diverged`. fleet-6 already does the detection: on
reconnect the device compared its active config against its persisted baseline, found a local
emergency edit, held (kept running its config, did not auto-apply the hub config), and reported
`diverged`. fleet-7 adds the resolution path so the operator can decide which config wins,
through a familiar commit-style diff at the hub.

Mechanism:

1. **Push up.** A diverged device sends its current config to the hub with a new `config-push`
   verb, so the hub holds both versions (its own authoritative copy plus the device's pushed
   copy as a pending candidate).
2. **Diff.** The hub's per-device view shows a warning and a commit-style diff. Labels from the
   operator's seat: **Local = the hub config**, **Remote / new = the router config**.
3. **Decide.**
   - **Adopt (router wins):** the router config becomes the hub's authoritative copy; the baseline
     becomes the router config on both sides; the device is already running it, so they are in sync.
   - **Revert (hub wins):** the hub pushes its config down via `config-changed`; the device applies
     it; the baseline becomes the hub config. The emergency change is discarded, but only on an
     explicit operator action after the diff has been seen.

`config-push` rides the **existing TLS transport** (a new verb on the same `#id verb json`
framing; `pkg/fleet/envelope.go` confirms only four verbs exist today and none carry config
upstream). It is a bounded, operator-gated up-channel, not continuous bidirectional sync.

Naming: the fleet state is `diverged` / divergence, NOT `conflict` -- the editor already uses
`Conflict`/`ConflictStale`/`ConflictLive` for concurrent edit sessions (`editor_commit.go`), a
different concept in the same subsystem.

### Decisions inherited from the umbrella direction update and fleet-6

| Decision | Detail |
|----------|--------|
| Detection lives in fleet-6 | The persisted baseline, dirty check, hold, and `diverged` state are fleet-6; fleet-7 is resolution only |
| Keep TLS | `config-push` is a new verb on the existing transport; no SSH |
| Hub holds both configs | The device pushes its config up, so the diff renders at the hub with no proxy into the router |
| Reuse the commit diff | The resolution UI is the existing config commit diff renderer, sourced from (hub config, pushed router config) |
| Operator-gated | Neither side is adopted without an explicit operator action after seeing the diff |

### Post-wave corrections (2026-07-10)

New obligation from the 2026-07 implementation wave (verified against current code): the
transport `config-push` rides now enforces write timeouts. `pkg/plugin/rpc/conn.go` applies a
default 30s write deadline when the context carries none (`defaultWriteDeadline`, conn.go;
applied in `writeAppended`, conn.go, :309); on transports without `SetWriteDeadline`
a fail-fast watchdog closes the connection on a stalled write (conn.go). Both managed
endpoints are TLS `net.Conn`s wrapped in `rpc.NewConn` (client:
`internal/component/managed/client.go`; hub:
`internal/component/plugin/server/managed_serve.go`), so the deadline path applies.

This matters more here than for fleet-4 because `config-push` carries a full device config
upstream, the largest payload on this channel. Two consequences for the design and for R-1's
"bound payload size" mitigation:

| Consequence | Detail | Citation |
|-------------|--------|----------|
| Hard frame bound already exists | A frame larger than 16 MB is rejected at write time, before any bytes hit the socket | `pkg/plugin/rpc/framing.go`; enforced in conn.go |
| Slow-link abort | A `config-push` write that cannot complete within 30s (slow WAN link, stalled hub) fails with a deadline error and the connection may close; the client must treat this as a normal disconnect and retry after reconnect, still in the `diverged` state | conn.go, :292-294 |

R-1's payload bound should be chosen with the 30s write window in mind, not only storage
concerns.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- two-phase config change, version hash, config-fetch/ack
  -> Decision: resolution is hub-side; the device only pushes its config up and awaits a decision
  -> Constraint: keep the existing four verbs working; add `config-push` alongside them
- [ ] `docs/architecture/hub-api-commands.md` -- RPC framing conventions
  -> Constraint: `config-push` uses the same `#id verb [json]` framing

**Key insights:**
- `config-ack` already carries `{"ok":false,"error":...}`; the device's diverged hold can be reported by acking with a `diverged` status, so no extra surfacing message is needed
- The hub already has the device's authoritative config; `config-push` adds the divergent copy as a pending candidate so a diff can render
- `ManagedConfigService.connected` and `HandleConfigFetch` are the hub-side hooks; `config-push` is a new handler beside them

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/managed/client.go` -- after fleet-6, a dirty device holds instead of applying; there is still no client-to-hub config push
  -> Decision: add a `config-push` send for diverged devices
- [ ] `pkg/fleet/envelope.go` -- verbs: `config-fetch`, `config-changed`, `config-ack`, `ping`; no upstream config
  -> Decision: add a `config-push` verb constant and payload type
- [ ] `internal/component/plugin/server/managed.go` -- `HandleConfigFetch` serves the hub config; `BuildConfigChanged` notifies one device
  -> Decision: add a `config-push` handler that stores the device config as a pending candidate and marks the device `diverged`
- [ ] `internal/component/config/diff/` -- diff engine used by the editor commit diff
  -> Decision: reuse it to render Local (hub) vs Remote/new (router)
- [ ] `internal/component/web/editor.go` -- commit diff rendering
  -> Decision: the divergence view reuses this renderer

**Behavior to preserve:**
- The four existing verbs and their payloads unchanged
- Normal (non-diverged) reconnect: fleet-6 applies any hub update and resumes -- unchanged
- The hub-apply path (`OnCommit`) unchanged

**Behavior to change:**
- A diverged device pushes its config up via `config-push`
- New `config-push` verb (client -> hub) and hub-side handler
- Hub stores the pushed config as a pending candidate; device marked `diverged` in the registry (`fleet-1`)
- Hub UI: warning + commit-style diff on the per-device view; adopt / revert actions
- `ze fleet resolve --adopt | --revert` (or resolve from the hub UI)

## Data Flow (MANDATORY)

### Entry Point
- A device that fleet-6 marked `diverged` (re)connects and holds
- Operator opens the per-device view on the hub for a diverged device
- Operator runs `ze fleet resolve --adopt|--revert` (or clicks adopt/revert in the UI)

### Transformation Path
1. Diverged device sends `config-push` with its current config, and acks with a `diverged` status
2. Hub stores the pushed config as a pending candidate; marks the device `diverged`
3. Operator views the diff (Local = hub, Remote/new = router)
4. Adopt: hub stores the router config as authoritative; baseline := router config; divergence cleared; device already running it
5. Revert: hub `config-changed` -> device applies hub config; baseline := hub; divergence cleared

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Device to Hub (up) | new `config-push` verb over TLS MuxConn | [ ] |
| Hub to Registry | mark device `diverged`; store pending candidate | [ ] |
| Hub to Web/CLI | diff render + resolve actions | [ ] |
| Resolution to Device | adopt (no-op, already running) or revert (`config-changed`) | [ ] |

### Integration Points
- `pkg/fleet/` -- `config-push` verb constant and payload type
- `internal/component/managed/client.go` -- send `config-push` when diverged
- `internal/component/plugin/server/managed.go` -- `config-push` handler; pending candidate; resolve actions
- `internal/component/plugin/server/registry.go` (fleet-1) -- `diverged` device state
- `internal/component/config/diff/` + web/CLI -- diff render and resolve commands

### Architectural Verification
- [ ] No bypassed layers (detection in fleet-6; resolution hub-side)
- [ ] No unintended coupling (non-managed devices unaffected; existing verbs unchanged)
- [ ] No duplicated functionality (reuses the commit diff renderer and the version hash)
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | fleet-6 is implemented: baseline, dirty check, hold, and `diverged` state exist | `spec-fleet-6` | fleet-7 has nothing to resolve and detection is missing | confirm fleet-6 landed before fleet-7 coding | unvalidated |
| A-2 | The editor's commit diff renderer can render two arbitrary config blobs (not just session-vs-committed) | `internal/component/config/diff/`, `web/editor.go` | A new diff path is needed | spike: render (hub config, pushed config) | unvalidated |
| A-3 | A device's authoritative hub config and its pushed config are both available to the hub at resolve time | `HandleConfigFetch` reads hub config; `config-push` adds the other | Diff cannot render | functional test stores and reads both | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A pushed config is large or malicious | Oversized `config-push` payload | Bound payload size; validate/parse before storage; one pending candidate per device |
| R-2 | Operator never resolves; device stuck diverged indefinitely | Long-lived `diverged` state in the dashboard | Surface prominently in fleet-1 dashboard + audit; device keeps running safely meanwhile |
| R-3 | Revert discards a load-bearing emergency fix by mistake | Operator clicks revert without reading | Require the diff to be shown before revert is enabled; audit who chose what |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Diverged device reconnects | -> | Sends `config-push`; hub stores candidate, marks diverged | `test/managed/fleet-divergence.ci` |
| Operator adopt (router wins) | -> | Router config becomes hub-authoritative | `test/managed/fleet-resolve-adopt.ci` |
| Operator revert (hub wins) | -> | Hub config pushed down, divergence cleared | `test/managed/fleet-resolve-revert.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A device fleet-6 marked `diverged` reconnects | Sends `config-push` with its config; acks with a `diverged` status |
| AC-2 | Hub receives `config-push` | Stores the device config as a pending candidate; marks the device `diverged` in the registry |
| AC-3 | Operator opens the per-device view for a diverged device | Warning shown plus a commit-style diff: Local = hub config, Remote/new = router config |
| AC-4 | Operator chooses adopt (router wins) | Router config becomes the hub authoritative copy; baseline := router config; device and hub in sync; divergence cleared |
| AC-5 | Operator chooses revert (hub wins) | Hub pushes its config down via `config-changed`; device applies; baseline := hub; divergence cleared |
| AC-6 | Device is diverged and unresolved | Device stays connected and fully operational on its current config |
| AC-7 | `config-push` from client X | Scoped to the authenticated client name; X cannot push another device's config |
| AC-8 | `ze fleet status` on a diverged device | Reports `diverged` and which config it is running |
| AC-9 | Existing four verbs after `config-push` is added | All still work; non-managed and in-sync devices behave exactly as before |
| AC-10 | A device that actually applied the hub config (active == baseline) reconnects | Not diverged (fleet-6); no `config-push`, no resolution prompt -- the deferred-update / lost-ack case does not produce a false divergence |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reconnects a device that was edited offline | fleet-6 holds + diverged -> `config-push` -> hub candidate | `fleet-divergence.ci` |
| 2 | Reviews the difference on the hub | per-device view -> diff (Local=hub, Remote=router) | `fleet-divergence.ci` |
| 3 | Keeps the router's emergency fix | adopt -> router config authoritative -> in sync | `fleet-resolve-adopt.ci` |
| 4 | Discards the router's change | revert -> `config-changed` -> device applies hub config | `fleet-resolve-revert.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigPushEnvelope` | `pkg/fleet/envelope_test.go` | `config-push` payload round-trips | |
| `TestConfigPushHandlerStoresCandidate` | `internal/component/plugin/server/managed_test.go` | Hub stores pending candidate, marks diverged | |
| `TestResolveAdopt` | `internal/component/plugin/server/managed_test.go` | Adopt promotes router config to authoritative; baseline updated | |
| `TestResolveRevert` | `internal/component/plugin/server/managed_test.go` | Revert pushes hub config down; baseline updated | |
| `TestConfigPushScopedToClient` | `internal/component/plugin/server/managed_test.go` | A client cannot push another device's config | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-divergence` | `test/managed/fleet-divergence.ci` | Diverged device pushes up; hub shows the diff | |
| `fleet-resolve-adopt` | `test/managed/fleet-resolve-adopt.ci` | Operator adopts the router config | |
| `fleet-resolve-revert` | `test/managed/fleet-resolve-revert.ci` | Operator reverts to the hub config | |

## Files to Modify
- `pkg/fleet/envelope.go` -- add `config-push` verb constant and payload type
- `internal/component/managed/client.go` -- send `config-push` when diverged (state from fleet-6)
- `internal/component/plugin/server/managed.go` -- `config-push` handler; pending candidate; resolve actions
- `internal/component/plugin/server/registry.go` -- `diverged` device state (from fleet-1)
- `internal/component/web/editor.go` (or fleet page) -- divergence warning + diff + adopt/revert
- `cmd/ze/hub/main.go` -- wire the `config-push` handler and resolve commands

## Files to Create
- `internal/component/plugin/server/divergence.go` -- pending-candidate store and resolve logic
- `test/managed/fleet-divergence.ci` -- functional test
- `test/managed/fleet-resolve-adopt.ci` -- functional test
- `test/managed/fleet-resolve-revert.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- `config-push` verb + hub handler skeleton; failing wiring tests
   - Tests: `fleet-divergence.ci` (initially failing)
   - Files: `pkg/fleet/envelope.go`, `managed.go` handler stub, `cmd/ze/hub/main.go`
   - Verify: verb routes to a stub handler; a diverged client can send it

2. **Phase: Push + candidate** -- diverged client sends config up; hub stores candidate, marks diverged
   - Tests: `TestConfigPushEnvelope`, `TestConfigPushHandlerStoresCandidate`, `TestConfigPushScopedToClient`, `fleet-divergence.ci`
   - Files: `client.go`, `managed.go`, `registry.go`, `divergence.go`
   - Verify: device shows `diverged`; both configs held at the hub

3. **Phase: Diff + resolution** -- adopt / revert, diff UI, `ze fleet resolve`
   - Tests: `TestResolveAdopt`, `TestResolveRevert`, `fleet-resolve-adopt.ci`, `fleet-resolve-revert.ci`
   - Files: `managed.go`, `divergence.go`, web/CLI
   - Verify: adopt and revert both clear the divergence and converge; baseline updated

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Detection is fleet-6's job; fleet-7 only resolves an already-`diverged` device; no auto-apply |
| Naming | New verb is `config-push` (kebab-case); device state is `diverged` (not `conflict`) |
| Data flow | Push up; resolve hub-side; revert via `config-changed`; baseline updated on both outcomes |
| Rule: wiring-completeness | `config-push` handler and resolve actions reachable from `cmd/ze/hub/main.go` |
| Rule: dependency | Relies on `fleet-6` (baseline + diverged state); validate A-1 before coding |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `config-push` payload bounded (max size); parsed/validated before storage |
| Auth scope | `config-push` scoped to the authenticated client name (AC-7); cannot push another device's config |
| Resolve auth | Adopt / revert require admin authorization |
| Candidate growth | One pending candidate per device; bounded; cleared on resolve |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Architecture docs updated (`docs/architecture/fleet-config.md`, `docs/architecture/hub-api-commands.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
