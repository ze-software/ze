# Spec: bgp-api-notification-producers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Post-release feature: owner ruling T-4 (2026-11-24) keeps the seven declared `ze-bgp-api` notifications on the temporary `yangcontract` allowlist introduced by `plan/release/fixes/spec-yang-single-source-of-truth.md` and schedules wiring their runtime producers after the first release. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-yang-single-source-of-truth.md` (adds the notification producer gate and temporary allowlist entries naming this spec) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`internal/component/bgp/yang/ze-bgp-api.yang` declares seven YANG `notification`
blocks (`peer-state-change`, `route-received`, `route-update-sent`,
`eor-received`, `graceful-restart-state`, `session-established`,
`session-closed`), each carrying a `ze:method "ze-bgp:..."` extension and typed
leaves, and each description currently states that no producer emits it.
The BGP reactor emits JSON events on the plugin EventBus (`appendPeerJSON` /
`appendStateChangeJSON` / `appendUpdateJSON`), which is a separate stream and
not these YANG notifications.

Per Thomas's T-4 decision (2026-11-24) on `plan/release/fixes/spec-yang-single-source-of-truth.md`,
these seven notifications remain on the temporary notification allowlist for the
first release and are wired to live producers after the first release. Closing
this spec wires a producer for each of the seven notifications (or removes any
notification Thomas declines at design time), proves delivery to a subscriber
through a functional `.ci` test, removes the seven entries from the temporary
notification allowlist, and flips the 2026-09-09 `unwired-feature.md` journal
row to `fixed:`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - event delivery and notification stream contract
  → Decision: engine-to-plugin event delivery is lossless on queue overflow
  → Constraint: every engine operation or notification names the message it carries
- [ ] `docs/architecture/core-design.md` - BGP reactor and plugin EventBus architecture
  → Constraint: producers register their notification types rather than hardcoding lists

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - BGP FSM state transitions for `peer-state-change`, `session-established`, and `session-closed`
  → Constraint: emitted state names match RFC 4271 Section 8.2.2 FSM states

**Key insights:**
- Seven notifications are declared in `internal/component/bgp/yang/ze-bgp-api.yang` lines 1007-1134 (`peer-state-change`, `route-received`, `route-update-sent`, `eor-received`, `graceful-restart-state`, `session-established`, `session-closed`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-api.yang` - declares all seven `notification` blocks with descriptions stating no producer emits them
- [ ] `internal/le/doc/yangcontract/contract.go` - validates YANG commands and RPCs; extended by `plan/release/fixes/spec-yang-single-source-of-truth.md` to check notification producers and temporary allowlist entries

**Behavior to preserve:**
- Existing BGP EventBus JSON/text event delivery to subscribed plugins and ExaBGP bridges remains unchanged.

**Behavior to change:**
- Emit the seven declared `ze-bgp-api` notifications from their BGP reactor / graceful-restart producers, update their YANG descriptions, and remove them from the temporary allowlist.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| EventBus notification emission | `internal/core/events/events.go` `RegisterEventType`; `internal/component/bgp/server/events.go` | Emits BGP events onto the plugin EventBus | Extends: binds or emits the seven declared YANG notifications from the existing reactor hooks |
| YANG command and notification contract | `internal/le/doc/yangcontract/contract.go` `contractSatisfied` | Checks declared RPCs and (after Piece E) notifications against registered producers and allowlist | Extends: removes the seven temporary allowlist entries once producers are registered |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- BGP session state transitions, received/sent UPDATEs, received EOR markers, and GR state changes inside the BGP reactor and `bgp-gr` plugin.

### Transformation Path
1. Reactor / GR state transition produces a structured notification matching the `ze-bgp-api.yang` notification schema.
2. EventBus / notification dispatcher delivers the notification to subscribed clients.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP reactor / `bgp-gr` to notification subscribers | `ze-bgp:*` YANG notification payloads | No |

### Integration Points
- `internal/component/bgp/reactor/` session lifecycle and UPDATE hooks - emit `peer-state-change`, `session-established`, `session-closed`, `route-received`, `route-update-sent`, `eor-received`.
- `internal/component/bgp/plugins/gr/` - emits `graceful-restart-state`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All seven declared `ze-bgp-api` notifications remain desired post-release | Owner T-4 decision (2026-11-24) | Any notification Thomas drops at design time is removed from `ze-bgp-api.yang` | Design review with Thomas | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | High-rate UPDATE notifications (`route-received`, `route-update-sent`) add allocation pressure on the forwarding hot path | Benchmark regression in `./le perf` | Emit only when at least one subscriber is active for that notification |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Subscribers receive malformed notification payloads or missing events; UPDATE hot path slows down if unguarded |
| How is it reverted? | Single commit revert restoring the allowlist entries |
| Who else touches this path? | `plan/release/fixes/spec-yang-single-source-of-truth.md` (owns the allowlist and notification gate) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP peer session establish, UPDATE exchange, EOR, GR state change, and close | → | Registered notification producers for all seven `ze-bgp-api` notifications | `test/plugin/bgp-api-notifications.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A BGP peer establishes, exchanges UPDATEs and EOR, enters GR, and closes while a subscriber listens for `ze-bgp:*` notifications | Every one of the seven declared notifications (`peer-state-change`, `route-received`, `route-update-sent`, `eor-received`, `graceful-restart-state`, `session-established`, `session-closed`) is emitted with fields matching `ze-bgp-api.yang` | None |
| AC-2 | `./le doc yang-contract command-contract` runs after removing the seven allowlist entries | Gate passes with zero allowlist entries for `ze-bgp-api` notifications | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Subscribes to `ze-bgp:session-established` and brings up a peer | Peer OPEN/KEEPALIVE -> reactor -> notification producer -> subscriber | `test/plugin/bgp-api-notifications.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPAPINotificationProducersRegistered` | `internal/component/bgp/server/notifications_test.go` | AC-1, AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric config inputs added | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-api-notifications` | `test/plugin/bgp-api-notifications.ci` | Subscriber receives all seven `ze-bgp-api` notifications during peer lifecycle | |

### Interop Tests (Scope: protocol)
N-A: internal daemon notification stream only, no BGP protocol wire change.

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-api.yang` - remove "No producer emits this notification today" from the seven notification descriptions
- `internal/component/bgp/server/events.go` - wire notification producers for reactor events
- `internal/component/bgp/plugins/gr/` - emit `ze-bgp:graceful-restart-state`
- `internal/le/doc/yangcontract/contract.go` - remove the seven temporary allowlist entries

## Files to Create
- `internal/component/bgp/server/notifications_test.go`
- `test/plugin/bgp-api-notifications.ci`

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the seven notification producers and add failing wiring test.
2. **Phase: Producers** -- wire reactor and GR event hooks to emit each notification when subscribed.
3. **Phase: Contract** -- update `ze-bgp-api.yang` descriptions, delete the seven allowlist entries, verify `./le doc yang-contract command-contract`.

## Owned journal rows

Moved here on 2026-11-26 from `plan/release/fixes/spec-yang-single-source-of-truth.md` per owner T-4 decision (2026-11-24).

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-09-09 | `ze-bgp-api` notifications have no producer (the row names four; the model declares seven) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Defer producer wiring to post-release on temporary allowlist | Wire all seven before first release; delete all seven declarations | Owner T-4 decision (2026-11-24): keep the seven `ze-bgp-api` notifications on the temporary allowlist with a post-release spec to wire their producers |

## Known Limitations
- None beyond post-release scheduling.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass, run in the main checkout by the main thread; no `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal row marked `fixed:`
- [ ] **Commit B:** `remove plan/post-release/features/spec-bgp-api-notification-producers.md` only
