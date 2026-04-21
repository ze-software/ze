---
name: ze-review-deep
description: Use when working in the Ze repo and the user asks for ze-review-deep or for an exhaustive review across multiple lenses. Review the selected scope for security, concurrency, errors, tests, logic, data flow, API compatibility, rule compliance, and docs; use parallel sub-agents only when the user explicitly asks for parallel review.
---

# Ze Review Deep

This is the broad, multi-lens review workflow for substantial Ze changes.

## Workflow

1. Determine the review scope from the working tree, a path, or a branch diff.
2. Decide which lenses to run: security, concurrency, error handling, test coverage, logic, data flow, API compatibility, project rules, and documentation.
3. If the user explicitly requests parallel or delegated review, split independent lenses across sub-agents. Otherwise run the lenses sequentially.
4. For each lens, read the changed files themselves and collect concrete findings only.
5. Merge the findings into one severity-ordered report and note any lens that was intentionally skipped or timed out.

## Rules

- Do not edit code during the review.
- If the user does not choose lenses, ask which ones to run instead of assuming.
