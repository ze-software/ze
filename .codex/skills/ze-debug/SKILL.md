---
name: ze-debug
description: Use when working in the Ze repo and the user asks for ze-debug or wants failing tests investigated and fixed. Start from the failing output, test concrete hypotheses, implement the smallest correct fix, and re-run verification.
---

# Ze Debug

Use this for test-failure investigation in the Ze repo.

## Workflow

1. Read the failing output carefully and extract test names, packages, and mismatches.
2. Read the relevant code and tests before changing anything.
3. Form a few concrete hypotheses: formatting/parsing mismatch, data-flow break, setup or initialization bug, wrong wiring or wrong entry point.
4. Investigate and verify hypotheses one by one. If the user explicitly asks for parallel investigation, split the hypotheses across sub-agents; otherwise do it locally.
5. Implement the smallest fix that explains the failure.
6. Re-run focused tests first, then `make ze-lint`, `make ze-unit-test`, and `make ze-functional-test`.
7. Report the root cause, the fix, and the verification result.

## Rules

- Do not weaken tests unless the test is wrong relative to the spec or code contract.
- If three distinct approaches fail, stop and report what was ruled out.
