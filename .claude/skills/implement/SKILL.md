# Implement Spec

Implement the selected spec end-to-end with built-in review loops.

## Steps

1. **Read the spec:** Read `.claude/selected-spec`, then read `docs/plan/<spec-name>`
2. **Audit first:** Run `/spec-audit` logic — check what's already implemented, partially done, or missing. Do not redo existing work.
3. **Implement:** For every requirement in the spec:
   - Write tests first (TDD — test must fail before implementation)
   - Implement minimal code to pass
   - Run `make ze-unit-test` until green
4. **Run full verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
5. **Critical review:** Review the implementation against the spec:
   - Completeness: every requirement implemented (file:line)?
   - Correctness: does the code match the spec's intent?
   - Naming conventions: kebab-case JSON, YANG suffixes, Go patterns
   - Data flow: boundaries respected (Engine/Plugin, Wire/RIB, FSM/Reactor)?
   - Rule violations: no layering, no identity wrappers, no YAGNI, no duplicate code
   - Do NOT agree with the spec blindly — challenge architectural assumptions
6. **Fix every issue found** in the review
7. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
8. **Repeat steps 5-7** until the review finds zero issues and all tests pass. Maximum 2 review passes.
9. **Deliverables review:** Re-read the spec from scratch. For every deliverable, requirement, and acceptance criterion:
   - Verify it is implemented (file:line evidence)
   - Verify it behaves correctly (test name or manual check)
   - If anything is missing or incomplete, go back to step 3 and implement it before proceeding
10. **Security review:** Act as a security vulnerability researcher. Review all new and modified code for:
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
11. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
12. **Present summary:** List all changes made (files modified/created, tests added, issues found and fixed). Ask user to commit.

## Rules

- Do NOT skip the audit step — re-implementing existing code wastes time
- Do NOT mark items as deferred/external without asking the user
- If stuck after 2 review passes, list remaining issues and ask for guidance instead of looping
