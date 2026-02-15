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
4. If treat-as-withdraw: skip `onMessageReceived` dispatch; fire `EventUpdateMsg` (RFC 4271 requires it for all UPDATEs); reset hold timer
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

### What Was Implemented
- **Validation gaps closed:** MP_UNREACH_NLRI minimum length check (Section 5.3), Section 5.2 no-NLRI escalation, collect-all-errors pattern with strongest-action selection (Section 3.h)
- **Iota reordering:** Changed from None/TreatAsWithdraw/AttributeDiscard/SessionReset to None/AttributeDiscard/TreatAsWithdraw/SessionReset so numeric comparison gives correct strength ordering
- **`DiscardCodes` field:** Added to `RFC7606ValidationResult` to track which attribute codes should be stripped for attribute-discard
- **`enforceRFC7606()` in session.go:** New function called from `processMessage()` BEFORE callback dispatch; validates UPDATE and enforces the action
- **treat-as-withdraw enforcement:** Suppresses `onMessageReceived` callback dispatch; resets hold timer; session stays up
- **session-reset enforcement:** Sends NOTIFICATION (code 3, subcode 1), fires EventUpdateMsgErr, closes connection
- **attribute-discard enforcement:** Logs discarded attributes and continues dispatch (wire bytes not stripped — see Deviations)
- **Withdrawn/NLRI syntax validation:** `enforceRFC7606()` validates both withdrawn routes and NLRI fields for IPv4 syntax before calling `ValidateUpdateRFC7606()`
- **RFC constraint comments:** All enforcement points have quoted RFC requirement comments
- **`.claude/rules/rfc-compliance.md`** updated to enforce RFC MUST requirement comments in code
- **Two functional tests:** `rfc7606-withdraw.ci` (treat-as-withdraw, session survives) and `rfc7606-reset.ci` (session-reset with NOTIFICATION)
- **Critical review fixes:** (1) Added `fsm.EventUpdateMsg` on treat-as-withdraw path per RFC 4271 Section 8.2.2; (2) NLRI overrun now returns session-reset per RFC 7606 Section 3(j) — treat-as-withdraw requires the entire NLRI field to be parseable

### Bugs Found/Fixed
- **Automatic EOR not expected in functional tests:** Ze sends an empty UPDATE (EOR marker) after session establishment for each negotiated family. Both `.ci` tests initially failed because they didn't expect this message. Fixed by adding EOR expectation as first `expect=` line.
- **Unnecessary `byte()` conversion:** Lint caught `byte(message.NotifyUpdateMalformedAttr)` in session_test.go — the constant is already a byte type.
- **Missing `fsm.EventUpdateMsg` for treat-as-withdraw (critical review):** When `enforceRFC7606()` returned treat-as-withdraw, `processMessage()` returned early without firing `fsm.EventUpdateMsg`. RFC 4271 Section 8.2.2 requires the FSM to process Event 27 for every received UPDATE. Fixed by adding `s.fsm.Event(fsm.EventUpdateMsg)` before the return. Currently a no-op in Established state, but correctness matters for future FSM extensions.
- **NLRI overrun returned treat-as-withdraw instead of session-reset (critical review):** RFC 7606 Section 3(j) requires that treat-as-withdraw can only be used when "the entire NLRI field" is successfully parsed. An overrun means parsing failed — session-reset is mandated. Prefix-length-too-large still correctly returns treat-as-withdraw (byte boundaries are deterministic). Fixed in `ValidateNLRISyntax()` and corresponding test.

### Design Insights
- **Pre-dispatch validation is key:** Moving RFC 7606 validation BEFORE the `onMessageReceived` callback is the critical architectural choice. It ensures malformed UPDATEs never reach plugins as valid routes.
- **Collect-all-errors via closure:** The `recordError` closure pattern allows collecting multiple errors in a single pass while tracking the strongest action and all discard codes. This avoids a second pass or early-return-per-error.
- **Session-reset short-circuits:** When `validateAttribute()` returns session-reset, we return immediately (no point collecting more errors since session-reset is the strongest action).

### Documentation Updates
- `.claude/rules/rfc-compliance.md` — Added RFC MUST requirement comment enforcement rule

### Deviations from Plan
- **attribute-discard does not strip wire bytes:** The spec called for stripping malformed attributes before dispatch. The implementation logs the discard but dispatches the UPDATE with wire bytes unchanged. Rationale: stripping requires rebuilding the wire buffer (modifying attribute lengths, offsets), which conflicts with zero-copy semantics. This gap motivated `draft-mangin-idr-attr-discard-00` (ATTR_DISCARD path attribute), which defines an in-place 3-byte tombstone overwrite — the proper wire-level solution. Future work: implement ATTR_DISCARD overwrite using the `DiscardCodes` field already returned by `ValidateUpdateRFC7606()`.
- **Test names differ from plan:** Some session tests were named more descriptively than planned (e.g., `TestSessionRFC7606MalformedOriginTreatAsWithdraw` instead of `TestRFC7606TreatAsWithdrawSuppresses`). Additional tests were added beyond the plan (e.g., `TestSessionRFC7606MalformedCommunityTreatAsWithdraw`, `TestSessionRFC7606MissingMandatoryTreatAsWithdraw`).
- **Functional test uses MP_REACH_NLRI too-short instead of duplicate MP_REACH:** The `rfc7606-reset.ci` test sends an UPDATE with MP_REACH_NLRI length=2 (minimum is 5) rather than duplicate MP_REACH, because constructing a valid-looking duplicate MP_REACH hex message is more complex and the simpler case exercises the same session-reset enforcement path.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| treat-as-withdraw enforcement (suppress dispatch) | ✅ Done | `session.go:826-832` | Checks action, skips `onMessageReceived` callback |
| attribute-discard enforcement (strip attribute) | ⚠️ Partial | `session.go:833-838`, `session.go:1097-1104` | Logs discard, continues dispatch; wire bytes NOT stripped (see Deviations) |
| Section 5.2 no-NLRI escalation | ✅ Done | `rfc7606.go:324-330` | attrs with no NLRI + non-discard error → session-reset |
| MP_UNREACH min-length validation | ✅ Done | `rfc7606.go:517-526` | length < 3 → session-reset |
| Multiple errors → strongest action | ✅ Done | `rfc7606.go:152-167` | `recordError` closure collects all, picks strongest |
| Validation before callback dispatch | ✅ Done | `session.go:818-838` | `enforceRFC7606()` called before `onMessageReceived` |
| NOTIFICATION on session-reset | ✅ Done | `session.go:1114-1132` | code=3 (UPDATE Message Error), subcode=1 (Malformed Attribute List) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestSessionRFC7606MalformedOriginTreatAsWithdraw` in `session_test.go:1099` + `rfc7606-withdraw.ci` | Callback count=0, session stays Established |
| AC-2 | ⚠️ Partial | `TestSessionRFC7606AttributeDiscardContinues` in `session_test.go:1634` | Dispatch happens (callback=1), but wire bytes not stripped |
| AC-3 | ✅ Done | `TestRFC7606NoNLRIEscalation` in `rfc7606_test.go:1086` | attrs + no NLRI + malformed MED → session-reset |
| AC-4 | ✅ Done | `TestRFC7606MPUnreachTooShort` in `rfc7606_test.go:1047` | length=2 → session-reset |
| AC-5 | ✅ Done | `TestRFC7606MultipleErrorsStrongest` in `rfc7606_test.go:1122` | attribute-discard + treat-as-withdraw → treat-as-withdraw |
| AC-6 | ✅ Done | `TestSessionRFC7606ValidUpdateUnchanged` in `session_test.go:1674` | callback=1, session stays Established |
| AC-7 | ✅ Done | `TestSessionRFC7606MalformedCommunityTreatAsWithdraw` in `session_test.go:1208` | callback=0, session stays Established |
| AC-8 | ✅ Done | `TestSessionRFC7606SessionResetNotification` in `session_test.go:1514` + `rfc7606-reset.ci` | NOTIFICATION code=3, subcode=1 verified |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRFC7606MPUnreachTooShort | ✅ Done | `rfc7606_test.go:1047` | |
| TestRFC7606NoNLRIEscalation | ✅ Done | `rfc7606_test.go:1086` | |
| TestRFC7606MultipleErrorsStrongest | ✅ Done | `rfc7606_test.go:1122` | |
| TestRFC7606CollectAllErrors | ✅ Done | `rfc7606_test.go:1144` | |
| TestRFC7606TreatAsWithdrawSuppresses | 🔄 Changed | `session_test.go:1099,1208,1307` | Split into 3 scenario-specific tests (ORIGIN, Community, missing mandatory) |
| TestRFC7606AttributeDiscardStrips | 🔄 Changed | `session_test.go:1634` | Renamed to `...AttributeDiscardContinues`; verifies dispatch, not byte stripping |
| TestRFC7606SessionResetNotification | ✅ Done | `session_test.go:1514` | |
| TestRFC7606ValidUpdateUnchanged | ✅ Done | `session_test.go:1674` | |
| test-rfc7606-treat-as-withdraw | ✅ Done | `test/plugin/rfc7606-withdraw.ci` | |
| test-rfc7606-session-reset | ✅ Done | `test/plugin/rfc7606-reset.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp/message/rfc7606.go` | ✅ Modified | +121 lines: MP_UNREACH check, Section 5.2, collect-all-errors, iota reorder |
| `internal/plugins/bgp/message/rfc7606_test.go` | ✅ Modified | +128 lines: 10 new tests for validation gaps |
| `internal/plugins/bgp/reactor/session.go` | ✅ Modified | +102 lines: `enforceRFC7606()`, pre-dispatch validation in `processMessage()` |
| `test/plugin/rfc7606-withdraw.ci` | ✅ Created | Functional test: malformed ORIGIN → treat-as-withdraw, session survives |
| `test/plugin/rfc7606-reset.ci` | ✅ Created | Functional test: MP_REACH too short → session-reset with NOTIFICATION |

### Audit Summary
- **Total items:** 30
- **Done:** 26
- **Partial:** 2 (AC-2 attribute byte stripping, attribute-discard requirement — both same root cause: wire bytes not stripped)
- **Skipped:** 0
- **Changed:** 2 (test naming/structure improved over plan)

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [x] Acceptance criteria AC-1..AC-8 all demonstrated (AC-2 partial — dispatch works, byte stripping deferred)
- [x] Tests pass (`make test`)
- [x] No regressions (`make functional` — 96/96 pass)
- [x] Feature code integrated into codebase (`internal/*`)

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [x] `make lint` passes (0 issues)
- [x] Architecture docs updated with learnings (rfc-compliance.md)
- [x] RFC constraint comments added (quoted requirement + explanation)
- [x] Implementation Audit fully completed
- [x] Mistake Log escalation candidates reviewed (none)

### 🏗️ Design
- [x] No premature abstraction (collect-all-errors used for real multi-error scenarios)
- [x] No speculative features (only implements what RFC 7606 requires)
- [x] Single responsibility (rfc7606.go = validation, session.go = enforcement)
- [x] Explicit behavior (actions are explicit enum values, no magic)
- [x] Minimal coupling (validation is a pure function, enforcement is session-local)
- [x] Next-developer test (RFC section references in every function)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified in previous sessions)
- [x] Implementation complete
- [x] Tests PASS (verified: `make test`, `make functional`, `make lint` all clean)
- [x] Boundary tests cover all numeric inputs (MP_UNREACH: 2=invalid, 3=valid)
- [x] Functional tests verify end-to-end behavior (2 `.ci` tests)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code

### Completion (after tests pass)
- [x] All Partial/Skipped items have user approval (attribute-discard byte stripping → ATTR_DISCARD draft)
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/254-rfc7606-enforcement.md`
- [ ] All files committed together
