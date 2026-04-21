---
name: ze-explore
description: Use when working in the Ze repo and the user asks for ze-explore or for topic research before changes. Search source, tests, specs, docs, and rules for the topic, read the relevant files, summarize current behavior, and propose a change path without editing.
---

# Ze Explore

Use this to understand a Ze topic before proposing code changes.

## Workflow

1. Search for the topic across `internal/`, `pkg/`, `cmd/`, tests, `plan/`, `docs/`, config files, and repo rules.
2. Read every relevant match; do not rely on filenames alone.
3. Summarize:
   - what files exist
   - current behavior
   - key patterns and constraints
   - how data moves through the system
4. Propose the most likely extension path using existing patterns instead of duplicating behavior.

## Rules

- Read-only.
- Do not suggest changes until you have read the relevant files.
