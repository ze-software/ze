# Spec: ddos-direction-allowlist-deferred-flowspec-withdraw -- withdraw on late exemption

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
3. `plan/learned/1110-ddos-direction-allowlist.md` - the source spec's learned summary
4. `internal/plugins/ddos/flowspec/responder.go` - `onCharacterized`

## Task

**The flowspec responder returns on `SuppressMitigation` without withdrawing an
already-installed blackhole-fallback announce.** The local responder withdraws in the
same situation; flowspec does not. The two responders disagree.

Verified 2026-07-16 at the producer, `internal/plugins/ddos/flowspec/responder.go`:

| Line | Code | Consequence |
|------|------|-------------|
| `:154` | `func (r *responder) onCharacterized(e *ddosevent.AttackCharacterized)` | entry |
| `:158-162` | `if r.cfg.ResponseLevel != responseEnforce { log; return }` | alert mode, bare return |
| `:163-166` | `if e.SuppressMitigation { log("policy exempts mitigation, not announcing"); return }` | **bare return: no withdraw** |
| `:167-170` | `if e.Direction == DirectionLocal { log; return }` | bare return |

The blackhole-fallback announce is installed by `onDetected`, which acts on
`AttackDetected` and is deliberately NOT confidence-gated (comment at `:175-177`). So the
sequence that bites is: detect (blackhole announced) -> characterize with a source-allow
exemption -> `onCharacterized` returns at `:164` -> the blackhole announce stays up.

**Severity: minor, and the row said so.** The path is rare and opt-in: it needs
blackhole-fallback enabled AND critical severity AND a late source-allow exemption. The
announce still clears normally on attack-end and on max-mitigation-duration, so it is a
delayed withdraw, not a permanent one.

**Provenance.** Recorded as a Review Gate NOTE on `spec-ddos-direction-allowlist`
2026-07-12; the spec was closed in `0814dc93f` and `git rm`'d, so that NOTE now exists
only in git history. `plan/learned/1110-ddos-direction-allowlist.md` does NOT record it
(verified: no `onCharacterized` mention, no Known Limitations section). Without this
file the finding would survive only in a dangling deferral row.

**Open design question.** Whether the fix is "withdraw at `:164`" or "make the two
responders share one exemption path". Three of the four early returns in
`onCharacterized` have the same shape, so a targeted patch at one of them may just move
the inconsistency rather than remove it.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ddos.md` - blackhole fallback and flowspec response levels
  → Constraint: an operator-visible announce must not outlive the policy that justifies it
- [ ] `ai/rules/fail-closed-guards.md` - a guard that neither denies nor speaks does not exist
  → Constraint: the exemption branch currently logs but leaves state installed; logging is not withdrawing
- [ ] `plan/learned/1110-ddos-direction-allowlist.md` - the direction/allowlist design
  → Decision: `SuppressMitigation` (not `Mitigate`) so the bool zero value means mitigate, fail-safe

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8955.md` - BGP Flow Specification; the announce/withdraw this spec corrects
  → Constraint: a flowspec rule is withdrawn by withdrawing its NLRI; there is no implicit expiry on the peer

**Key insights:**
- `onDetected`'s blackhole fallback is intentionally ungated (`:175-177`), which is exactly why a later characterization must be able to undo it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/flowspec/responder.go` - `onCharacterized` (`:154`) returns bare at `:164` on `SuppressMitigation`, at `:161` in alert mode, and at `:169` for local victims; `announce` is called only at the end (`:182`); `onDetected` installs the ungated blackhole fallback
- [ ] `internal/plugins/ddos/local/responder.go` - the local responder DOES withdraw in the same situation; this is the reference behavior
- [ ] `internal/core/ddosevent/event.go` - `SuppressMitigation` / `Direction` / `Confidence` carried on the event

**Behavior to preserve:**
- `SuppressMitigation` polarity: the zero value means mitigate (`plan/learned/1110`). A withdraw must never be triggered by a default-constructed event.
- Attack-end and max-mitigation-duration withdraw paths must keep working; this spec adds a withdraw, it does not replace the existing ones.
- Alert mode (`ResponseLevel != responseEnforce`) must continue to announce nothing.
- Non-exempt characterizations must announce exactly as they do today.

**Behavior to change:**
- On `SuppressMitigation`, an already-installed blackhole-fallback announce is withdrawn instead of left up.

## Data Flow (MANDATORY)

### Entry Point
- `ddosevent.AttackDetected` reaches `onDetected` (may install the blackhole fallback), then `ddosevent.AttackCharacterized` reaches `onCharacterized` (`responder.go:154`).

### Transformation Path
1. Detector emits `AttackDetected`; policy outcome is already encoded on the event (1110's decision).
2. `onDetected` installs a blackhole-fallback announce if configured and severity is critical. It is not confidence-gated.
3. Detector later emits `AttackCharacterized`, this time carrying `SuppressMitigation` because a source-allow rule matched.
4. `onCharacterized` hits `:163-166`, logs, and returns.
5. The announce from stage 2 is never withdrawn here; it clears only at attack-end or max-mitigation-duration.

Stage 4 is the defect: the branch knows mitigation is exempt and is the only place that can undo stage 2 promptly.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Detector ↔ flowspec responder | `ddosevent` struct fields | [ ] |
| Responder ↔ BGP | flowspec NLRI announce/withdraw | [ ] |
| flowspec responder ↔ local responder | independent responders that must agree on exemption semantics | [ ] |

### Integration Points
- `internal/plugins/ddos/flowspec/responder.go` `announce` (`:182`) and its withdraw counterpart - the pair that must be symmetric.
- `internal/plugins/ddos/local/responder.go` - the reference implementation that already withdraws.
- `r.active` (`:171`) - the responder's installed-state flag the withdraw must respect.

### Architectural Verification
- [ ] No bypassed layers (withdraw goes through the responder's existing announce/withdraw path)
- [ ] No unintended coupling (flowspec and local responders stay independent)
- [ ] No duplicated functionality (reuse the existing withdraw, do not add a second one)
- [ ] Registration over hardcoding — no new per-feature switch in shared ddos code (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An installed blackhole announce SHOULD be withdrawn when a late exemption arrives | The local responder does this; the deferral row calls flowspec's behavior the odd one out | The current behavior is correct and the local responder is the bug; the fix inverts | Confirm intended semantics with Thomas before coding | unvalidated |
| A-2 | `r.active` accurately reflects whether an announce is installed | `:171` reads it as a guard | The withdraw may fire with nothing installed, or miss one that is | Read the `onDetected` path and every `r.active` write | unvalidated |
| A-3 | The other two bare returns (`:161` alert mode, `:169` local victim) do not need the same withdraw | Only the `SuppressMitigation` branch was reported | The same defect exists three times and a one-line fix leaves two | Trace each branch against an installed blackhole announce | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Withdrawing on a spurious exemption drops mitigation during a live attack | Announce flaps during a flood | Gate the withdraw on `r.active`; add a functional test that flaps the exemption |
| R-2 | A one-line patch fixes the reported branch and leaves A-3's siblings | Review finds the same shape nearby | Address all three branches together or record why not |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Detected (blackhole announced) then characterized with `SuppressMitigation` | → | `onCharacterized` withdraw branch | `TestFlowspecWithdrawsOnLateExemption` |
| Detected then characterized without exemption | → | `onCharacterized` announce path | `TestFlowspecAnnouncesWhenNotExempt` |
| End-to-end: flood, blackhole, late source-allow | → | detection → flowspec → BGP withdraw on the wire | `test/plugin/ddos-flowspec-late-exempt.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Blackhole fallback installed, then `SuppressMitigation` characterization | The announce is withdrawn promptly, not at attack-end |
| AC-2 | `SuppressMitigation` characterization with nothing installed | No withdraw is sent; no error |
| AC-3 | Characterization without exemption | Announce behavior byte-identical to today |
| AC-4 | Alert mode (`ResponseLevel != responseEnforce`) | Still announces nothing and withdraws nothing |
| AC-5 | Default-constructed event (`SuppressMitigation` false) | Mitigates; polarity preserved (`plan/learned/1110`) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator's late source-allow rule exempts an in-progress mitigation | detect → blackhole announce → characterize (exempt) → withdraw on the wire | `test/plugin/ddos-flowspec-late-exempt.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFlowspecWithdrawsOnLateExemption` | `internal/plugins/ddos/flowspec/responder_test.go` | AC-1 | |
| `TestFlowspecNoWithdrawWhenNothingInstalled` | `internal/plugins/ddos/flowspec/responder_test.go` | AC-2 | |
| `TestFlowspecAnnouncesWhenNotExempt` | `internal/plugins/ddos/flowspec/responder_test.go` | AC-3, AC-5 | |
| `TestFlowspecAlertModeSilent` | `internal/plugins/ddos/flowspec/responder_test.go` | AC-4 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Confidence` vs `ConfidenceMin` | 0-100 | ConfidenceMin | ConfidenceMin-1 (no announce) | N/A |

<!-- Confidence is listed because the withdraw must not be confidence-gated the way the
     announce is: onDetected's blackhole fallback is ungated (:175-177), so a withdraw
     gated on confidence could strand it. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-flowspec-late-exempt` | `test/plugin/ddos-flowspec-late-exempt.ci` | blackhole announced, late exemption, withdraw observed on the BGP wire | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/plugins/ddos/flowspec/responder.go` - withdraw at the `SuppressMitigation` branch (`:163-166`); address A-3's sibling branches
- `plan/learned/1110-ddos-direction-allowlist.md` - record this finding, which the closed spec's Review Gate NOTE carried and the learned summary omitted

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | none expected: no config surface |
| Functional test for new RPC/API | [ ] | `test/plugin/ddos-flowspec-late-exempt.ci` |
| Prometheus counters/metrics | [ ] | consider a counter for exemption-triggered withdraws |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | no: a correctness fix |
| 6 | Has a user guide page? | [ ] | `docs/guide/ddos.md` if exemption timing is documented |

## Files to Create
- `test/plugin/ddos-flowspec-late-exempt.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; resolve A-1 with Thomas |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 14. Present summary + close | two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Settle semantics (MANDATORY FIRST)** — resolve A-1 and A-3
   - Verify: Thomas confirms withdraw-on-exemption is intended, and every affected branch is enumerated
2. **Phase: Wiring** — failing test for the withdraw
   - Tests: `TestFlowspecWithdrawsOnLateExemption`
   - Verify: FAILS (no withdraw today)
3. **Phase: Fix** — withdraw at the exemption branch, guarded by `r.active` (A-2)
   - Tests: all four unit tests
   - Verify: pass; AC-3/AC-5 prove no regression
4. **Phase: Functional proof** — `ddos-flowspec-late-exempt.ci` observes the withdraw on the wire
5. **Full verification** → `make ze-verify`
6. **Complete spec** → learned summary + the 1110 record; TWO commits (A: code+tests+spec+learned; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have a test |
| Fail-closed | The withdraw never fires on a default-constructed event (AC-5) |
| Correctness | flowspec and local responders now agree on exemption semantics |
| Data flow | The withdraw uses the existing announce/withdraw pair, not a new path |
| Rule: no-workarounds | Fix the branch, do not paper over it at attack-end |
| Registration over hardcoding | No new per-feature switch in shared ddos code (`ai/rules/plugin-self-containment.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Withdraw wired at the exemption branch | read `onCharacterized`; the `SuppressMitigation` branch withdraws before returning |
| 1110 records the finding | grep 1110 for the flowspec withdraw note |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Mitigation bypass | Can an attacker induce a spurious exemption and force a withdraw mid-attack? (R-1) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 inverted (current behavior is correct) | Re-scope: the local responder becomes the bug; report to Thomas before touching either |
| A-3 confirms three defective branches | Widen scope to all three in this spec; do not leave siblings |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The finding survived in `plan/learned/1110` | It lived only in the closed spec's Review Gate NOTE and a deferral row; 1110 never recorded it | Grep during the 2026-07-16 deferral sweep | A closed spec's Review NOTE is not a durable home; this spec is |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- A Review Gate NOTE on a spec that is later `git rm`'d evaporates unless it lands in the learned summary. That is a closure-process gap, and it is why this finding needed a spec of its own to survive.

## Known Limitations
- (fill during design)

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/plugins/ddos/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
