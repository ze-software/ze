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
   - **CLI grammar (BLOCKING):** If any CLI command was added or changed, verify it follows action-before-identifier per `ai/rules/cli-grammar.md`. Run the mechanical check: `args[0]` must always be a keyword, never a user identifier.
   - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, socket, kernel module, port, TLS cert, external binary), verify a `ze doctor` check exists per `ai/rules/doctor-checks.md`. Register diagnostic codes in `internal/core/diagnostic/codes.go`.
   - **Prometheus counters:** If the feature has observable state (connections, errors, rates, gauges), verify counters are defined, registered in telemetry, and listed in the spec's Integration Checklist.
   - **YANG validation:** If YANG leaves were added, verify each has maximum native constraints (`range`, `length`, `pattern`, `enumeration`). If native is insufficient, verify a custom validator with `CompleteFn` exists per `ai/patterns/config-option.md`. A leaf with `type string` and no constraint is a red flag.
   - Do NOT agree with the spec blindly -- challenge architectural assumptions
8. **Fix every issue found** in the review
9. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
10. **Repeat steps 7-9** until the review finds zero issues and all tests pass. No cap on review passes -- each fix is new code that needs a fresh review. Stop only when a pass finds nothing.
11. **Deliverables review:** Use the spec's **Deliverables Checklist** table. For each row:
    - Run the verification method specified in the table
    - Paste evidence (grep output, test output, ls output)
    - If anything is missing or incomplete, go back to step 4 and implement it
    - Also re-read Acceptance Criteria -- verify each AC-N with file:line evidence
    - **Goal Validation (BLOCKING):** Fill the spec's **Goal Validation** table. For each goal stated in the Task section, provide concrete evidence (test name, interop scenario, benchmark result) that the goal is achieved. Per `ai/rules/interop-and-goal-validation.md`: "tests pass" alone is not sufficient; map goals to evidence.
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
    - Do NOT say "update the docs." Name the specific file, the specific section, and what to add.
    - Categories: feature list, user guide, config syntax, CLI reference, API/RPC docs, plugin SDK, wire format, RFC compliance, comparison table, test infrastructure, architecture design.
    - If the spec has no Documentation Update Checklist, use `ai/rules/planning.md` "Documentation Update Checklist" as the reference and fill it for the spec.
    - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, external socket, kernel module, listen port, external binary, TLS cert), verify a corresponding `ze doctor` check exists per `ai/rules/doctor-checks.md`. Add missing checks and register diagnostic codes in `internal/core/diagnostic/codes.go`.
    - Write the doc updates. Include them in the commit.
15. **Close spec and present commit (BLOCKING -- do ALL of this BEFORE presenting the commit script):**
    The user runs the commit script and considers the work FINISHED. They will not come back
    to ask "what's next" or "close the spec now." There is no step 16. The script is the
    final deliverable. Everything below MUST be in that single script.

    a. Write the learned summary to `plan/learned/NNN-<spec-stem>.md` following `plan/learned/METHODOLOGY.md`.
       Number NNN: read `plan/learned/.counter` (contains the next available number).
       Use the extraction recipe: Context from Task + Current Behavior, Decisions from Key Design Decisions + annotations, Consequences from Design Insights + Limitations, Gotchas from Deviations + Mistake Log.
    b. Update `ai/LEARNED-INDEX.md` if the summary contains a structural decision (not just task completion).
    c. Remove your line from `tmp/session/selected-spec`.
    d. List all changes made (files modified/created, tests added, docs updated, issues found and fixed).
    e. Prepare ONE commit script (`tmp/commit-SESSION.sh`) that produces TWO commits:
       - Guard: `if ls plan/learned/NNN-*.md 1>/dev/null 2>&1; then echo "ERROR: NNN already taken, re-read .counter"; exit 1; fi`
       - **Commit A (implementation + spec):**
         - `git add` all implementation files (code, tests, docs, schema)
         - `git add plan/learned/NNN-<spec-stem>.md`
         - `git add ai/LEARNED-INDEX.md` (if updated)
         - `git add plan/<spec-name>` (preserves all edits from implementation in git history)
         - Bump `plan/learned/.counter` to NNN+1 and `git add plan/learned/.counter`
         - Commit with feature description message
       - **Commit B (spec closure):**
         - `git rm plan/<spec-name>`
         - Commit with spec closure message
    f. Present the commit script to the user. This is the end.

    **Why one script, two commits, no follow-up:** the user will not ask for a second step.
    They will not remember that the spec needs closing. They will not prompt you for the
    learned summary. If closure is not in the script, it will never happen and the spec
    rots in `plan/` forever. Include everything. There is nothing after this step.
    Two commits because `git rm` destroys the working copy. Commit A preserves the
    edited spec in git history; commit B cleanly removes it.

## Rules

- **No deferred work.** Every item in the spec must be implemented fully before reporting completion. No TODOs, no stubs, no placeholder implementations, no "left as future work" notes, no comments like "// TODO: handle X later". If an item turns out to be blocked, ambiguous, or harder than expected, stop and raise it with the user to re-negotiate scope. Never silently skip or defer.
- **Design-doc "Deferred to a later phase" sections are not authoritative.** When the user picks an option whose design doc carves out follow-on work as deferred, do NOT parrot that carve-out. Treat the entire problem as in scope and ask before excluding anything.
- Do NOT skip the audit step -- re-implementing existing code wastes time
- If the same issue reappears after 3 fix attempts (3-Fix Rule, `ai/rules/anti-rationalization.md`), STOP and ask for guidance. Otherwise keep reviewing -- there is no pass limit.
- If the spec is missing a **Critical Review Checklist**, **Deliverables Checklist**, **Security Review Checklist**, or **Documentation Update Checklist**, STOP and inform the user that the spec needs updating before implementation can proceed
- Before reporting done, re-read the spec and confirm each item is actually implemented in the code
