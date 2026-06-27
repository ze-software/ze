# Spec: DDoS Local Responder — On-Host nft Drop (ftagent local parity)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cp-survival-5-detect-1-detector, spec-cp-survival-5-detect-0-umbrella |
| Phase | - |
| Updated | 2026-06-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-cp-survival-5-detect-1-detector.md` (the event contract this consumes)
3. `plan/spec-cp-survival-5-detect-0-umbrella.md` (modes, safety controls, risks)
4. `internal/plugins/flowspec-firewall/engine.go` (the closest analogue: external input → firewall terms), `internal/component/firewall/model.go` (RegisterTables/ApplyAll/GetCounters)

## Task

Build the **local responder**: a plugin that subscribes to the detector's `AttackDetected` /
`AttackCleared` events and mitigates **on-host** by installing a surgical nftables drop rule via the
`firewall` component, then removing it when the attack clears. This is the "local mode" of the umbrella —
`ftagent` local parity (detect on local rate, drop on-host, clear when the rate falls).

It owns no detection and no wire/BGP work. It is one of several responders that may run concurrently;
removing it removes local mitigation only.

Why local-mode clear is clean (umbrella key insight): nftables drops occur **after** the kernel NIC RX
counter increments, so the detector's rate signal keeps reflecting the arriving flood while it is
dropped. The detector therefore emits a valid `AttackCleared` when the attacker actually stops, and this
responder simply removes its rule on that event. (Caveat R-1: an XDP drop backend would break this.)

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` (EventBus Typed Payloads) - event subscription
  → Decision: subscribe to `ddosevent.AttackDetected`/`AttackCleared` via `events.Register[T]`; import ONLY `internal/core/ddosevent`, never the detector plugin.
  → Constraint: handler must be non-blocking and idempotent (events can repeat / arrive out of order).
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Constraint: this plugin owns its YANG, its `clear` command, and its firewall table name (`ddos-local`); removing it leaves the detector and other responders working.
- [ ] `internal/plugins/flowspec-firewall/engine.go` - the analogue (external input → firewall terms)
  → Constraint: mirror its `firewall.RegisterTables`/`ApplyAll` usage; do NOT bolt onto flowspec-firewall (keep receive/originate/local-respond independent).
- [ ] `ai/rules/cli-grammar.md` - the manual clear verb
  → Constraint: action before identifier — `clear ddos-mitigation <target>`, not `ddos clear`.
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` - responder config
  → Decision: response-level, allowlist, and max-mitigation-duration are operational policy → YANG leaves (kebab-case); duration uses `ze-types.yang` duration type with `range`.

### RFC Summaries
- N/A — local responder touches no wire protocol (nftables only).

**Key insights:**
- The match must be **surgical** (from the event's vector tuple: dst prefix + proto + dst-port), not a
  blanket drop to the victim /32 — otherwise the responder finishes the attacker's job (umbrella #2).
- A **never-block allowlist** is subtracted from every match so auto-mitigation can never drop control
  plane / DNS / management traffic (umbrella R-4). If the match is fully allowlisted, install nothing.
- The `firewall` component abstracts nft vs VPP backends, so `RegisterTables` gives **dataplane parity**
  for free; but the detector's kernel-rate signal assumes kernel-visible traffic (R-2).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - `RegisterTables(name, tables)`, `ApplyAll()`, `GetCounters(name) []ChainCounters`, `ChainCounters{Chain, Terms}`, `TermCounter{Name, Packets, Bytes}`.
  → Constraint: install via `RegisterTables("ddos-local", …)` + `ApplyAll()`; remove by re-registering empty + `ApplyAll()`; read drop counts via `GetCounters("ddos-local")`.
- [ ] `internal/plugins/firewall/nft/backend_linux.go:269` - `GetCounters` reads the nft per-rule counter; `internal/plugins/firewall/vpp/backend_linux.go:396` - VPP backend parity.
  → Constraint: the responder is backend-agnostic; nft and VPP both supported through the firewall abstraction.
- [ ] `internal/plugins/flowspec-firewall/engine.go` - subscribes to BGP UPDATE events, translates match → firewall terms, `RegisterTables`+`ApplyAll`.
  → Constraint: reuse this translation pattern for vector-tuple → firewall term; this responder's input is an EventBus event, not a BGP UPDATE.
- [ ] `internal/core/ddosevent/event.go` - the event contract (created by child 1) carrying target, vector tuple, family, sources, observable flag.
  → Constraint: build the firewall term from these fields; do not re-derive the vector locally.

**Behavior to preserve:**
- `firewall` component behavior, existing firewall tables, and `flowspec-firewall` receive direction are
  unchanged. This responder registers its own table namespace (`ddos-local`) and touches no other.

**Behavior to change:** None in existing code. New opt-in plugin; it emits `MitigationApplied`/`MitigationRemoved` on the EventBus (event types defined by child 4) so the incident store can record the outcome and drop counters.

## Data Flow (MANDATORY)

### Entry Point
- `ddosevent.AttackDetected` / `AttackCleared` on the EventBus; the `clear ddos-mitigation` CLI verb.

### Transformation Path
1. Receive `AttackDetected`.
2. If response-level is `alert`: log the would-mitigate decision, record the incident, stop.
3. Build a firewall drop term from the vector tuple (dst prefix + proto + dst-port); subtract the never-block allowlist. If the resulting match is empty, skip and log.
4. `RegisterTables("ddos-local", term)` + `ApplyAll()` → nft (or VPP) drop installed; start the max-mitigation-duration timer.
5. On `AttackCleared` (or manual `clear`, or max-duration expiry): read `GetCounters("ddos-local")` for the incident record, re-register empty + `ApplyAll()` to remove, cancel the timer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| EventBus ↔ responder | subscribe `ddosevent` types (`events.Register[T]`) | [ ] |
| responder ↔ dataplane | `firewall.RegisterTables`/`ApplyAll`/`GetCounters` | [ ] |
| config tree ↔ responder | YANG `ddos-local` container → settings | [ ] |
| CLI ↔ responder | `clear ddos-mitigation <target>` verb | [ ] |

### Integration Points
- `internal/core/ddosevent` - event contract (consume)
- `internal/component/firewall` `RegisterTables`/`ApplyAll`/`GetCounters` - mitigation + counters
- `pkg/ze/eventbus.go` - subscription

### Architectural Verification
- [ ] No bypassed layers (mitigation via firewall component, not raw nft calls)
- [ ] No unintended coupling (imports the event leaf, not the detector plugin)
- [ ] No duplicated functionality (reuses firewall translation pattern from flowspec-firewall)
- [ ] Zero-copy preserved (no wire encoding; event-driven)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `firewall.RegisterTables`/`ApplyAll` are callable at runtime from this plugin | `flowspec-firewall/engine.go` does exactly this | needs a different hook | mirror flowspec-firewall + a unit test | unvalidated |
| A-2 | The `ddosevent` contract carries enough vector detail (dst prefix, proto, dst-port) to build a surgical term | child 1 event design | matches become coarse (/32 drop) | validate against `internal/core/ddosevent/event.go` | unvalidated |
| A-3 | Removing a table = re-register empty + `ApplyAll()` cleanly tears down the nft rules | firewall reconcile model | stale rules linger | functional test: install then remove, assert no rule | unvalidated |
| A-4 | `GetCounters("ddos-local")` returns packets/bytes dropped by our term | grep-confirmed `GetCounters` (backend_linux.go:269) | no incident drop stats | unit/functional test reading counters | confirmed (API exists) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | nft clear-signal validity assumes nft drops after the NIC RX counter; an XDP backend breaks the detector's rate-based clear | clear never fires under XDP | document nft-only for v1 (umbrella R-7); XDP would need a drop-counter clear path |
| R-2 | VPP dataplane: drop works via firewall, but the detector's kernel iface-rate signal may not see VPP-forwarded traffic | detector never triggers on a VPP box | flagged to umbrella/child 1 (detection-signal scope); out of scope for this responder |
| R-3 | Over-broad match drops legitimate traffic to the same dst (collateral) | legit users lose service | surgical match from vector + never-block allowlist; alert-mode-first rollout |
| R-4 | Stale rule if `AttackCleared` never arrives (detector died/restarted) | rule outlives the incident | max-mitigation-duration safety valve + manual `clear`; reconcile on restart |
| R-5 | Race: `AttackCleared` then immediate `AttackDetected` for the same target | rule flaps | idempotent table reconcile keyed by target; coalesce repeats |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `AttackDetected` event (enforce mode) | → | responder builds term → `RegisterTables`+`ApplyAll` (nft drop) | `ddos-local-respond.ci` |
| `AttackCleared` event | → | responder removes the `ddos-local` table | `ddos-local-respond.ci` |
| `clear ddos-mitigation <target>` | → | responder force-removes the active mitigation | `ddos-local-clear-cmd.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `AttackDetected` (enforce mode) with a vector tuple | a `ddos-local` firewall table is installed with a drop term matching dst prefix + proto + dst-port |
| AC-2 | The target overlaps a never-block allowlist entry | the allowlisted prefix is excluded from the match; a fully-allowlisted target installs no rule and logs why |
| AC-3 | `AttackCleared` for an active target | the `ddos-local` table is removed (no residual nft rule) |
| AC-4 | response-level `alert` | the responder logs a would-mitigate decision and installs nothing |
| AC-5 | max-mitigation-duration elapses while still mitigating | the responder force-removes the rule and emits a warning (safety valve) |
| AC-6 | `clear ddos-mitigation <target>` while mitigating | the active mitigation for that target is removed |
| AC-7 | A mitigation is removed | the dropped packets/bytes (from `GetCounters`) are recorded for the incident |
| AC-8 | VPP firewall backend active | the drop term is installed via the firewall abstraction (dataplane parity), same ACs hold |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables local mode; a flood hits | `AttackDetected` → nft drop installed; flood stops → `AttackCleared` → rule removed | `ddos-local-respond.ci` |
| 2 | runs alert-only | `AttackDetected` → logged, nothing installed | `ddos-local-alertmode.ci` |
| 3 | manually clears a stuck mitigation | `clear ddos-mitigation <target>` → rule removed | `ddos-local-clear-cmd.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTermFromVector` | `internal/plugins/ddoslocal/match_test.go` | surgical term from the event vector tuple (AC-1) | |
| `TestAllowlistSubtraction` | `internal/plugins/ddoslocal/match_test.go` | allowlist excluded; fully-allowlisted target → no rule (AC-2) | |
| `TestAlertModeInstallsNothing` | `internal/plugins/ddoslocal/responder_test.go` | alert mode logs, installs nothing (AC-4) | |
| `TestClearedRemovesTable` | `internal/plugins/ddoslocal/responder_test.go` | AttackCleared / clear cmd removes the table (AC-3, AC-6) | |
| `TestMaxDurationForceRemoves` | `internal/plugins/ddoslocal/responder_test.go` | safety valve force-removes after max-duration (AC-5) | |
| `TestIdempotentReconcile` | `internal/plugins/ddoslocal/responder_test.go` | repeated/out-of-order events do not double-install (R-5) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max-mitigation-duration (s) | 0-86400 | 86400 | N/A (0 = no cap) | 86401 |
| allowlist prefix length v4 | 0-32 | 32 | N/A | 33 |
| allowlist prefix length v6 | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-local-respond` | `test/plugin/ddos-local-respond.ci` | detect → nft drop → clear on AttackCleared | |
| `ddos-local-allowlist` | `test/plugin/ddos-local-allowlist.ci` | allowlisted target is not blocked | |
| `ddos-local-alertmode` | `test/plugin/ddos-local-alertmode.ci` | alert mode installs nothing | |
| `ddos-local-clear-cmd` | `test/plugin/ddos-local-clear-cmd.ci` | manual clear removes the mitigation | |

### Interop Tests
N/A — local responder touches no wire protocol (nftables only). Justified: BGP/wire mitigation is child 3.

### Future (deferred tests)
- XDP-backend clear path (R-1) — only if/when an XDP firewall backend lands.

## Files to Modify
- `internal/component/plugin/all/all.go` - add the responder to the composition root (`make generate`)
- `docs/features.md` - add the "DDoS local mitigation" row
- `docs/guide/plugins.md` - list the `ddoslocal` plugin
- `docs/guide/command-reference.md` - document `clear ddos-mitigation`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (config) | Yes | `internal/plugins/ddoslocal/yang/ze-ddos-local-conf.yang` — response-level enum, allowlist, max-mitigation-duration; native constraints per `ai/patterns/config-option.md` |
| CLI command | Yes | `clear ddos-mitigation <target>` — action-before-identifier per `ai/rules/cli-grammar.md` |
| Functional test | Yes | `test/plugin/ddos-local-*.ci` |
| Prometheus counters | Yes | active-mitigations gauge, packets/bytes dropped (from GetCounters) — surfaced in child 4 |
| Doctor check | Deferred to child 4 | firewall-backend availability check |
| Env var registration | No | all settings are YANG config |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin/command changed? | Yes | `docs/plugin-overview.md` |

## Files to Create
- `internal/plugins/ddoslocal/register.go` - plugin registration, event subscription, YANG binding
- `internal/plugins/ddoslocal/responder.go` - event handlers, state, max-duration timer, GetCounters on removal
- `internal/plugins/ddoslocal/match.go` - vector tuple → firewall term; allowlist subtraction
- `internal/plugins/ddoslocal/cmd.go` - `clear ddos-mitigation` verb
- `internal/plugins/ddoslocal/config.go` - config parse + validation
- `internal/plugins/ddoslocal/yang/ze-ddos-local-conf.yang` - config schema
- `internal/plugins/ddoslocal/*_test.go` - unit tests above
- `test/plugin/ddos-local-respond.ci`, `ddos-local-allowlist.ci`, `ddos-local-alertmode.ci`, `ddos-local-clear-cmd.ci` - functional tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the detector spec (event contract) |
| 2. Audit | Files to Create/Modify, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — plugin skeleton (`register.go`, YANG) subscribing to `ddosevent` + a failing `ddos-local-respond.ci`; handler is a stub.
   - Verify: plugin registers; `make generate` wires it; wiring test fails (no rule installed yet).
2. **Phase: Match construction** — vector tuple → firewall term + allowlist subtraction.
   - Tests: `TestBuildTermFromVector`, `TestAllowlistSubtraction`.
3. **Phase: Mitigate / clear** — install on AttackDetected, remove on AttackCleared, GetCounters on removal, idempotent reconcile.
   - Tests: `TestClearedRemovesTable`, `TestIdempotentReconcile`; `ddos-local-respond.ci` passes.
4. **Phase: Safety** — response-level alert/enforce, max-mitigation-duration valve, manual `clear` verb.
   - Tests: `TestAlertModeInstallsNothing`, `TestMaxDurationForceRemoves`; `ddos-local-alertmode.ci`, `ddos-local-clear-cmd.ci`, `ddos-local-allowlist.ci`.
5. **Full verification** → `make ze-verify-changed` + `make generate`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC-N has implementation with file:line |
| Correctness | match is surgical (vector tuple), never blanket /32; allowlist always subtracted |
| Naming | YANG kebab-case; firewall table name `ddos-local`; CLI action-before-identifier |
| Data flow | imports `ddosevent` leaf only; mitigation via firewall component |
| Goroutine lifecycle | max-duration timer cancelled on clear; no leak |
| Rule: plugin-self-containment | removing the plugin removes local mitigation + the clear verb, nothing else |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| responder plugin registered | `make ze-inventory` lists `ddoslocal` |
| clear verb registered | `make ze-command-list` shows `clear ddos-mitigation` |
| functional tests pass | `ze-test bgp plugin test/plugin/ddos-local-*.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Collateral damage | match cannot be broader than the vector; allowlist enforced before install |
| Lock-out | allowlist must cover control plane / mgmt by default; alert-mode default |
| Stale state | max-duration valve + reconcile on restart prevent orphaned rules |
| Input validation | YANG ranges + allowlist prefix bounds enforced |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Local mitigation drops the flood **at this host**; it does not protect the upstream access link. For
  volumetric attacks above link capacity, child 3 (flowspec) signals upstream instead/as well.
- nft-only clear-signal validity (R-1); VPP detection-signal visibility is a child-1/umbrella concern (R-2).

## Design Insights
- Because nft drops after the NIC RX counter, local mode needs no special clear logic: the detector's
  rate-based `AttackCleared` is already correct, and the responder just tears down its table.

## Implementation Audit

| AC | Evidence |
|----|----------|
| AC-1 | `buildDropTerm` builds surgical term from vector (`match.go:19-39`); `TestBuildTermFromVector` passes |
| AC-2 | `shouldMitigate` checks allowlist overlap (`match.go:42-52`); `TestAllowlistSubtraction` passes |
| AC-3 | `onCleared` calls `removeMitigation` (`responder.go:93-100`); `TestClearedDeactivates` passes |
| AC-4 | `onDetected` returns early on alert mode (`responder.go:54-57`); `TestAlertModeInstallsNothing` passes |
| AC-8 | `familyFromPrefix` selects FamilyIP/FamilyIP6 (`responder.go:108-112`); firewall abstraction gives VPP parity |

### Files created
- `internal/plugins/ddoslocal/match.go` -- vector → firewall term + allowlist subtraction
- `internal/plugins/ddoslocal/match_test.go` -- 3 tests
- `internal/plugins/ddoslocal/responder.go` -- event handlers, install/remove, injectable firewall
- `internal/plugins/ddoslocal/responder_test.go` -- 3 tests
- `internal/plugins/ddoslocal/config.go` -- config parse + validation
- `internal/plugins/ddoslocal/register.go` -- plugin registration, YANG, lifecycle
- `internal/plugins/ddoslocal/yang/` -- YANG schema, embed, register

### Deferred to follow-up
- AC-5 (max-mitigation-duration timer): timer infrastructure not wired yet
- AC-6 (`clear ddos-mitigation` CLI verb): CLI command registration not yet implemented
- AC-7 (GetCounters for incident recording): depends on child 4 (observability)

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ddoslocal`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-2-local-responder.md`
