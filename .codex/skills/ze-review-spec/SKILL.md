---
name: ze-review-spec
description: Use when working in the Ze repo and the user asks for ze-review-spec or wants implementation checked against the selected spec. Verify every acceptance criterion, planned test, planned file, wiring check, and required docs update, then report gaps without fixing them.
---

# Ze Review Spec

This is the post-implementation conformance review for a Ze spec.

## Workflow

1. Read `tmp/session/selected-spec` and the spec file.
2. Check recent git history so already-landed work is not reported as missing.
3. For each acceptance criterion, find the implementation evidence and judge completeness.
4. Verify the TDD tests exist with the expected names.
5. Verify every file in the planned file lists was actually touched or created.
6. Check the named wiring tests and required documentation updates.
7. Report only concrete findings.

## Rules

- Do not fix anything.
- Keep the focus on spec alignment, not general code quality or security.
