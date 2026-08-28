# Spec: wire-edit-2-deferred-ci-substitution -- did child 2 close on substitute tests it was allowed to substitute?

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | docs |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Status set to `blocked` on 2026-08-05**, from `skeleton`. Reason: blocked: this spec is a JUDGEMENT for Thomas, not work. Its own Task says nobody may answer it on his behalf. It was
reachable from `/ze-status` as actionable until now, which is what a triage of every
`*-deferred-*` spec found.

## Task

**This spec exists to put a JUDGEMENT in front of Thomas. It is a decision to be
taken, not work to be done.** Nobody may answer it on his behalf, because the
answer decides whether an already-closed spec closed correctly.

Child 2 of the wire-edit series (`spec-wire-edit-2-edit-apply`, closed) named three `.ci` files in its Wiring Test
table and its Functional Tests table. None was ever created. Confirmed absent on
2026-08-02:

| Planned `.ci` | What its wiring row claimed to prove |
|---------------|--------------------------------------|
| `test/plugin/wire-edit-single-materialise.ci` | one edit set, one size query, one materialization into a per-peer buffer |
| `test/plugin/wire-edit-rr-attr-order.ci` | ORIGINATOR_ID and CLUSTER_LIST merge-inserted at their ascending positions |
| `test/plugin/wire-edit-oversize-suppress.ci` | the size query fails, the route is suppressed for that destination, the counter increments |

The spec was closed by `65c5eb401` (commit A) and `7ee1dd947` (commit B). Its
closure recorded substitutes rather than the planned files. All three substitutes
exist and were confirmed on 2026-08-02:

| Substitute | Location |
|------------|----------|
| `modify-oversize-suppress.ci` | `test/plugin/modify-oversize-suppress.ci` |
| `wire-edit-api-origin-order.ci` | `test/plugin/wire-edit-api-origin-order.ci` |
| `TestModifyPathZeroAlloc` | `internal/component/bgp/reactor/forward_build_merge_test.go` |

The closure also recorded a reason the second planned file could not be written
as specified. Child 2's closure record states it as a
gotcha: a route-reflector `.ci` cannot discriminate merge-insert from append,
because RR adds ORIGINATOR_ID (9) and CLUSTER_LIST (10) to a base of 1, 2, 3 and
5, so appending is ALREADY ascending. The announce case discriminates, because it
injects 2, 3 and 5 before the caller's 8 and 32. That is why
`wire-edit-api-origin-order.ci` stands in for `wire-edit-rr-attr-order.ci`.

**The question for Thomas.** Is that substitution accepted?

| Answer | Consequence |
|--------|-------------|
| Accepted | Nothing to build. Record the acceptance in the deferral row so the next reader is not left to re-derive it, close that row, and delete this spec. |
| Rejected | Child 2 closed with an unmet wiring row, which `ai/rules/completion.md` says is never deferrable. The three planned `.ci` files must be written, and the closure record corrected to say the spec closed early. |

Two of child 2's five wiring rows named EXISTING tests
(`bgp-rs-community-strip-multi.ci`, `bgp-rs-fastpath-ebgp-shared.ci`) and are not
in question. Only the three new-file rows are.

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 2. Raised in
the closure report and filed here because it is Thomas's call, not the closing
agent's.

## Required Reading

- [ ] `ai/rules/completion.md` - "Wiring Tests (BLOCKING -- NEVER deferrable)"
  → Constraint: a wiring row that cannot be written means the feature is blocked, not done.
- [ ] `ai/rules/interop-and-goal-validation.md` - "Prove the test discriminates"
  → Constraint: a test that passes whether or not the behavior is present is not evidence. This is the exact argument behind the RR substitution.
- [ ] child 2's closure record (`spec-wire-edit-2-edit-apply`, closed; its text is in the history of commits `65c5eb401` and `7ee1dd947`) - the closure record and its gotcha
  → Decision: merge-insert at the ascending type-code position was a deliberate wire change Thomas approved on 2026-08-01.

**Key insights:**
- The RR reason is a discrimination argument, not a convenience argument. It is the strongest form the substitution case can take.
- The other two substitutions carry no such recorded argument. They must be judged on their own.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_build_merge_test.go` - `TestModifyPathZeroAlloc`, the unit-test substitute
- [ ] `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload`, the materialization the planned single-materialise test would have counted

**Behavior to preserve:** every substitute test keeps passing whichever way the decision goes. No test is deleted to tidy the record.

**Behavior to change:** none, unless Thomas rejects the substitution. Then three `.ci` files are added and no production code moves.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A reader looking for what proved child 2's acceptance criteria.

### Transformation Path
1. The reader finds child 2's closure record and its gotcha about the RR case.
2. The reader looks for the wiring rows the closed spec named, which git history holds at `65c5eb401`.
3. The reader finds three named `.ci` files that do not exist on disk.
4. Without a recorded decision, the reader cannot tell an approved substitution from an unfinished closure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Closed spec ↔ learned summary | the closure record is the only surviving statement of what was proven | Yes, read on 2026-08-02 |
| Wiring claim ↔ test on disk | a named `.ci` path | Yes, all three named files confirmed absent |

### Integration Points
- `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` - where the answer must be recorded either way.
- `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` - the row this spec homes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | N-A | no production code in scope |
| No unintended coupling | N-A | no production code in scope |
| No duplicated functionality | No | fill during design: a new `.ci` must not duplicate what `modify-oversize-suppress.ci` already covers |
| Zero-copy preserved where applicable | N-A | no code path touched |
| Registration over hardcoding (`ai/rules/plugins.md`) | N-A | no command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The RR discrimination argument is sound: an RR `.ci` genuinely cannot tell merge-insert from append. | Child 2's closure gotcha, and the attribute type codes it cites. | The RR test IS writable and must be written. | re-derive the type-code ordering from RFC 4271 Section 5 before asking | unvalidated |
| A-2 | `modify-oversize-suppress.ci` covers everything `wire-edit-oversize-suppress.ci` would have. | Both name the same behavior. | The oversize row is unmet and the planned file is owed. | read the `.ci` and compare it against the planned wiring row | unvalidated |
| A-3 | `TestModifyPathZeroAlloc` is an acceptable substitute for a `.ci`. | The closure recorded it as one. | A unit test is not a `.ci` (`ai/rules/testing.md`), so the single-materialise row is unmet. | ask Thomas; this is the weakest of the three substitutions | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The decision is taken by an agent rather than by Thomas, which reproduces the failure this spec exists to correct. | A closure that answers the question without a recorded reply from Thomas. | The spec may not be closed on an agent's own judgement. Ask, then record the reply verbatim. |
| R-2 | Accepting the substitution normalizes closing a spec on tests other than the ones its wiring table named. | Later specs cite this one as precedent. | Whatever the answer, record the REASON, so the precedent is the argument and not the outcome. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at runtime. A wrong answer leaves either a false coverage claim in a learned summary, or three tests written for no reason. |
| How is it reverted? | Single commit revert. |
| Who else touches this path? | Any session asking what child 2 proved. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator configures an export policy whose modification exceeds the destination's maximum message size | → | the size query fails and the route is suppressed for that destination | existing `test/plugin/modify-oversize-suppress.ci` |
| An API announce injects attributes 2, 3 and 5 before a caller's 8 and 32 | → | merge-insert places each slot at its ascending type-code position | existing `test/plugin/wire-edit-api-origin-order.ci` |
| A peer sends an UPDATE to an eBGP peer with next-hop-self and a community tag | → | one edit set, one size query, one materialization | `TestModifyPathZeroAlloc` today; a `.ci` is owed if the substitution is rejected |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Thomas is asked the question above with the evidence attached | A recorded answer exists, in his words, not paraphrased |
| AC-2 | The answer is "accepted" | The deferral row states which planned test each substitute replaced and why, so the next reader does not re-derive it |
| AC-3 | The answer is "rejected" | The three planned `.ci` files exist and pass, and the closure record says child 2 closed before its wiring table was met |
| AC-4 | Either answer | The deferral row in `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` is resolved with the evidence |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModifyPathZeroAlloc` (existing) | `internal/component/bgp/reactor/forward_build_merge_test.go` | the single-materialise claim, as a unit test | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `modify-oversize-suppress.ci` (existing) | `test/plugin/modify-oversize-suppress.ci` | an oversize modification suppresses the route rather than leaking an unmodified one | |
| `wire-edit-api-origin-order.ci` (existing) | `test/plugin/wire-edit-api-origin-order.ci` | an announced route carries its attributes in ascending type-code order | |
| `wire-edit-single-materialise.ci` (owed if rejected) | `test/plugin/` | a modified route is built once per policy group | |
| `wire-edit-rr-attr-order.ci` (owed if rejected) | `test/plugin/` | a reflector client sees ORIGINATOR_ID and CLUSTER_LIST in ascending code order | |
| `wire-edit-oversize-suppress.ci` (owed if rejected) | `test/plugin/` | the suppression counter increments on an oversize modification | |

## Files to Modify
- `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` - resolve the row

## Files to Create
- `test/plugin/wire-edit-single-materialise.ci` - only if the substitution is rejected
- `test/plugin/wire-edit-rr-attr-order.ci` - only if the substitution is rejected
- `test/plugin/wire-edit-oversize-suppress.ci` - only if the substitution is rejected

## Implementation Steps

1. Validate A-1, A-2 and A-3 by reading the substitutes against the planned wiring rows. Do not ask before the evidence is assembled.
2. Put the question to Thomas with the two tables above and the three assumption verdicts. Ask which way he wants it, never whether it may be skipped (`ai/rules/rfc-compliance.md`, `ai/rules/completion.md`).
3. Record his answer verbatim.
4. Take the branch his answer selects.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three substitutions judged, not only the RR one that has a recorded argument |
| Correctness | The recorded reason is the argument, not the outcome |
| Rule: `ai/rules/evidence.md` | The answer is Thomas's, and is not inferred from the closure report |
| Registration over hardcoding | N-A |

## Known Limitations
- This spec judges child 2 only. The other wire-edit children are not in scope, and their closures are not re-opened by whatever answer lands here.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] `./le verify current mode full` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
