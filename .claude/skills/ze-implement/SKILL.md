# Implement Spec

Implement the selected spec end-to-end with built-in review loops.

See also: `/ze-audit` (check what exists first), `/ze-review-spec` (post-impl verification), `/ze-verify` (run tests)

## Spec Sections Used by Each Stage

| Stage | Spec Section(s) Consumed |
|-------|--------------------------|
| 1. Read spec | Entire spec |
| 2. Update status | Spec metadata |
| 3. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 4. Implement | Implementation Phases, TDD Test Plan, Acceptance Criteria |
| 5. Verify | (make targets) |
| 6. Critical review | **Critical Review Checklist** (feature-specific checks) |
| 10. Deliverables review | **Deliverables Checklist** (verification methods per deliverable) |
| 11. Security review | **Security Review Checklist** (feature-specific concerns) |
| 13. Documentation review | **Documentation Update Checklist** (per-category doc updates) |

## Steps

1. **Read the spec:** Read `tmp/session/selected-spec`, then read `plan/<spec-name>`
2. **Update spec status (BLOCKING -- do this FIRST, before any other work):**
   Edit the spec file NOW: set `Status` to `in-progress`, `Phase` to `1/N`, `Updated` to today.
   This is the FIRST action after reading. Not after audit, not after implementation, not at the end.
   Do not proceed to step 3 until the spec file on disk shows `in-progress`.
   **Why this is BLOCKING:** other sessions check spec status to avoid collisions. A spec that
   stays in `design` or `ready` during implementation lies about its state.
3. **Audit first:** Run `/ze-audit` logic. Check Files to Modify, Files to Create, and TDD Test Plan against the codebase. Identify what's already implemented, partially done, or missing. Do not redo existing work.
4. **Implement:** Follow the spec's **Implementation Phases** section in order. For each phase:
   - Write the tests listed for that phase (TDD -- test must fail before implementation)
   - Implement minimal code to pass
   - Run `make ze-unit-test` until green
   - Move to next phase
5. **Run full verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
6. **Critical review:** Use the spec's **Critical Review Checklist** table. For each row:
   - Verify the "What to verify" column against the actual implementation
   - Document pass/fail for each check
   - Also apply generic checks from `ai/rules/quality.md` (Correctness, Simplicity, Consistency, Completeness, Quality, Tests)
   - Do NOT agree with the spec blindly -- challenge architectural assumptions
7. **Fix every issue found** in the review
8. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
9. **Repeat steps 6-8** until the review finds zero issues and all tests pass. No cap on review passes -- each fix is new code that needs a fresh review. Stop only when a pass finds nothing.
10. **Deliverables review:** Use the spec's **Deliverables Checklist** table. For each row:
    - Run the verification method specified in the table
    - Paste evidence (grep output, test output, ls output)
    - If anything is missing or incomplete, go back to step 4 and implement it
    - Also re-read Acceptance Criteria -- verify each AC-N with file:line evidence
11. **Security review:** Use the spec's **Security Review Checklist** table as the starting point. For each row:
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
12. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
13. **Documentation review (BLOCKING):** Use the spec's **Documentation Update Checklist** table. For each row:
    - Answer Yes or No. Every Yes MUST name the file and describe the update needed.
    - Do NOT say "update the docs." Name the specific file, the specific section, and what to add.
    - Categories: feature list, user guide, config syntax, CLI reference, API/RPC docs, plugin SDK, wire format, RFC compliance, comparison table, test infrastructure, architecture design.
    - If the spec has no Documentation Update Checklist, use `ai/rules/planning.md` "Documentation Update Checklist" as the reference and fill it for the spec.
    - Write the doc updates. Include them in the commit.
14. **Present summary:** List all changes made (files modified/created, tests added, docs updated, issues found and fixed). Ask user to commit.

## Rules

- Do NOT skip the audit step -- re-implementing existing code wastes time
- Do NOT mark items as deferred/external without asking the user
- If the same issue reappears after 3 fix attempts (3-Fix Rule, `ai/rules/anti-rationalization.md`), STOP and ask for guidance. Otherwise keep reviewing -- there is no pass limit.
- If the spec is missing a **Critical Review Checklist**, **Deliverables Checklist**, **Security Review Checklist**, or **Documentation Update Checklist**, STOP and inform the user that the spec needs updating before implementation can proceed
