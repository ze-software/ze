---
name: ze-handoff
description: Use when working in the Ze repo and the user asks for ze-handoff or wants a fresh session handoff. Summarize the selected spec and current state, then produce a short set of exact next edits and a verification command without changing files.
---

# Ze Handoff

This skill prepares a continuation document for a later session.

## Workflow

1. Read `tmp/session/selected-spec` and the current spec if one is selected.
2. List what is done, in progress, remaining, and formally deferred.
3. Record which files were already read and understood so the next session can skip re-reading them.
4. Produce up to five explicit edits with file locations and exact replacement or insertion instructions.
5. End with one verification command that should be run after the edits.

## Rules

- Read-only.
- Each edit must be self-contained; never say "update similarly."
- Split into phases if more than five edits remain.
