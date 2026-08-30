# Spec: fixit-rfc-drain-quota-never-armed

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (create `plan/deferrals/fixit-rfc-drain-quota-never-armed.md` on the first deferral) |
| Handoff | - |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`rfc/drain-budget.txt` is a forcing function that forces nothing. It ships
`rate 0` (owner decision D5, 2026-07-29), so `checkDrainFloor` computes a floor of
zero on every run and the comparison passes whatever the backlog is. On
2026-08-30 the backlog is 165 of 171 enrolled RFCs, and no rule obliges anyone to
reduce it on any timescale.

D5 was not a decision to leave the backlog alone. It was a decision not to invent
a number before there is a measurement behind it, recorded as an accepted risk in
`plan/deferrals/rfcgate-0-umbrella.md`. The risk it names is that a quota which
ships inert is a quota that may never arm. A month of clock has run since
`start 2026-07-29` with no measurement taken and no rate proposed.

This spec supplies the missing input and closes the risk. It does three things:

1. **Measures.** Walk four representative RFCs end to end, timing each one
   including the defect tail that extraction turns up. The walks are not overhead:
   each one is a real sign-off that lowers the backlog.
2. **Proves the arithmetic.** `requiredFloor`, `parseDrainBudget` and
   `checkDrainFloor` have no unit test. Nothing anywhere demonstrates that a
   non-zero rate produces the violation it claims to produce, that the month count
   is right at a month boundary, or that the enrolled-set cap behaves as its
   comment says. The day the rate is armed is the wrong day to find out.
3. **Puts one number to Thomas.** The file's own comment reserves arming to the
   owner, and D5 says the same. This spec does not arm the quota. It hands him a
   measured rate, the one-line diff, and the `start` question the arithmetic
   raises.

The question put to the owner is **which rate**, never **whether to have one**.
Leaving the schedule inert is not offered as an option (`ai/rules/rfc-compliance.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "The drain schedule" section, and the eight ratchets it sits beside
  → Decision: the drain floor is a SCHEDULE, not a ratchet. A ratchet compares against HEAD and judges nothing where git cannot answer. A schedule compares against a wall clock and always judges.
  → Constraint: "A rule that reds the gate on unrelated work gets removed rather than obeyed." The rate must be one a session doing unrelated work can meet.
- [ ] `rfc/extraction/README.md` - what a sign-off is, what it is not, and why a generated skeleton can never pass
  → Constraint: only dispositions, reasons and the two relocation fields are authored. Every count is derived and compared at check time, so a walk cannot be shortened by editing a number.
  → Constraint: a `manual-walk` sign-off earns drain credit. An RFC whose authors wrote no RFC 2119 keyword must have some route out of the backlog, so the measurement must include one.
- [ ] `ai/rules/rfc-compliance.md` - the extraction completeness section and the ask-only-before-doing-less directive
  → Constraint: asking "which rate" is a HOW question and is permitted. Offering "leave it at 0" is banned.

### RFC Summaries (Scope: protocol)

N-A. Scope is tooling. This spec changes no protocol behavior. The four walks it
performs read RFC text and classify sites; they add no requirement id and change
no `rfc/short/` level (AC-8).

**Key insights:**
- The floor is CUMULATIVE from `start`, not per month. `requiredFloor` returns
  `min(enrolled, ceil(rate x whole calendar months since start))`. Arming rate `r`
  today, with `start` left at 2026-07-29, immediately demands `ceil(r x 1)`
  sign-offs and 6 exist, so any rate up to 6 lands satisfied. The bill arrives
  later: at month `m` the tree owes `ceil(r x m)` cumulative, and a session that
  skips a month never catches the schedule up by working at rate `r` again.
- `rate` is a `float64`, so a rate below one per month is expressible. Two per
  quarter is `rate 0.667`.
- A rate greater than the enrolled count is refused outright with "no schedule
  can be met" (`checkDrainFloor`), which is the only rate validation that exists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `rfc/drain-budget.txt` - two authored fields, `start 2026-07-29` and `rate 0`. Its comment bans any per-stem row, count or stem list, and states that arming is a one-line commit Thomas takes.
- [ ] `internal/le/rfc/check_extraction.go` - `parseDrainBudget` reads the file, rejects an unreadable file rather than defaulting to zero, rejects an unknown key, a duplicate key, a non-date `start`, and a non-numeric, NaN, infinite or negative `rate`. `requiredFloor` computes the month count and the cap. `checkDrainFloor` compares the credited sign-off count against the floor and emits one violation string.
- [ ] `internal/le/rfc/check.go` - `Check` is the single call site: `checkDrainFloor(tree, collected.Enrolled, signed, today)`, appended to `violations` beside `checkExtractionRatchet`. So the gate is `./le rfc check`.
- [ ] `internal/le/rfc/extraction_test.go` - `TestCreditIsScopedToTheEnrolledSet` proves `credited` scopes to the enrolled set. No test in the package names `requiredFloor`, `parseDrainBudget` or `checkDrainFloor`.
- [ ] `internal/le/rfc/check_test.go` - the in-process fixture pins `start 2026-07-29` and `rate 0`, so every existing check test runs against an inert budget.
- [ ] `plan/deferrals/rfcgate-0-umbrella.md` - the D5 row, its rationale, and its re-homing to `plan/spec-followup-rfc-enrollment.md` on 2026-07-30.

**Behavior to preserve:**
- The two-field format of `rfc/drain-budget.txt`. It may never gain a stem, a count or a register column.
- Every existing `parseDrainBudget` refusal, including the refusal to read an absent file as "nothing owed".
- The violation string's shape. It names the floor, the rate, the start date, the enrolled cap, the credited total, the register split and the remaining backlog, and it ends with the command that walks another RFC.
- `./le rfc check` staying green on the tree as it stands, at whatever rate is finally armed.

**Behavior to change:**
- `rate 0` becomes a measured non-zero rate, and `start` moves to the arming date if the owner rules that way (AC-7).
- The drain arithmetic gains unit tests (AC-1 through AC-4). No production behavior changes with them.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, run by a session and by the verification gate. Also `rfc.Check(tree)` from `internal/le/site/rfccompliance.go`, which publishes the verdict on the website.
- Format at entry: a checkout path. Everything else is read from the tree.

### Transformation Path
1. `rfc.Check` collects the enrolled set from `rfc/enrolled.txt` and evaluates every artifact under `rfc/extraction/`.
2. `credited` scopes the valid sign-offs to the enrolled set, proven by `TestCreditIsScopedToTheEnrolledSet`.
3. `parseDrainBudget` reads `rfc/drain-budget.txt` into a `drainBudget` carrying `start` and `rate`.
4. `requiredFloor` turns `start`, `rate`, the enrolled count and today into an integer floor.
5. `checkDrainFloor` compares the credited total against that floor and returns zero or one violation string.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Authored policy ↔ derived count | `rfc/drain-budget.txt` carries policy only; the count comes from `rfc/extraction/*.json` | No |
| Gate ↔ website | `rfc.Check` also feeds `internal/le/site/rfccompliance.go` | No |
| Wall clock ↔ gate verdict | `today` is a parameter of `checkDrainFloor`, not a `time.Now()` call inside the leaf | No |

### Integration Points
- `internal/le/rfc/check.go`, `Check` - the call site. Unchanged by this spec.
- `internal/le/rfc/check_test.go` - the in-process fixture's budget string. A second fixture with a non-zero rate is added beside it, never in place of it.
- `./le rfc extraction-create stem <stem>` - the command each measured walk runs.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Four walks give a usable throughput estimate | The register split in `./le rfc extraction-status` has three classes, and the artifacts already in `rfc/extraction/` span two orders of magnitude in size | A rate set from too small a sample is a guess with a stopwatch attached, which is the failure D5 exists to prevent | The four measured times, reported with their spread. A spread wider than 4x means the sample is too small, and WP-3 says so rather than proposing a rate | unvalidated |
| A-2 | A walk's cost is dominated by classification, not by the defect tail | `rfc/extraction/README.md` says dispositions are authored and everything else is derived | If the defect tail dominates, the rate must be set from the tail and the walks must report it separately | Each walk records classification time and defect-tail time as two numbers, not one | unvalidated |
| A-3 | `requiredFloor` behaves as its comment says at a month boundary | Read in `internal/le/rfc/check_extraction.go`, `requiredFloor` | An off-by-one in the month count arms the quota a month early or a month late, and nothing would catch it | AC-2 and AC-3 | unvalidated |
| A-4 | The owner will answer the `start` question | The file's comment and D5 both reserve arming to him | The spec stops at `verification` with the measurement recorded and the quota still inert, which is worse than today only in that it cost the walks | WP-4. If no answer arrives, the four sign-offs still land and the tests still land | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A rate is proposed that reds the tree on unrelated work, and the schedule is deleted rather than obeyed | The proposed rate demands more sign-offs than the measurement says a month of ordinary work produces | Propose from the SLOWEST measured walk, not the mean. State the arithmetic in the proposal so the owner can see the margin |
| R-2 | The retroactive floor bites. Arming rate `r` with `start` at 2026-07-29 owes `ceil(r x months since then)` on the arming day | The floor at the arming date exceeds the 6 sign-offs that exist plus the 4 this spec adds | WP-3 computes the floor at the arming date for each candidate rate and puts the `start` question to the owner explicitly (AC-7) |
| R-3 | The four walks turn up product defects that dwarf the walk itself | A walk's defect tail exceeds its classification time | Each defect gets one row in `plan/journal/<class>.md` and the work in hand closes (`ai/rules/completion.md`). A defect that BLOCKS the walk is fixed |
| R-4 | The measurement is taken by a session whose throughput is not representative | One walk's time differs from the others by more than 4x | Report the spread, not the mean alone. A-1's validation says what to do |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A rate set too high reds `./le rfc check` for every session, on work that has nothing to do with RFCs. That is the failure `docs/contributing/rfc-conformance-gates.md` says ends in the rule being deleted. A rate set too low costs nothing beyond another inert schedule. The unit tests break nothing: they read a fixture tree |
| How is it reverted? | Single-line revert of `rfc/drain-budget.txt`. Nothing persists, nothing migrates, no peer sees it |
| Who else touches this path? | `plan/spec-rfcgate-6-supported-extraction-signoff.md` proposes walking 49 stems and reads `rate 0` at closure (its AC-9). `plan/spec-followup-rfc-enrollment.md` owns the re-homed D5 row. Both must be updated when the rate is armed (AC-9) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture tree with a non-zero rate and an under-quota corpus | → | `rfc.Check` → `checkDrainFloor` | `TestRFCCheckReportsTheDrainFloorViolation` |
| `./le rfc check` over the real checkout at the armed rate | → | `rfc.Check` → `checkDrainFloor` | `TestRFCCheckRunsOverTheRealCheckout` (existing, must stay green after WP-4) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A fixture tree whose budget reads a non-zero rate, whose enrolled set is larger than its signed set, and whose clock is far enough past `start` to owe more than exist | `./le rfc check` reports one violation naming the floor, the rate, the start date, the enrolled cap, the credited total and the remaining backlog |
| AC-2 | `requiredFloor` with today one day BEFORE the calendar-day anniversary of `start` | The month count excludes the incomplete month |
| AC-3 | `requiredFloor` with `start` on the 31st and today in a 30-day month | The anniversary clamps to the last day of the month, and a whole month is counted on that day |
| AC-4 | `requiredFloor` with a rate that would demand more than the enrolled count | The floor caps at the enrolled count, never at the remaining backlog |
| AC-5 | Four RFC stems walked end to end with `./le rfc extraction-create` | Four new artifacts under `rfc/extraction/`, each classifying every site and every section, and `./le rfc extraction-status` reports `signed` risen by 4 and `backlog` fallen by 4 |
| AC-6 | The four walks are timed | Each walk reports classification time and defect-tail time as separate numbers, and the spread across the four is stated |
| AC-7 | The measurement is complete | Thomas receives one proposed rate, the floor that rate produces at the arming date under both `start` choices, and the one-line diff. He is not offered the option of leaving the rate at 0 |
| AC-8 | The four walks complete | No `rfc/short/` requirement level is lowered and no requirement is reclassified `{gap}` or `{not-applicable}` to make a walk finish. A site the walk cannot map is `excluded` with a reason, or the obligation is raised to Thomas |
| AC-9 | The rate is armed | `plan/spec-rfcgate-6-supported-extraction-signoff.md`, `plan/deferrals/rfcgate-0-umbrella.md` and `docs/contributing/rfc-conformance-gates.md` no longer state that the schedule ships inert |
| AC-10 | The rate is armed and `./le rfc check` runs over the real checkout | The gate is green. An armed rate that reds the tree on the day it is armed is not armed, it is broken |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFCCheckReportsTheDrainFloorViolation` | `internal/le/rfc/check_test.go` | AC-1, driven from `rfc.Check` rather than from the leaf, per `ai/rules/evidence.md` on guards | |
| `TestRequiredFloorExcludesTheIncompleteMonth` | `internal/le/rfc/extraction_test.go` | AC-2 | |
| `TestRequiredFloorClampsTheAnniversaryToTheShortMonth` | `internal/le/rfc/extraction_test.go` | AC-3 | |
| `TestRequiredFloorCapsAtTheEnrolledSet` | `internal/le/rfc/extraction_test.go` | AC-4 | |
| `TestParseDrainBudgetRefusesAnAbsentFile` | `internal/le/rfc/extraction_test.go` | The absent-file refusal, which is the guard that stops a missing policy reading as "nothing owed" | |
| `TestParseDrainBudgetRefusesAStemRow` | `internal/le/rfc/extraction_test.go` | The policy-only invariant the file's comment states | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `rate` | 0 to the enrolled count (171 today) | 171 | -0.001 (negative, refused) | 171.001 ("no schedule can be met") |
| `months` in `requiredFloor` | 0 upward | any | clamped to 0, never negative | N/A |
| `floor` | 0 to the enrolled count | 171 | N/A | capped, never exceeds enrolled |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestRFCCheckRunsOverTheRealCheckout` | `internal/le/rfc/check_test.go` (existing) | A session runs `./le rfc check` on the tree as it stands and the armed rate does not red it | |

### Interop Tests (Scope: protocol)

N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify
- `rfc/drain-budget.txt` - the `rate` line, and the `start` line if the owner rules that way. WP-4 only.
- `internal/le/rfc/check_test.go` - a second in-process fixture carrying a non-zero rate, beside the existing inert one.
- `internal/le/rfc/extraction_test.go` - the `requiredFloor` and `parseDrainBudget` cases.
- `docs/contributing/rfc-conformance-gates.md` - "The drain schedule" says the check ships inert at rate 0. That sentence becomes wrong the moment WP-4 lands, and `ai/rules/documentation.md` puts the page edit in the same work.
- `plan/deferrals/rfcgate-0-umbrella.md` - resolve the D5 row.
- `plan/spec-rfcgate-6-supported-extraction-signoff.md` - every statement in it that the rate is 0.
- `plan/spec-followup-rfc-enrollment.md` - it holds the re-homed D5 row.

## Files to Create
- `rfc/extraction/<stem>.json` x4 - the four measured sign-offs. Stems are chosen in WP-1 and named in the Implementation Steps before any walk starts.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No runtime config. `rfc/drain-budget.txt` is a build-time policy file read by `le`, not by the daemon |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | No | `./le rfc check` and `./le rfc extraction-status` already exist and are unchanged |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no config leaf added |
| Functional test for new RPC/API | N-A | no RPC added |
| Pipe completeness | N-A | `./le rfc extraction-status` already emits structured JSON, unchanged |
| Env var registration | N-A | no env var added |
| Doctor check for runtime dependencies | N-A | `rfc/drain-budget.txt` is a repository file, not a runtime dependency of the daemon |
| Prometheus counters/metrics | N-A | build-time gate, no daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The schedule already exists. This arms it |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The four walks bound what four summaries MISSED. They add no requirement and change no level (AC-8). A walk that finds an unextracted obligation raises it to Thomas rather than editing a level silently |
| 10 | Test infrastructure changed? | No | Tests are added inside an existing package. No runner, tag or suite changes |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, do not answer from memory: run `./le spec citation anchors spec plan/spec-fixit-rfc-drain-quota-never-armed.md` before closure and name every doc it lists. `docs/contributing/rfc-conformance-gates.md` is already named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/rfc-conformance-gates.md` and `rfc/extraction/README.md` both quote `rate 0`. Check both after WP-4 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the floor can red
   - Tests: `TestRFCCheckReportsTheDrainFloorViolation`
   - Files: `internal/le/rfc/check_test.go`
   - Verify: the test fails first because no fixture carries a non-zero rate. Then it passes, and `./le rfc check` is proven to refuse an under-quota corpus. Nothing anywhere demonstrates this today
2. **Phase: The arithmetic** -- unit-test `requiredFloor` and `parseDrainBudget`
   - Tests: `TestRequiredFloorExcludesTheIncompleteMonth`, `TestRequiredFloorClampsTheAnniversaryToTheShortMonth`, `TestRequiredFloorCapsAtTheEnrolledSet`, `TestParseDrainBudgetRefusesAnAbsentFile`, `TestParseDrainBudgetRefusesAStemRow`
   - Files: `internal/le/rfc/extraction_test.go`
   - Verify: A-3 is confirmed or broken. A broken A-3 is a defect in `requiredFloor`, fixed here, because the whole spec depends on that arithmetic
3. **Phase: WP-1, measure** -- four walks, timed
   - Choose four stems spanning the registers and the size range, and NAME them here before starting. One must be a candidate for `manual-walk`, because `rfc/extraction/README.md` says an RFC with no RFC 2119 keyword needs a route out of the backlog and its cost differs
   - Files: `rfc/extraction/<stem>.json` x4
   - Verify: `./le rfc extraction-status` reports `signed` 10 and `backlog` 161. Each walk carries two timings (AC-6)
4. **Phase: WP-2, derive** -- turn four timings into one rate
   - Compute from the SLOWEST walk, not the mean (R-1). State the hours per month of RFC work the rate implies, so the owner is choosing a schedule rather than a number
   - Compute the floor each candidate rate produces at the arming date, under `start 2026-07-29` and under a `start` moved to the arming date (R-2)
   - Verify: the proposal states its own margin
5. **Phase: WP-3, ask** -- put the rate and the `start` question to Thomas
   - The question is which rate and which start date. Not whether to arm
   - Verify: the spec stops here if no answer arrives. Status `verification`, the four sign-offs and the tests landed, the quota still inert, and the report says so
6. **Phase: WP-4, arm** -- the one-line edit, on his answer
   - Files: `rfc/drain-budget.txt`, then every file in Files to Modify that states the rate is 0
   - Verify: AC-10. `./le rfc check` green over the real checkout

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol. AC-7 and AC-9 are satisfied by an artifact, not by an intention |
| Correctness | The month arithmetic in the tests is computed by hand from the calendar, never copied from what `requiredFloor` returns. A test that asserts what the code does proves nothing about what the code owes |
| Guard discipline | AC-1 drives the violation from `rfc.Check`, the entry point, never from `checkDrainFloor` alone (`ai/rules/evidence.md`) |
| Naming | The new fixture is added BESIDE the existing inert one. Changing the existing fixture would silently re-point every other check test |
| Data flow | `rfc/drain-budget.txt` still carries two fields. A test that needs a third field is a test proving the wrong thing |
| Rule: `ai/rules/rfc-compliance.md` | No walk lowers a level, marks a `{gap}`, or excludes a site to finish faster (AC-8) |
| Rule: `ai/rules/completion.md` | Defects the walks turn up get one journal row each, not a characterisation and not a spec (R-3) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The drain floor can red | `go test ./internal/le/rfc/ -run TestRFCCheckReportsTheDrainFloorViolation` |
| The month arithmetic is proven | `go test ./internal/le/rfc/ -run TestRequiredFloor` returns three passing cases |
| Four RFCs drained | `./le rfc extraction-status` reports `"signed": 10` and `"backlog": 161` |
| A measured rate exists | The WP-2 table in this spec: four timings and one proposed rate with its arithmetic |
| The quota is armed | `grep '^rate' rfc/drain-budget.txt` returns a non-zero number |
| The tree is green at that rate | `./le rfc check` |
| No document still says the schedule is inert | `grep -rn 'rate 0' docs/ plan/ ai/ rfc/` returns nothing that states it as current fact |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `parseDrainBudget` reads a repository file, not untrusted input. The one property that matters is that it never returns a zero-valued budget on a read failure, which is what `TestParseDrainBudgetRefusesAnAbsentFile` pins |
| Fail closed | An unparseable budget must produce a violation, never a silent pass. This is the `ai/rules/principles.md` zero-value rule applied to a gate |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| `requiredFloor` disagrees with the hand-computed calendar | A-3 is broken. Fix `requiredFloor`, not the test |
| A walk turns up a product defect | One row in `plan/journal/<class>.md`, close the walk, continue. A defect that BLOCKS the walk is fixed |
| A walk turns up an unextracted MUST-level obligation | `ai/rules/rfc-compliance.md`: raise it to Thomas as "which way do I fix it". Never exclude it to finish the walk |
| The owner does not answer WP-3 | Stop at `verification`. Report the measurement and the four landed sign-offs. Do not arm the quota on your own judgement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The floor is cumulative and retroactive, which makes `start` as consequential as `rate`. Arming late with an old `start` bills the tree for every month it was inert. Nothing in the file's comment says this, and a reader who assumes a per-month quota will set a rate that reds the tree on day one. This is the sharpest edge in the mechanism.
- `rate` being a `float64` is what makes the mechanism usable at low throughput. The measurement may well land below one per month, and that is expressible.
- The gate that will one day bind the entire RFC backlog has no unit test. It landed inert and was never driven, which is how an inert mechanism decays: nobody exercises what nobody can trip.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Measure with four real walks that land as sign-offs | Time one walk; estimate from the extraction artifacts already in the tree | An estimate from existing artifacts reads a file's size, not a session's cost, and the defect tail leaves no trace in the JSON. Four walks cost four walks and buy four sign-offs, so the measurement is not overhead |
| Propose from the slowest walk | Propose from the mean | R-1. A rate the median session can meet and the slow session cannot is a rate that reds the tree on unrelated work, and `docs/contributing/rfc-conformance-gates.md` records what happens to such a rule |
| Land the unit tests before the measurement | Arm first, test after | The arithmetic decides what the armed rate demands. Testing it after arming means the first evidence about the month count arrives as a red tree on someone else's commit |
| The spec does not arm the quota | Arm it at the measured rate and report | The file's comment and D5 both reserve arming to Thomas. Asking which rate is a HOW question and is permitted; taking the decision for him is not |

## Known Limitations
- The measurement is four walks by one session. It bounds nothing about a different session's throughput, and the spec says so rather than implying a population estimate.
- Arming the quota does not schedule the work, chase it, or notice it has stalled. It refuses a commit once the tree falls behind, which is a blunt instrument by design.
- This spec does not walk `plan/spec-rfcgate-6-supported-extraction-signoff.md`'s 49 stems. It updates that spec's statements about the rate (AC-9) and nothing else.

## RFC Documentation (Scope: protocol)

N-A. Scope is tooling. No enforcing code is added, so no `// RFC NNNN Section X.Y`
comment is owed. The four walks classify existing RFC text and add no requirement id.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
