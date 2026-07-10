# Spec: finish-l2tp

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

Close the remaining L2TP test-coverage and documentation gaps. Core L2TP subsystem shipped (7b/7c done); these are proof-run and unit-level residuals.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Release-proof run (L44)** - interop harness complete (`ze-deployment-l2tp-ppp-test`, xl2tpd/pppd LAC + FRR lab). Open item is the release-proof RUN on a host with `/dev/ppp` + PPPoL2TP kernel support.
- **accel-ppp LCP-Opened+MTU `.ci` (L162)** - needs `/dev/ppp` + root + accel-ppp peer.
- **offline-show-tunnels `.ci` (L194)** - `ze l2tp show tunnels` SSH-creds round-trip; needs ci-harness SSH-cred plumbing.
- **NCP unit-test gaps (L41,L42,L43)** - backend-error injection (L41, mock `addAddrP2PErr` never set), renegotiation-after-Opened behavioural (L42), IPCP DNS Configure-Reject absorb (L43, `ncp.go:444` unexercised).
- **LCP restart-counter (L163)** - restart-timer landed; IRC/ZRC restart-counter/backoff + AckRcvd coverage still deferred.
- **ppp component-imports doc row (L164)** - `docs/architecture/core-design.md` component-imports table has no `ppp` row.
- **authradius coa-port YANG leaf + CoA end-to-end `.ci` (from spec-startup-resilience 2026-07-10)** - `coa-port` is parsed (`internal/component/l2tp/plugins/authradius/config.go:93-100`) but has NO YANG leaf, so the whole CoA listener branch (`register.go:200-210`) is unreachable via production config (the parser rejects unknown fields, `config/parser.go:372-380`). Add the leaf, then a functional `.ci` exercising CoA source filtering. The apply-path DNS lookup on that branch is already bounded (spec-startup-resilience FIX 2: one shared 750ms deadline < the plugin's 1s ApplyBudget); the unit tests `TestServerIPs*` pin it now.

## Required Reading

### Source files / docs

- [ ] `internal/component/l2tp/ppp/ncp.go` (absorb/error paths at :444,:696)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/l2tp/ppp/session_run.go` (LCP restart timer / IRC-ZRC at :206,:306,:896)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/l2tp-interop/`, ~~`mk/test-integration.mk:112`~~ mk/test-integration.mk :129-131 (interop harness; :112 is stale, see Post-wave corrections 2026-07-10)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/component/l2tp/ppp/ncp.go`
- [ ] `internal/component/l2tp/ppp/session_run.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- L2TP/PPP session establishment against a LAC; `ze l2tp show` CLI

### Transformation Path
1. A LAC establishes an L2TP tunnel + PPP session
2. NCP negotiates addresses/DNS; LCP restart timer governs retransmit
3. Operator queries state via `ze l2tp show tunnels`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LAC -> LNS | L2TP control + PPP over the wire | [ ] |
| CLI -> daemon | `ze l2tp show` over SSH | [ ] |

### Integration Points
- `internal/component/l2tp/ppp/`
- `test/l2tp-interop/`
- `docs/architecture/core-design.md`

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
| Real LAC establishes PPP against ze LNS | → | LCP reaches Opened + MTU set | (fill during design) |
| `ze l2tp show tunnels` against a live daemon | → | tunnel state rendered | (fill during design) |

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
| accel-ppp-lcp, offline-show-tunnels (new) (`.ci`) | test/l2tp-interop, test/plugin | LNS behaviour vs a real LAC / live daemon | |

## Files to Modify

- `internal/component/l2tp/ppp/ncp.go` - see Task work items
- `internal/component/l2tp/ppp/session_run.go` - see Task work items

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

### Post-wave corrections (2026-07-10)

- Stale line ref fixed: `mk/test-integration.mk:112` no longer points at the l2tp interop harness (line 112 now falls inside the deployment-evidence VPP block, between the `ze-deployment-vpp-test` recipe at :109-111 and `ze-deployment-vpp-iface-test` at :113). Verified current l2tp locations: the l2tp `.PHONY` declarations are at mk/test-integration.mk:23-24 (plus `ze-qemu-l2tp-ppp-test` in the QEMU line :27); the target recipes are at :121-139 (`ze-deployment-l2tp-test` :121, `ze-deployment-l2tp-ppp-test` :125, `ze-deployment-l2tp-ppp-docker-test` :129, which invokes the `test/l2tp-interop/run.py` harness at :131, `ze-deployment-gokrazy-l2tp-ppp-test` :137) and `ze-qemu-l2tp-ppp-test` at :337. Core NCP/LCP evidence (`ncp.go:444`, `session_run.go` refs) is untouched by the wave.
- Coordination note: this spec is DISTINCT from the in-flight `plan/spec-followup-l2tp-call.md` (designed, in-progress as of 2026-07-10). Whoever picks this skeleton up must check that spec's state at design time and coordinate scope so neither duplicates nor contradicts the other's l2tp test work.
