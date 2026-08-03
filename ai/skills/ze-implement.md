---
name: ze-implement
description: Implement Spec
---

# Implement Spec

Build the selected spec: wiring first, then feature phases, with review loops
until the diff is clean. **Closure is a separate skill** -- when this one ends,
run `/ze-close`.

See also: `/ze-close` (deliverables, security, docs, Review Gate, the two closure commits), `/ze-audit` (check what exists first), `/ze-review` (the adversarial pass), `/ze-verify` (run tests)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Do not run the steps below inline. You do
  not need to ask permission first (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
  Independent work goes out in ONE message with parallel `Agent` calls.
- **If you are that agent:** run the steps below. You have no LSP tool and cannot
  ask the user, so when you hit a STOP-and-ask condition, halt and put the
  question in your report for the main thread to carry.
- **Either way:** every claim in the report names the function that PRODUCES the
  behavior, as the file plus the symbol (`ai/rules/evidence.md`). The main
  thread verifies each one against source before acting; relaying a report
  unverified is fabrication with an extra hop. Report the conclusion and the
  evidence that would overturn it, never the search. Under 40 lines
  (`ai/rules/writing.md`).

## Scope: this skill stops before closure

This skill ends when the implementation is complete and green. It does NOT run
the deliverables review, the security review, the documentation review, the
Review Gate, or the commits. Those are `/ze-close`, for two reasons:

- **Context.** Closure instructions reached at the tail of a long skill get
  partially followed. Across 161 specs the closure tables were byte-identical to
  the template in 65-75% of in-progress specs, while sections authors added when
  they needed them were untouched in 0%.
- **Model.** `ai/rules/planning.md` puts implementation on Opus 4.8 and
  the Review Gate, spec closure, and implementation audit on Opus 5. Announce
  the boundary at the end of this skill; do not cross it silently.

## Spec Sections Used by Each Stage

| Stage | Spec Section(s) Consumed |
|-------|--------------------------|
| 1. Read spec | Entire spec |
| 2. Update status | Spec metadata |
| 3. Audit | Files to Modify, Files to Create, TDD Test Plan, **Risks & Assumptions** (validate assumptions before coding) |
| 4. Wiring phase | **Wiring Test** table (entry points, registration, skeleton) |
| 5. Implement | Implementation Phases, TDD Test Plan, Acceptance Criteria |
| 6. Verify | (make targets) |
| 7. Critical review | **Critical Review Checklist** (feature-specific checks) |

The spec carries only design-time sections while this skill runs. The closure
half lives in `plan/TEMPLATE-CLOSURE.md` and `/ze-close` appends it when it is
first needed.

**Verification: inner loop vs gate.** Steps 6 and 9 use the fast targets to
iterate. `make ze-verify` is the pre-commit GATE (`ai/rules/git-safety.md`) and
is the only command the spec's Goal Gates name. Do not add a third spelling.

## Steps

1. **Read the spec:** Run `scripts/dev/spec-session.sh current` to find this session's spec. If empty, use the spec named in the conversation and claim it with `scripts/dev/spec-session.sh claim <spec-name>`. Then read `plan/<spec-name>`.
   - If `claim` exits 3, the WIP cap refused it: too many specs are already in-progress. That is a decision for the user, not something to route around. Show them the list it printed and ask whether to close one first or raise `ZE_SPEC_WIP_CAP`. Do NOT edit the spec's Status by hand to skip the check.
2. **Update spec status (BLOCKING -- do this FIRST, before any other work):**
   Edit the spec file NOW: set `Status` to `in-progress`, `Phase` to `1/N`, `Updated` to today.
   This is the FIRST action after reading. Not after audit, not after implementation, not at the end.
   Do not proceed to step 3 until the spec file on disk shows `in-progress`.
   **Why this is BLOCKING:** other sessions check spec status to avoid collisions. A spec that
   stays in `design` or `ready` during implementation lies about its state.
3. **Audit first:** Run `/ze-audit` logic. Check Files to Modify, Files to Create, and TDD Test Plan against the codebase. Identify what's already implemented, partially done, or missing. Do not redo existing work.
   - **BGP Family Checklist (BLOCKING):** If the spec involves a new SAFI, capability, or attribute
     but has no "BGP Family Checklist" section, STOP. Read `ai/patterns/bgp-family.md`, add the
     checklist section to the spec, and present to the user before coding. This gate exists because
     SR-Policy shipped incomplete (3 commits) due to missing integration points that the generic
     wiring rules did not catch.
   - **Validate assumptions (BLOCKING):** Read the spec's **Risks & Assumptions** Assumptions
     table. For every A-N row whose validation method is cheap (grep, read a file, run an
     existing test), run it NOW — before any feature code — and flip Status to `confirmed`
     or `broken`. A `broken` assumption gets a Mistake Log "Wrong Assumptions" row; if it
     invalidates the approved design, STOP and present to the user before coding.
   - If the spec has the section but with only placeholder rows, treat it as missing (see Rules).
4. **Wiring phase (MANDATORY FIRST — before any feature code):**
   Read the spec's **Wiring Test** table. For each row:
   - Identify the entry point (CLI command, web route, config leaf, plugin event, RPC handler).
   - If the entry point does not exist yet: implement the registration/skeleton now (handler that returns "not implemented" or equivalent). This is Phase 1 regardless of what the spec's Implementation Phases say.
   - If the entry point exists: verify it with `grep` or LSP and record file + symbol.
   - Write the wiring test (the `.ci` or `_test.go` that exercises entry-point-to-feature-code). It should fail because the feature logic is a stub.
   Gate: every Wiring Test row has a registered entry point and a failing test before proceeding.
5. **Implement feature phases:** Follow the spec's **Implementation Phases** section in order, filling in the stubs created in step 4. For each phase:
   - Write the tests listed for that phase (TDD — test must fail before implementation)
   - Implement minimal code to pass
   - Run `make ze-unit-test` until green
   - Confirm the wiring test from step 4 now passes (or progresses) after each phase
   - Update the **Risks & Assumptions** tables: flip A-N statuses as evidence arrives;
     when an assumption breaks mid-phase, add the Mistake Log row immediately and STOP
     if the approved design no longer holds. Add new A-N/R-N rows as they surface.
   - Move to next phase
6. **Run full verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
7. **Critical review:** Use the spec's **Critical Review Checklist** table. For each row:
   - Verify the "What to verify" column against the actual implementation
   - Document pass/fail for each check
   - Also apply generic checks from `ai/rules/quality.md` (Correctness, Simplicity, Consistency, Completeness, Quality, Tests)
   - **CLI grammar (BLOCKING):** If any CLI command was added or changed, verify it follows action-before-identifier per `ai/rules/cli.md`. Run the mechanical check: `args[0]` must always be a keyword, never a user identifier.
   - **Invocation-form change (BLOCKING):** If the change REMOVES or ALTERS how a binary is invoked (a launch/dispatch form, a positional's meaning, a flag's meaning), enumerate EVERY invocation site by grepping the bare invocation token (`\bze <positional>`), NOT just the framework directive (`exec=ze`). Invocations hide in `.ci` `exec=` directives, **embedded `tmpfs=*.sh` script bodies** (run via `exec=./script.sh`), helper `.sh`/`.py`, the test-runner launch code, wrapper scripts (`test/exabgp-compat/bin/exabgp`), and docs. A directive-only grep is blind to shell-script-mediated launches. Then prove the change against the **FULL affected suite, never a sample** -- only the full run executes the embedded launches, so a passing sample is a false green. (Learned 1248: removing the bare `ze <config>` sink broke 26 auth `.ci` that launched the daemon from an embedded `tmpfs=*.sh` `ze <config>` line the migration grep never saw; the full functional suite caught it, a sampled run would not have.)
   - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, socket, kernel module, port, TLS cert, external binary), verify a `ze doctor` check exists per `ai/rules/repo-maintenance.md`. Register diagnostic codes in `internal/core/diagnostic/codes.go`.
   - **Prometheus counters:** If the feature has observable state (connections, errors, rates, gauges), verify counters are defined, registered in telemetry, and listed in the spec's Integration Checklist.
   - **YANG validation:** If YANG leaves were added, verify each has maximum native constraints (`range`, `length`, `pattern`, `enumeration`). If native is insufficient, verify a custom validator with `CompleteFn` exists per `ai/patterns/config-option.md`. A leaf with `type string` and no constraint is a red flag.
   - Do NOT agree with the spec blindly -- challenge architectural assumptions
8. **Fix every issue found** in the review. For each fix apply `ai/rules/completion.md`: write the root cause traced to the producing function and choose the `[source]` fix over the `[workaround]` before editing. Never make a finding disappear by weakening a test, renaming a symbol, or special-casing the failing input — that fixes where the problem shows up, not where it is.
9. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
10. **Repeat steps 7-9** until the review finds zero issues and all tests pass. There is no cap on the NUMBER of passes, because each fix is new code that needs a fresh review. Each pass covers LESS than the one before it: round 1 the whole diff, round N+1 only round N's fixes and what they touched. Stop when a pass finds no BLOCKER and no ISSUE inside its own scope. "Stop only when a pass finds nothing anywhere" has no state in which it stops, which is why finished work fails to close (`ai/rules/planning.md`, "Bounding the loop").
11. **Stop here and hand off to `/ze-close`.** The implementation is done when
    steps 7-9 find nothing and every target is green. Report what was built, what
    the tests prove, and any surviving risk from the spec's R-N rows. Then state
    plainly that closure (deliverables, security, docs, Review Gate, commits) is
    `/ze-close`, and that `ai/rules/planning.md` puts it on the review
    model. Do NOT append `plan/TEMPLATE-CLOSURE.md`, do NOT run `/ze-review` as
    the gate, and do NOT prepare a commit script here.

## Rules

- **Diagnosis before fix (BLOCKING).** When a test, gate, or review finding fails, write the five-part Diagnosis before editing (`ai/rules/completion.md`): symptom, root cause traced to the producing function, owning layer, two fixes labeled `[workaround]`/`[source]`, why not the workaround. Fix the root cause at the owning layer. Renaming, skipping, special-casing, or weakening a test to reach green is a workaround, not a fix. When a check rejects you, ask: is the check wrong, is the input wrong, or is the check's data/config incomplete?
- **No deferred work.** Every item in the spec must be implemented fully before reporting completion. No TODOs, no stubs, no placeholder implementations, no "left as future work" notes, no comments like "// TODO: handle X later". If an item turns out to be blocked, ambiguous, or harder than expected, stop and raise it with the user to re-negotiate scope. Never silently skip or defer.
- **Design-doc "Deferred to a later phase" sections are not authoritative.** When the user picks an option whose design doc carves out follow-on work as deferred, do NOT parrot that carve-out. Treat the entire problem as in scope and ask before excluding anything.
- Do NOT skip the audit step -- re-implementing existing code wastes time
- If the same issue reappears after 3 fix attempts (3-Fix Rule, `ai/rules/completion.md`), STOP and ask for guidance. Otherwise keep reviewing -- there is no pass limit.
- If the spec is missing a **Critical Review Checklist**, STOP and inform the user that the spec needs updating before implementation can proceed. (`/ze-close` makes the same check for the Deliverables, Security Review, and Documentation Update checklists it consumes.)
- If the spec has a **Risks & Assumptions** section containing only template placeholder rows, STOP and ask the user to complete it (or confirm there are genuinely none). Specs created before the section existed are exempt -- do not retrofit without user request.
- Before handing off, re-read the spec and confirm each item is actually implemented in the code
- **"Implemented" is not "done".** This skill produces a clean diff, not a closed spec. Do not say done, complete, or ready to commit at step 11 -- say the implementation is green and closure is next (`ai/rules/completion.md`).
- **The Review Gate is BLOCKING and lives in `/ze-close`.** The inline reviews in steps 7-10 do NOT satisfy it: they check the spec's own checklists, `/ze-review` checks what nobody planned for. This is the Review Gate from `ai/rules/planning.md`.
