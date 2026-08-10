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
3. `internal/plugins/ddos/flowspec/responder.go` - `onCharacterized`

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

**The deferral row's "minor" severity is STALE. Read this before scoping.**

The row justified "minor" with "the announce clears normally on attack-end /
max-mitigation-duration", making this a delayed withdraw rather than a permanent one.
**All three legs of that are false**, re-verified at the producers on 2026-07-16:

| Claimed clearing path | Reality | Producer |
|-----------------------|---------|----------|
| Attack-end clears it | `onCleared` **explicitly ignores** the detector's clear while active, logging "ignoring detector clear while mitigating (leak-probe decides)" | `flowspec/responder.go` |
| The leak-probe clears it | `withdraw()` has exactly one caller, `probeTick`, and **`probeTick` has no production caller**: its only callers are `responder_test.go` and `:223`. `register.go` subscribes `onDetected`/`onCharacterized`/`onCleared` and nothing else, so there is no ticker and no traffic feed | `flowspec/responder.go`, `flowspec/register.go` |
| `max-mitigation-duration` clears it | Parsed, defaulted to 3600, range-checked, and **read nowhere else** | `flowspec/config.go,57,117-119,172-173` |

So a flowspec announce, once made, is **never withdrawn in production by any path**. The
withdraw asymmetry this spec was opened for is a symptom of that larger gap, not an
isolated `SuppressMitigation` bug. Scope accordingly: fixing only the `SuppressMitigation`
branch would add the sole working withdraw path to a responder that otherwise cannot let
go of a route. The triggering path is still rare and opt-in (blackhole-fallback enabled
AND critical severity AND a late source-allow exemption), but its consequence is a
permanently stranded upstream announce, not a delayed one.

Three questions therefore precede the two-line fix:

| # | Point | Known constraint |
|---|-------|------------------|
| 1 | Decide whether `SuppressMitigation` should withdraw an active announce | The local responder's answer is yes (`local/responder.go`), and its comment states the reason: "the characterized decision is authoritative" |
| 2 | Establish whether the leak-probe is meant to be driven, and by what | `probe.Tick` (`probe.go`) is a complete state machine with no production driver. Either wire its input or record why it is inert |
| 3 | Decide whether `max-mitigation-duration` is enforced or removed | A validated config leaf that does nothing is a promise the box does not keep. `detect/characterize.go` already notes it "is not enforced" for ddos-local |

**Provenance.** Recorded as a Review Gate NOTE on `spec-ddos-direction-allowlist`
2026-07-12; the spec was closed in `0814dc93f` and `git rm`'d, so that NOTE now exists
only in git history. The source spec's learned summary did NOT record it either
(verified: no `onCharacterized` mention, no Known Limitations section), and that summary
was retired with the learned corpus. Without this file the finding would survive only in a
dangling deferral row.

**Open design question.** Whether the fix is "withdraw at `:164`" or "make the two
responders share one exemption path". Three of the four early returns in
`onCharacterized` have the same shape, so a targeted patch at one of them may just move
the inconsistency rather than remove it.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ddos.md` - blackhole fallback and flowspec response levels
  → Constraint: an operator-visible announce must not outlive the policy that justifies it
- [ ] `ai/rules/evidence.md` - a guard that neither denies nor speaks does not exist
  → Constraint: the exemption branch currently logs but leaves state installed; logging is not withdrawing
- [ ] The direction/allowlist design record (retired with the learned corpus)
  → Decision: `SuppressMitigation` (not `Mitigate`) so the bool zero value means mitigate, fail-safe

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8955.md` - BGP Flow Specification; the announce/withdraw this spec corrects
  → Constraint: a flowspec rule is withdrawn by withdrawing its NLRI; there is no implicit expiry on the peer

**Key insights:**
- `onDetected`'s blackhole fallback is intentionally ungated, which is exactly why a later characterization must be able to undo it.
- The `SuppressMitigation` return at `:163-166` sits **before** the `if r.active` check at `:171`, so an active announce is never even considered. The fix is not only "call withdraw" but "reach the state check at all".
- `withdraw()` already exists and is correct. It is simply unreachable in production. This spec is mostly about reachability, not about writing a withdraw.
- `probe.Tick` (`probe.go`) has no internal timer: it advances only when called, which is why an unwired `probeTick` leaves the whole probe state machine inert rather than merely slow.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/flowspec/responder.go` - `onCharacterized` returns bare at `:164` on `SuppressMitigation`, at `:161` in alert mode, and at `:169` for local victims; `announce` is called only at the end; `onDetected` installs the ungated blackhole fallback
- [ ] `internal/plugins/ddos/local/responder.go` - the local responder DOES withdraw in the same situation; this is the reference behavior
- [ ] `internal/core/ddosevent/event.go` - `SuppressMitigation` / `Direction` / `Confidence` carried on the event

**Behavior to preserve:**
- `SuppressMitigation` polarity: the zero value means mitigate. A withdraw must never be triggered by a default-constructed event.
- Alert mode (`ResponseLevel != responseEnforce`) must continue to announce nothing.
- Non-exempt characterizations must announce exactly as they do today.
- The withdraw's wire form: `withdraw()` re-renders with mode "del", and `renderFlowspecCommand` omits the traffic-action community for "del" so the withdraw byte-matches the announced NLRI (`responder.go`, `:229`).
- `onDetected`'s "await characterization" default: flowspec announces only on the blackhole fallback fast path, never on a normal detect (`responder.go`, AC-8 of the parent).
- The local-victim skip: flowspec leaves box-owned victims to on-host mitigation.
- The confidence gate on the characterized path only.

<!-- An earlier draft of this section required that "attack-end and max-mitigation-duration
     withdraw paths must keep working". Do not reinstate it: no such paths exist. See the
     stale-severity table in the Task section, verified at the producers 2026-07-16. An
     implementer who believes those paths work will scope this fix far too narrowly. -->

**Do NOT assume these exist:**
- There is no attack-end withdraw (`onCleared` ignores the clear while active, `:203-211`).
- There is no probe-driven withdraw in production (`probeTick` has no production caller).
- There is no `max-mitigation-duration` enforcement (the leaf is validated and never read).

**Behavior to change:**
- On `SuppressMitigation`, an already-installed blackhole-fallback announce is withdrawn instead of left up.

## Data Flow (MANDATORY)

### Entry Point
- `ddosevent.AttackDetected` reaches `onDetected` (may install the blackhole fallback), then `ddosevent.AttackCharacterized` reaches `onCharacterized` (`responder.go`).

### Transformation Path
1. Detector emits `AttackDetected`; policy outcome is already encoded on the event (1110's decision).
2. `onDetected` installs a blackhole-fallback announce if configured and severity is critical. It is not confidence-gated.
3. Detector later emits `AttackCharacterized`, this time carrying `SuppressMitigation` because a source-allow rule matched.
4. `onCharacterized` hits `:163-166`, logs, and returns.
5. The announce from stage 2 is never withdrawn: not here, and not anywhere else in production (see the stale-severity table in the Task section).

Stage 4 is the defect: the branch knows mitigation is exempt and is the only place that can undo stage 2 at all, not merely the only place that can undo it promptly.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Detector ↔ flowspec responder | `ddosevent` struct fields | [ ] |
| Responder ↔ BGP | flowspec NLRI announce/withdraw | [ ] |
| flowspec responder ↔ local responder | independent responders that must agree on exemption semantics | [ ] |

### Integration Points
- `internal/plugins/ddos/flowspec/responder.go` `announce` and its withdraw counterpart - the pair that must be symmetric.
- `internal/plugins/ddos/local/responder.go` - the reference implementation that already withdraws.
- `r.active` - the responder's installed-state flag the withdraw must respect.

### Architectural Verification
- [ ] No bypassed layers (withdraw goes through the responder's existing announce/withdraw path)
- [ ] No unintended coupling (flowspec and local responders stay independent)
- [ ] No duplicated functionality (reuse the existing withdraw, do not add a second one)
- [ ] Registration over hardcoding — no new per-feature switch in shared ddos code (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An installed blackhole announce SHOULD be withdrawn when a late exemption arrives | The local responder does this; the deferral row calls flowspec's behavior the odd one out | The current behavior is correct and the local responder is the bug; the fix inverts | Confirm intended semantics with Thomas before coding | unvalidated |
| A-2 | `r.active` accurately reflects whether an announce is installed | `:171` reads it as a guard | The withdraw may fire with nothing installed, or miss one that is | Read the `onDetected` path and every `r.active` write | unvalidated |
| A-3 | The other two bare returns (`:161` alert mode, `:169` local victim) do not need the same withdraw | Only the `SuppressMitigation` branch was reported | The same defect exists three times and a one-line fix leaves two | Trace each branch against an installed blackhole announce | unvalidated |
| A-4 | The leak-probe is MEANT to be driven in production and its missing driver is a bug | `probe.Tick` is a complete state machine (`probe.go`) that nothing calls; `register.go` wires no ticker or traffic feed | The probe is deliberately inert and the real withdraw design is something else entirely; this spec's scope changes shape | Ask Thomas; check whether a driver was ever specced | unvalidated |
| A-5 | `max-mitigation-duration` is meant to be enforced for flowspec | It is parsed, defaulted and range-validated (`config.go,57,117-119,172-173`) but never read; `detect/characterize.go` notes it is unenforced for ddos-local too | The leaf should be removed rather than honored, and the YANG surface changes | Ask Thomas whether the leaf is a promise or a leftover | unvalidated |

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
| AC-5 | Default-constructed event (`SuppressMitigation` false) | Mitigates; polarity preserved |
| AC-6 | (from A-4) The leak-probe has a production driver, or its inertness is recorded as a deliberate decision | (fill during design) |
| AC-7 | (from A-5) `max-mitigation-duration` is either enforced or removed from the flowspec config surface | (fill during design) |

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
- `internal/plugins/ddos/flowspec/responder.go` - withdraw at the `SuppressMitigation` branch, which must first reach the `r.active` check now sitting below it at `:171`; address A-3's sibling branches
- `internal/plugins/ddos/flowspec/register.go` - a probe driver, if A-4 lands here (`:105-107` is where the three handlers are wired and where a fourth input would go)
- `internal/plugins/ddos/flowspec/config.go` - `max-mitigation-duration`, if A-5 enforces rather than removes it

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
| Registration over hardcoding | No new per-feature switch in shared ddos code (`ai/rules/plugins.md`) |

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
| The finding survived in the source spec's learned summary | It lived only in the closed spec's Review Gate NOTE and a deferral row; that summary never recorded it | Grep during the 2026-07-16 deferral sweep | A closed spec's Review NOTE is not a durable home; this spec is |
| The stranded announce "clears normally on attack-end / max-mitigation-duration", making this minor (asserted by deferral row L63 AND by this spec's own first draft) | No production path withdraws a flowspec announce at all: `onCleared` ignores the clear while active, `probeTick` has no production caller, and `max-mitigation-duration` is never read | Re-verified at the producers 2026-07-16 while reconciling two drafts of this spec | Severity is understated in the row. The fix's real scope is the responder's missing withdraw reachability, not one branch |
| The two drafts of this spec were equivalent, so either could be deleted | They were not: one carried the stale "clears normally" severity, the other carried the producer-verified refutation. Deleting the wrong one would have shipped an implementer a spec instructing them to preserve withdraw paths that do not exist | Diffing the pair before deletion, then verifying each claim at the producer | Never resolve duplicate specs by name alone; diff the content and verify the claims first |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- A Review Gate NOTE on a spec that is later `git rm`'d evaporates unless it lands in the learned summary. That is a closure-process gap, and it is why this finding needed a spec of its own to survive.
- The flowspec responder can announce but cannot let go. `announce()` is reachable from two paths while `withdraw()` is reachable from none, and nothing in the type system or the tests notices, because `responder_test.go` calls `probeTick` directly and so exercises a withdraw path that production never takes. A unit test that reaches past the wiring can make a dead feature look live.
- A validated config leaf that nothing reads (`max-mitigation-duration`) is indistinguishable from a working one from the operator's side: it parses, it range-checks, it rejects bad input. Validation is not enforcement, and the gap is invisible until someone greps for the read.

## Known Limitations
- Scoping this spec to the `SuppressMitigation` branch alone would leave the responder with exactly one working withdraw path and no others (A-4, A-5). That may still be the right first step, but it should be a decision rather than an accident.

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
