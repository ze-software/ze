# Spec: followup-test-infra

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Build the missing test infrastructure that blocks four classes of deferred tests. Each needs a framework/runner that does not exist yet in-tree; the individual tests are cheap once it lands.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Property-test framework (L92,L93,L94,L96)** - no `testing/quick`/`rapid` harness in-tree (only vendored). Adopt one, then: listener-conflict symmetric+transitive (L92), round-trip migration (L93), overflow ordering under concurrency (L94), filter-chain random UPDATEs (L96).
- **Privileged CI runner (L200,L197)** - no CAP_NET_ADMIN/root+netns in the shared `.ci` runner. L200: `tc qdisc show` kernel-state assertion on `test/traffic/001-boot-apply.ci`. L197: 1M-prefix pprof comparison run (benchmark+harness exist, only the privileged run is outstanding).
- **Two-peer wire-forwarding proof (L121,L80)** - conn->peer determinism already resolved (`internal/test/peer/peer_connmap.go`). Remaining: MP_REACH next-hop-self two-peer `.ci` (L121) and LLGR egress-suppress multi-peer test (L80).
- **Stress / chaos gaps (L95,L97,L98)** - iface chaos harness - ze-chaos is BGP-only (L95); web concurrent-edit stress (L97); fleet >100-concurrent-client perf (L98).

## Required Reading

### Source files / docs

- [ ] `internal/component/config/listener.go` (conflict detection)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/test/peer/peer_connmap.go` (deterministic conn->peer already exists)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/plugins/traffic/netlink/integration_linux_test.go` (privileged backend test pattern)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `mk/test-chaos.mk`, `cmd/ze-chaos/` (multi-peer chaos harness, BGP-only today)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/test/peer/peer_connmap.go`
- [ ] `internal/component/config/listener.go`
- [ ] `cmd/ze-chaos/`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Test runner (`ze-test`, `ze-chaos`) and Go `testing` harness
- Config listener conflict detection; traffic backend; multi-peer BGP forwarding

### Transformation Path
1. A framework/runner gap is closed (property harness, privileged runner, chaos multi-peer, or two-peer wire)
2. The corresponding deferred test is written against it
3. The test exercises the real path through the daemon / backend

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner -> kernel | CAP_NET_ADMIN/netns for real qdisc/route state | [ ] |
| Go test -> property engine | generated inputs drive the unit under test | [ ] |

### Integration Points
- `internal/test/runner/` (the .ci runner)
- `internal/test/peer/` (multi-peer harness)
- `mk/test-chaos.mk` (chaos orchestration)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Property test generates random listener configs | → | conflict detector is symmetric+transitive | (fill during design) |
| Privileged runner applies HTB qdisc | → | `tc qdisc show` reflects it | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (new .ci per item) | test/property, test/chaos, test/traffic | property/chaos/privileged coverage | |

## Files to Modify

- `internal/test/peer/peer_connmap.go` - see Task work items
- `internal/component/config/listener.go` - see Task work items
- `cmd/ze-chaos/` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

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
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
