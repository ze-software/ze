---
name: ze-review
description: Use when working in the Ze repo and the user asks for ze-review or for a focused review of current changes. Read the changed files and their intent, inspect edge cases, security risks, and missing tests, and report findings without fixing anything.
---

# Ze Review

This is the fast issue-finding review for Ze changes.

## Workflow

1. Identify the relevant diff or changed files.
2. Read the actual code, not just the diff.
3. Check recent history and local comments around the changed regions to understand why the old code existed.
4. Trace data flow through the touched paths.
5. Apply edge-case, security, allocation, and plugin-traversal checks where relevant.
6. Filter out false positives and report only concrete findings.

## Reporting

- Use ordered findings with `BLOCKER`, `ISSUE`, or `NOTE`.
- Include file references and the specific failure mode.

## Rules

- Do not fix anything.
- Prioritize behavioral bugs, regressions, and missing tests over style commentary.
