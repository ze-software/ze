---
name: ze-implement
description: Implement Spec
---

# Implement Spec

Implement the selected spec end-to-end with built-in review loops.

See also: `/ze-review` (the BLOCKING Review Gate this runs before closure), `/ze-audit` (check what exists first), `/ze-review-spec` (post-impl verification), `/ze-verify` (run tests)

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
| 11. Deliverables review | **Deliverables Checklist**, then APPEND `plan/TEMPLATE-CLOSURE.md` and fill **Goal Validation** + **Implementation Summary** |
| 12. Security review | **Security Review Checklist** (feature-specific concerns) |
| 14. Documentation review | **Documentation Update Checklist** (per-category doc updates) |
| 15. /ze-review gate | **Review Gate** (closure section): record via `scripts/dev/review_gate.py`, loop to 0 BLOCKER/0 ISSUE |
| 16. Close + commit | **Implementation Audit**, **Pre-Commit Verification**, **Deferrals Resolved** (closure sections), `plan/learned/METHODOLOGY.md` |

The spec carries only design-time sections until stage 11. The closure half lives
in `plan/TEMPLATE-CLOSURE.md` and is appended when it is first needed, because
sections copied at spec creation reach closure unfilled: measured across 161
specs, the closure tables were byte-identical to the template in 65-75% of
in-progress specs, while the sections authors added when they needed them were
untouched in 0%.

**Verification: inner loop vs gate.** Steps 6/9/13 use the fast targets to
iterate. `make ze-verify` is the pre-commit GATE (`ai/rules/git-safety.md`) and
is the only command the spec's Goal Gates name. Do not add a third spelling.

## Steps

1. **Read the spec:** Run `scripts/dev/spec-session.sh current` to find this session's spec. If empty, use the spec named in the conversation and claim it with `scripts/dev/spec-session.sh claim <spec-name>`. Then read `plan/<spec-name>`.
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
   - If the entry point exists: verify it with `grep` or LSP and record file:line.
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
   - **CLI grammar (BLOCKING):** If any CLI command was added or changed, verify it follows action-before-identifier per `ai/rules/cli-grammar.md`. Run the mechanical check: `args[0]` must always be a keyword, never a user identifier.
   - **Invocation-form change (BLOCKING):** If the change REMOVES or ALTERS how a binary is invoked (a launch/dispatch form, a positional's meaning, a flag's meaning), enumerate EVERY invocation site by grepping the bare invocation token (`\bze <positional>`), NOT just the framework directive (`exec=ze`). Invocations hide in `.ci` `exec=` directives, **embedded `tmpfs=*.sh` script bodies** (run via `exec=./script.sh`), helper `.sh`/`.py`, the test-runner launch code, wrapper scripts (`test/exabgp-compat/bin/exabgp`), and docs. A directive-only grep is blind to shell-script-mediated launches. Then prove the change against the **FULL affected suite, never a sample** -- only the full run executes the embedded launches, so a passing sample is a false green. (Learned 1248: removing the bare `ze <config>` sink broke 26 auth `.ci` that launched the daemon from an embedded `tmpfs=*.sh` `ze <config>` line the migration grep never saw; the full functional suite caught it, a sampled run would not have.)
   - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, socket, kernel module, port, TLS cert, external binary), verify a `ze doctor` check exists per `ai/rules/doctor-checks.md`. Register diagnostic codes in `internal/core/diagnostic/codes.go`.
   - **Prometheus counters:** If the feature has observable state (connections, errors, rates, gauges), verify counters are defined, registered in telemetry, and listed in the spec's Integration Checklist.
   - **YANG validation:** If YANG leaves were added, verify each has maximum native constraints (`range`, `length`, `pattern`, `enumeration`). If native is insufficient, verify a custom validator with `CompleteFn` exists per `ai/patterns/config-option.md`. A leaf with `type string` and no constraint is a red flag.
   - Do NOT agree with the spec blindly -- challenge architectural assumptions
8. **Fix every issue found** in the review. For each fix apply `ai/rules/diagnosis-before-fix.md`: write the root cause traced to `file:line` and choose the `[source]` fix over the `[workaround]` before editing. Never make a finding disappear by weakening a test, renaming a symbol, or special-casing the failing input — that fixes where the problem shows up, not where it is.
9. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
10. **Repeat steps 7-9** until the review finds zero issues and all tests pass. No cap on review passes -- each fix is new code that needs a fresh review. Stop only when a pass finds nothing.
11. **Deliverables review:** Use the spec's **Deliverables Checklist** table. For each row:
    - **Append the closure sections FIRST (BLOCKING):** copy everything below the
      horizontal rule in `plan/TEMPLATE-CLOSURE.md` to the end of the spec. Those
      sections (Implementation Summary, Mistake Log, Implementation Audit, Goal
      Validation, Deferrals Resolved, Review Gate, Pre-Commit Verification) are
      what steps 11-16 fill. They are deliberately absent until now.
    - Run the verification method specified in the table
    - Paste evidence (grep output, test output, ls output)
    - If anything is missing or incomplete, go back to step 4 and implement it
    - Also re-read Acceptance Criteria -- verify each AC-N with file:line evidence
    - **Goal Validation (BLOCKING):** Fill the spec's **Goal Validation** table. For each goal stated in the Task section, provide concrete evidence (test name, interop scenario, benchmark result) that the goal is achieved. Per `ai/rules/interop-and-goal-validation.md`: "tests pass" alone is not sufficient; map goals to evidence.
    - **Assumptions Resolved (BLOCKING):** Every A-N row in **Risks & Assumptions** must be `confirmed` or `broken` with evidence -- none left `unvalidated`. Fill the spec's Pre-Commit Verification "Assumptions Resolved" table. Broken assumptions need Mistake Log + Deviations entries. Copy surviving R-N risks into the Executive Summary "Risks & observations".
    - **Interop (BLOCKING for protocol features):** If the spec adds/changes protocol behavior, verify an interop test scenario exists and passes. If none exists, create one before proceeding.
12. **Security review:** Use the spec's **Security Review Checklist** table as the starting point. For each row:
    - Check the specific concern described
    - Also apply generic security checks:
      - Injection flaws (command injection, SQL injection, format string)
      - Buffer overflows, out-of-bounds access, integer overflow/underflow
      - Untrusted input handling (missing validation, missing bounds checks, missing sanitization)
      - Path traversal and symlink attacks
      - Race conditions and TOCTOU vulnerabilities
      - Cryptographic misuse (weak algorithms, hardcoded secrets, predictable randomness)
      - Denial of service vectors (unbounded allocations, infinite loops, resource exhaustion)
      - Privilege escalation and missing authorization checks
      - Information leakage (error messages exposing internals, sensitive data in logs)
      - Any OWASP Top 10 relevant to the code's context
    - Fix every issue found. If a fix requires design changes, present to user before proceeding.
13. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
14. **Documentation review (BLOCKING):** Use the spec's **Documentation Update Checklist** table. For each row:
    - Answer Yes or No. Every Yes MUST name the file and describe the update needed.
    - Every No MUST be backed by source-aware evidence. At minimum, grep `docs/` for source anchors pointing at changed files and check the category does not apply.
    - Do NOT say "update the docs." Name the specific file, the specific section, and what to add.
    - Categories: feature list, user guide, config syntax, CLI reference, API/RPC docs, plugin SDK, wire format, RFC compliance, comparison table, test infrastructure, architecture design.
    - If the spec has no Documentation Update Checklist, use `ai/rules/planning.md` "Documentation Update Checklist" as the reference and fill it for the spec.
    - If config syntax changed, verify examples against the actual YANG/parser before writing docs.
    - If CLI/API/RPC changed, verify docs against the actual handler or RPC type, not the spec.
    - If plugins, commands, event types, send types, or capabilities changed, refresh runtime inventory docs from the registry or binary output, not memory.
    - If any existing doc has a `<!-- source: changed/file.go -- ... -->` anchor, re-read that doc claim and update it if stale.
    - Every factual doc update MUST include or update a `<!-- source: path -- symbol -->` anchor immediately after the claim.
    - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, external socket, kernel module, listen port, external binary, TLS cert), verify a corresponding `ze doctor` check exists per `ai/rules/doctor-checks.md`. Add missing checks and register diagnostic codes in `internal/core/diagnostic/codes.go`.
    - **RFC status (BLOCKING):** If the change implements, changes, or newly proves any RFC-level protocol behavior, update the matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`, and reconcile `docs/comparison.md` / `docs/features.md` when the support level changes. Per `ai/rules/discovery-updates.md`.
    - Write the doc updates, run `make ze-doc-test`, and record the result in the spec's Documentation Updates or Pre-Commit Verification section. Include docs in Commit A.
15. **/ze-review gate (BLOCKING -- the final review before closure):** Steps 7-14 check the diff against the spec's own checklists. This gate runs the generic adversarial `/ze-review` over the COMPLETE diff -- including every fix those reviews produced -- and loops until it is clean. It satisfies the Review Gate defined in `ai/rules/planning.md`; the inline reviews do not substitute for it (they check the spec's own checklists; `/ze-review` checks what nobody planned for).
    - Invoke `/ze-review` on the uncommitted changes. It runs its own automated pre-checks (`make ze-validate`, `scripts/dev/audit-test-relaxation.py`) as its step 0.
    - **Record the machine artifact, not just prose:** `python3 scripts/dev/review_gate.py record --spec <spec> ...`, then `check`. `commit_helper.py` runs that same `check` on the closure commit and refuses without a fresh, hash-pinned, CLEAN artifact, so a hand-written table alone does not satisfy the gate. Put the artifact path and the `check` result in the Review Gate table.
    - Record every BLOCKER/ISSUE under `### Findings fixed` (Severity / Finding / Location / Fixed by) so the learned summary can carry them forward. NOTEs do not block: record and proceed.
    - Fix every BLOCKER and ISSUE (anything above NOTE) per `ai/rules/diagnosis-before-fix.md`: write the root cause traced to `file:line`, take the `[source]` fix, and record it under `### Fixes applied`. NOTE-only findings do not block -- record them and proceed.
    - Re-run `make ze-lint && make ze-unit-test && make ze-functional-test`.
    - Re-run `/ze-review`; add a `### Run 2+` block. Loop until a run reports 0 BLOCKER and 0 ISSUE. No cap on re-runs -- each fix is new code that needs a fresh review. If the same finding survives 3 fix attempts (3-Fix Rule, `ai/rules/anti-rationalization.md`), STOP and ask the user.
    - Paste the final clean run into the Review Gate section. The gate is satisfied only when the last run shows 0 BLOCKER, 0 ISSUE.
16. **Close spec and commit (BLOCKING -- do ALL of this BEFORE running the commit script):**
    Precondition: the spec's **Review Gate** section (step 15) shows a final `/ze-review` run with
    0 BLOCKER and 0 ISSUE. Do not prepare the commit script until it does.

    Two more closure sections gate the script, both checked by `commit_helper.py`:
    - **Pre-Commit Verification:** EVERY sub-table needs at least one evidence row.
      `pre_commit_verification_gaps` checks them one at a time and names the empty
      ones, so a row in `Files Exist` is not evidence for `AC Verified`.
    - **Deferrals Resolved:** account for every row in the shard named in the spec
      metadata. `deferral_unassigned_problems` blocks on a live row with no
      destination, and closure deletes the spec's own shard
      (`ai/rules/deferral-tracking.md`).
    Running the commit script finishes the work. There is no step 17. The script is the
    final action. Everything below MUST be in that single script.

    a. Reserve the number and create the file: `python3 scripts/dev/commit_helper.py learned-next <spec-stem>` allocates NNN (max(existing `plan/learned/NNNN-*.md` prefixes) + 1) and creates `plan/learned/NNN-<spec-stem>.md` immediately, so concurrent sessions in one tree cannot collide. Write the learned summary into that file following `plan/learned/METHODOLOGY.md`.
       Use the extraction recipe: Context from Task + Current Behavior, Decisions from Key Design Decisions + annotations, Consequences from Design Insights + Limitations, Gotchas from Deviations + Mistake Log.
    b. Update `ai/LEARNED-INDEX.md` if the summary contains a structural decision (not just task completion).
    c. Release this session's spec claim: `scripts/dev/spec-session.sh release`.
    d. List all changes made (files modified/created, tests added, docs updated, issues found and fixed).
    e. Prepare ONE commit script with `scripts/dev/commit_helper.py` that produces TWO commits:
       - **Commit A (implementation + spec):** run `scripts/dev/commit_helper.py create --replace` with `--file` for every implementation file (code, tests, docs, schema), `plan/learned/NNN-<spec-stem>.md`, `ai/LEARNED-INDEX.md` if updated, and `plan/<spec-name>` to preserve all implementation edits in git history.
       - **Commit B (spec closure):** run `scripts/dev/commit_helper.py create --append --remove plan/<spec-name>` with the spec closure commit message.
       - Use `--lesson-required` for Commit A. Use `--lesson-not-needed "spec closure only; lesson is in Commit A"` for Commit B.
       - The helper owns the session ID, message files, executable script, ignored-path rejection, `git commit -F`, and learned-summary checks.
    f. Run the generated script yourself (`bash tmp/commit-<SESSION>.sh`), then report the resulting commit SHA(s), the script path, message files, commit subjects, and included files. This is the end.

    **Why one script, two commits, no follow-up:** the user will not ask for a second step.
    They will not remember that the spec needs closing. They will not prompt you for the
    learned summary. If closure is not in the script, it will never happen and the spec
    rots in `plan/` forever. Include everything. There is nothing after this step.
    Two commits because `git rm` destroys the working copy. Commit A preserves the
    edited spec in git history; commit B cleanly removes it.

## Rules

- **Diagnosis before fix (BLOCKING).** When a test, gate, or review finding fails, write the five-part Diagnosis before editing (`ai/rules/diagnosis-before-fix.md`): symptom, root cause traced to `file:line`, owning layer, two fixes labeled `[workaround]`/`[source]`, why not the workaround. Fix the root cause at the owning layer. Renaming, skipping, special-casing, or weakening a test to reach green is a workaround, not a fix. When a check rejects you, ask: is the check wrong, is the input wrong, or is the check's data/config incomplete?
- **No deferred work.** Every item in the spec must be implemented fully before reporting completion. No TODOs, no stubs, no placeholder implementations, no "left as future work" notes, no comments like "// TODO: handle X later". If an item turns out to be blocked, ambiguous, or harder than expected, stop and raise it with the user to re-negotiate scope. Never silently skip or defer.
- **Design-doc "Deferred to a later phase" sections are not authoritative.** When the user picks an option whose design doc carves out follow-on work as deferred, do NOT parrot that carve-out. Treat the entire problem as in scope and ask before excluding anything.
- Do NOT skip the audit step -- re-implementing existing code wastes time
- If the same issue reappears after 3 fix attempts (3-Fix Rule, `ai/rules/anti-rationalization.md`), STOP and ask for guidance. Otherwise keep reviewing -- there is no pass limit.
- If the spec is missing a **Critical Review Checklist**, **Deliverables Checklist**, **Security Review Checklist**, or **Documentation Update Checklist**, STOP and inform the user that the spec needs updating before implementation can proceed
- If the spec has a **Risks & Assumptions** section containing only template placeholder rows, STOP and ask the user to complete it (or confirm there are genuinely none). Specs created before the section existed are exempt -- do not retrofit without user request.
- Before reporting done, re-read the spec and confirm each item is actually implemented in the code
- Before reporting done, verify documentation stayed current: source anchors for changed files checked, examples validated against code, and `make ze-doc-test` run when docs changed
- **The /ze-review gate (step 15) is BLOCKING and is not optional.** Closure (step 16) may not prepare the commit script until the spec's **Review Gate** section shows a final `/ze-review` run with 0 BLOCKER and 0 ISSUE. The inline reviews in steps 7-14 do NOT satisfy this gate -- they check the spec's own checklists, `/ze-review` checks what nobody planned for. This is the Review Gate from `ai/rules/planning.md`.
