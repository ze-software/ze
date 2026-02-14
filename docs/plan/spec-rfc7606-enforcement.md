# Spec: RFC 7606 Enforcement

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - system architecture
4. `docs/architecture/wire/messages.md` - UPDATE wire format
5. `rfc/short/rfc7606.md` - RFC requirements
6. `internal/plugins/bgp/message/rfc7606.go` - current validation logic
7. `internal/plugins/bgp/reactor/session.go` - integration point (lines 806-1096)

## Task

Close the gaps between RFC 7606 validation detection and enforcement. The current implementation correctly **detects** malformed attributes and determines the correct action (treat-as-withdraw, attribute-discard, session-reset), but does not **enforce** the actions. Malformed UPDATEs are logged but processed normally.

**Priority:** Medium — correctness issue affecting RFC compliance

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - system architecture, UPDATE flow
  → Decision: UPDATEs are dispatched to plugins via callback; validation runs after dispatch
  → Constraint: Callback ordering means validation must either move before dispatch or suppress the UPDATE retroactively
- [ ] `docs/architecture/wire/messages.md` - UPDATE wire format
  → Constraint: UPDATE body is: withdrawn-len + withdrawn + attr-len + attrs + NLRI
- [ ] `docs/architecture/behavior/fsm.md` - FSM event handling
  → Decision: `EventUpdateMsg` is fired for all valid UPDATEs after validation

### RFC Summaries
- [ ] `rfc/short/rfc7606.md` - full RFC 7606 requirements
  → Constraint: treat-as-withdraw MUST remove routes from Adj-RIB-In (Section 2)
  → Constraint: attribute-discard MUST strip attribute, continue processing (Section 2)
  → Constraint: UPDATE with attrs but no NLRI + non-discard error → session-reset (Section 5.2)
  → Constraint: Multiple errors → use strongest action (Section 3.h)
  → Constraint: MP_UNREACH minimum length < 3 → AFI/SAFI disable or session-reset (Section 5.3)

**Key insights:**
- Validation detection is comprehensive (Section 7 per-attribute checks all implemented)
- Enforcement of detected actions is missing — treat-as-withdraw does not withdraw, attribute-discard does not discard
- Callback dispatches UPDATE to plugins before validation runs
- Section 5.2 (no-NLRI escalation) is not implemented

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/message/rfc7606.go` - validates attributes, returns action + description; does not modify the UPDATE
- [ ] `internal/plugins/bgp/reactor/session.go:806-1096` - `processMessage()` calls `onMessageReceived` callback (dispatches to plugins) THEN calls `handleUpdate()` which runs `validateUpdateRFC7606()`; all three non-session-reset actions return nil (session continues normally)
- [ ] `internal/plugins/bgp/reactor/session.go:142-144` - `MessageCallback` type dispatches raw bytes + WireUpdate to reactor
- [ ] `internal/plugins/bgp/message/notification.go` - NOTIFICATION constants exist for UPDATE errors

**Behavior to preserve:**
- All existing validation detection logic (per-attribute checks in Section 7)
- Session-reset action already works correctly (returns error → FSM transitions)
- NLRI syntax validation (`ValidateNLRISyntax`)
- Callback dispatches for valid UPDATEs (plugins must still receive good updates)
- WireUpdate zero-copy semantics
- Logging of validation results

**Behavior to change:**
- treat-as-withdraw must suppress the UPDATE (prevent plugin dispatch or signal withdrawal)
- attribute-discard must strip the malformed attribute before processing
- Section 5.2: attrs-with-no-NLRI + non-discard error must escalate to session-reset
- MP_UNREACH minimum length validation must be added
- Validation must run BEFORE callback dispatch (move validation before `onMessageReceived`)

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes arrive via TCP, read into pool buffer in `readAndProcessMessage()`
- `processMessage()` receives parsed header + body + pool buffer

### Transformation Path (current — broken)
1. `processMessage()` creates `WireUpdate` from body
2. `onMessageReceived` callback fires — dispatches UPDATE to plugins (BEFORE validation)
3. `handleUpdate()` calls `validateUpdateRFC7606()`
4. Validation returns action, but treat-as-withdraw/attribute-discard just log and return nil
5. `fsm.EventUpdateMsg` fires regardless of validation result

### Transformation Path (target — correct)
1. `processMessage()` creates `WireUpdate` from body
2. For UPDATE messages: run `validateUpdateRFC7606()` FIRST
3. If session-reset: return error (no dispatch, FSM handles)
4. If treat-as-withdraw: skip `onMessageReceived` dispatch; do NOT fire `EventUpdateMsg`; optionally send synthetic withdrawal event to plugins
5. If attribute-discard: mark discarded attributes in the validation result; dispatch with modified attribute set
6. If none: dispatch normally via `onMessageReceived`, fire `EventUpdateMsg`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session → Plugins | `onMessageReceived` callback with WireUpdate | [ ] |
| Validation → Session | `RFC7606ValidationResult` return value | [ ] |
| Session → FSM | `fsm.EventUpdateMsg` event | [ ] |

### Integration Points
- `processMessage()` in `session.go:806` - reorder validation vs dispatch
- `handleUpdate()` in `session.go:998` - enforce actions
- `validateUpdateRFC7606()` in `session.go:1024` - return richer result for attribute-discard
- `ValidateUpdateRFC7606()` in `rfc7606.go:127` - add MP_UNREACH validation, Section 5.2 check
- `MessageCallback` type - may need updated contract to signal suppressed updates

### Architectural Verification
- [ ] No bypassed layers (validation runs in session, dispatch goes through callback)
- [ ] No unintended coupling (validation remains pure function, session enforces)
- [ ] No duplicated functionality (extends existing validation, adds enforcement)
- [ ] Zero-copy preserved where applicable (WireUpdate still used for valid messages)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | UPDATE with malformed ORIGIN (length != 1) | Validation returns treat-as-withdraw; UPDATE NOT dispatched to plugins; session stays up |
| AC-2 | UPDATE with LOCAL_PREF from EBGP peer | Validation returns attribute-discard; UPDATE dispatched WITHOUT LOCAL_PREF attribute |
| AC-3 | UPDATE with path attrs but no NLRI and malformed MED | Validation escalates to session-reset per Section 5.2; NOTIFICATION sent |
| AC-4 | UPDATE with MP_UNREACH_NLRI length < 3 | Validation returns session-reset; NOTIFICATION sent |
| AC-5 | UPDATE with multiple errors (attribute-discard + treat-as-withdraw) | Strongest action (treat-as-withdraw) used per Section 3.h |
| AC-6 | Valid UPDATE (all attributes well-formed) | Dispatched to plugins normally; FSM event fires; no change from current behavior |
| AC-7 | UPDATE with malformed Community (length not multiple of 4) | UPDATE NOT dispatched to plugins; session stays up |
| AC-8 | Session-reset actions send NOTIFICATION with correct error code/subcode | NOTIFICATION includes UPDATE Message Error code (3) with appropriate subcode |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7606MPUnreachTooShort` | `internal/plugins/bgp/message/rfc7606_test.go` | MP_UNREACH length < 3 → session-reset | |
| `TestRFC7606NoNLRIEscalation` | `internal/plugins/bgp/message/rfc7606_test.go` | Attrs with no NLRI + non-discard error → session-reset (Section 5.2) | |
| `TestRFC7606MultipleErrorsStrongest` | `internal/plugins/bgp/message/rfc7606_test.go` | Multiple errors → strongest action selected (Section 3.h) | |
| `TestRFC7606CollectAllErrors` | `internal/plugins/bgp/message/rfc7606_test.go` | Validator scans all attributes, collects multiple errors | |
| `TestRFC7606TreatAsWithdrawSuppresses` | `internal/plugins/bgp/reactor/session_test.go` | treat-as-withdraw prevents callback dispatch | |
| `TestRFC7606AttributeDiscardStrips` | `internal/plugins/bgp/reactor/session_test.go` | attribute-discard strips attribute before dispatch | |
| `TestRFC7606SessionResetNotification` | `internal/plugins/bgp/reactor/session_test.go` | session-reset sends NOTIFICATION with correct code/subcode | |
| `TestRFC7606ValidUpdateUnchanged` | `internal/plugins/bgp/reactor/session_test.go` | Valid UPDATE behavior unchanged (callback + FSM event) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MP_UNREACH length | 3+ | 3 (AFI=2 + SAFI=1) | 2 | N/A |
| MP_REACH length | 5+ | 5 (already tested) | 4 (already tested) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rfc7606-treat-as-withdraw` | `test/plugin/rfc7606-withdraw.ci` | Peer sends malformed ORIGIN; session stays up; route not accepted | |
| `test-rfc7606-session-reset` | `test/plugin/rfc7606-reset.ci` | Peer sends duplicate MP_REACH; session reset with NOTIFICATION | |

### Future (if deferring any tests)
- IPv6 Extended Community (code 25) length % 20 validation — uncommon attribute, defer
- attribute-discard enforcement for all discardable attributes — start with LOCAL_PREF (most common)

## Files to Modify
- `internal/plugins/bgp/message/rfc7606.go` - add MP_UNREACH min-length check; add Section 5.2 no-NLRI escalation; support collecting multiple errors and returning strongest; return list of discardable attribute codes
- `internal/plugins/bgp/message/rfc7606_test.go` - new unit tests for gaps
- `internal/plugins/bgp/reactor/session.go` - reorder validation before callback dispatch in `processMessage()`; enforce treat-as-withdraw (suppress dispatch); enforce attribute-discard (strip before dispatch); send NOTIFICATION on session-reset with correct subcodes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] No | N/A |
| RPC count in architecture docs | [x] No | N/A |
| CLI commands/flags | [x] No | N/A |
| CLI usage/help text | [x] No | N/A |
| API commands doc | [x] No | N/A |
| Plugin SDK docs | [x] No | N/A |
| Editor autocomplete | [x] No | N/A |
| Functional test for new RPC/API | [x] No — but functional tests for validation enforcement needed | `test/plugin/` |

## Files to Create
- `test/plugin/rfc7606-withdraw.ci` - functional test: malformed attr → treat-as-withdraw (session survives)
- `test/plugin/rfc7606-reset.ci` - functional test: structural error → session reset with NOTIFICATION

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for validation gaps** - MP_UNREACH min-length, Section 5.2 no-NLRI escalation, multiple-error strongest action
   → **Review:** Do tests cover all gap items? Boundary values for MP_UNREACH?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason? Not syntax errors?

3. **Implement validation gaps** - Add MP_UNREACH case to `validateAttribute()`; add Section 5.2 check after attribute loop; refactor to collect all errors and pick strongest
   → **Review:** Does collecting all errors change the return type? Is it backwards-compatible?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL existing tests still pass? Any regressions?

5. **Write unit tests for enforcement** - treat-as-withdraw suppresses dispatch, attribute-discard strips attribute, session-reset sends NOTIFICATION
   → **Review:** Can we test session behavior without a full reactor? Mock callback sufficient?

6. **Run tests** - Verify FAIL (paste output)
   → **Review:** Tests fail for right reason?

7. **Implement enforcement in session.go** - Move validation before callback dispatch in `processMessage()`; add enforcement logic per action type
   → **Review:** Does reordering break any existing behavior? Does the callback still get called for valid UPDATEs?

8. **Run tests** - Verify PASS (paste output)
   → **Review:** All tests pass? Callback ordering correct?

9. **RFC refs** - Add RFC reference comments for Section 5.2, Section 3.h enforcement
   → **Review:** Are all protocol decisions documented?

10. **Functional tests** - Create `.ci` files for treat-as-withdraw and session-reset scenarios
    → **Review:** Do tests exercise the real code path?

11. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests deterministic?

12. **Final self-review** - Re-read all changes for bugs, edge cases, improvements

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Compilation error | `go build` fails | Step 3 or 7 — fix syntax |
| Existing tests break | Tests that passed before now fail | Step 7 — reordering broke callback contract |
| Callback not called for valid UPDATEs | Plugins stop receiving good updates | Step 7 — validation gate too aggressive |
| NOTIFICATION not sent on session-reset | No wire bytes sent | Step 7 — need sendNotification call in enforcement |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## RFC Documentation

### Reference Comments
- RFC 7606 Section 2 — treat-as-withdraw semantics (MUST remove from Adj-RIB-In)
- RFC 7606 Section 3.h — multiple errors use strongest action
- RFC 7606 Section 5.2 — attrs with no NLRI escalation
- RFC 7606 Section 5.3 — MP_UNREACH minimum length

### Constraint Comments
Every enforcement point must have a quoted RFC constraint comment above it.

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Design Insights
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| treat-as-withdraw enforcement (suppress dispatch) | | | |
| attribute-discard enforcement (strip attribute) | | | |
| Section 5.2 no-NLRI escalation | | | |
| MP_UNREACH min-length validation | | | |
| Multiple errors → strongest action | | | |
| Validation before callback dispatch | | | |
| NOTIFICATION on session-reset | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRFC7606MPUnreachTooShort | | | |
| TestRFC7606NoNLRIEscalation | | | |
| TestRFC7606MultipleErrorsStrongest | | | |
| TestRFC7606CollectAllErrors | | | |
| TestRFC7606TreatAsWithdrawSuppresses | | | |
| TestRFC7606AttributeDiscardStrips | | | |
| TestRFC7606SessionResetNotification | | | |
| TestRFC7606ValidUpdateUnchanged | | | |
| test-rfc7606-treat-as-withdraw | | | |
| test-rfc7606-session-reset | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp/message/rfc7606.go` | | |
| `internal/plugins/bgp/message/rfc7606_test.go` | | |
| `internal/plugins/bgp/reactor/session.go` | | |
| `test/plugin/rfc7606-withdraw.ci` | | |
| `test/plugin/rfc7606-reset.ci` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-8 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Feature code integrated into codebase (`internal/*`)

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make lint` passes
- [ ] Architecture docs updated with learnings
- [ ] RFC constraint comments added (quoted requirement + explanation)
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🏗️ Design
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Functional tests verify end-to-end behavior

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read
- [ ] RFC references added to code

### Completion (after tests pass)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
