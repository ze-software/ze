# Spec: the Review Gate loop has no termination bound, and cmd_check mechanically drives another pass after every fix

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**AUDITED CLOSABLE 2026-08-17.** An independent audit judged every row at its
producer. The mechanical bound is `ROUND_CAP` and `ROUND_OWNER_CAP` in
`scripts/dev/review_gate.py`, enforced by `cmd_record`. The prose bound is the
"Bounding the loop" section of `ai/rules/planning.md`. `scripts/dev/review_gate_test.py`
passes, 22 tests.

→ Decision: the eight always-in-scope classes stay uncapped. Their severity
floor is ISSUE, and no bound on a review loop can retire an obligation owed
outside this repository (`ai/rules/rule-precedence.md`, rung 2).

→ The residual this spec's audit noted is closed. `ai/rules/planning.md` no
longer states that there is no cap on the number of passes. Commit `2a9773663`
reconciled that sentence with the cap.

## Task

**The problem.** The Review Gate loop (`ai/rules/planning.md`, "Critical Review Is the
Central Deliverable" and "Bounding the loop") has no state in which it is guaranteed to
stop. Its exit condition is "a round finds no BLOCKER and no ISSUE inside its OWN scope,
AND no always-in-scope finding anywhere". Two of its terms are unbounded, so a run that
never ends is permitted by the rule as written rather than prevented by it.

**Term one: the shrinking scope never reaches zero.** Round N+1's scope is round N's
fixes. Every fix is new code. The rule states the governing fact itself, but only about
round 1: "On a diff of any size, a full-diff pass always finds something." That sentence
is equally true of a one-line fix diff, and the rule does not notice. Scope shrinks in
size, never to nothing, so no fixed point is guaranteed.

**Term two: the eight always-in-scope classes are unscoped by construction.** They apply
"whatever round surfaces them and whoever caused them", their severity floor is ISSUE,
and "Where the round's scope and that list disagree, the list wins and the loop takes
another pass". They can therefore re-open the loop from outside the shrinking scope,
without limit.

**The symptom.** Termination rests on a reviewer grading findings NOTE, and the rule
forbids exactly that move: "tagging one down is the cheapest exit from a list whose
purpose is to have no exits". The rule closes the only door it exits through. "There is
no cap on the NUMBER of passes" is a deliberate choice in the same section, so this is
not a wording slip. A session that follows the rule literally cannot close its work.

**The loop's engine is mechanical, not only prose.** `cmd_check` (`scripts/dev/review_gate.py`)
holds no concept of a round: it checks the artifact's verdict, then its coverage of the
commit's code files, then whether any reviewed file's hash still matches. That third
branch is what turns the rule's advice into a machine: any fix changes a reviewed file,
the hash stops matching, and the gate BLOCKS with "Every fix is new code that needs a
fresh review. Re-review and re-record." So each fix mechanically forces another pass, and
nothing anywhere counts passes or can stop them. `cmd_record` writes the artifact and
counts nothing either.

**What a bound has to respect.** The two kinds of finding behave differently and the rule
treats them alike. The eight classes (unwired symbol, vacuous test, acceptance criterion
with no test, user-facing behavior with no functional test, Linux-only code with no QEMU
test, removed guard, newly added guard that fails open, RFC or interop non-conformance)
are CONVERGENT: each names a concrete checkable defect, fixing one keeps it fixed, and
the set shrinks monotonically. Open-ended quality findings (naming, structure, "this
could be simpler") are NON-CONVERGENT: they regenerate on every diff. Whether the bound
is a pass cap on the second kind with overflow routed to a spec, a strictly-shrinking
scope measure, a severity floor that rises per round, or something else, is the design
question. Weakening the first kind is not on the table: RFC and interop non-conformance
sit on rung 2 of `ai/rules/rule-precedence.md`, and no bound on a review loop can retire
an obligation owed outside this repository.

**Reproduction.** Read `ai/rules/planning.md`, "Bounding the loop", and look for a term
that decreases to a stopping value. Then read `cmd_check` (`scripts/dev/review_gate.py`)
and look for a round counter. Neither exists.

**Provenance.** Raised by Thomas on 2026-08-08 while the deferral-shard gate spec
(`spec-fixit-deferral-shard-gate-reads-only-head`, since closed) was in design: "this code can
not NOT forever run and find something as code can never be perfect and there will always
be something to complain about, ie: it will run forever". The rule text and `cmd_check`
were read to confirm the mechanism before this spec was written. No deferral row: a
source spec did not surface this, Thomas did, and `ai/rules/planning.md` exempts work
that was never in a spec's scope.

## Required Reading

### Architecture Docs

- [ ] `ai/rules/planning.md`, "Critical Review Is the Central Deliverable" and
      "Bounding the loop" -- the prose half of the bound
  → Decision: five passes belong to the session. The sixth belongs to Thomas.
  → Constraint: a NOTE never re-opens a round, and an always-in-scope finding
    is never a NOTE.
- [ ] `ai/rules/rule-precedence.md` -- the ladder that keeps the bound off
      rung 2
  → Constraint: a review bound MUST NOT retire an RFC or interop obligation.

**Key insights:**

- The loop needs two bounds, and they act on different halves. A cap prices the
  extra pass. A demotion to NOTE removes the class that regenerates.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `scripts/dev/review_gate.py` -- `cmd_record` and `cmd_check`
  → Constraint: `cmd_check` counts nothing, so the count has to live in
    `cmd_record`, which is the moment a review is claimed.
- [ ] `scripts/dev/commit_helper.py` -- `review_gate_problems`
  → Constraint: the commit gate runs `check`, never `record`, so a cap in
    `record` cannot block a commit by itself.
- [ ] `ai/rules/planning.md` -- the loop's exit condition

**Behavior to preserve:**

- `cmd_check`'s three branches: verdict, coverage, hash freshness.
- The artifact format `tmp/review/<stem>-<session-id>.md`, and its header.
- The always-in-scope list, its ISSUE floor, and its precedence over the round's
  scope.

**Behavior to change:**

- `cmd_record` counts the passes and refuses a count the rule does not allow.
- The rule demotes a record defect and a harness-only test defect to NOTE.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- A closing session runs `python3 scripts/dev/review_gate.py record --spec <stem>
  --verdict clean --rounds <N> --files <files>` at step 5 of `/ze-close`.
- `--rounds N` is the pass count the session claims.

### Transformation Path

1. `cmd_record` reads the model boundary first, through `_model_refusal`.
2. It rejects `--rounds` below 1.
3. It rejects a count above `ROUND_CAP` that carries no `--rounds-reason`.
4. It rejects a count above `ROUND_OWNER_CAP` that carries no `--owner-authorised`.
5. It writes `tmp/review/<stem>-<session-id>.md`, with `rounds=N` in the header.
   The reason and the authorisation each get their own line in the body.
6. `review_gate_problems` (`scripts/dev/commit_helper.py`) runs `cmd_check` over
   the commit's code files, which reads the verdict and the hashes.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ artifact | `cmd_record` writes the file that names the session | Yes, read in `artifact_path` |
| Artifact ↔ commit | `review_gate_problems` shells out to `cmd_check` | Yes, read in `commit_helper.py` |
| Rule ↔ script | the cap's number appears in the rule and in `ROUND_CAP` | Yes, both read |

### Integration Points

- `/ze-close` step 5 records the artifact and reports the round count.
- `plan/TEMPLATE-CLOSURE.md` carries the Review Gate table the count lands in.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The count is checked where it is claimed, in `cmd_record` |
| No unintended coupling (components stay isolated) | Yes | `cmd_check` is untouched, so the commit gate keeps its three branches |
| No duplicated functionality (extends existing, does not recreate) | Yes | One cap constant pair, read by one function |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling, not a hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No registration surface |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A script cannot tell who typed a flag | `cmd_record`, which reads only its own arguments | The owner cap would need a different mechanism | read at the producer | confirmed |
| A-2 | The non-convergent class is what makes the loop run forever | the 2026-08-09 case of seven passes, cited in `review_gate.py` | A cap alone would not stop the loop | the rule's demotion of record defects to NOTE | confirmed |
| A-3 | The commit gate cannot enforce the cap | `review_gate_problems` runs `check`, never `record` | The cap would be checkable at commit time | read at the producer | confirmed |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A session sets `--owner-authorised` without Thomas's word | the artifact carries an authorisation nobody remembers | The rule bans it, the flag's help names him, and the value is recorded |
| R-2 | The cap teaches one number in the script and another in a rule | a reader quotes three where the script refuses four | Every surface that carries the number moves in the same commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator sees. A wrong cap refuses a closure or lets an unbounded loop run |
| How is it reverted? | One commit. The constants and the rule text carry the whole change |
| Who else touches this path? | Every closing session, through `/ze-close` step 5 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `review_gate.py record --rounds 6` with no reason | → | `cmd_record`'s `ROUND_CAP` branch | `test_over_the_cap_is_refused_without_a_reason` |
| `review_gate.py record --rounds 6 --rounds-reason X` | → | `cmd_record`'s `ROUND_OWNER_CAP` branch | `test_a_product_defect_alone_does_not_lift_the_owner_cap` |
| `review_gate.py record --rounds 5` | → | `cmd_record`'s write path | `test_the_fifth_round_is_the_last_a_session_spends_alone` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `record` runs with `--rounds` between 1 and 5, and no reason | The artifact is written, and its header carries `rounds=N` |
| AC-2 | `record` runs with `--rounds 0` | Refused, exit 2. An artifact claiming zero passes is a review that never ran |
| AC-3 | `record` runs with `--rounds 6` and no `--rounds-reason` | Refused, exit 2, naming the cap of five |
| AC-4 | `record` runs with `--rounds 6` and `--rounds-reason` alone | Refused, exit 2. The message says more than five passes is Thomas's call |
| AC-5 | `record` runs with `--rounds 6`, a reason and `--owner-authorised` | The artifact is written, and both values reach the file |
| AC-6 | A blank reason, or a blank authorisation | Refused, exit 2. Whitespace lifts neither cap |
| AC-7 | `check` reads an artifact written before the cap existed | Accepted. The cap governs `record`, never `check` |
| AC-8 | The rule text a session reads | It states the cap of five, the owner's sixth pass, and the ban on setting the flag unasked |
| AC-9 | A round whose findings are all record defects, or harness-only test defects | The rule grades them NOTE, and a NOTE never re-opens a round |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_rounds_under_the_cap_records_and_is_written_down` | `scripts/dev/review_gate_test.py` | AC-1 | pass |
| `test_zero_rounds_is_refused` | `scripts/dev/review_gate_test.py` | AC-2 | pass |
| `test_over_the_cap_is_refused_without_a_reason` | `scripts/dev/review_gate_test.py` | AC-3 | pass |
| `test_a_product_defect_alone_does_not_lift_the_owner_cap` | `scripts/dev/review_gate_test.py` | AC-4 | pass |
| `test_over_the_cap_records_with_a_defect_and_the_owners_word` | `scripts/dev/review_gate_test.py` | AC-5 | pass |
| `test_the_owners_word_alone_does_not_lift_the_reason_requirement` | `scripts/dev/review_gate_test.py` | AC-4, AC-6 | pass |
| `test_a_blank_reason_does_not_lift_the_cap` | `scripts/dev/review_gate_test.py` | AC-6 | pass |
| `test_a_blank_owner_authorisation_does_not_lift_the_owner_cap` | `scripts/dev/review_gate_test.py` | AC-6 | pass |
| `test_the_fifth_round_is_the_last_a_session_spends_alone` | `scripts/dev/review_gate_test.py` | AC-1 | pass |
| `test_rounds_is_required` | `scripts/dev/review_gate_test.py` | AC-1 | pass |
| `test_check_accepts_an_artifact_that_predates_the_cap` | `scripts/dev/review_gate_test.py` | AC-7 | pass |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--rounds`, with no reason and no authorisation | 1 to 5 | 5 | 0 | 6 |
| `--rounds`, with a reason alone | 1 to 5 | 5 | 0 | 6 |
| `--rounds`, with a reason and an authorisation | 1 upward | no upper bound | 0 | N-A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `scripts/dev/review_gate_test.py` | `scripts/dev/` | A closing session records a review and the gate accepts or refuses the count. The suite drives the real script, so no `.ci` applies to this surface | pass |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible behavior. This is repository tooling | N-A |

## Files to Modify

- `scripts/dev/review_gate.py` - `ROUND_CAP`, `ROUND_OWNER_CAP`, `cmd_record`,
  and the two flags
- `scripts/dev/review_gate_test.py` - the `RoundCapCase` tests
- `scripts/dev/commit_helper.py` - the remedy text `review_gate_problems` prints
- `ai/rules/planning.md` - "Bounding the loop", plus the demotions to NOTE
- `ai/rules/points/planning/critical-review-is-the-central-deliverable/a-finding-in-the-record-is-not-a-finding-in-the-product.md` -
  the canonical point behind that rule text
- `ai/skills/ze-close.md` - step 5's statement of the cap
- `ai/skills/ze-review.md` - step 1 and the Review Integrity notes
- `plan/TEMPLATE-CLOSURE.md` - the Review Gate table's Rounds row

## Files to Create

- None. Every surface existed.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Repository tooling, no daemon config |
| YANG validation constraints | N-A | Same |
| YANG custom validators | N-A | Same |
| CLI commands/flags | Yes | `--rounds-reason` and `--owner-authorised` on `scripts/dev/review_gate.py` |
| CLI grammar (keyword before value) | N-A | A dev tool's `argparse` flags, not the `ze` grammar |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | Yes | `scripts/dev/review_gate_test.py`, `RoundCapCase` |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | No new environment leaf |
| Doctor check for runtime dependencies | N-A | No new runtime dependency. The artifact directory already existed |
| Prometheus counters/metrics | N-A | A closure gate, not daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A closure gate for agents. `docs/features.md` describes the daemon |
| 2 | Config syntax changed? | No | No YANG leaf and no config key changed |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` documents `ze`, not `scripts/dev/` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | The audience is an agent, and its pages are the rule and the skills |
| 7 | Wire format changed? | No | No wire surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior |
| 10 | Test infrastructure changed? | No | `docs/functional-tests.md` covers the `.ci` suites. This is a Python suite beside its script |
| 11 | Affects daemon comparison? | No | No daemon behavior |
| 12 | Internal architecture changed? | No | No component boundary moved |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | No metric |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entry |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `ai/rules/repo-maintenance.md` names `ROUND_CAP` in its gate table, and `ai/rules/context-economy.md` names the cap in its size-the-change rule. Both still teach three |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-review.md` and `plan/TEMPLATE-CLOSURE.md` quoted the old number, and this closure corrects both |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add the two flags and the cap constants
   - Tests: the `RoundCapCase` rows of the TDD plan
   - Files: `scripts/dev/review_gate.py`
   - Verify: `record` refuses a count above the cap, and the tests are red first
2. **Phase: Prose bound** -- state the cap and the demotions in the rule
   - Tests: none. The rule is read by a person
   - Files: `ai/rules/planning.md` and its canonical point
   - Verify: the rule states one number, and the loop's exit condition holds
3. **Phase: Sweep the number** -- move every surface that teaches the cap
   - Files: `scripts/dev/commit_helper.py`, `ai/skills/ze-close.md`,
     `ai/skills/ze-review.md`, `plan/TEMPLATE-CLOSURE.md`
   - Verify: `grep -rn "more than three"` finds no surface about review rounds

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a test in `RoundCapCase`, or a rule sentence for AC-8 and AC-9 |
| Feature completeness | A session can record 1 to 5 passes alone, and a sixth with two flags |
| Correctness | The refusal messages name the cap, the flag, and the owner ruling |
| Naming | The flag is `--owner-authorised`, spelled the same in the script, the rule and the skills |
| Data flow | The count is checked in `cmd_record`. `cmd_check` keeps its three branches |
| Rule: `ai/rules/planning.md` | One number in every surface. A reader cannot learn three from one place and five from another |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `ROUND_CAP` and `ROUND_OWNER_CAP` in the script | `grep -n "ROUND_CAP\|ROUND_OWNER_CAP" scripts/dev/review_gate.py` |
| `cmd_record` refuses a count below 1 | `test_zero_rounds_is_refused` |
| `cmd_record` refuses a sixth round without both flags | `test_over_the_cap_is_refused_without_a_reason`, `test_a_product_defect_alone_does_not_lift_the_owner_cap` |
| Both values reach the artifact | `test_over_the_cap_records_with_a_defect_and_the_owners_word` |
| The rule states the cap and the ban | `grep -n "owner-authorised" ai/rules/planning.md` |
| No surface teaches the old number | `grep -rn "more than three" ai/ plan/ scripts/` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `--rounds` is read with `int()`, and a non-integer raises before any file is written |
| Authorization that could fail open | A missing flag must refuse, never record. Both refusals return 2 before the write |
| Information leakage | The artifact carries the reason and the authorisation text a session passes, and it lives under `tmp/` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A cap the capped party can lift is not a cap. The first bound was three passes
  with a `--rounds-reason` the same session wrote, so any loop could authorise
  its own continuation.
- Two bounds were needed, on different halves of the loop. The cap prices an
  extra pass. The demotion to NOTE removes the findings that regenerate.
- A script cannot check who typed a flag. It can make the flag hard to type by
  accident, and name the owner in its own text, so an unasked use is a recorded
  false statement.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The cap is 5, and the sixth pass needs the owner's word | Keep 3 with a self-written reason; ban the sixth pass outright | Owner ruling of 2026-08-17. Five passes are enough for a real defect, and a sixth is a decision about cost |
| Both tolls are owed past the cap | Let `--owner-authorised` replace `--rounds-reason` | The product defect and the owner's word answer different questions, so neither substitutes for the other |
| The cap lives in `cmd_record` | Put it in `cmd_check`, so the commit gate enforces it | `record` is the moment a review is claimed. `cmd_check` reads an artifact that may predate the cap |
| Record defects and harness-only test defects are NOTE | Keep every finding at its found severity | They regenerate on each prose fix, which is what made a test-only change take seven passes on 2026-08-09 |

## Known Limitations

- Nothing verifies that Thomas authorised a sixth pass. The flag records a claim,
  and a false claim is visible in the artifact.
- The cap governs `record` alone. An artifact written before the cap existed
  still passes `check`, which `test_check_accepts_an_artifact_that_predates_the_cap`
  pins.
- Two rule surfaces still teach the cap of three. See Deviations from Plan.

## RFC Documentation (Scope: protocol)

N-A. This spec changes repository tooling and states no protocol behavior.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- `ROUND_CAP = 5` and `ROUND_OWNER_CAP = 5` in `scripts/dev/review_gate.py`.
  `cmd_record` refuses a count below 1. It refuses a count above `ROUND_CAP`
  that carries no `--rounds-reason`. It refuses a count above
  `ROUND_OWNER_CAP` that carries no `--owner-authorised`. Both values reach the
  artifact, each on its own line.
- The prose bound sits in `ai/rules/planning.md`, "Bounding the loop": the cap,
  the owner's sixth pass, the ban on the flag, and the demotion of record
  defects and harness-only test defects to NOTE. A round whose findings are all
  record defects is the last round.
- The final code landed in commit `2a9773663` on 2026-08-17, which implements the
  owner ruling of that day. That commit also reconciled the sentence which said
  there is no cap on the number of passes.
- This closure swept two surfaces that commit missed. `ai/skills/ze-review.md`
  taught a cap of three in step 1 and again in its Review Integrity notes.
  `plan/TEMPLATE-CLOSURE.md` asked for a product defect at `N>3`.

### Bugs Found/Fixed

- The stale number in the two surfaces above. A reviewer who read the skill
  would price a fourth pass at a written reason, when the script takes five
  without one. Worse in the other direction: the skill said nothing about the
  owner's authorisation, so a session at six passes would meet the refusal with
  no idea that the answer is to stop and ask Thomas.
- Found and NOT fixed here: `ai/rules/repo-maintenance.md` and
  `ai/rules/context-economy.md` still teach the cap of three, and so do their
  canonical point renders under `ai/rules/points/`. An edit to a rule file
  requires `make ze-rules-condensed-update`, and its digests are `ai/rules/CORE.md`
  and `ai/rules/TRIGGERS.md`, which another live session is mid-edit on for the
  RFC rules. A row records it in
  `plan/journal/comment-describes-superseded-behaviour.md`, and the closure
  report names it for the main thread to route.

### Documentation Updates

- `ai/skills/ze-review.md` and `plan/TEMPLATE-CLOSURE.md` now carry five and the
  owner cap. `make ze-ai-skills-sync` copies the skill to the tool directories,
  which are ignored by git.
- `make ze-doc-verify` is green.

### Deviations from Plan

- The spec was a skeleton, and the work was owner-driven. Its design-time
  sections are written at closure from the landed code, so they record what is
  true rather than what was planned.
- No learned summary. `plan/learned/` no longer receives closure artifacts, and
  the journal row is the artifact this closure writes.
- The first bound this work shipped was a cap of three that any `--rounds-reason`
  lifted. The owner ruling of 2026-08-17 replaced it.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first bound was a cap of three, liftable by a `--rounds-reason` the same session wrote | A hatch the constrained party writes is not a bound | Thomas ruled on it on 2026-08-17 | `ROUND_OWNER_CAP` refuses a sixth round without his word, and `--rounds-reason` stays owed beside it |
| escalation | The number lives in six surfaces, and the landing commit moved four of them | A number in prose drifts unless every surface moves together | This closure grepped for the old number | Two surfaces fixed here. Two rule surfaces are journaled and reported, because their digests are contended |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The loop reaches a state in which it stops | Done | `cmd_record` and `ai/rules/planning.md` | Five passes are the session's. A sixth needs a product defect and Thomas's word |
| The bound must not weaken the convergent classes | Done | `ai/rules/planning.md`, "Bounding the loop" | The always-in-scope list keeps its ISSUE floor and its precedence over the round's scope |
| The non-convergent class must stop re-opening the loop | Done | same section | A record defect is a NOTE, and so is a harness-only test defect. A NOTE never re-opens a round |
| The mechanical engine must count passes | Done | `cmd_record` | `cmd_check` still counts nothing, by design: it reads artifacts that predate the cap |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test_rounds_under_the_cap_records_and_is_written_down`, `test_the_fifth_round_is_the_last_a_session_spends_alone`, `test_rounds_is_required` | The header carries `rounds=N` |
| AC-2 | Done | `test_zero_rounds_is_refused` | Exit 2 |
| AC-3 | Done | `test_over_the_cap_is_refused_without_a_reason` | The message names the cap |
| AC-4 | Done | `test_a_product_defect_alone_does_not_lift_the_owner_cap`, `test_the_owners_word_alone_does_not_lift_the_reason_requirement` | Both tolls are owed |
| AC-5 | Done | `test_over_the_cap_records_with_a_defect_and_the_owners_word` | Both values reach the file |
| AC-6 | Done | `test_a_blank_reason_does_not_lift_the_cap`, `test_a_blank_owner_authorisation_does_not_lift_the_owner_cap` | `strip()` runs before each test |
| AC-7 | Done | `test_check_accepts_an_artifact_that_predates_the_cap` | The cap governs `record` alone |
| AC-8 | Done | `ai/rules/planning.md`, "Bounding the loop" | `grep -n "owner-authorised" ai/rules/planning.md` returns the cap, the ruling and the ban |
| AC-9 | Done | same section | The record-defect and test-only cuts, plus the load-bearing exception for a test that leads to no testing |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The eleven `RoundCapCase` tests | Done | `scripts/dev/review_gate_test.py` | `python3 scripts/dev/review_gate_test.py` -> `Ran 22 tests`, `OK` |
| Boundary rows for `--rounds` | Done | same file | 0 refused, 5 accepted, 6 refused without the flags, 6 accepted with both |
| Functional row | Done | same file | The suite runs the real script as a subprocess |
| Interop row | N-A | - | No wire surface |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/review_gate.py` | Done | `2a9773663` |
| `scripts/dev/review_gate_test.py` | Done | `2a9773663` |
| `scripts/dev/commit_helper.py` | Done | `2a9773663`, the remedy text |
| `ai/rules/planning.md` | Done | `2a9773663` |
| the canonical planning point | Done | `2a9773663` |
| `ai/skills/ze-close.md` | Done | `2a9773663` |
| `ai/skills/ze-review.md` | Done | this closure |
| `plan/TEMPLATE-CLOSURE.md` | Done | `2a9773663` for the comment, this closure for the Rounds row |

### Audit Summary

- **Total items:** 25
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the two N-A rows: no interop surface, no file to create)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The loop has a state in which it stops | functional, the suite driving the real script | `cmd_record` refuses a sixth pass unless a product defect and Thomas's word are both recorded. Proven by `test_over_the_cap_is_refused_without_a_reason` and `test_a_product_defect_alone_does_not_lift_the_owner_cap`. `python3 scripts/dev/review_gate_test.py` -> `Ran 22 tests in 1.965s`, `OK` |
| The bound cannot be lifted by the party it binds | unit, negative cases | A blank value lifts neither cap: `test_a_blank_reason_does_not_lift_the_cap` and `test_a_blank_owner_authorisation_does_not_lift_the_owner_cap`. The rule bans an unasked `--owner-authorised` in the same shape as the standing ban on `--push` |
| The non-convergent findings stop re-opening the loop | rule text a session reads | `ai/rules/planning.md` grades a record defect and a harness-only test defect NOTE, and states that a round of record defects alone is the last round. The one exception keeps its severity: a test defect that leads to no testing |
| No obligation owed outside the repository is retired | rule text, precedence | The always-in-scope list keeps its ISSUE floor, and the list still wins against the round's scope. `ai/rules/rule-precedence.md` rung 2 is untouched |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The metadata names no shard, and `grep -rn "review-loop-has-no-termination" plan/deferrals/` returns nothing. No row anywhere names this spec as a destination |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-review-loop-has-no-termination-bound-7584d469-e988-48fc-910f-c68d4a139d89.md` |
| `review_gate.py check` | clean |
| Rounds | 2. Round 2 re-read the two record corrections against `RoundCapCase` and the rule text |
| Reviewer lenses used | claim versus producer (`cmd_record`, `cmd_check`, `review_gate_problems` and the rule text), guard behavior (does each refusal fail closed, and does a blank value lift it), number drift (`grep` for the old cap across `ai/`, `plan/` and `scripts/`) |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Two surfaces taught a cap of three after the cap became five, and neither mentioned the owner's authorisation | `ai/skills/ze-review.md` step 1 and its Review Integrity note, `plan/TEMPLATE-CLOSURE.md` Rounds row | Both rewritten to five plus the owner cap, in this closure |
| 2 | ISSUE | Two rule surfaces teach the same stale number | `ai/rules/repo-maintenance.md`, `ai/rules/context-economy.md`, and their point renders | NOT fixed here. Their digests are contended by another live session, so the finding is journaled and reported for routing |
| 3 | ISSUE | The closure record stated that `grep -n "owner-authorised" ai/rules/planning.md` returns three lines. It returns two | this spec, AC Verified row for AC-8 | Ran the grep again and corrected the row |
| 4 | ISSUE | The closure record stated that the cap tests assert no artifact is written. They assert exit 2 and the flag named in stderr | this spec, Wiring Verified | Read the `RoundCapCase` bodies and restated both rows from the assertions |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/review_gate.py` | Yes | `grep -n "ROUND_OWNER_CAP" scripts/dev/review_gate.py` returns the constant and its two readers |
| `scripts/dev/review_gate_test.py` | Yes | `python3 scripts/dev/review_gate_test.py` -> `Ran 22 tests`, `OK` |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 1 to 5 passes record with no reason | `test_the_fifth_round_is_the_last_a_session_spends_alone` passes in the 22-test run |
| AC-2 | Zero passes is refused | `test_zero_rounds_is_refused` passes in the same run |
| AC-3 | A sixth pass needs a product defect | `test_over_the_cap_is_refused_without_a_reason` passes |
| AC-4 | A sixth pass needs Thomas's word as well | `test_a_product_defect_alone_does_not_lift_the_owner_cap` passes |
| AC-5 | Both values reach the artifact | `test_over_the_cap_records_with_a_defect_and_the_owners_word` passes |
| AC-6 | A blank value lifts neither cap | The two blank-value tests pass |
| AC-7 | `check` still accepts an older artifact | `test_check_accepts_an_artifact_that_predates_the_cap` passes |
| AC-8 | The rule states the cap and the ban | `grep -n "owner-authorised" ai/rules/planning.md` returns two lines: the refusal of a sixth round, and the ban on setting the flag unasked, which cites the same ban on `--push` |
| AC-9 | The demotions are stated | `grep -n "record defects is the last round" ai/rules/planning.md` returns the sentence |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `review_gate.py record --rounds 6` with no reason | `scripts/dev/review_gate_test.py` | Yes. `run_gate` runs the real script as a subprocess, and the test asserts exit 2 with `--rounds-reason` named in stderr |
| `review_gate.py record --rounds 6 --rounds-reason X` | same | Yes. The owner-cap branch is asserted separately, on exit 2, `--owner-authorised` in stderr, and the phrase "own initiative" |
| `review_gate.py record --rounds 5` | same | Yes. The artifact is written, and the test reads `rounds=5` back from the header |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `cmd_record` reads its own arguments and nothing else. No caller identity is available to it |
| A-2 | confirmed | The seven-pass case of 2026-08-09 is recorded in the comment above `ROUND_CAP`, and the rule's demotion to NOTE addresses it |
| A-3 | confirmed | `review_gate_problems` (`scripts/dev/commit_helper.py`) runs `check`, never `record`, so the cap cannot be a commit-time gate |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No skill teaches the old cap | `grep -rn "more than three" ai/skills/` returns nothing | Yes |
| The closure template asks for the right threshold | `grep -n "N>5" plan/TEMPLATE-CLOSURE.md` returns the Rounds row | Yes |
| Two rule surfaces still teach three | `grep -rn "more than three" ai/rules/` returns `repo-maintenance.md`, `context-economy.md` and their two point renders | Yes, and recorded as finding 2 |

## Core Insight

A cap whose exemption the capped party writes is a switch, not a cap. The fix is
not a higher number. It is a second toll only somebody else can pay.
