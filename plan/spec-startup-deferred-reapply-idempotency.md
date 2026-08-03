# Spec: startup-deferred-reapply-idempotency

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/transaction/orchestrator.go` - `filterDiffs`, `runApply`
4. `internal/plugins/ntp/register.go` - the one confirmed non-idempotent apply handler
5. `plan/learned/1104-startup-resilience.md` - the closed source spec's record

## Task

**Provenance.** Deferred out of `plan/spec-startup-resilience.md` (Known
Limitations, risk R-1). That spec is CLOSED and removed from disk; its record is
`plan/learned/1104-startup-resilience.md` and its text is in git history (deleted
by `e9a4ced83`). The closed spec was reachability-only and said so: "the audit did
not hunt for idempotency gaps and none were incidentally observed." So this is not
a known-broken list. It is an UNHUNTED SURFACE, and the first job here is the hunt.

**What "re-apply idempotency" means.** The closed spec named its own reference:
osvbng commits 424e2d0/2259ce6 fixed "startup and config re-apply resilience" as
one effort. Half was reachability (done, that spec); half was idempotency (this
spec). osvbng's concrete failures were a panicking BGP re-apply and a
non-idempotent VRF table re-apply, plus bond/subinterface fixes. Restated for Ze:
**applying config a subsystem has already applied must be a cheap no-op, not a
teardown, a rebuild, a duplicate object, or an error.** Two triggers produce a
re-apply:

| Trigger | Why the same config arrives twice |
|---------|-----------------------------------|
| Unrelated sibling edit | The coordinator delivers diffs at CONFIG-ROOT granularity, not leaf granularity. Any edit under a root re-delivers every section of that root to every participant declaring it |
| Daemon restart | Boot applies the full config against a dataplane that may still hold objects from the previous run (interfaces, routes, nft rules) |

**Mechanism, verified at the producer.** `filterDiffs`
(`internal/component/config/transaction/orchestrator.go`) matches a
participant's `ConfigRoots` / `WantsConfig` against the diff map's ROOT keys and
appends `sections...` for every matched root. It never compares a section against
what the participant last applied. `runApply` then emits one
`ApplyEvent` per participant with a non-empty filtered set. Root sharing is wide:
`environment` is declared by ntp (`register.go,182`); `service` is declared by
four independent plugins (dhcpserver, geodns, imageserver, tftpserver, each
`register.go` `configRootService = "service"`).

**One confirmed instance (the seed, not the scope).** NTP's `OnConfigApply`
(`internal/plugins/ntp/register.go`) calls `startWorker(*cfg)` whenever
`pendingCfg` is non-nil. `pendingCfg` is set by `OnConfigVerify` for
ANY delivered `environment` section, with no comparison against the running
config. `startWorker` unconditionally calls `worker.stopAndWait()`,
unsubscribes, then builds and starts a fresh worker. So an unrelated `environment`
edit tears down and rebuilds the NTP sync worker even when the ntp config is
byte-identical. This is churn, not a crash: startup-resilience fixed the BLOCKING
half of this path (stop-checks between queries, `ApplyBudget` 5->10) and
deliberately left the unconditional restart.

**The work.**

1. Audit every participant declaring `ConfigRoots` / `WantsConfig` for apply-handler
   idempotency under an unchanged section. Classify each with producer `file:line`,
   as the closed spec classified its eight reachability touchpoints. Some handlers
   already guard: static compares via `routesEqual` (`internal/plugins/static/diff.go`);
   `internal/component/iface/config_apply.go` guards on `interfaceExists` (`:443`,
   `:549`, `:1123`) and tolerates wireguard "already exists" (`:495-503`).
2. Audit the restart trigger: does boot apply collide with surviving dataplane objects?
3. Fix what the audit finds, seeded by the confirmed NTP restart.
4. Decide the durable shape: per-handler compare (the static/iface precedent) or a
   shared last-applied comparison. Do not invent a central manager without evidence
   that per-handler guards are insufficient.

**Out of scope:** changing root-granularity delivery itself. Other participants
depend on receiving sibling sections; narrowing the coordinator is a larger spec.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - small core + registration; participant discovery
  -> Constraint: no plugin spelling in the coordinator; a shared helper registers, it does not switch on plugin name
- [ ] `plan/learned/1104-startup-resilience.md` - the closed source spec's record
  -> Constraint: NTP's worker handoff must stay synchronous (single clock-writer invariant), so a fix must SKIP the handoff, never make it async

**Key insights:**
- `orchestrator.go`: `filterDiffs` selects by root, appends all sections, compares nothing
- `ntp/register.go` -> `:111-134`: apply restarts the worker unconditionally
- `static/diff.go`, `iface/config_apply.go,495-503,549`: the existing guard precedent

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/transaction/orchestrator.go` - `Execute` (:198) runs verify then apply; `filterDiffs` (:472-488) selects participant sections by config root and compares nothing; `runApply` (:374-399) emits one ApplyEvent per participant with a non-empty set; apply deadline aborts the transaction (:428-429)
- [ ] `internal/plugins/ntp/register.go` - declares root `environment` (:52, :182), `ApplyBudget: 10` (:190); `OnConfigVerify` (:154-167) stores any delivered section as `pendingCfg`; `OnConfigApply` (:169-177) calls `startWorker` unconditionally; `startWorker` (:111-134) stops and waits for the old worker, then builds a new one
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `OnConfigApply` (:328-331) registers the plugin-side apply handler
- [ ] `internal/plugins/static/diff.go` - `routesEqual` (:10), the existing unchanged-object compare
- [ ] `internal/component/iface/config_apply.go` - `interfaceExists` guard (:443, :549, :1123); wireguard already-exists tolerance (:495-503)

**Behavior to preserve:**
- Root-granularity diff delivery in `filterDiffs`: participants rely on receiving sibling sections
- A genuinely CHANGED section still re-applies with the same observable effect it has today
- NTP's single clock-writer invariant: the `startWorker` handoff stays synchronous
- Per-participant `ApplyBudget` / `VerifyBudget` deadlines and the abort-on-deadline contract
- The guards that already work: static's `routesEqual` skip, iface's `interfaceExists` and wireguard tolerance
- Registration-based participant discovery: the coordinator must not learn any plugin's name

**Behavior to change:**
- To be decided by the audit. One confirmed candidate: an apply carrying an unchanged ntp section should not restart the sync worker.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit (CLI editor commit, `ze config commit`, or a hub-driven apply) hands `TxCoordinator.Execute` a diff map keyed by config root: `map[string][]DiffSection` (`orchestrator.go`).
- Second entry point: daemon boot, applying the full config to a dataplane that may already hold objects from a previous run.

### Transformation Path
1. `Execute` (`orchestrator.go`) runs `runVerify` then `runApply`.
2. `filterDiffs` matches each participant's declared roots against the diff map's ROOT keys and appends every section under a matched root. No last-applied comparison happens here.
3. `runApply` emits one `ApplyEvent` per participant whose filtered set is non-empty, carrying the shared deadline.
4. The SDK delivers the event to the plugin's `OnConfigApply` handler (`pkg/plugin/sdk/sdk_callbacks.go`).
5. The handler re-applies its whole section. NTP (`ntp/register.go`) calls `startWorker` with the verify-phase `pendingCfg`, unconditionally.
6. `startWorker` (`ntp/register.go`) stops and waits for the existing worker, unsubscribes, builds and starts a fresh one; the plugin then acks apply.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree -> coordinator | diff map keyed by config root | [ ] |
| Coordinator -> plugin | `ApplyEvent` JSON over the plugin event gateway | [ ] |
| Plugin -> subsystem worker | in-process handler call (`startWorker`, `applyRoutes`) | [ ] |
| Plugin -> dataplane | netlink / nft object creation, where restart collisions live | [ ] |

### Integration Points
- `filterDiffs` / `runApply` (`orchestrator.go`) - the delivery producer; read-only for this spec
- `OnConfigApply` handlers across every participant declaring a config root - the audit surface
- `routesEqual` (`static/diff.go`), `interfaceExists` (`iface/config_apply.go`) - the guard precedent to follow or generalize

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing guards, doesn't recreate)
- [ ] Registration over hardcoding - any shared idempotency helper is registry-discovered; the coordinator never spells a plugin name

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The audit finds gaps beyond NTP | Root sharing is wide (`service` has 4 participants); the closed spec never hunted (R-1) | Spec shrinks to a one-plugin NTP fix | The audit in step 1 | unvalidated |
| A-2 | Skipping an unchanged NTP re-apply is safe | `startWorker` is the only clock-writer handoff (learned 1104) | Need a fix that keeps the handoff | Reload-cycle unit test | unvalidated |
| A-3 | Per-handler guards suffice; no central manager | static and iface already guard this way | Shared helper needed, larger design | Audit outcome | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A "skip if unchanged" guard skips a re-apply doing real reconciliation | A subsystem stops converging after an unrelated edit | Compare against last-APPLIED state, not last-delivered config |
| R-2 | Scope creep into narrowing root-granularity delivery | Design starts editing `filterDiffs` | Out of scope per the Task; route to a coordinator spec |
| R-3 | The audit finds nothing and the spec is a no-op | Every handler already guards | A clean audit with producer evidence is a valid outcome; record and close |

## Wiring Test (MANDATORY - NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Commit editing an unrelated `environment` leaf | -> | ntp apply handler skips the worker restart | `test/plugin/reapply-idempotency.ci` |
| Commit editing the ntp section itself | -> | ntp apply handler restarts the worker | `TestNTPApplyChangedConfigRestartsWorker` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Audit of every participant declaring a config root | Each classified idempotent / non-idempotent with producer `file:line` |
| AC-2 | Apply delivers an unchanged ntp section | Sync worker is not stopped or restarted; apply still acks within budget |
| AC-3 | Apply delivers a changed ntp section | Worker restarts, exactly as today |
| AC-4 | Every non-idempotent handler the audit finds | Fixed, or recorded as a tracked deferral with a destination spec |
| AC-5 | Daemon restart against surviving dataplane objects | Boot apply does not error or duplicate objects |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNTPApplyUnchangedConfigKeepsWorker` | `internal/plugins/ntp/register_test.go` | An apply carrying a byte-identical ntp section does not stop or restart the worker | |
| `TestNTPApplyChangedConfigRestartsWorker` | `internal/plugins/ntp/register_test.go` | AC-3: a changed section still restarts (guard is not too broad) | |
| `TestFilterDiffsDeliversWholeRoot` | `internal/component/config/transaction/orchestrator_test.go` | Pins the preserved root-granularity contract | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reapply-idempotency` | `test/plugin/reapply-idempotency.ci` | Operator edits an unrelated `environment` leaf and commits; the ntp worker is undisturbed and the commit succeeds | |

### Interop Tests
Not applicable: config-apply lifecycle work, no wire protocol behavior changes.

## Files to Modify
- `internal/plugins/ntp/register.go` - the one confirmed non-idempotent apply handler
- `internal/plugins/ntp/register_test.go` - reload-cycle unit tests
- `docs/architecture/core-design.md` - if a shared idempotency contract lands, document it
- (extended by the audit in step 1)

## Implementation Steps

1. **Phase: Audit (MANDATORY FIRST)** - classify every participant declaring a config root for unchanged-section apply behavior, with producer `file:line`. This phase decides the real scope; the classification table lands in Current Behavior.
2. **Phase: Wiring** - write the failing `.ci` and the failing NTP reload-cycle unit test; both fail because the worker restarts today.
3. **Phase: NTP fix** - skip the `startWorker` handoff when the delivered section is unchanged, keeping the handoff synchronous when it does run (A-2).
4. **Phase: Audit fixes** - fix each non-idempotent handler found, one test per fix.
5. **Phase: Restart trigger** - audit and fix boot-apply against surviving objects (AC-5).
6. **Full verification** -> `make ze-verify`.
7. **Complete spec** -> fill audit tables, write learned summary. Two commits per `ai/rules/planning.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | A skip compares against last-APPLIED state, not last-delivered config (R-1) |
| Data flow | `filterDiffs` root-granularity delivery is unchanged |
| Registration over hardcoding | No plugin name in the coordinator; a shared helper is registry-discovered |
| Rule: no-workarounds | A handler that cannot be made idempotent is a tracked deferral, never a weakened test |

## Known Limitations
- Root-granularity delivery is preserved, not fixed. Handlers become tolerant of it.
- The scope is unknown until the audit runs. This is captured intent, not a designed spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Audit classification table filled with producer `file:line` for every participant
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass - defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING - before ANY commit)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
