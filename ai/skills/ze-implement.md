# Implement Spec

Implement the selected spec end-to-end with built-in review loops.

See also: `/ze-audit` (check what exists first), `/ze-review-spec` (post-impl verification), `/ze-verify` (run tests)

## Spec Sections Used by Each Stage

| Stage | Spec Section(s) Consumed |
|-------|--------------------------|
| 1. Read spec | Entire spec |
| 2. Update status | Spec metadata |
| 3. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 4. Wiring phase | **Wiring Test** table (entry points, registration, skeleton) |
| 5. Implement | Implementation Phases, TDD Test Plan, Acceptance Criteria |
| 6. Verify | (make targets) |
| 7. Critical review | **Critical Review Checklist** (feature-specific checks) |
| 11. Deliverables review | **Deliverables Checklist** (verification methods per deliverable) |
| 12. Security review | **Security Review Checklist** (feature-specific concerns) |
| 14. Documentation review | **Documentation Update Checklist** (per-category doc updates) |
| 15. Close + commit | Entire spec (extraction recipe), `plan/learned/METHODOLOGY.md` |

## Steps

1. **Read the spec:** Read `tmp/session/selected-spec`, then read `plan/<spec-name>`
2. **Update spec status (BLOCKING -- do this FIRST, before any other work):**
   Edit the spec file NOW: set `Status` to `in-progress`, `Phase` to `1/N`, `Updated` to today.
   This is the FIRST action after reading. Not after audit, not after implementation, not at the end.
   Do not proceed to step 3 until the spec file on disk shows `in-progress`.
   **Why this is BLOCKING:** other sessions check spec status to avoid collisions. A spec that
   stays in `design` or `ready` during implementation lies about its state.
3. **Audit first:** Run `/ze-audit` logic. Check Files to Modify, Files to Create, and TDD Test Plan against the codebase. Identify what's already implemented, partially done, or missing. Do not redo existing work.
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
   - Move to next phase
6. **Run full verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
7. **Critical review:** Use the spec's **Critical Review Checklist** table. For each row:
   - Verify the "What to verify" column against the actual implementation
   - Document pass/fail for each check
   - Also apply generic checks from `ai/rules/quality.md` (Correctness, Simplicity, Consistency, Completeness, Quality, Tests)
   - Do NOT agree with the spec blindly -- challenge architectural assumptions
8. **Fix every issue found** in the review
9. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
10. **Repeat steps 7-9** until the review finds zero issues and all tests pass. No cap on review passes -- each fix is new code that needs a fresh review. Stop only when a pass finds nothing.
11. **Deliverables review:** Use the spec's **Deliverables Checklist** table. For each row:
    - Run the verification method specified in the table
    - Paste evidence (grep output, test output, ls output)
    - If anything is missing or incomplete, go back to step 4 and implement it
    - Also re-read Acceptance Criteria -- verify each AC-N with file:line evidence
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
    - Do NOT say "update the docs." Name the specific file, the specific section, and what to add.
    - Categories: feature list, user guide, config syntax, CLI reference, API/RPC docs, plugin SDK, wire format, RFC compliance, comparison table, test infrastructure, architecture design.
    - If the spec has no Documentation Update Checklist, use `ai/rules/planning.md` "Documentation Update Checklist" as the reference and fill it for the spec.
    - Write the doc updates. Include them in the commit.
15. **Close spec and present commit (BLOCKING -- do ALL of this BEFORE presenting the commit script):**
    The user expects that running the commit script completes ALL work. Nothing may be left
    over, deferred, or require a second script. Do everything below before showing the script.
    a. Write the learned summary to `plan/learned/NNN-<spec-stem>.md` following `plan/learned/METHODOLOGY.md`.
       Number NNN = next unused number. To find it: `for f in plan/learned/[0-9]*.md; do basename "$f"; done | grep -oE '^[0-9]+' | sort -rn | head -1`
       then add 1. Use the extraction recipe: Context from Task + Current Behavior, Decisions from Key Design Decisions + annotations, Consequences from Design Insights + Limitations, Gotchas from Deviations + Mistake Log.
    b. Update `ai/LEARNED-INDEX.md` if the summary contains a structural decision (not just task completion).
    c. Remove your line from `tmp/session/selected-spec`.
    d. List all changes made (files modified/created, tests added, docs updated, issues found and fixed).
    e. Prepare ONE commit script (`tmp/commit-SESSION.sh`) that does EVERYTHING in a single commit:
       - `git add` all implementation files (code, tests, docs, schema)
       - `git add plan/learned/NNN-<spec-stem>.md`
       - `git add ai/LEARNED-INDEX.md` (if updated)
       - `git rm plan/<spec-name>`
       - Commit message file with both the feature description and the spec closure
    f. Present the commit script to the user.

    **Why one script, one commit:** the user runs the script and the work is done. No second
    script, no "now run this other thing", no leftover steps. If there are ANY remaining
    actions after the user runs the script, the step is not complete. Go back and include them.

## Rules

- **No deferred work.** Every item in the spec must be implemented fully before reporting completion. No TODOs, no stubs, no placeholder implementations, no "left as future work" notes, no comments like "// TODO: handle X later". If an item turns out to be blocked, ambiguous, or harder than expected, stop and raise it with the user to re-negotiate scope. Never silently skip or defer.
- **Design-doc "Deferred to a later phase" sections are not authoritative.** When the user picks an option whose design doc carves out follow-on work as deferred, do NOT parrot that carve-out. Treat the entire problem as in scope and ask before excluding anything.
- Do NOT skip the audit step -- re-implementing existing code wastes time
- If the same issue reappears after 3 fix attempts (3-Fix Rule, `ai/rules/anti-rationalization.md`), STOP and ask for guidance. Otherwise keep reviewing -- there is no pass limit.
- If the spec is missing a **Critical Review Checklist**, **Deliverables Checklist**, **Security Review Checklist**, or **Documentation Update Checklist**, STOP and inform the user that the spec needs updating before implementation can proceed
- Before reporting done, re-read the spec and confirm each item is actually implemented in the code
