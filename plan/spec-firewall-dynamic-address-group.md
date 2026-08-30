# Spec: firewall-dynamic-address-group

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

Anchor refresh (2026-07-22 plan review, design unchanged; the dead-knob claim
re-verified -- `applySet` sets only `Interval`, misleading comment at
`backend_linux.go`): citations below updated in-body -- `applySet`
`backend_linux.go+`; `list set` `ze-firewall-conf.yang`,
`flags-dynamic` `:663`. `lowerAction` (`lower_linux.go`) still exact.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/firewall/nft/lower_linux.go` - `lowerAction` (rule action lowering)
4. `internal/plugins/firewall/nft/backend_linux.go` - `applySet` (named-set creation)
5. `internal/component/firewall/yang/ze-firewall-conf.yang` - `list set` and its flags

## Task

Ze firewall supports static named sets (address groups) and can match a packet
against them, but it cannot populate a set at runtime. There is no rule action
that adds a matched packet's source or destination address into a named set, so
patterns like automatic offender/blocklists (a rule observes traffic and inserts
the address into a group that later rules drop) are impossible. The nftables
`flags dynamic` / set-level timeout knobs already exist in config and parse, but
are dead on the apply path.

Add a firewall rule action that inserts the matched packet's source or
destination address into a named set at runtime (nftables `set update` /
`expr.Dynset`), and finish wiring the set-level dynamic/timeout flags so the
target set actually holds runtime entries with expiry.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `ai/rules/plugins.md` - firewall lowering is self-contained in the nft backend.
  → Constraint: the new action registers as another `firewall.Action` case in `lowerAction`; no core change.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - the new rule-action leaf.
  → Constraint: the action names an existing `set`; validation must reject an unknown target set.

**Key insights:**
- The nftables primitive is `expr.Dynset` (emit `add @set { ip saddr }` / `{ ip daddr }`) in the rule's expression list.
- The target set must carry `flags dynamic` (and usually `flags timeout` + a default timeout) for the kernel to accept runtime insertion with expiry.
- Ze already models the flags (`SetFlagDynamic`/`SetFlagTimeout`/`SetFlagConstant`) and reads them back, but `applySet` only wires `Interval`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerAction` (lower_linux.go) switches over every action type (accept, drop, reject, jump, goto, masquerade, notrack, counter, log, limit, mark, connmark, dscp, tcp-mss, flow-offload, snat, dnat, redirect). There is no set-update / add-to-group action and no `expr.Dynset` anywhere in the package.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `applySet` (backend_linux.go+) builds the kernel `nftables.Set` but the struct literal (:217-222) sets only `Interval`; it never sets `Dynamic`/`HasTimeout`/`Constant`. The comment at :234 claims the timeout flag is applied "at set construction above", which the literal does not do.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `list set` (ze-firewall-conf.yang) with `flags-timeout`/`flags-constant`/`flags-dynamic` containers (:653-663) that parse into model flags but are inert on apply.

**Behavior to preserve:**
- Static named sets, `@set` matching, and per-element values/timeouts keep working unchanged.
- All existing rule actions and their lowering are untouched.
- A set without the dynamic flag behaves exactly as today.

**Behavior to change:**
- A new rule action lowers to `expr.Dynset` populating a named set with `ip saddr`/`ip daddr`.
- `applySet` wires `Dynamic`/`HasTimeout`/`Constant` onto the kernel set so runtime entries (with expiry) are accepted.

## Data Flow (MANDATORY)

### Entry Point
- Config: a new rule action naming a target `set` and a source/destination selector (which address to insert).
- Config: an existing `set` marked `flags-dynamic` (and typically `flags-timeout` + default timeout).

### Transformation Path
1. The rule action parses into a new `firewall.Action` value carrying the target set name and the saddr/daddr selector.
2. Validation confirms the named set exists and is a compatible ipv4/ipv6 type and is flagged dynamic.
3. `applySet` creates the kernel set with `Dynamic`/`HasTimeout` so it can hold runtime entries with a default timeout.
4. `lowerAction` emits `expr.Dynset` (plus the payload load for saddr/daddr) so a matching packet inserts its address into the set.
5. Later rules match `@set` as they already do; entries expire per the set timeout.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ firewall model | new action → `firewall.Action` with set + selector | [ ] |
| Model ↔ nft backend | `lowerAction` emits `expr.Dynset` | [ ] |
| Backend ↔ kernel | `applySet` sets `Dynamic`/`HasTimeout` on `nftables.Set` | [ ] |

### Integration Points
- `internal/component/firewall/yang/ze-firewall-conf.yang` - new rule-action leaf in the `then` block.
- `internal/component/firewall/model.go` - new `Action` type (target set + selector).
- `internal/component/firewall/config.go` - parse the action.
- `internal/component/firewall/validate.go` - reject unknown/incompatible/non-dynamic target set.
- `internal/plugins/firewall/nft/lower_linux.go` - `lowerAction` case emitting `expr.Dynset`.
- `internal/plugins/firewall/nft/backend_linux.go` - wire `Dynamic`/`HasTimeout`/`Constant` in `applySet`.

### Architectural Verification
- [ ] No bypassed layers (runtime insertion goes through the nft backend like every other action)
- [ ] No unintended coupling (the action names a set by string; no cross-package pointer)
- [ ] No duplicated functionality (reuse the existing set model and matching)
- [ ] Registration over hardcoding - the new action is another `firewall.Action` case in `lowerAction`; no per-feature field added to a core/shared package.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The vendored `google/nftables` exposes `expr.Dynset` | package already imports `expr` | need a raw-expr fallback | check the lib API during audit | unvalidated |
| A-2 | `nftables.Set` exposes `Dynamic`/`HasTimeout` fields the readback already reads | readback_linux.go maps them | apply cannot set the flags | audit the struct | unvalidated |
| A-3 | A dynamic set needs a default timeout to be useful | nftables semantics | entries never expire | require/derive a set timeout | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Inserting into a non-dynamic set errors at commit | kernel rejects `add @set` | validate the set is `flags-dynamic` before apply |
| R-2 | Unbounded set growth without a timeout | set fills memory | require flags-timeout + default timeout for dynamic-populated sets |
| R-3 | ipv4 action targeting an ipv6 set (or vice versa) | type mismatch at apply | validate set-type vs rule family |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| rule action add-to-group (source) on match | → | `lowerAction` emits `expr.Dynset` saddr | `test/ci/firewall-dynamic-group.ci` |
| target set `flags-dynamic` | → | `applySet` sets `Dynamic`/`HasTimeout` | `test/ci/firewall-dynamic-group.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | rule matches, action adds source to group | matched src appears in the named set |
| AC-2 | later rule matches `@group` | subsequent traffic from that src is actioned |
| AC-3 | set has `flags-timeout` + timeout | runtime entry expires after the timeout |
| AC-4 | action targets an unknown set | commit rejected with a clear error |
| AC-5 | action targets a non-dynamic set | commit rejected with a clear error |
| AC-6 | ipv4 action targeting ipv6 set | commit rejected (family mismatch) |
| AC-7 | destination selector variant | matched dst inserted instead of src |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds an auto-blocklist: one rule adds offenders to a group, another drops the group | action → `expr.Dynset` → dynamic set → `@set` match | `test/ci/firewall-dynamic-group.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerActionDynset` | `internal/plugins/firewall/nft/lower_linux_test.go` | add-to-group lowers to `expr.Dynset` saddr/daddr | |
| `TestApplySetDynamicFlags` | `internal/plugins/firewall/nft/backend_linux_test.go` | `applySet` sets Dynamic/HasTimeout | |
| `TestValidateAddToGroupTarget` | `internal/component/firewall/validate_test.go` | unknown/non-dynamic/family-mismatch rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| set timeout (seconds) | 0..uint32 | 4294967295 | - | overflow rejected |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-dynamic-group` | `test/ci/firewall-dynamic-group.ci` | offender auto-added to group, then dropped; entry expires | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - kernel nftables feature; validated by functional test | - | - | dynamic set population is a kernel behaviour | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang` - add the add-to-group rule action
- `internal/component/firewall/model.go` - new `Action` type (target set + saddr/daddr selector)
- `internal/component/firewall/config.go` - parse the action
- `internal/component/firewall/validate.go` - validate target set exists, is dynamic, family matches
- `internal/plugins/firewall/nft/lower_linux.go` - `lowerAction` case emitting `expr.Dynset`
- `internal/plugins/firewall/nft/backend_linux.go` - wire `Dynamic`/`HasTimeout`/`Constant` in `applySet`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new action) | [ ] yes | `ze-firewall-conf.yang` then-block; `ai/rules/config.md` |
| Functional test | [ ] yes | `test/ci/firewall-dynamic-group.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/ci/firewall-dynamic-group.ci` - functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the rule-action leaf + `Action` type (parsed, unused); failing `test/ci/firewall-dynamic-group.ci`.
2. **Phase: Set flags** - wire `Dynamic`/`HasTimeout`/`Constant` in `applySet`; fix the misleading comment.
   - Tests: `TestApplySetDynamicFlags`
3. **Phase: Action lowering** - `lowerAction` case emitting `expr.Dynset` for saddr/daddr.
   - Tests: `TestLowerActionDynset`
4. **Phase: Validation** - reject unknown/non-dynamic/family-mismatch target set.
   - Tests: `TestValidateAddToGroupTarget`
5. **Functional test** - auto-blocklist end to end incl. expiry.
6. **Full verification** → `./le verify current mode full`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | Dynset targets the right set; flags wired; expiry works; non-dynamic rejected |
| No dead knobs | flags-dynamic/timeout now reach the kernel (comment matches code) |
| Registration over hardcoding | new action is a `lowerAction` case; no core/shared change |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| action lowering | `go test ./internal/plugins/firewall/nft -run Dynset` |
| set flags | `go test ./internal/plugins/firewall/nft -run DynamicFlags` |
| functional | `test/ci/firewall-dynamic-group.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource bounds | dynamic set requires a timeout to bound growth |
| Input validation | target set name validated; family match enforced |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (set timeout)
- [ ] Functional tests for end-to-end behavior
