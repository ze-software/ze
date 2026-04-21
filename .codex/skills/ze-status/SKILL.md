---
name: ze-status
description: Use when working in the Ze repo and the user asks for ze-status or wants a dashboard view of current work. Read the selected spec, open specs, git state, test freshness, and deferrals, then summarize what needs attention and which single command would address it.
---

# Ze Status

This skill gives a compact dashboard for the Ze repo.

## Workflow

1. Read the selected spec and its status if one is active.
2. Scan open specs under `plan/` and collect their status and update dates.
3. Summarize git state, current branch, and recent commits.
4. Check open deferrals and the freshness of `tmp/ze-verify.log`.
5. Produce a concise status report with one attention list ordered by urgency.

## Rules

- Read-only.
- Keep it short and action-oriented.
